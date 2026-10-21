package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"messenger/migrations"
)

// envDatabaseURLFile — путь к файлу со строкой подключения. Файлом, а не переменной со
// строкой: в ней пароль, а окружение видно в docker inspect и /proc/<pid>/environ —
// так же, как ключ подписи JWT (урок 1.17).
const envDatabaseURLFile = "MESSENGER_DATABASE_URL_FILE"

// migrateConnectTimeout — срок на установку соединения, если строка подключения не задала
// свой connect_timeout. Без него недоступная база повесила бы шаг migrate навсегда, и
// выкладка ждала бы его вместо того, чтобы упасть с понятной ошибкой.
const migrateConnectTimeout = 10 * time.Second

// undefinedTable — SQLSTATE 42P01: таблицы goose_db_version нет, миграции не применялись.
const undefinedTable = "42P01"

const migrateUsage = "использование: messenger migrate up|status"

// exitPending — код выхода status, если есть неприменённые миграции: отдельный от
// ошибки (1) и от неверного вызова (2), чтобы проверку перед выкладкой можно было
// написать по коду выхода, не разбирая текст вывода.
const exitPending = 3

// errPending — status отработал, но схема отстаёт от бинаря.
var errPending = errors.New("есть неприменённые миграции")

// #region run-migrate
// runMigrate — подкоманда `messenger migrate up|status`. Миграции — отдельный шаг до
// старта сервера (в compose — одноразовый сервис migrate), а не часть старта: так их
// не применяют наперегонки несколько экземпляров, а серверу не нужны права на DDL.
func runMigrate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "up" && args[0] != "status") {
		_, _ = fmt.Fprintln(stderr, migrateUsage)
		return 2
	}
	err := migrate(ctx, args[0], stdout)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errPending):
		_, _ = fmt.Fprintf(stderr, "миграции: %v — messenger migrate up\n", err)
		return exitPending
	default:
		_, _ = fmt.Fprintf(stderr, "миграции: %v\n", err)
		return 1
	}
}

func migrate(ctx context.Context, cmd string, out io.Writer) (err error) {
	dsn, err := databaseURLFromEnv()
	if err != nil {
		return err
	}
	db, err := openMigrationDB(dsn)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	// Блокировка на время работы — advisory lock Postgres на соединении goose: второй
	// migrate up (два деплоя сразу, повтор задания) ждёт первого, а не применяет те же
	// миграции параллельно.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("блокировка: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if cmd == "up" {
		return migrateUp(ctx, p, out)
	}
	return migrateStatus(ctx, db, p, out)
}

// #endregion run-migrate

// #region open-migration-db
// openMigrationDB — единственный продакшен-путь через database/sql: goose принимает
// *sql.DB. Драйвер — pgx через stdlib, тот же, что у пула сервера. Одно соединение:
// goose и так работает на одном (на нём же держит блокировку), а лишние ему не нужны.
func openMigrationDB(dsn string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		// Текст ошибки разбора не выводим: pgx скрывает пароль «по возможности», а в
		// искажённой строке может его не узнать — и пароль попал бы в журнал деплоя.
		return nil, fmt.Errorf("%s: строка подключения не разобрана (текст не выводится — в нём может быть пароль)", envDatabaseURLFile)
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = migrateConnectTimeout
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	return db, nil
}

// #endregion open-migration-db

func migrateUp(ctx context.Context, p *goose.Provider, out io.Writer) error {
	results, err := p.Up(ctx)
	// При сбое goose возвращает *PartialError: применённые до сбоя миграции остаются
	// применёнными (каждая — в своей транзакции), неудачная откачена целиком.
	var partial *goose.PartialError
	if errors.As(err, &partial) {
		results = partial.Applied
	}
	for _, r := range results {
		_, _ = fmt.Fprintln(out, r)
	}
	if err != nil {
		return fmt.Errorf("up: %w", err)
	}
	if len(results) == 0 {
		_, _ = fmt.Fprintln(out, "применять нечего")
	}
	v, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("версия схемы: %w", err)
	}
	_, _ = fmt.Fprintf(out, "версия схемы: %d\n", v)
	return nil
}

// migrateStatus печатает состояние каждой миграции и сводку. Расхождение видно в обе
// стороны: миграции, ожидающие применения, и база новее бинаря (миграции следующего
// выпуска уже применены — нормально при выкладке, но этому бинарю о них ничего не известно).
//
// На базе без таблицы goose_db_version status не зовёт goose: его Status и GetDBVersion
// (v3.28.0) сначала создают таблицу версий, и «посмотреть» превратилось бы в DDL — с
// правами на него и с таблицей, которой до up не было. Опции «только читать» у Provider нет.
func migrateStatus(ctx context.Context, db *sql.DB, p *goose.Provider, out io.Writer) error {
	exists, err := versionTableExists(ctx, db)
	if err != nil {
		return err
	}
	if !exists {
		return printNothingApplied(p, out)
	}
	statuses, err := p.Status(ctx)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	pending := 0
	for _, s := range statuses {
		name := path.Base(s.Source.Path)
		if s.State == goose.StatePending {
			pending++
			_, _ = fmt.Fprintf(out, "ожидает     %s\n", name)
			continue
		}
		_, _ = fmt.Fprintf(out, "применена   %s  %s\n", name, s.AppliedAt.UTC().Format(time.DateTime))
	}
	current, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("версия схемы: %w", err)
	}
	_, _ = fmt.Fprintf(out, "версия схемы: %d, ожидают применения: %d\n", current, pending)
	if latest := latestSource(p); current > latest {
		_, _ = fmt.Fprintf(out, "база новее бинаря: его последняя миграция — %d\n", latest)
	}
	if pending > 0 {
		return errPending
	}
	return nil
}

func printNothingApplied(p *goose.Provider, out io.Writer) error {
	sources := p.ListSources()
	for _, s := range sources {
		_, _ = fmt.Fprintf(out, "ожидает     %s\n", path.Base(s.Path))
	}
	_, _ = fmt.Fprintf(out, "версия схемы: 0, ожидают применения: %d\n", len(sources))
	if len(sources) > 0 {
		return errPending
	}
	return nil
}

// versionTableExists ищет таблицу goose так же, как goose её создаёт, — без схемы в
// имени, по search_path.
func versionTableExists(ctx context.Context, db *sql.DB) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, goose.DefaultTablename).Scan(&exists); err != nil {
		return false, fmt.Errorf("таблица версий: %w", err)
	}
	return exists, nil
}

func latestSource(p *goose.Provider) int64 {
	var v int64
	for _, s := range p.ListSources() {
		v = max(v, s.Version)
	}
	return v
}

// databaseURLFromEnv читает строку подключения из файла MESSENGER_DATABASE_URL_FILE.
// Сама строка нигде не печатается: в ней пароль.
func databaseURLFromEnv() (string, error) {
	file := os.Getenv(envDatabaseURLFile)
	if file == "" {
		return "", errors.New("задайте " + envDatabaseURLFile + " — путь к файлу со строкой подключения postgres://…")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("%s: %w", envDatabaseURLFile, err)
	}
	// Файл секрета обычно кончается переводом строки (echo, редактор) — он не часть строки.
	dsn := strings.TrimSpace(string(raw))
	if dsn == "" {
		return "", fmt.Errorf("%s: файл %s пуст", envDatabaseURLFile, file)
	}
	return dsn, nil
}

// #region schema-version
// schemaVersion — версия схемы в базе: наибольшая применённая миграция в таблице goose.
// Таблицы нет — миграции не применялись, версия 0. Читает через пул сервера: при старте
// это один запрос, и goose (database/sql) серверу для него не нужен.
func schemaVersion(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var v int64
	err := pool.QueryRow(ctx, `SELECT coalesce(max(version_id), 0) FROM goose_db_version WHERE is_applied`).Scan(&v)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == undefinedTable {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("версия схемы: %w", err)
	}
	return v, nil
}

// checkSchema — проверка при старте сервера: схема не старше той, на которую рассчитан
// бинарь (migrations.Required). Иначе сервер не открывает порт: запросы падали бы с
// 42P01 на первой же новой таблице, а оркестратор считал бы процесс здоровым.
func checkSchema(ctx context.Context, pool *pgxpool.Pool) error {
	v, err := schemaVersion(ctx, pool)
	if err != nil {
		return err
	}
	if v < migrations.Required {
		return fmt.Errorf("версия схемы %d, бинарю нужна %d — примените миграции: messenger migrate up", v, migrations.Required)
	}
	return nil
}

// #endregion schema-version

package pgtest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"messenger/migrations"
)

const (
	// Image — тот же образ, что у сервиса db в messenger/compose.yaml: адаптеры
	// проверяются на той версии Postgres, на которой работает мессенджер. Digest — как в
	// compose: тег могут перепубликовать с другой базой Debian. Совпадение строк
	// проверяет TestImageMatchesCompose.
	Image = "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336"
	// dbName — не «postgres»: снимок пересоздаёт базу (DROP DATABASE + CREATE … TEMPLATE),
	// а системную базу postgres, к которой подключается сам снимок, удалить нельзя.
	dbName = "messenger"
	// sqlDriver — драйвер database/sql для снимков testcontainers и для goose: pgx через
	// stdlib, а не lib/pq, — тот же драйвер, что у сервера, и без второй зависимости.
	sqlDriver = "pgx"
	// EnvRequireDocker — "1" превращает пропуск без Docker в провал (CI).
	EnvRequireDocker = "MESSENGER_REQUIRE_DOCKER"
	// startTimeout — запуск контейнера и миграции. Первый запуск на машине тянет образ
	// (~150 МБ), поэтому с запасом; дальше старт занимает пару секунд.
	startTimeout = 3 * time.Minute
	// stopTimeout — остановка контейнера после тестов: Postgres по SIGTERM выходит
	// за доли секунды, 30 с — на медленный Docker в CI.
	stopTimeout = 30 * time.Second
)

// DB — запущенный контейнер с применёнными миграциями и снимком.
type DB struct {
	ctr *postgres.PostgresContainer
	dsn string
}

// Состояние на уровне пакета — вынужденное: TestMain и тесты не передают друг другу
// значений, а контейнер один на тестовый бинарь. Запуск ленивый (первый вызов Pool):
// прогон, где отобраны только юнит-тесты пакета (go test -run …), Docker не трогает.
var (
	started  atomic.Bool // Start вызван из TestMain — значит, контейнер будет остановлен
	inUse    atomic.Bool // открыт пул текущего теста; см. Pool
	initOnce sync.Once
	shared   *DB
	skip     string // непусто — Docker недоступен; тесты с Pool пропускаются
	startErr error  // Docker есть, но контейнер или миграции не поднялись — провал
)

// Start — для TestMain: прогоняет тесты пакета и останавливает контейнер, если его
// запускали. Код выхода передаётся в os.Exit.
func Start(m *testing.M) (code int) {
	started.Store(true)
	code = m.Run()
	if shared == nil {
		return code
	}
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	if err := testcontainers.TerminateContainer(shared.ctr, testcontainers.StopContext(ctx)); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pgtest: остановка контейнера: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// Pool возвращает пул к базе, только что восстановленной из снимка: схема есть, данных
// нет. Пул закрывается после теста. Тесты с Pool в одном пакете не идут параллельно
// (t.Parallel): Restore пересоздаёт базу и обрывает соединения соседа.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if !started.Load() {
		t.Fatal("pgtest: в пакете нет TestMain с pgtest.Start — контейнер некому остановить")
	}
	initOnce.Do(initShared)
	switch {
	case skip != "" && os.Getenv(EnvRequireDocker) == "1":
		t.Fatalf("pgtest: %s=1, а Docker недоступен: %s", EnvRequireDocker, skip)
	case skip != "":
		t.Skipf("pgtest: Docker недоступен, интеграционный тест пропущен: %s", skip)
	case startErr != nil:
		t.Fatalf("pgtest: %v", startErr)
	}
	// Второй пул при живом первом — параллельный тест или два Pool в одном тесте: Restore
	// оборвал бы соединения первого посреди работы, и тест упал бы непонятно где.
	if !inUse.CompareAndSwap(false, true) {
		t.Fatal("pgtest: пул предыдущего вызова ещё открыт — Pool не для t.Parallel и не дважды за тест")
	}
	t.Cleanup(func() { inUse.Store(false) }) // Cleanup идут в обратном порядке: этот — после pool.Close

	if err := shared.ctr.Restore(t.Context()); err != nil {
		t.Fatalf("pgtest: восстановление снимка: %v", err)
	}
	pool, err := pgxpool.New(t.Context(), shared.dsn)
	if err != nil {
		t.Fatalf("pgtest: пул: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func initShared() {
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	if err := dockerAvailable(ctx); err != nil {
		skip = err.Error()
		return
	}
	shared, startErr = startDB(ctx)
}

// dockerAvailable — есть ли с кем говорить. testcontainers не находит Docker паникой
// (MustExtractDockerHost), поэтому проверка до запуска и с recover, как в его же
// SkipIfProviderIsNotHealthy, — но без *testing.T: решение «пропуск или провал» за Pool.
func dockerAvailable(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	p, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()
	return p.Health(ctx)
}

func startDB(ctx context.Context) (_ *DB, err error) {
	ctr, err := postgres.Run(ctx, Image,
		postgres.WithDatabase(dbName),
		postgres.WithSQLDriver(sqlDriver),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		// Run возвращает и контейнер, и ошибку, если он создан, но не дождался готовности.
		return nil, errors.Join(fmt.Errorf("запуск контейнера: %w", err), testcontainers.TerminateContainer(ctr))
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, testcontainers.TerminateContainer(ctr))
		}
	}()
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("строка подключения: %w", err)
	}
	if err := migrate(ctx, dsn); err != nil {
		return nil, err
	}
	// Снимок — после того как соединение миграций закрыто: CREATE DATABASE … TEMPLATE
	// не копирует базу, к которой кто-то подключён.
	if err := ctr.Snapshot(ctx); err != nil {
		return nil, fmt.Errorf("снимок: %w", err)
	}
	return &DB{ctr: ctr, dsn: dsn}, nil
}

// migrate применяет все миграции. Без блокировки goose (WithSessionLocker), в отличие
// от подкоманды migrate: базу контейнера видит только этот процесс.
func migrate(ctx context.Context, dsn string) (err error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("строка подключения: %w", err)
	}
	db := stdlib.OpenDB(*cfg)
	defer func() { err = errors.Join(err, db.Close()) }()
	p, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("миграции: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("миграции: %w", err)
	}
	return nil
}

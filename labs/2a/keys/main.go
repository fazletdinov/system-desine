// Программа keys (урок 2.3) вставляет одинаковое число строк в три таблицы, которые отличаются
// только первичным ключом: bigint identity, UUIDv4 (gen_random_uuid()) и UUIDv7 (uuidv7()), — и
// печатает, во что каждая обошлась: размер индекса PK, заполнение его листовых страниц, размер
// таблицы, объём WAL и время. Затем дозагружает десятую часть строк в уже заполненный индекс после
// контрольной точки — так вставки идут в проде — и печатает WAL и число полных образов страниц.
//
// Запуск из labs/ при поднятом 2a/compose.yaml: go run ./2a/keys -n 1000000
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// defaultRows — миллион строк: на нём разница в размере индекса и WAL видна без шума,
	// а прогон трёх ключей занимает секунды.
	defaultRows = 1_000_000
	// batchRows — строк в одном COPY, и каждый COPY — своя транзакция. Так грузят данные пачками
	// в проде; коммит на каждую строку мерил бы задержку подтверждения, а не цену ключа в индексе.
	batchRows = 10_000
	// topUpDivisor — дозагрузка составляет 1/10 основной загрузки (см. topUpRows).
	topUpDivisor = 10
	// defaultDSN — база из 2a/compose.yaml (порт по умолчанию LAB2A_PORT=55432).
	defaultDSN = "postgres://lab:lab@127.0.0.1:55432/lab?sslmode=disable"
	schema     = "keys"
	kib        = 1024
	mib        = 1024 * 1024
)

// keyKind — вариант первичного ключа: имя в выводе и определение столбца id (без PRIMARY KEY —
// ограничение с явным именем добавляет measure).
type keyKind struct {
	name   string
	column string
}

// #region kinds
// Значение id всегда генерирует база (DEFAULT или identity): все три варианта получают
// одинаковый поток COPY без столбца id, и различие остаётся только в ключе.
var kinds = []keyKind{
	{name: "bigint", column: "id bigint GENERATED ALWAYS AS IDENTITY"},
	{name: "uuidv4", column: "id uuid DEFAULT gen_random_uuid()"},
	{name: "uuidv7", column: "id uuid DEFAULT uuidv7()"},
}

// #endregion kinds

type config struct {
	rows int
	dsn  string
}

// phase — одна серия вставок: сколько строк, за какое время, сколько WAL и полных образов
// страниц (full page image) она записала.
type phase struct {
	rows     int
	elapsed  time.Duration
	walBytes int64
	walFPI   int64
}

// result — замер одного варианта ключа: загрузка в пустую таблицу, размеры индекса и таблицы после
// неё и дозагрузка в уже заполненный индекс после контрольной точки.
type result struct {
	kind       string
	load       phase
	topUp      phase
	tableBytes int64
	indexBytes int64
	leafPages  int64
	leafFill   float64 // средняя заполненность листовых страниц индекса, %
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		stop()
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "keys:", err)
		os.Exit(1)
	}
}

func parseFlags(args []string, getenv func(string) string) (config, error) {
	fs := flag.NewFlagSet("keys", flag.ContinueOnError)
	dsn := defaultDSN
	if v := getenv("LAB2A_DSN"); v != "" {
		dsn = v
	}
	cfg := config{}
	fs.IntVar(&cfg.rows, "n", defaultRows, "строк на каждый вариант ключа")
	fs.StringVar(&cfg.dsn, "dsn", dsn, "строка подключения (по умолчанию $LAB2A_DSN или база 2a/compose.yaml)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.rows <= 0 {
		return config{}, fmt.Errorf("-n должно быть больше нуля, получено %d", cfg.rows)
	}
	return cfg, nil
}

func run(ctx context.Context, args []string, out io.Writer) (err error) {
	cfg, err := parseFlags(args, os.Getenv)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, cfg.dsn)
	if err != nil {
		return fmt.Errorf("подключение к базе: %w", err)
	}
	// WithoutCancel: после Ctrl+C соединение всё равно закрывается вежливо, а не обрывом.
	defer func() { err = errors.Join(err, conn.Close(context.WithoutCancel(ctx))) }()

	var server string
	if err := conn.QueryRow(ctx, "SHOW server_version").Scan(&server); err != nil {
		return fmt.Errorf("версия сервера: %w", err)
	}
	if err := prepare(ctx, conn); err != nil {
		return err
	}
	results := make([]result, 0, len(kinds))
	for _, k := range kinds {
		r, err := measure(ctx, conn, k, cfg.rows)
		if err != nil {
			return fmt.Errorf("%s: %w", k.name, err)
		}
		results = append(results, r)
	}
	return report(out, server, results)
}

func prepare(ctx context.Context, conn *pgx.Conn) error {
	// pgstattuple даёт pgstatindex — заполненность листовых страниц B-tree; в официальном образе
	// postgres расширение есть среди contrib.
	stmts := []string{
		"CREATE EXTENSION IF NOT EXISTS pgstattuple",
		"DROP SCHEMA IF EXISTS " + pgx.Identifier{schema}.Sanitize() + " CASCADE",
		"CREATE SCHEMA " + pgx.Identifier{schema}.Sanitize(),
	}
	for _, s := range stmts {
		if _, err := conn.Exec(ctx, s); err != nil {
			return fmt.Errorf("подготовка (%s): %w", s, err)
		}
	}
	return nil
}

func measure(ctx context.Context, conn *pgx.Conn, k keyKind, rows int) (result, error) {
	table := pgx.Identifier{schema, "t_" + k.name}
	// autovacuum выключен: его проход по таблице посреди вставки добавил бы в окно замера свой WAL.
	// Имя ограничения PK задано явно: по нему ниже ищется индекс, а не по умолчанию «<таблица>_pkey».
	pkey := "t_" + k.name + "_pk"
	// #region measure
	ddl := fmt.Sprintf("CREATE TABLE %s (%s, n bigint NOT NULL, CONSTRAINT %s PRIMARY KEY (id)) WITH (autovacuum_enabled = false)",
		table.Sanitize(), k.column, pgx.Identifier{pkey}.Sanitize())
	if _, err := conn.Exec(ctx, ddl); err != nil {
		return result{}, fmt.Errorf("создание таблицы: %w", err)
	}
	r := result{kind: k.name}
	var err error
	if r.load, err = load(ctx, conn, table, 0, rows); err != nil {
		return result{}, err
	}
	index := pgx.Identifier{schema, pkey}.Sanitize()
	if err := conn.QueryRow(ctx,
		`SELECT pg_relation_size($1::regclass), pg_relation_size($2::regclass), s.leaf_pages, s.avg_leaf_density
		 FROM pgstatindex($2::regclass) AS s`,
		table.Sanitize(), index,
	).Scan(&r.tableBytes, &r.indexBytes, &r.leafPages, &r.leafFill); err != nil {
		return result{}, fmt.Errorf("размеры и pgstatindex: %w", err)
	}
	if r.topUp, err = load(ctx, conn, table, rows, topUpRows(rows)); err != nil {
		return result{}, err
	}
	// #endregion measure
	return r, nil
}

// topUpRows — размер дозагрузки: десятая часть основной, чтобы она шла в уже большой индекс,
// а не строила новый.
func topUpRows(rows int) int { return max(1, rows/topUpDivisor) }

// #region load
// load вставляет count строк пачками COPY, начиная с номера from+1, и меряет время и WAL.
func load(ctx context.Context, conn *pgx.Conn, table pgx.Identifier, from, count int) (phase, error) {
	// После контрольной точки первое изменение каждой уже существующей страницы пишет в WAL её
	// полный образ. CHECKPOINT перед каждой серией ставит все варианты в одинаковые условия.
	if _, err := conn.Exec(ctx, "CHECKPOINT"); err != nil {
		return phase{}, fmt.Errorf("контрольная точка: %w", err)
	}
	lsn, fpi, err := walPosition(ctx, conn)
	if err != nil {
		return phase{}, err
	}
	start := time.Now()
	for done := 0; done < count; done += batchRows {
		size := min(batchRows, count-done)
		first := from + done + 1
		src := pgx.CopyFromSlice(size, func(i int) ([]any, error) { return []any{int64(first + i)}, nil })
		if _, err := conn.CopyFrom(ctx, table, []string{"n"}, src); err != nil {
			return phase{}, fmt.Errorf("COPY строк %d–%d: %w", first, first+size-1, err)
		}
	}
	p := phase{rows: count, elapsed: time.Since(start)}
	if err := conn.QueryRow(ctx,
		"SELECT pg_wal_lsn_diff(pg_current_wal_lsn(), $1::pg_lsn)::bigint", lsn,
	).Scan(&p.walBytes); err != nil {
		return phase{}, fmt.Errorf("объём WAL: %w", err)
	}
	_, fpiAfter, err := walPosition(ctx, conn)
	if err != nil {
		return phase{}, err
	}
	p.walFPI = fpiAfter - fpi
	return p, nil
}

// #endregion load

// walPosition — текущая позиция WAL и счётчик полных образов страниц из pg_stat_wal.
// Оба счётчика общие для всего кластера, а не для нашей таблицы: в окно замера попадает и чужой
// WAL. Здесь это допустимо — база одноразовая, кроме keys в неё никто не пишет, autovacuum у таблиц
// выключен, а повторные прогоны совпадают в пределах 1–2 % (results/keys-n1000000-*.txt).
func walPosition(ctx context.Context, conn *pgx.Conn) (lsn string, fpi int64, err error) {
	// Статистика процессов попадает в pg_stat_wal не сразу, а раз в секунду; без принудительного
	// сброса последние пачки COPY не попали бы в счётчик.
	if _, err := conn.Exec(ctx, "SELECT pg_stat_force_next_flush()"); err != nil {
		return "", 0, fmt.Errorf("сброс статистики: %w", err)
	}
	if err := conn.QueryRow(ctx,
		"SELECT pg_current_wal_lsn()::text, wal_fpi FROM pg_stat_wal",
	).Scan(&lsn, &fpi); err != nil {
		return "", 0, fmt.Errorf("позиция WAL: %w", err)
	}
	return lsn, fpi, nil
}

// report печатает шапку, две таблицы (загрузка и дозагрузка) и строки-соотношения
// «<ключ> к <ключ>: …» — по последним сверяет scenario.sh.
func report(w io.Writer, server string, results []result) error {
	if len(results) == 0 {
		return errors.New("нет замеров")
	}
	load, topUp := results[0].load.rows, results[0].topUp.rows
	var b strings.Builder
	fmt.Fprintf(&b, "PostgreSQL %s, %s; пачка COPY: %d строк, своя транзакция\n", server, runtime.Version(), batchRows)

	fmt.Fprintf(&b, "\nзагрузка %d строк в пустую таблицу:\n", load)
	rows := []string{"ключ\tиндекс PK, КиБ\tбайт индекса на строку\tлистовых страниц\tзаполнение листьев, %\tтаблица, КиБ\tWAL, МиБ\tвремя, мс\tстрок/с\t"}
	for _, r := range results {
		rows = append(rows, fmt.Sprintf("%s\t%d\t%.1f\t%d\t%.1f\t%d\t%.1f\t%d\t%.0f\t",
			r.kind, r.indexBytes/kib, float64(r.indexBytes)/float64(r.load.rows), r.leafPages, r.leafFill,
			r.tableBytes/kib, float64(r.load.walBytes)/mib, r.load.elapsed.Milliseconds(), r.load.rate()))
	}
	if err := writeTable(&b, rows); err != nil {
		return err
	}

	fmt.Fprintf(&b, "\nещё %d строк после CHECKPOINT, индекс уже заполнен:\n", topUp)
	rows = []string{"ключ\tWAL, МиБ\tWAL на строку, байт\tполных образов страниц\tвремя, мс\tстрок/с\t"}
	for _, r := range results {
		rows = append(rows, fmt.Sprintf("%s\t%.1f\t%.0f\t%d\t%d\t%.0f\t",
			r.kind, float64(r.topUp.walBytes)/mib, float64(r.topUp.walBytes)/float64(r.topUp.rows),
			r.topUp.walFPI, r.topUp.elapsed.Milliseconds(), r.topUp.rate()))
	}
	if err := writeTable(&b, rows); err != nil {
		return err
	}

	b.WriteString("\n")
	for _, p := range comparisons {
		if p.of >= len(results) || p.to >= len(results) {
			continue
		}
		r, base := results[p.of], results[p.to]
		fmt.Fprintf(&b, "%s к %s: индекс ×%.2f, WAL загрузки ×%.2f, WAL дозагрузки ×%.2f, время загрузки ×%.2f\n",
			r.kind, base.kind,
			ratio(r.indexBytes, base.indexBytes),
			ratio(r.load.walBytes, base.load.walBytes),
			ratio(r.topUp.walBytes, base.topUp.walBytes),
			ratio(r.load.elapsed.Nanoseconds(), base.load.elapsed.Nanoseconds()))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeTable выравнивает по правому краю строки со столбцами через табуляцию.
func writeTable(w io.Writer, rows []string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	if _, err := io.WriteString(tw, strings.Join(rows, "\n")+"\n"); err != nil {
		return err
	}
	return tw.Flush()
}

func (p phase) rate() float64 {
	if p.elapsed <= 0 {
		return 0
	}
	return float64(p.rows) / p.elapsed.Seconds()
}

// comparisons — пары индексов в kinds для строк-соотношений: UUIDv4 и UUIDv7 к bigint, UUIDv4 к UUIDv7.
var comparisons = []struct{ of, to int }{{1, 0}, {2, 0}, {1, 2}}

func ratio(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

package migrations_test

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"

	"messenger/migrations"
)

// versions — номера встроенных миграций по возрастанию.
func versions(t *testing.T) []int64 {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	vs := make([]int64, 0, len(names))
	for _, name := range names {
		v, err := goose.NumericComponent(name)
		if err != nil {
			t.Fatalf("миграция %s: %v", name, err)
		}
		vs = append(vs, v)
	}
	slices.Sort(vs)
	return vs
}

// Required может отставать от последней миграции (expand для следующего выпуска), но
// не может указывать на миграцию, которой нет: такой бинарь не стартовал бы ни на какой
// базе, которую сам умеет создать.
func TestRequiredWithinMigrations(t *testing.T) {
	vs := versions(t)
	if len(vs) == 0 {
		t.Fatal("нет ни одной миграции")
	}
	if latest := vs[len(vs)-1]; migrations.Required < 1 || migrations.Required > latest {
		t.Fatalf("Required = %d, допустимо 1…%d (последняя миграция)", migrations.Required, latest)
	}
}

// Номера идут подряд с 1: последовательная нумерация — правило мессенджера (урок 2.8),
// и пропуск номера значит потерянный или переименованный файл.
func TestVersionsAreContiguous(t *testing.T) {
	for i, v := range versions(t) {
		if want := int64(i + 1); v != want {
			t.Fatalf("миграция с номером %d на месте %d: номера должны идти подряд с 1", v, want)
		}
	}
}

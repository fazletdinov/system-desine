//go:build migratedemo

package main

import (
	"errors"
	"io/fs"
	"os"
)

// envDemoMigrationsDir — каталог с миграциями для учебной сборки (тег migratedemo).
// Нужна scripts/migrate-demo.sh (урок 2.8): показать настоящий goose и настоящий вывод
// messenger migrate на миграции 00002, которой в мессенджере нет. В продакшен-сборку
// этот файл не попадает: тега нет, и бинарь применяет только встроенные миграции.
const envDemoMigrationsDir = "MESSENGER_DEMO_MIGRATIONS_DIR"

func migrationSource() (fs.FS, error) {
	dir := os.Getenv(envDemoMigrationsDir)
	if dir == "" {
		return nil, errors.New("учебная сборка migratedemo: задайте " + envDemoMigrationsDir + " — каталог с миграциями")
	}
	return os.DirFS(dir), nil
}

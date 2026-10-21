//go:build !migratedemo

package main

import (
	"io/fs"

	"messenger/migrations"
)

// migrationSource — файлы миграций, которые применяет подкоманда migrate: в обычной
// сборке это только встроенные в бинарь (migrations.FS), и подменить их нечем.
// Учебная сборка с тегом migratedemo читает их из каталога (migrate_source_demo.go).
func migrationSource() (fs.FS, error) { return migrations.FS, nil }

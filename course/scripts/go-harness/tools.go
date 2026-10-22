//go:build tools

// Package snippet держит в go.mod зависимости Go-фрагментов уроков. Своего кода здесь нет:
// фрагменты check:go собирает во временном каталоге с копией go.mod и go.sum. Без этого
// импорта go mod tidy (и Dependabot, который его запускает) вычеркнул бы pgx из require.
// Тег tools: файл не попадает ни в одну сборку.
package snippet

import _ "github.com/jackc/pgx/v5/pgxpool"

// Модуль, в котором check:go собирает каждый фрагмент урока: здесь закреплены
// версии внешних пакетов, которые фрагменты импортируют (pgx — драйвер PostgreSQL курса).
// go mod tidy здесь не запускать: .go-файлов нет, и tidy вычеркнет все require.
module snippet

go 1.27

require github.com/jackc/pgx/v5 v5.11.0

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

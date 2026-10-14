// Package userbridge — антикоррупционный слой между auth и user: переводит вызовы и ошибки
// одной модели в другую. Единственное место, где auth знает о контексте user (правило
// auth-context в .golangci.yml). В модуле 6 его заменит клиент сервиса users.
package userbridge

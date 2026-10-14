// Package httpauth — аутентификация на входе: middleware спрашивает Authenticator, кто
// вызывает, и кладёт ответ в context через principal. Как именно узнаётся личность —
// заголовком-заглушкой (до урока 1.14) или токеном — решает реализация Authenticator.
package httpauth

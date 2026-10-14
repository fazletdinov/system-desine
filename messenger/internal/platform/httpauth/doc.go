// Package httpauth — аутентификация на входе: middleware спрашивает Authenticator, кто
// вызывает, и кладёт ответ в context через principal. Как именно узнаётся личность —
// решает реализация Authenticator; в сборке это Bearer — проверка токена
// из заголовка Authorization (RFC 6750).
package httpauth

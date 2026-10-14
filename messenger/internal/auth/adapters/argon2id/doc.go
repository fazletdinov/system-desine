// Package argon2id — PasswordHasher на Argon2id (golang.org/x/crypto/argon2), первом
// выборе OWASP для хранения паролей. Хеш — в формате PHC: параметры записаны в нём самом,
// поэтому их можно поднять для новых паролей, не сломав проверку старых.
package argon2id

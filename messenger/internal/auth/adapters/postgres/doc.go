// Package postgres — хранилище учётных данных в PostgreSQL (таблица auth.credentials):
// реализует порты CredentialsSaver и CredentialsFinder из auth/usecase и проходит тот же
// контракт usecase/storetest, что memory.Store.
//
// Запросы идут через pgtx.Q: при регистрации — в транзакции use case'а SignUp, вместе с
// профилем из контекста user (ADR-0015), иначе — через пул. Своих транзакций адаптер не
// открывает.
package postgres

// Package postgres — хранилище пользователей в PostgreSQL (таблица users.users):
// реализует порты UserCreator, UserFinder и UserByUsernameFinder из user/usecase и
// проходит тот же контракт usecase/storetest, что memory.Store.
//
// Запросы идут через pgtx.Q: внутри транзакции use case'а (регистрация пишет профиль и
// учётные данные вместе, ADR-0015) — в ней, иначе — через пул. Своих транзакций адаптер
// не открывает.
package postgres

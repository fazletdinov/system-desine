// Package postgres — хранилище чатов в PostgreSQL (схема chat): реализует порты
// хранилища из chat/usecase и проходит тот же контракт usecase/storetest, что
// memory.Store. Здесь же UserChecker — порт chat/usecase.UserChecker поверх
// users.users (users.go).
//
// Запросы идут через pgtx.Q: внутри транзакции use case'а (попытка отправки
// сообщения, ADR-0015) — в ней, иначе — через пул. Своих транзакций адаптер не
// открывает: запись, которая трогает две таблицы (чат с участниками, сообщение с новым
// состоянием чата), — один оператор SQL с CTE, и атомарна сама по себе (см. store.go).
package postgres

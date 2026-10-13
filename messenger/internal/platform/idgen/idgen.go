package idgen

import "github.com/google/uuid"

type UUIDv7 struct{}

// NewID не возвращает ошибку. uuid.NewV7 ошибается, только если ошибку вернул источник
// случайности — по умолчанию crypto/rand.Reader. С Go 1.24 стандартный Reader ошибок
// не возвращает: при сбое ОС он сам аварийно завершает программу. uuid.Must — страховка
// на случай, если источник подменят через uuid.SetRand.
func (UUIDv7) NewID() string { return uuid.Must(uuid.NewV7()).String() }

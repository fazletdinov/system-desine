package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/auth/domain"
	"messenger/internal/platform/pgtx"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("postgres: nil-пул")
	}
	return &Store{pool: pool}, nil
}

// created_at не передаём: домен его не несёт, отметку ставит DEFAULT now() (миграция).
const (
	insertCredentials = `INSERT INTO auth.credentials (user_id, password_hash) VALUES ($1, $2)`
	selectCredentials = `SELECT user_id, password_hash FROM auth.credentials WHERE user_id = $1`
)

// SaveCredentials — один INSERT: «уже есть» отвечает первичный ключ, а не SELECT перед
// записью, — из двух одновременных сохранений пройдёт ровно одно.
func (s *Store) SaveCredentials(ctx context.Context, c domain.Credentials) error {
	_, err := pgtx.Q(ctx, s.pool).Exec(ctx, insertCredentials, c.UserID().String(), c.PasswordHash().String())
	if err != nil {
		return saveCredentialsError(err)
	}
	return nil
}

func (s *Store) FindCredentials(ctx context.Context, id domain.UserID) (domain.Credentials, error) {
	var rawID, rawHash string
	err := pgtx.Q(ctx, s.pool).QueryRow(ctx, selectCredentials, id.String()).Scan(&rawID, &rawHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Credentials{}, domain.ErrNoCredentials
	}
	if err != nil {
		return domain.Credentials{}, fmt.Errorf("postgres: чтение учётных данных: %w", err)
	}
	return toCredentials(rawID, rawHash)
}

// toCredentials — строка базы в доменные учётные данные.
func toCredentials(rawID, rawHash string) (domain.Credentials, error) {
	userID, err := domain.ParseUserID(rawID)
	if err != nil {
		return domain.Credentials{}, corrupt(rawID, err)
	}
	hash, err := domain.NewPasswordHash(rawHash)
	if err != nil {
		return domain.Credentials{}, corrupt(rawID, err)
	}
	c, err := domain.NewCredentials(userID, hash)
	if err != nil {
		return domain.Credentials{}, corrupt(rawID, err)
	}
	return c, nil
}

// corrupt — строка, которую записали в обход домена. Только текст причины, без %w:
// ValidationError конструктора — не ошибка данных клиента, и HTTP-адаптер не должен
// ответить на неё 400.
func corrupt(id string, err error) error {
	return fmt.Errorf("postgres: учётные данные %q не проходят проверки домена: %s", id, err.Error())
}

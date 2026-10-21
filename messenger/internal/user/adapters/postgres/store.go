package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/platform/pgtx"
	"messenger/internal/user/domain"
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

const (
	insertUser = `INSERT INTO users.users (id, username, display_name, created_at)
VALUES ($1, $2, $3, $4)`
	selectUser = `SELECT id, username, display_name, created_at FROM users.users`
)

// CreateUser — один INSERT: занятость имени проверяет UNIQUE в базе, а не SELECT перед
// записью, — между проверкой и записью имя успел бы занять другой запрос.
func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	_, err := pgtx.Q(ctx, s.pool).Exec(ctx, insertUser,
		user.ID().String(), user.Username().String(), user.DisplayName().String(), user.CreatedAt())
	if err != nil {
		return createUserError(err)
	}
	return nil
}

// #region find-user
func (s *Store) FindUser(ctx context.Context, id domain.UserID) (domain.User, error) {
	return scanUser(pgtx.Q(ctx, s.pool).QueryRow(ctx, selectUser+` WHERE id = $1`, id.String()))
}

func (s *Store) FindUserByUsername(ctx context.Context, username domain.Username) (domain.User, error) {
	return scanUser(pgtx.Q(ctx, s.pool).QueryRow(ctx, selectUser+` WHERE username = $1`, username.String()))
}

// #endregion find-user

// scanUser читает строку users.users и собирает пользователя конструкторами домена.
func scanUser(row pgx.Row) (domain.User, error) {
	var (
		rawID, rawUsername, rawDisplay string
		createdAt                      time.Time
	)
	err := row.Scan(&rawID, &rawUsername, &rawDisplay, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: чтение пользователя: %w", err)
	}
	return toUser(rawID, rawUsername, rawDisplay, createdAt)
}

// toUser — строка базы в доменного пользователя.
func toUser(rawID, rawUsername, rawDisplay string, createdAt time.Time) (domain.User, error) {
	id, err := domain.ParseUserID(rawID)
	if err != nil {
		return domain.User{}, corrupt(rawID, err)
	}
	username, err := domain.NewUsername(rawUsername)
	if err != nil {
		return domain.User{}, corrupt(rawID, err)
	}
	display, err := domain.NewDisplayName(rawDisplay)
	if err != nil {
		return domain.User{}, corrupt(rawID, err)
	}
	return domain.NewUser(id, username, display, createdAt), nil
}

// corrupt — строка, которую записали в обход домена. Только текст причины, без %w:
// ValidationError конструктора — не ошибка данных клиента, и HTTP-адаптер не должен
// ответить на неё 400.
func corrupt(id string, err error) error {
	return fmt.Errorf("postgres: пользователь %q не проходит проверки домена: %s", id, err.Error())
}

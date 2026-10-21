package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/pgtx"
)

// UserChecker реализует chat/usecase.UserChecker одним запросом к users.users. Чтение
// чужой схемы — та же цена одной базы на все контексты, что и внешний ключ
// chat.members → users.users (ADR-0014): контекст chat знает таблицу пользователей, но
// не код контекста user. Порт остаётся швом: при выделении сервиса пользователей сюда
// встанет адаптер над его API (как userchecker сегодня — над use case'ом), а use case
// CreateChat не изменится.
type UserChecker struct {
	pool *pgxpool.Pool
}

func NewUserChecker(pool *pgxpool.Pool) (*UserChecker, error) {
	if pool == nil {
		return nil, errors.New("postgres: nil-пул")
	}
	return &UserChecker{pool: pool}, nil
}

// #region missing-users
// missingUsers — «запрошенные минус существующие» одним запросом, сколько бы id ни
// пришло. Правая часть EXCEPT ограничена тем же списком: так она читает первичный ключ
// users.users по нескольким id, а не всю таблицу. EXCEPT убирает и повторы; ORDER BY
// — чтобы ошибка «нет таких пользователей» перечисляла их в одном порядке.
const missingUsers = `
SELECT id FROM unnest($1::uuid[]) AS id
EXCEPT
SELECT id FROM users.users WHERE id = ANY ($1::uuid[])
ORDER BY 1`

// #endregion missing-users

func (c *UserChecker) MissingUsers(ctx context.Context, ids []domain.UserID) ([]domain.UserID, error) {
	rows, err := pgtx.Q(ctx, c.pool).Query(ctx, missingUsers, userIDStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("postgres: проверка пользователей: %w", err)
	}
	raw, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("postgres: проверка пользователей: %w", err)
	}
	missing := make([]domain.UserID, len(raw))
	for i, s := range raw {
		if missing[i], err = domain.ParseUserID(s); err != nil {
			return nil, corrupt("пользователь", s, err)
		}
	}
	return missing, nil
}

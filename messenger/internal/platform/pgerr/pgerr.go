package pgerr

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// #region is-unique-violation
// uniqueViolation — SQLSTATE unique_violation: нарушены UNIQUE или PRIMARY KEY.
const uniqueViolation = "23505"

// IsUniqueViolation сообщает, что err — нарушение уникальности ограничения constraint.
// Нарушение другого ограничения — false: дубль первичного ключа, например, значит сбой
// генератора id, а не «имя занято», и переводить его в ту же доменную ошибку нельзя.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == constraint
}

// #endregion is-unique-violation

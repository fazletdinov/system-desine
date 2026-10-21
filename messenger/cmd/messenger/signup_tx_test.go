package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	authpostgres "messenger/internal/auth/adapters/postgres"
	"messenger/internal/auth/adapters/userbridge"
	authdomain "messenger/internal/auth/domain"
	authusecase "messenger/internal/auth/usecase"
	"messenger/internal/platform/clock"
	"messenger/internal/platform/idgen"
	"messenger/internal/platform/pgtest"
	"messenger/internal/platform/pgtx"
	userpostgres "messenger/internal/user/adapters/postgres"
	userusecase "messenger/internal/user/usecase"
)

// Регистрация пишет в два контекста — профиль (users.users) и учётные данные
// (auth.credentials) — в одной транзакции SignUp (ADR-0015). Тест собирает её так же,
// как composition root: настоящие Postgres-адаптеры обоих контекстов, мост userbridge и
// pgtx. Поэтому он здесь, а не в пакете адаптера: auth не импортирует user даже в тестах.
func TestSignUpRollsBackProfileOnCredentialsFailure(t *testing.T) {
	errCredentials := errors.New("сбой записи учётных данных")
	tests := []struct {
		name      string
		failAfter bool // учётные данные записаны, а затем шаг вернул ошибку
		wantRows  int  // строк в users.users и в auth.credentials после регистрации
	}{
		// Контроль: без сбоя обе строки на месте — иначе «0 строк» ниже ничего бы не значило.
		{"без сбоя", false, 1},
		{"сбой после записи учётных данных", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := pgtest.Pool(t)
			signUp := newPostgresSignUp(t, pool, tt.failAfter, errCredentials)

			_, err := signUp.Execute(t.Context(), authusecase.SignUpCommand{
				Username: "alice", DisplayName: "Алиса", Password: "correct horse battery staple",
			})
			if tt.failAfter && !errors.Is(err, errCredentials) {
				t.Fatalf("ждали сбой учётных данных, получили %v", err)
			}
			if !tt.failAfter && err != nil {
				t.Fatal(err)
			}
			if n := countRows(t, pool, `SELECT count(*) FROM users.users WHERE username = 'alice'`); n != tt.wantRows {
				t.Fatalf("профилей alice: %d, ждали %d", n, tt.wantRows)
			}
			if n := countRows(t, pool, `SELECT count(*) FROM auth.credentials`); n != tt.wantRows {
				t.Fatalf("учётных данных: %d, ждали %d", n, tt.wantRows)
			}
		})
	}
}

func newPostgresSignUp(t *testing.T, pool *pgxpool.Pool, failAfter bool, errFail error) *authusecase.SignUp {
	t.Helper()
	users, err := userpostgres.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	register, err := userusecase.NewRegisterUser(users, clock.System{}, idgen.UUIDv7{})
	if err != nil {
		t.Fatal(err)
	}
	find, err := userusecase.NewFindByUsername(users)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := userbridge.New(register, find)
	if err != nil {
		t.Fatal(err)
	}
	creds, err := authpostgres.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	var saver authusecase.CredentialsSaver = creds
	if failAfter {
		saver = failingSaver{next: creds, err: errFail}
	}
	signUp, err := authusecase.NewSignUp(fixedHasher{}, bridge, bridge, saver, tx)
	if err != nil {
		t.Fatal(err)
	}
	return signUp
}

// failingSaver записывает учётные данные настоящим адаптером и затем отказывает: в базе
// внутри транзакции лежат уже обе строки, и откатиться должны обе.
type failingSaver struct {
	next authusecase.CredentialsSaver
	err  error
}

func (f failingSaver) SaveCredentials(ctx context.Context, c authdomain.Credentials) error {
	if err := f.next.SaveCredentials(ctx, c); err != nil {
		return err
	}
	return f.err
}

// fixedHasher — вместо Argon2id: тест о транзакции, а не о хеше, и полный хеш стоил бы
// сотни миллисекунд.
type fixedHasher struct{}

func (fixedHasher) Hash(context.Context, authdomain.Password) (authdomain.PasswordHash, error) {
	return authdomain.NewPasswordHash("fixed-hash")
}

func (fixedHasher) Verify(context.Context, string, authdomain.PasswordHash) (bool, error) {
	return false, errors.New("fixedHasher: Verify не нужен регистрации")
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

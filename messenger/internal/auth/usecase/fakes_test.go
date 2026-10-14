package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// countingHasher — прозрачный «хеш» "h:" + пароль: тесту use case'а важно не устройство
// хеша, а сколько раз и с чем его вызвали. Счётчики атомарные: тесты конструкторов
// и подтесты не делят хешер, но гонки детектор всё равно проверит.
type countingHasher struct {
	hashes    atomic.Int32
	verifies  atomic.Int32
	hashErr   error
	verifyErr error
}

const fakePrefix = "h:"

func (h *countingHasher) Hash(p domain.Password) (domain.PasswordHash, error) {
	h.hashes.Add(1)
	if h.hashErr != nil {
		return domain.PasswordHash{}, h.hashErr
	}
	return domain.NewPasswordHash(fakePrefix + p.Plaintext())
}

func (h *countingHasher) Verify(plaintext string, hash domain.PasswordHash) (bool, error) {
	h.verifies.Add(1)
	if h.verifyErr != nil {
		return false, h.verifyErr
	}
	if !strings.HasPrefix(hash.String(), fakePrefix) {
		return false, errors.New("испорченный хеш")
	}
	return hash.String() == fakePrefix+plaintext, nil
}

// profiles — фейк контекста user: и ProfileRegistrar, и UserLookup. Имена «как в user»:
// только латиница в нижнем регистре — остальное для поиска «нет такого».
type profiles struct {
	mu      sync.Mutex
	byName  map[string]usecase.Profile
	n       int
	calls   int
	failErr error
}

func newProfiles() *profiles { return &profiles{byName: make(map[string]usecase.Profile)} }

func (p *profiles) RegisterProfile(_ context.Context, username, displayName string) (usecase.Profile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.failErr != nil {
		return usecase.Profile{}, p.failErr
	}
	if _, ok := p.byName[username]; ok {
		return usecase.Profile{}, domain.ErrUsernameTaken
	}
	p.n++
	id, err := domain.ParseUserID(fmt.Sprintf("00000000-0000-7000-a000-%012d", p.n))
	if err != nil {
		return usecase.Profile{}, err
	}
	profile := usecase.Profile{ID: id, Username: username, DisplayName: displayName, CreatedAt: t0}
	p.byName[username] = profile
	return profile, nil
}

func (p *profiles) UserIDByUsername(_ context.Context, username string) (domain.UserID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	profile, ok := p.byName[username]
	if !ok {
		return domain.UserID{}, domain.ErrNoSuchUser
	}
	return profile.ID, nil
}

// countingSaver считает обращения к хранилищу и умеет падать.
type countingSaver struct {
	usecase.CredentialsSaver
	calls   atomic.Int32
	failErr error
}

func (s *countingSaver) SaveCredentials(ctx context.Context, c domain.Credentials) error {
	s.calls.Add(1)
	if s.failErr != nil {
		return s.failErr
	}
	return s.CredentialsSaver.SaveCredentials(ctx, c)
}

// fixedIssuer выдаёт токен «tok:<id>»: тест видит, кому выпущен токен.
type fixedIssuer struct{}

func (fixedIssuer) Issue(id domain.UserID) (usecase.Token, error) {
	return usecase.Token{Value: "tok:" + id.String(), ExpiresAt: t0.Add(15 * time.Minute)}, nil
}

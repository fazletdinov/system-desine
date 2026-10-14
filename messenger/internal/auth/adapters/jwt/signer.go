package jwt

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

// #region claims
const (
	Issuer   = "messenger"
	Audience = "messenger-api"
	// TTL короткий: отозвать выпущенный JWT нельзя (он проверяется без обращения к нам),
	// поэтому его срок — это и есть худшее время жизни украденного токена. Refresh-токены,
	// которые позволяют не логиниться каждые 15 минут, — модуль 6.7.
	TTL = 15 * time.Minute
	// Leeway — допуск на расхождение часов выпустившего и проверяющего.
	Leeway = 30 * time.Second
)

// #endregion claims

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type Signer struct {
	key   ed25519.PrivateKey
	kid   string
	clock Clock
	ids   IDGenerator
}

func NewSigner(key ed25519.PrivateKey, clock Clock, ids IDGenerator) (*Signer, error) {
	if len(key) != ed25519.PrivateKeySize || clock == nil || ids == nil {
		return nil, errors.New("jwt signer: нужен ключ Ed25519, часы и генератор id")
	}
	return &Signer{key: key, kid: KeyID(key.Public().(ed25519.PublicKey)), clock: clock, ids: ids}, nil
}

// #region issue
func (s *Signer) Issue(userID domain.UserID) (usecase.Token, error) {
	// NumericDate — секунды; обрезаем сразу, чтобы ExpiresAt в ответе совпал с exp в токене.
	now := s.clock.Now().UTC().Truncate(time.Second)
	exp := now.Add(TTL)
	claims := gojwt.RegisteredClaims{
		Issuer:    Issuer,
		Subject:   userID.String(),
		Audience:  gojwt.ClaimStrings{Audience},
		ExpiresAt: gojwt.NewNumericDate(exp),
		NotBefore: gojwt.NewNumericDate(now),
		IssuedAt:  gojwt.NewNumericDate(now),
		ID:        s.ids.NewID(), // jti — чтобы в 6.7 можно было вести список отозванных
	}
	tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = s.kid
	signed, err := tok.SignedString(s.key)
	if err != nil {
		return usecase.Token{}, fmt.Errorf("jwt: подпись: %w", err)
	}
	return usecase.Token{Value: signed, ExpiresAt: exp}, nil
}

// #endregion issue

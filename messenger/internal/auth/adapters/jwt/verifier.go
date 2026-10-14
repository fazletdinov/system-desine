package jwt

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"
)

type Verifier struct {
	keys  map[string]ed25519.PublicKey // kid → ключ; несколько — на время смены ключа
	clock Clock
}

func NewVerifier(clock Clock, keys ...ed25519.PublicKey) (*Verifier, error) {
	if clock == nil || len(keys) == 0 {
		return nil, errors.New("jwt verifier: нужны часы и хотя бы один ключ")
	}
	byKID := make(map[string]ed25519.PublicKey, len(keys))
	for _, k := range keys {
		if len(k) != ed25519.PublicKeySize {
			return nil, errors.New("jwt verifier: ключ не Ed25519")
		}
		byKID[KeyID(k)] = k
	}
	return &Verifier{keys: byKID, clock: clock}, nil
}

// #region verify
// Verify возвращает sub проверенного токена. Алгоритм — только EdDSA: поле alg пишет
// автор токена, и верить ему — значит дать атакующему выбрать none или HS256 с нашим
// открытым ключом в роли секрета. Ключ — только из своего списка по kid.
func (v *Verifier) Verify(_ context.Context, token string) (string, error) {
	var claims gojwt.RegisteredClaims
	_, err := gojwt.ParseWithClaims(token, &claims, v.key,
		gojwt.WithValidMethods([]string{gojwt.SigningMethodEdDSA.Alg()}),
		gojwt.WithIssuer(Issuer),
		gojwt.WithAudience(Audience),
		gojwt.WithExpirationRequired(),
		gojwt.WithIssuedAt(),
		gojwt.WithLeeway(Leeway),
		gojwt.WithTimeFunc(v.clock.Now),
	)
	if err != nil {
		return "", fmt.Errorf("jwt: %w", err)
	}
	if claims.Subject == "" {
		return "", errors.New("jwt: в токене нет sub")
	}
	return claims.Subject, nil
}

// #endregion verify

func (v *Verifier) key(t *gojwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	k, ok := v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("неизвестный kid %q", kid)
	}
	return k, nil
}

package jwt_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"messenger/internal/auth/adapters/jwt"
	authdomain "messenger/internal/auth/domain"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

type ids struct{}

func (ids) NewID() string { return "00000000-0000-7000-f000-000000000001" }

const alice = "00000000-0000-7000-8000-000000000001"

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func newKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func pub(k ed25519.PrivateKey) ed25519.PublicKey { return k.Public().(ed25519.PublicKey) }

// forge подписывает произвольные claims нашим ключом и нашим kid — так атакующий,
// укравший ключ, или наш же сервис с ошибкой выпустили бы «почти правильный» токен.
func forge(t *testing.T, key ed25519.PrivateKey, claims gojwt.RegisteredClaims) string {
	t.Helper()
	tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = jwt.KeyID(pub(key))
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() gojwt.RegisteredClaims {
	return gojwt.RegisteredClaims{
		Issuer: jwt.Issuer, Subject: alice, Audience: gojwt.ClaimStrings{jwt.Audience},
		IssuedAt: gojwt.NewNumericDate(t0), NotBefore: gojwt.NewNumericDate(t0), ExpiresAt: gojwt.NewNumericDate(t0.Add(jwt.TTL)),
	}
}

func TestIssueAndVerify(t *testing.T) {
	key := newKey(t)
	clk := &clock{t: t0}
	signer, err := jwt.NewSigner(key, clk, ids{})
	if err != nil {
		t.Fatal(err)
	}
	uid, err := authdomain.ParseUserID(alice)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := signer.Issue(uid)
	if err != nil {
		t.Fatal(err)
	}
	if !tok.ExpiresAt.Equal(t0.Add(jwt.TTL)) {
		t.Fatalf("срок %v, ждали %v", tok.ExpiresAt, t0.Add(jwt.TTL))
	}
	v, err := jwt.NewVerifier(clk, pub(key))
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Duration{0, jwt.TTL + jwt.Leeway - time.Second} {
		clk.t = t0.Add(at)
		sub, err := v.Verify(t.Context(), tok.Value)
		if err != nil || sub != alice {
			t.Fatalf("через %v: sub %q, ошибка %v", at, sub, err)
		}
	}
}

// #region attacks
func TestVerifierRejects(t *testing.T) {
	key, other := newKey(t), newKey(t)
	good := forge(t, key, validClaims())
	parts := strings.Split(good, ".")
	tests := []struct {
		name  string
		token func(t *testing.T) string
		now   time.Time
	}{
		{"alg: none", func(t *testing.T) string {
			tok := gojwt.NewWithClaims(gojwt.SigningMethodNone, validClaims())
			tok.Header["kid"] = jwt.KeyID(pub(key))
			s, err := tok.SignedString(gojwt.UnsafeAllowNoneSignatureType)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, t0},
		{"HS256 с открытым ключом в роли секрета", func(t *testing.T) string {
			tok := gojwt.NewWithClaims(gojwt.SigningMethodHS256, validClaims())
			tok.Header["kid"] = jwt.KeyID(pub(key))
			s, err := tok.SignedString([]byte(pub(key)))
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, t0},
		{"чужой ключ, наш kid", func(t *testing.T) string {
			tok := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, validClaims())
			tok.Header["kid"] = jwt.KeyID(pub(key))
			s, err := tok.SignedString(other)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}, t0},
		{"неизвестный kid", func(t *testing.T) string { return forge(t, other, validClaims()) }, t0},
		{"изменён payload", func(t *testing.T) string {
			c := validClaims()
			c.Subject = "00000000-0000-7000-8000-000000000666"
			forged := strings.Split(forge(t, key, c), ".")
			return parts[0] + "." + forged[1] + "." + parts[2] // чужие данные со старой подписью
		}, t0},
		{"обрезанная подпись", func(*testing.T) string { return good[:len(good)-4] }, t0},
		{"подпись не base64url", func(*testing.T) string { return parts[0] + "." + parts[1] + ".!!!" }, t0},
		{"просрочен сверх допуска", func(*testing.T) string { return good }, t0.Add(jwt.TTL + jwt.Leeway + time.Second)},
		{"выпущен в будущем сверх допуска", func(*testing.T) string { return good }, t0.Add(-jwt.Leeway - time.Minute)},
		{"чужой aud", func(t *testing.T) string {
			c := validClaims()
			c.Audience = gojwt.ClaimStrings{"billing-api"}
			return forge(t, key, c)
		}, t0},
		{"чужой iss", func(t *testing.T) string {
			c := validClaims()
			c.Issuer = "evil"
			return forge(t, key, c)
		}, t0},
		{"без exp", func(t *testing.T) string {
			c := validClaims()
			c.ExpiresAt = nil
			return forge(t, key, c)
		}, t0},
		{"без sub", func(t *testing.T) string {
			c := validClaims()
			c.Subject = ""
			return forge(t, key, c)
		}, t0},
		{"мусор", func(*testing.T) string { return "это.не.токен" }, t0},
		{"пусто", func(*testing.T) string { return "" }, t0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := jwt.NewVerifier(&clock{t: tt.now}, pub(key))
			if err != nil {
				t.Fatal(err)
			}
			if sub, err := v.Verify(t.Context(), tt.token(t)); err == nil {
				t.Fatalf("токен принят, sub %q", sub)
			}
		})
	}
}

// #endregion attacks

func TestKeyRoundTrip(t *testing.T) {
	key := newKey(t)
	pemBytes, err := jwt.EncodePrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jwt.ParsePrivateKey(pemBytes)
	if err != nil || !got.Equal(key) {
		t.Fatalf("ключ после PEM другой: %v", err)
	}
	if _, err := jwt.ParsePrivateKey([]byte("-----BEGIN RSA PRIVATE KEY-----\nAAAA\n-----END RSA PRIVATE KEY-----\n")); err == nil {
		t.Fatal("RSA-ключ принят как Ed25519")
	}
}

// Отпечаток RFC 7638 для ключа из RFC 8037 (приложение A.3): kid = kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k.
func TestKeyIDMatchesRFC8037(t *testing.T) {
	x, err := base64.RawURLEncoding.DecodeString("11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo")
	if err != nil {
		t.Fatal(err)
	}
	if got := jwt.KeyID(ed25519.PublicKey(x)); got != "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k" {
		t.Fatalf("kid %q", got)
	}
}

package argon2id

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"

	"messenger/internal/auth/domain"
)

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// #region params
// OWASPMinimum — минимальная конфигурация Argon2id из OWASP Password Storage Cheat Sheet:
// m = 19 МиБ (19 456 КиБ), t = 2, p = 1. Соль 16 байт и ключ 32 байта — как в
// рекомендованных вариантах RFC 9106, §4.
var OWASPMinimum = Params{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}

// #endregion params

const (
	// Границы параметров, прочитанных из хеша: подменённый хеш не должен заставить
	// сервер выделить гигабайты или крутиться минуты.
	maxMemoryKiB  = 1 << 20 // 1 ГиБ
	maxIterations = 16
	minSaltLen    = 16
	minKeyLen     = 16
	maxKeyLen     = 64
	// minMemoryPerLane — Argon2 требует не меньше 8 блоков по 1 КиБ на каждую дорожку (RFC 9106, §3.1).
	minMemoryPerLane = 8
)

var b64 = base64.RawStdEncoding // PHC: стандартный алфавит без «=»

// Hasher реализует usecase.PasswordHasher.
type Hasher struct {
	p    Params
	rand io.Reader
}

func New(p Params) (*Hasher, error) {
	switch {
	case p.Parallelism == 0 || p.Iterations == 0 || p.Iterations > maxIterations:
		return nil, errors.New("argon2id: нужны t от 1 до 16 и p ≥ 1")
	case p.MemoryKiB < minMemoryPerLane*uint32(p.Parallelism) || p.MemoryKiB > maxMemoryKiB:
		return nil, errors.New("argon2id: память от 8·p КиБ до 1 ГиБ")
	case p.SaltLen < minSaltLen || p.KeyLen < minKeyLen || p.KeyLen > maxKeyLen:
		return nil, errors.New("argon2id: соль от 16 байт, ключ от 16 до 64 байт")
	}
	return &Hasher{p: p, rand: rand.Reader}, nil
}

// #region hash
// ctx проверяется только до вычисления: внутри argon2.IDKey нет точки, где можно
// остановиться, — это один вызов, который заполняет m КиБ и проходит их t раз.
// Отменённый запрос не начнёт хеш, но начатый досчитается (≈ 18 мс в одиночку, до ~40 мс при N параллельных, урок 1.5.6; OWASPMinimum).
func (h *Hasher) Hash(ctx context.Context, pw domain.Password) (domain.PasswordHash, error) {
	if err := ctx.Err(); err != nil {
		return domain.PasswordHash{}, fmt.Errorf("argon2id: %w", err)
	}
	salt := make([]byte, h.p.SaltLen)
	if _, err := io.ReadFull(h.rand, salt); err != nil {
		return domain.PasswordHash{}, fmt.Errorf("argon2id: соль: %w", err)
	}
	key := argon2.IDKey([]byte(pw.Plaintext()), salt, h.p.Iterations, h.p.MemoryKiB, h.p.Parallelism, h.p.KeyLen)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.p.MemoryKiB, h.p.Iterations, h.p.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key))
	return domain.NewPasswordHash(encoded)
}

func (h *Hasher) Verify(ctx context.Context, plaintext string, hash domain.PasswordHash) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("argon2id: %w", err)
	}
	p, salt, key, err := parse(hash.String())
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plaintext), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key)))
	// ConstantTimeCompare: обычное сравнение остановилось бы на первом несовпавшем байте,
	// и время ответа подсказало бы, сколько байт угадано.
	return subtle.ConstantTimeCompare(got, key) == 1, nil
}

// #endregion hash

func parse(s string) (Params, []byte, []byte, error) {
	fail := func(why string) (Params, []byte, []byte, error) {
		return Params{}, nil, nil, fmt.Errorf("argon2id: испорченный хеш: %s", why)
	}
	parts := strings.Split(s, "$") // "", "argon2id", "v=19", "m=…,t=…,p=…", соль, ключ
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return fail("не PHC-строка argon2id")
	}
	var v int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return fail("версия")
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return fail("параметры")
	}
	if p.Parallelism == 0 || p.Iterations == 0 || p.Iterations > maxIterations ||
		p.MemoryKiB < minMemoryPerLane*uint32(p.Parallelism) || p.MemoryKiB > maxMemoryKiB {
		return fail("параметры вне допустимых границ")
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < minSaltLen {
		return fail("соль")
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < minKeyLen || len(key) > maxKeyLen {
		return fail("ключ")
	}
	return p, salt, key, nil
}

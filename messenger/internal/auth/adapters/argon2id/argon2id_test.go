package argon2id_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"messenger/internal/auth/adapters/argon2id"
	"messenger/internal/auth/domain"
)

// fast — крошечные параметры: тестам важна логика формата и сравнения, а не стойкость.
var fast = argon2id.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

func newHasher(t testing.TB, p argon2id.Params) *argon2id.Hasher {
	t.Helper()
	h, err := argon2id.New(p)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func password(t testing.TB, s string) domain.Password {
	t.Helper()
	p, err := domain.NewPassword(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func hash(t testing.TB, s string) domain.PasswordHash {
	t.Helper()
	h, err := domain.NewPasswordHash(s)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHashAndVerify(t *testing.T) {
	t.Parallel()
	h := newHasher(t, fast)
	const pw = "правильный-пароль-длинный"
	hashed, err := h.Hash(context.Background(), password(t, pw))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hashed.String(), "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("хеш не в формате PHC с параметрами: %s", hashed)
	}
	if strings.Contains(hashed.String(), pw) {
		t.Fatal("пароль виден в хеше")
	}
	if ok, err := h.Verify(context.Background(), pw, hashed); err != nil || !ok {
		t.Fatalf("свой пароль не прошёл: %v, %v", ok, err)
	}
	if ok, err := h.Verify(context.Background(), "неправильный-пароль-длинный", hashed); err != nil || ok {
		t.Fatalf("чужой пароль прошёл: %v, %v", ok, err)
	}
}

// Одинаковые пароли дают разные хеши: у каждого своя соль, и по базе не видно,
// у кого пароли совпадают.
func TestSaltMakesHashesDiffer(t *testing.T) {
	t.Parallel()
	h := newHasher(t, fast)
	a, err := h.Hash(context.Background(), password(t, "один-и-тот-же-пароль"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.Hash(context.Background(), password(t, "один-и-тот-же-пароль"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("два хеша одного пароля совпали: %s", a)
	}
}

// Параметры читаются из хеша: старые хеши проверяются и после того, как сервер
// перешёл на новые параметры.
func TestVerifyUsesParamsFromHash(t *testing.T) {
	t.Parallel()
	old := newHasher(t, argon2id.Params{MemoryKiB: 32, Iterations: 2, Parallelism: 2, SaltLen: 16, KeyLen: 16})
	const pw = "пароль-со-старыми-параметрами"
	hashed, err := old.Hash(context.Background(), password(t, pw))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := newHasher(t, fast).Verify(context.Background(), pw, hashed); err != nil || !ok {
		t.Fatalf("хеш со старыми параметрами не проверился новым хешером: %v, %v", ok, err)
	}
}

func TestVerifyRejectsCorruptHash(t *testing.T) {
	t.Parallel()
	h := newHasher(t, fast)
	good, err := h.Hash(context.Background(), password(t, "правильный-пароль-длинный"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good.String(), "$") // "", argon2id, v=19, m=…, соль, ключ
	with := func(i int, v string) string {
		p := append([]string(nil), parts...)
		p[i] = v
		return strings.Join(p, "$")
	}
	tests := []struct{ name, hash string }{
		{"5 частей", strings.Join(parts[:5], "$")},
		{"argon2i вместо argon2id", with(1, "argon2i")},
		{"версия 16", with(2, "v=16")},
		{"память 999999999 КиБ", with(3, "m=999999999,t=1,p=1")},
		{"t=0", with(3, "m=64,t=0,p=1")},
		{"соль не base64", with(4, "!!!")},
		{"ключ 8 байт", with(5, "AAAAAAAAAAA")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if ok, err := h.Verify(context.Background(), "правильный-пароль-длинный", hash(t, tt.hash)); err == nil || ok {
				t.Fatalf("испорченный хеш %q принят: %v, %v", tt.hash, ok, err)
			}
		})
	}
}

// Отменённый ctx — отказ до вычисления: запрос, который уже никому не нужен, не должен
// тратить 19 МиБ и десятки миллисекунд. Начатое вычисление ctx не прерывает (комментарий у Hash).
func TestCanceledContextSkipsWork(t *testing.T) {
	t.Parallel()
	h := newHasher(t, fast)
	good, err := h.Hash(context.Background(), password(t, "правильный-пароль-длинный"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, password(t, "правильный-пароль-длинный")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Hash с отменённым ctx: ждали context.Canceled, получили %v", err)
	}
	if ok, err := h.Verify(ctx, "правильный-пароль-длинный", good); !errors.Is(err, context.Canceled) || ok {
		t.Fatalf("Verify с отменённым ctx: ждали context.Canceled, получили %v, %v", ok, err)
	}
}

func TestNewRejectsBadParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(p *argon2id.Params)
	}{
		{"t=0", func(p *argon2id.Params) { p.Iterations = 0 }},
		{"t=17", func(p *argon2id.Params) { p.Iterations = 17 }},
		{"p=0", func(p *argon2id.Params) { p.Parallelism = 0 }},
		{"память меньше 8·p", func(p *argon2id.Params) { p.MemoryKiB = 7 }},
		{"память больше 1 ГиБ", func(p *argon2id.Params) { p.MemoryKiB = 1<<20 + 1 }},
		{"соль 8 байт", func(p *argon2id.Params) { p.SaltLen = 8 }},
		{"ключ 8 байт", func(p *argon2id.Params) { p.KeyLen = 8 }},
		{"ключ 65 байт", func(p *argon2id.Params) { p.KeyLen = 65 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := fast
			tt.edit(&p)
			if _, err := argon2id.New(p); err == nil {
				t.Fatalf("New(%+v): ждали ошибку", p)
			}
		})
	}
	if _, err := argon2id.New(argon2id.OWASPMinimum); err != nil {
		t.Fatalf("OWASPMinimum отвергнут: %v", err)
	}
}

// BenchmarkHashOWASP — сколько стоит один хеш с параметрами из main.go. Цифры — для урока.
func BenchmarkHashOWASP(b *testing.B) {
	h := newHasher(b, argon2id.OWASPMinimum)
	pw := password(b, "правильный-пароль-длинный")
	for b.Loop() {
		if _, err := h.Hash(context.Background(), pw); err != nil {
			b.Fatal(err)
		}
	}
}

package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// #region password-policy
const (
	// NIST SP 800-63B-4, §3.1.1.2: пароль как единственный фактор — не короче 15 символов;
	// верхнюю границу верификатору следует (SHOULD) допускать не меньше 64. 128 — с запасом.
	// Символ — кодовая точка Unicode, а не байт. Требований к составу («цифра, заглавная,
	// спецсимвол») нет сознательно: NIST их запрещает (SHALL NOT) — они делают пароли
	// предсказуемее, а не сильнее.
	PasswordMinLen = 15
	PasswordMaxLen = 128
	// PasswordMaxBytes — верхняя граница в байтах для пароля при входе (до 4 байт UTF-8
	// на символ): мегабайтную строку незачем отдавать медленной функции хеширования.
	PasswordMaxBytes = 4 * PasswordMaxLen
)

// Password — новый пароль, прошедший политику. Нужен только при регистрации и смене:
// при входе пароль сравнивают как есть — политика могла ужесточиться после регистрации.
type Password struct{ value string }

func NewPassword(s string) (Password, error) {
	if !utf8.ValidString(s) {
		return Password{}, invalid("password", "некорректный UTF-8")
	}
	if n := utf8.RuneCountInString(s); n < PasswordMinLen || n > PasswordMaxLen {
		return Password{}, invalid("password", fmt.Sprintf("от %d до %d символов", PasswordMinLen, PasswordMaxLen))
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return Password{}, invalid("password", "управляющие символы запрещены")
	}
	return Password{value: s}, nil
}

// #endregion password-policy

// Plaintext — явный доступ к паролю: только для хеширования.
func (p Password) Plaintext() string { return p.value }

// String и GoString скрывают пароль: случайный fmt.Print или лог не раскроют его.
func (Password) String() string   { return "[пароль скрыт]" }
func (Password) GoString() string { return "domain.Password{[скрыт]}" }

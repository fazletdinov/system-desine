package cursor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// #region codec
const (
	// version — версия формата: курсор живёт у клиента дольше, чем версия сервера,
	// и старый курсор после смены формата должен дать понятную ошибку, а не мусор.
	version = 1
	// MaxLen ограничивает разбор: курсор приходит от клиента, и строка в мегабайт
	// не должна доходить до base64 и JSON.
	MaxLen = 512
)

var ErrInvalid = errors.New("некорректный курсор")

type envelope struct {
	V int             `json:"v"`
	K string          `json:"k"`
	P json.RawMessage `json:"p"`
}

// Encode упаковывает payload вида kind ("messages", "chats"): курсор одного списка
// не примется другим.
func Encode(kind string, payload any) (string, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("cursor: %w", err)
	}
	b, err := json.Marshal(envelope{V: version, K: kind, P: p})
	if err != nil {
		return "", fmt.Errorf("cursor: %w", err)
	}
	// base64url без '=': курсор кладут в строку запроса, и экранировать там нечего.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Decode — обратное Encode. Любая неудача — ErrInvalid (с причиной для журнала):
// клиенту хватает «курсор не годится, начните сначала».
func Decode(s, kind string, payload any) error {
	if s == "" || len(s) > MaxLen {
		return fmt.Errorf("%w: длина %d", ErrInvalid, len(s))
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("%w: base64: %w", ErrInvalid, err)
	}
	var env envelope
	if err := strictUnmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: конверт: %w", ErrInvalid, err)
	}
	if env.V != version || env.K != kind {
		return fmt.Errorf("%w: версия %d, вид %q", ErrInvalid, env.V, env.K)
	}
	if err := strictUnmarshal(env.P, payload); err != nil {
		return fmt.Errorf("%w: данные: %w", ErrInvalid, err)
	}
	return nil
}

// strictUnmarshal не прощает ни лишних полей, ни хвоста после значения: курсор пишет
// только наш Encode, и всё, чего он не пишет, — признак подделки или порчи.
func strictUnmarshal(b []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("лишние данные после JSON")
	}
	return nil
}

// #endregion codec

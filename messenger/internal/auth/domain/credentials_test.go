package domain_test

import (
	"errors"
	"testing"

	"messenger/internal/auth/domain"
)

const aliceID = "0192f0c1-7e3a-7b4c-8d5e-6f7a8b9c0d1e"

func TestParseUserID(t *testing.T) {
	t.Parallel()
	id, err := domain.ParseUserID(aliceID)
	if err != nil || id.String() != aliceID || id.IsZero() {
		t.Fatalf("ParseUserID(%q) = %q, %v", aliceID, id, err)
	}
	_, err = domain.ParseUserID("alice")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "user_id" {
		t.Fatalf("ждали ValidationError по user_id, получили %v", err)
	}
}

func TestNewPasswordHash(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewPasswordHash(""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("пустой хеш: ждали ErrInvalid, получили %v", err)
	}
	h, err := domain.NewPasswordHash("$argon2id$…")
	if err != nil || h.String() != "$argon2id$…" || h.IsZero() {
		t.Fatalf("NewPasswordHash = %q, %v", h, err)
	}
}

func TestNewCredentials(t *testing.T) {
	t.Parallel()
	id, err := domain.ParseUserID(aliceID)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := domain.NewPasswordHash("h")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		id    domain.UserID
		hash  domain.PasswordHash
		field string
	}{
		{name: "нулевой id", hash: hash, field: "user_id"},
		{name: "нулевой хеш", id: id, field: "password_hash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewCredentials(tt.id, tt.hash)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("ждали ValidationError по %s, получили %v", tt.field, err)
			}
		})
	}
	c, err := domain.NewCredentials(id, hash)
	if err != nil || c.UserID() != id || c.PasswordHash() != hash {
		t.Fatalf("NewCredentials = %+v, %v", c, err)
	}
}

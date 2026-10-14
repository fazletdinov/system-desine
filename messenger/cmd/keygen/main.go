// Command keygen создаёт закрытый ключ подписи JWT: Ed25519 в PKCS#8 PEM, права 0600.
// Существующий файл не перезаписывает: новый ключ сделал бы недействительными все
// выданные токены — такое действие должно быть осознанным (удалите файл сами).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"

	"messenger/internal/auth/adapters/jwt"
)

func main() {
	out := flag.String("out", "", "куда записать закрытый ключ, например jwt.pem")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	if out == "" {
		return errors.New("укажите -out")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("генерация ключа: %w", err)
	}
	pemBytes, err := jwt.EncodePrivateKey(priv)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("файл ключа: %w", err)
	}
	if _, err := f.Write(pemBytes); err != nil {
		_ = f.Close()
		return fmt.Errorf("запись ключа: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("запись ключа: %w", err)
	}
	fmt.Printf("ключ записан в %s, kid %s\n", out, jwt.KeyID(pub))
	return nil
}

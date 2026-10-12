// Package certs — собственный удостоверяющий центр (CA) для лабораторных:
// выпускает корневой сертификат и подписанные им сертификаты серверов.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// clockSkew — запас NotBefore назад: часы клиента могут отставать от часов сервера,
// и только что выпущенный сертификат иначе оказался бы «ещё не действительным».
const clockSkew = time.Minute

// CA — корневой сертификат и его закрытый ключ. Ключ не покидает процесс.
type CA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

// NewCA создаёт самоподписанный корневой сертификат с именем cn, действительный ttl.
func NewCA(cn string, ttl time.Duration) (*CA, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("create CA: ttl должен быть больше нуля, получено %v", ttl)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("create CA: generate key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("create CA: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             now.Add(-clockSkew),
		NotAfter:              now.Add(ttl),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLenZero:        true, // CA подписывает только конечные сертификаты, не промежуточные CA
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA: sign: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("create CA: parse: %w", err)
	}
	return &CA{
		cert: cert,
		key:  key,
		pem:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}, nil
}

// Issue выпускает сертификат сервера для имён names (SAN DNSNames), действительный ttl.
// Клиент проверяет имя хоста именно по SAN; Subject.CommonName для этого не используется.
func (ca *CA) Issue(names []string, ttl time.Duration) (tls.Certificate, error) {
	if len(names) == 0 {
		return tls.Certificate{}, errors.New("issue certificate: нужно хотя бы одно имя")
	}
	if ttl <= 0 {
		return tls.Certificate{}, fmt.Errorf("issue certificate: ttl должен быть больше нуля, получено %v", ttl)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("issue certificate: generate key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("issue certificate: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: names[0]},
		DNSNames:     names,
		NotBefore:    now.Add(-clockSkew),
		NotAfter:     now.Add(ttl),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("issue certificate: sign: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("issue certificate: parse: %w", err)
	}
	// Цепочка «лист, затем CA»: клиент, которому CA известен, получит полный путь
	// и сможет показать его, например, в tlsdemo get.
	return tls.Certificate{
		Certificate: [][]byte{der, ca.cert.Raw},
		PrivateKey:  key,
		Leaf:        leaf,
	}, nil
}

// Pool возвращает пул доверенных корней, содержащий только этот CA.
func (ca *CA) Pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	return pool
}

// PEM возвращает корневой сертификат в PEM — то, что раздают клиентам (ca.pem).
func (ca *CA) PEM() []byte {
	return append([]byte(nil), ca.pem...) // копия: вызывающий не испортит наш буфер
}

// randomSerial — случайный серийный номер в [1, 2^128−1], то есть не длиннее 128 бит.
// RFC 5280 требует положительное число не длиннее 20 байт; случайность делает номера
// уникальными без счётчика.
func randomSerial() (*big.Int, error) {
	maxSerial := new(big.Int).Lsh(big.NewInt(1), 128)
	maxSerial.Sub(maxSerial, big.NewInt(1)) // rand.Int даёт [0, max): после +1 — [1, 2^128−1]
	n, err := rand.Int(rand.Reader, maxSerial)
	if err != nil {
		return nil, fmt.Errorf("serial number: %w", err)
	}
	return n.Add(n, big.NewInt(1)), nil // ноль запрещён
}

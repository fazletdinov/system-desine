package certs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestIssuedCertificateVerifiesForItsNameOnly(t *testing.T) {
	ca, err := NewCA("lab-ca", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ca.Issue([]string{"api.messenger.example"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		dnsName string
		wantErr bool
	}{
		{"своё имя проходит", "api.messenger.example", false},
		{"чужое имя не проходит", "evil.example", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := leaf.Verify(x509.VerifyOptions{Roots: ca.Pool(), DNSName: tt.dnsName})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Verify err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnknownCAIsRejected(t *testing.T) {
	ca := mustCA(t, "lab-ca")
	other := mustCA(t, "other-ca")
	cert := mustIssue(t, ca, "api.messenger.example")
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: other.Pool(), DNSName: "api.messenger.example"}); err == nil {
		t.Fatal("certificate from unknown CA must be rejected")
	}
}

func TestTLSHandshakeWithIssuedCert(t *testing.T) {
	ca := mustCA(t, "lab-ca")
	cert := mustIssue(t, ca, "api.messenger.example")
	srv := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	cli := &tls.Config{RootCAs: ca.Pool(), ServerName: "api.messenger.example", MinVersion: tls.VersionTLS13}
	if err := handshakeOverPipe(srv, cli); err != nil {
		t.Fatal(err)
	}
}

func TestTLSHandshakeFailsForWrongName(t *testing.T) {
	ca := mustCA(t, "lab-ca")
	cert := mustIssue(t, ca, "api.messenger.example")
	srv := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	cli := &tls.Config{RootCAs: ca.Pool(), ServerName: "evil.example", MinVersion: tls.VersionTLS13}
	err := handshakeOverPipe(srv, cli)
	var hostErr x509.HostnameError
	if !errors.As(err, &hostErr) {
		t.Fatalf("ожидалась x509.HostnameError, получено %v", err)
	}
}

func TestPEMRoundTripsToTheSamePool(t *testing.T) {
	ca := mustCA(t, "lab-ca")
	block, rest := pem.Decode(ca.PEM())
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		t.Fatalf("PEM() должен содержать ровно один блок CERTIFICATE, got %v", block)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca.PEM()) {
		t.Fatal("PEM() не разбирается AppendCertsFromPEM")
	}
	leaf, err := x509.ParseCertificate(mustIssue(t, ca, "server").Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "server"}); err != nil {
		t.Fatalf("сертификат не проверяется пулом из PEM(): %v", err)
	}
}

func TestInvalidArgumentsAreRejected(t *testing.T) {
	if _, err := NewCA("lab-ca", 0); err == nil {
		t.Error("NewCA с нулевым ttl должен вернуть ошибку")
	}
	ca := mustCA(t, "lab-ca")
	if _, err := ca.Issue(nil, time.Hour); err == nil {
		t.Error("Issue без имён должен вернуть ошибку")
	}
	if _, err := ca.Issue([]string{"server"}, -time.Second); err == nil {
		t.Error("Issue с отрицательным ttl должен вернуть ошибку")
	}
}

func TestSerialIsPositiveAndFitsIn128Bits(t *testing.T) {
	for range 1000 {
		n, err := randomSerial()
		if err != nil {
			t.Fatal(err)
		}
		if n.Sign() <= 0 || n.BitLen() > 128 {
			t.Fatalf("серийный номер %v: знак %d, бит %d", n, n.Sign(), n.BitLen())
		}
	}
}

func mustCA(t *testing.T, cn string) *CA {
	t.Helper()
	ca, err := NewCA(cn, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return ca
}

func mustIssue(t *testing.T, ca *CA, name string) tls.Certificate {
	t.Helper()
	cert, err := ca.Issue([]string{name}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// handshakeOverPipe проводит TLS-рукопожатие в памяти. Обе стороны идут в своих горутинах:
// net.Pipe синхронный, и в одной горутине стороны ждали бы друг друга вечно.
func handshakeOverPipe(srvCfg, cliCfg *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sc, cc := net.Pipe()
	srv, cli := tls.Server(sc, srvCfg), tls.Client(cc, cliCfg)

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, side := range []struct {
		name string
		conn *tls.Conn
		raw  net.Conn
	}{{"server", srv, sc}, {"client", cli, cc}} {
		wg.Go(func() {
			if err := side.conn.HandshakeContext(ctx); err != nil {
				errs <- fmt.Errorf("%s handshake: %w", side.name, err)
				// Провал одной стороны должен разбудить другую, иначе она ждёт до таймаута.
				_ = side.raw.Close()
			}
		})
	}
	wg.Wait()
	close(errs)
	// Закрываем сами концы pipe, а не tls.Conn: tls.Conn.Close шлёт close_notify, а читать
	// его некому — синхронный pipe продержал бы Close до внутреннего дедлайна в 5 с.
	_ = sc.Close() // рукопожатие проверено, ошибки закрытия pipe тест не интересуют
	_ = cc.Close()

	var err error
	for e := range errs {
		err = errors.Join(err, e)
	}
	return err
}

package tlscfg

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExistingCertificateValid(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	notAfter := time.Now().Add(24 * time.Hour)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{CommonName: "trojan.example.com"},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatalf("WriteFile cert returned error: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey returned error: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("WriteFile key returned error: %v", err)
	}

	valid, gotNotAfter, err := ExistingCertificateValid(certPath, keyPath, time.Now())
	if err != nil {
		t.Fatalf("ExistingCertificateValid returned error: %v", err)
	}
	if !valid {
		t.Fatal("ExistingCertificateValid returned false for a valid certificate")
	}
	if gotNotAfter.IsZero() {
		t.Fatal("ExistingCertificateValid returned a zero NotAfter")
	}
}

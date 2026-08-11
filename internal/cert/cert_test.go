package cert

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureFilesAndLoad(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	if err := EnsureFiles(certFile, keyFile); err != nil {
		t.Fatalf("EnsureFiles: %v", err)
	}
	if _, err := os.Stat(certFile); err != nil {
		t.Fatalf("cert file: %v", err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Fatalf("key file: %v", err)
	}

	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}
	if len(pair.Certificate) == 0 {
		t.Fatal("expected certificate chain")
	}

	infoBefore, err := os.Stat(certFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureFiles(certFile, keyFile); err != nil {
		t.Fatalf("EnsureFiles second: %v", err)
	}
	infoAfter, err := os.Stat(certFile)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Fatal("expected existing cert file to be kept")
	}
}

package main

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalCertificatePersistsCAAndBindsIP(t *testing.T) {
	dir := t.TempDir()
	first, caPEM, err := localCertificate(dir, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	second, sameCA, err := localCertificate(dir, "192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	if string(caPEM) != string(sameCA) {
		t.Fatal("CA changed between runs")
	}
	for _, tc := range []struct {
		certIP string
		cert   []byte
	}{{"127.0.0.1", first.Certificate[0]}, {"192.168.1.20", second.Certificate[0]}} {
		leaf, err := x509.ParseCertificate(tc.cert)
		if err != nil {
			t.Fatal(err)
		}
		if len(leaf.IPAddresses) != 1 || !leaf.IPAddresses[0].Equal(net.ParseIP(tc.certIP)) {
			t.Fatalf("SAN: %v", leaf.IPAddresses)
		}
	}
	block, _ := pem.Decode(caPEM)
	if block == nil {
		t.Fatal("CA PEM missing")
	}
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !ca.IsCA {
		t.Fatalf("CA: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "ca-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("CA key permissions %v", info.Mode().Perm())
	}
}

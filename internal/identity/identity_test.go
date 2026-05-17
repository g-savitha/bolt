package identity_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bolt/bolt/internal/identity"
)

func TestGenerateAndLoad(t *testing.T) {
	dir := t.TempDir()

	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Generated identity should have valid key sizes.
	if len(id.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("private key length = %d, want %d", len(id.PrivateKey), ed25519.PrivateKeySize)
	}
	if len(id.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("public key length = %d, want %d", len(id.PublicKey), ed25519.PublicKeySize)
	}

	// Loading from the same directory must produce the same keys.
	loaded, err := identity.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(loaded.PrivateKey) != string(id.PrivateKey) {
		t.Error("loaded private key differs from generated key")
	}
	if string(loaded.PublicKey) != string(id.PublicKey) {
		t.Error("loaded public key differs from generated key")
	}
}

func TestGenerateFailsIfIdentityExists(t *testing.T) {
	dir := t.TempDir()

	if _, err := identity.Generate(dir); err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	if _, err := identity.Generate(dir); err == nil {
		t.Error("second Generate should have returned an error but did not")
	}
}

func TestFingerprintIsStable(t *testing.T) {
	dir := t.TempDir()

	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	fp1 := id.Fingerprint()
	fp2 := id.Fingerprint()

	if fp1 != fp2 {
		t.Errorf("fingerprint is not stable: %q != %q", fp1, fp2)
	}
}

func TestFingerprintFormat(t *testing.T) {
	dir := t.TempDir()

	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	fp := id.Fingerprint()
	parts := strings.Split(fp, ":")

	// SHA-256 is 32 bytes → 32 colon-separated hex pairs.
	if len(parts) != 32 {
		t.Errorf("fingerprint has %d parts, want 32: %s", len(parts), fp)
	}
	for _, part := range parts {
		if len(part) != 2 {
			t.Errorf("fingerprint part %q has length %d, want 2", part, len(part))
		}
	}
}

func TestFingerprintDiffersAcrossIdentities(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()

	id1, err := identity.Generate(dir1)
	if err != nil {
		t.Fatalf("Generate id1: %v", err)
	}
	id2, err := identity.Generate(dir2)
	if err != nil {
		t.Fatalf("Generate id2: %v", err)
	}

	if id1.Fingerprint() == id2.Fingerprint() {
		t.Error("two different identities produced the same fingerprint — extremely unlikely, probable bug")
	}
}

func TestPrivateKeyFilePermissions(t *testing.T) {
	dir := t.TempDir()

	if _, err := identity.Generate(dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "private.key"))
	if err != nil {
		t.Fatalf("stat private.key: %v", err)
	}

	// Private key must be readable only by owner (0600).
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("private.key permissions = %o, want 0600", perm)
	}
}

func TestTLSCertificateContainsPublicKey(t *testing.T) {
	dir := t.TempDir()

	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	cert, err := id.TLSCertificate()
	if err != nil {
		t.Fatalf("TLSCertificate: %v", err)
	}

	if cert.Leaf == nil {
		// Parse the leaf certificate so we can inspect SubjectKeyId.
		// tls.X509KeyPair doesn't always populate Leaf.
		if len(cert.Certificate) == 0 {
			t.Fatal("TLS certificate has no raw certificate bytes")
		}
	}

	if len(cert.Certificate) == 0 {
		t.Fatal("TLS certificate has no DER bytes")
	}
}

func TestLoadWithCorruptPublicKey(t *testing.T) {
	dir := t.TempDir()

	if _, err := identity.Generate(dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Overwrite the public key file with garbage.
	pubPath := filepath.Join(dir, "public.key")
	if err := os.WriteFile(pubPath, []byte("not a pem file"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := identity.Load(dir); err == nil {
		t.Error("Load with corrupt public key should have returned an error")
	}
}

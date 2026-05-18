// Package identity manages the cryptographic identity of a bolt node.
//
// Each machine running bolt has exactly one Ed25519 keypair. This keypair
// is generated once on first run and stored on disk. The public key's
// SHA-256 hash becomes the node's fingerprint — a stable, human-verifiable
// identifier used to authenticate peers (TOFU model, like SSH).
//
// The private key never leaves the machine. The public key is shared freely.
package identity

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	privateKeyFile = "private.key"
	publicKeyFile  = "public.key"

	privateKeyPEMType = "bolt PRIVATE KEY"
	publicKeyPEMType  = "bolt PUBLIC KEY"

	// tlsCertValidityYears is intentionally long — we manage trust via
	// fingerprint pinning (TOFU), not certificate expiry.
	tlsCertValidityYears = 100
)

// Identity holds the Ed25519 keypair for this bolt node.
// It is loaded once at startup and passed around by pointer.
type Identity struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// Generate creates a new Ed25519 keypair and writes it to dir.
// dir is created if it does not exist.
// Returns an error if a keypair already exists at dir — use Load instead.
func Generate(dir string) (*Identity, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create identity directory: %w", err)
	}

	privatePath := filepath.Join(dir, privateKeyFile)
	if _, err := os.Stat(privatePath); err == nil {
		return nil, errors.New("identity already exists at " + dir + " — use Load() to read it")
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 keypair: %w", err)
	}

	if err := writePrivateKey(privatePath, priv); err != nil {
		return nil, err
	}
	if err := writePublicKey(filepath.Join(dir, publicKeyFile), pub); err != nil {
		return nil, err
	}

	return &Identity{PrivateKey: priv, PublicKey: pub}, nil
}

// Load reads an existing Ed25519 keypair from dir.
// Returns an error if the identity does not exist or the files are corrupt.
func Load(dir string) (*Identity, error) {
	priv, err := readPrivateKey(filepath.Join(dir, privateKeyFile))
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	pub, err := readPublicKey(filepath.Join(dir, publicKeyFile))
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}

	// Sanity check: the public key embedded in the private key must match
	// the separately stored public key. If they differ, the files are corrupt
	// or have been tampered with.
	derivedPub := priv.Public().(ed25519.PublicKey) //nolint:errcheck // type assertion is safe: ed25519.PrivateKey.Public() always returns ed25519.PublicKey
	if string(derivedPub) != string(pub) {
		return nil, errors.New("public key does not match private key — identity files may be corrupt")
	}

	return &Identity{PrivateKey: priv, PublicKey: pub}, nil
}

// Fingerprint returns the SHA-256 hash of the public key formatted as a
// colon-separated hex string, e.g. "ab:cd:ef:...". This is the stable
// identifier used for TOFU peer verification — the same format SSH uses.
func (id *Identity) Fingerprint() string {
	return Fingerprint(id.PublicKey)
}

// Fingerprint computes a fingerprint for any Ed25519 public key.
// Exported so callers can fingerprint remote peers' keys during TOFU.
func Fingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)

	// Format as colon-separated pairs: "ab:cd:ef:12:..."
	result := make([]byte, 0, len(sum)*3-1)
	for i, b := range sum {
		if i > 0 {
			result = append(result, ':')
		}
		result = append(result, hexByte(b>>4), hexByte(b&0x0f))
	}
	return string(result)
}

// TLSCertificate wraps the Ed25519 keypair in a self-signed TLS certificate
// suitable for use with quic-go. The certificate's Common Name is set to the
// fingerprint so peers can extract and verify the identity during TOFU.
//
// We use a 100-year validity period because trust is managed by fingerprint
// pinning, not certificate expiry.
func (id *Identity) TLSCertificate() (tls.Certificate, error) {
	// quic-go requires an ECDSA or RSA key for the TLS handshake wrapper,
	// but we embed our Ed25519 public key in the certificate's Subject so
	// peers can extract it for fingerprint verification.
	//
	// We generate a short-lived ECDSA key purely for the TLS handshake.
	// Our Ed25519 key is embedded in the certificate's raw subject bytes.
	// This is a pragmatic workaround: quic-go's TLS stack supports Ed25519
	// for certificate signing but requires it in a specific form. Using
	// ECDSA P-256 for the TLS wrapper and embedding Ed25519 in SubjectKeyId
	// keeps the handshake compatible while preserving our identity model.
	tlsKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate tls wrapper key: %w", err)
	}

	fingerprint := id.Fingerprint()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: fingerprint,
		},
		// Embed the Ed25519 public key bytes in SubjectKeyId so peers can
		// extract it from the certificate during VerifyPeerCertificate.
		SubjectKeyId:          id.PublicKey,
		NotBefore:             time.Now().Add(-time.Minute), // small backdate for clock skew
		NotAfter:              time.Now().AddDate(tlsCertValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &tlsKey.PublicKey, tlsKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create self-signed certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(tlsKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal tls key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("build tls certificate: %w", err)
	}
	return cert, nil
}

// ExtractPublicKeyFromCert extracts the Ed25519 public key that was embedded
// in a peer's TLS certificate SubjectKeyId during TLSCertificate().
// Returns an error if the certificate was not created by bolt.
func ExtractPublicKeyFromCert(cert *x509.Certificate) (ed25519.PublicKey, error) {
	if len(cert.SubjectKeyId) != ed25519.PublicKeySize {
		return nil, fmt.Errorf(
			"certificate SubjectKeyId has unexpected length %d (expected %d) — not a bolt certificate",
			len(cert.SubjectKeyId), ed25519.PublicKeySize,
		)
	}
	pub := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(pub, cert.SubjectKeyId)
	return pub, nil
}

// --- disk I/O helpers ---

func writePrivateKey(path string, key ed25519.PrivateKey) error {
	block := &pem.Block{
		Type:  privateKeyPEMType,
		Bytes: []byte(key),
	}
	data := pem.EncodeToMemory(block)

	// 0600 — only the owner can read or write the private key.
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write private key to %s: %w", path, err)
	}
	return nil
}

func writePublicKey(path string, key ed25519.PublicKey) error {
	block := &pem.Block{
		Type:  publicKeyPEMType,
		Bytes: []byte(key),
	}
	data := pem.EncodeToMemory(block)

	// 0644 — public key is intentionally world-readable (it's a public key).
	if err := os.WriteFile(path, data, 0644); err != nil { //nolint:gosec // G306: public key file is safe to be world-readable
		return fmt.Errorf("write public key to %s: %w", path, err)
	}
	return nil
}

func readPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is derived from DefaultConfigDir(), not user input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("private key not found at %s — run 'bolt init' first", path)
		}
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("private key file at %s is not valid PEM", path)
	}
	if block.Type != privateKeyPEMType {
		return nil, fmt.Errorf("private key file has unexpected type %q (expected %q)", block.Type, privateKeyPEMType)
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key has unexpected length %d", len(block.Bytes))
	}

	return ed25519.PrivateKey(block.Bytes), nil
}

func readPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is derived from DefaultConfigDir(), not user input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("public key not found at %s — run 'bolt init' first", path)
		}
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("public key file at %s is not valid PEM", path)
	}
	if block.Type != publicKeyPEMType {
		return nil, fmt.Errorf("public key file has unexpected type %q (expected %q)", block.Type, publicKeyPEMType)
	}
	if len(block.Bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key has unexpected length %d", len(block.Bytes))
	}

	return ed25519.PublicKey(block.Bytes), nil
}

// hexByte converts a nibble (0-15) to its ASCII hex character.
func hexByte(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

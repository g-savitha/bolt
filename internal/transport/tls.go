// Package transport handles the QUIC network layer for flick.
//
// Connections between flick nodes are secured with QUIC over UDP, which
// provides TLS 1.3 encryption built-in. We layer our own TOFU identity
// verification on top of TLS rather than relying on certificate authorities.
//
// The TLS handshake uses self-signed certificates generated from each node's
// Ed25519 keypair. After TLS completes, both sides perform a flick-level
// handshake to verify the embedded Ed25519 identity (see peer_conn.go).
package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/bolt/bolt/internal/identity"
)

// flickALPN is the Application-Layer Protocol Negotiation token for flick.
// quic-go requires at least one ALPN token. Using our own makes it clear
// this is a flick connection and not another QUIC-based protocol.
const boltALPN = "bolt/1"

// ServerTLSConfig returns a TLS configuration for the QUIC listener.
// It presents our self-signed certificate to connecting peers.
// Certificate authority verification is disabled — we verify identity
// via fingerprint pinning in VerifyPeerCertificate instead.
func ServerTLSConfig(id *identity.Identity) (*tls.Config, error) {
	cert, err := id.TLSCertificate()
	if err != nil {
		return nil, fmt.Errorf("build server tls certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		// Require clients to present a certificate so we can inspect their
		// identity. We do not verify against a CA — we verify via fingerprint.
		ClientAuth: tls.RequireAnyClientCert,
		NextProtos: []string{boltALPN},
		MinVersion: tls.VersionTLS13,
	}, nil
}

// ClientTLSConfig returns a TLS configuration for outgoing QUIC connections.
// It presents our certificate and uses a custom verifier for TOFU checking.
//
// onUnknownPeer is called when we connect to a peer whose fingerprint we have
// not seen before. It should present the fingerprint to the user and return
// true if accepted, false if rejected.
func ClientTLSConfig(id *identity.Identity, onUnknownPeer PeerVerifier) (*tls.Config, error) {
	cert, err := id.TLSCertificate()
	if err != nil {
		return nil, fmt.Errorf("build client tls certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		// We skip standard CA verification and do our own fingerprint-based check.
		InsecureSkipVerify: true, //nolint:gosec // intentional: we verify via fingerprint below
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return verifyPeerCertificate(rawCerts, onUnknownPeer)
		},
		NextProtos: []string{boltALPN},
		MinVersion: tls.VersionTLS13,
	}, nil
}

// PeerVerifier is called during TLS handshake when we connect to a peer.
// It receives the peer's fingerprint and their nickname (from the TLS cert's
// CommonName). Return true to accept the connection, false to reject it.
//
// For known peers, this function is typically a fast lookup in PeerStore.
// For unknown peers, it should prompt the user to verify out-of-band.
type PeerVerifier func(fingerprint string, nickname string) (accepted bool, err error)

// verifyPeerCertificate is the core TOFU verification logic, called by the
// TLS stack during handshake. It extracts the peer's Ed25519 public key from
// their certificate and delegates the accept/reject decision to onUnknownPeer.
func verifyPeerCertificate(rawCerts [][]byte, verify PeerVerifier) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("peer presented no TLS certificate — cannot verify identity")
	}

	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("parse peer certificate: %w", err)
	}

	pub, err := identity.ExtractPublicKeyFromCert(cert)
	if err != nil {
		return fmt.Errorf("extract identity from peer certificate: %w", err)
	}

	fingerprint := identity.Fingerprint(pub)
	nickname := cert.Subject.CommonName

	accepted, err := verify(fingerprint, nickname)
	if err != nil {
		return fmt.Errorf("peer verification: %w", err)
	}
	if !accepted {
		return fmt.Errorf("peer %s (%s) was not accepted", nickname, fingerprint[:17]+"...")
	}
	return nil
}

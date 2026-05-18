package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/bolt/bolt/internal/config"
	"github.com/bolt/bolt/internal/identity"
	"github.com/bolt/bolt/internal/transport"
)

// ConnectPeer dials a remote peer by IP or host:port and adds it to the registry.
func (d *Daemon) ConnectPeer(ctx context.Context, address string) error {
	addr, err := normalizePeerAddress(address, d.cfg.Port)
	if err != nil {
		return err
	}

	tlsConf, err := transport.ClientTLSConfig(d.id, d.peerVerifier())
	if err != nil {
		return fmt.Errorf("build tls config: %w", err)
	}

	conn, err := transport.Dial(ctx, addr, tlsConf)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	tlsState := conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		_ = conn.CloseWithError(1, "no peer certificate")
		return fmt.Errorf("peer at %s presented no TLS certificate", addr)
	}

	peerPub, err := identity.ExtractPublicKeyFromCert(tlsState.PeerCertificates[0])
	if err != nil {
		_ = conn.CloseWithError(1, "bad certificate")
		return fmt.Errorf("read peer identity: %w", err)
	}

	fingerprint := identity.Fingerprint(peerPub)
	pc := transport.NewPeerConn(conn, fingerprint)

	if err := pc.Handshake(ctx, d.localPeer()); err != nil {
		_ = conn.CloseWithError(1, "handshake failed")
		return fmt.Errorf("handshake with %s: %w", addr, err)
	}

	d.savePeerRecord(pc, addr, true)
	go d.servePeer(ctx, pc)
	return nil
}

func normalizePeerAddress(host string, defaultPort int) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("address is empty")
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host, nil
	}
	return fmt.Sprintf("%s:%d", host, defaultPort), nil
}

func (d *Daemon) localPeer() transport.LocalPeer {
	return transport.LocalPeer{
		Identity: d.id,
		Nickname: d.cfg.Nickname,
	}
}

func (d *Daemon) peerVerifier() transport.PeerVerifier {
	return func(fingerprint, _ string) (bool, error) {
		existing := d.peers.Get(fingerprint)
		if existing != nil && existing.Trust == config.TrustBlock {
			return false, nil
		}
		return true, nil
	}
}

func (d *Daemon) savePeerRecord(pc *transport.PeerConn, addr string, manual bool) {
	fingerprint := pc.PeerFingerprint()
	existing := d.peers.Get(fingerprint)
	record := config.PeerRecord{
		Fingerprint:   fingerprint,
		Nickname:      pc.PeerNickname(),
		LastKnownIP:   addr,
		Trust:         config.TrustAllowOnce,
		ManuallyAdded: manual,
	}
	if existing != nil {
		record.Trust = existing.Trust
		record.FirstSeen = existing.FirstSeen
		record.ManuallyAdded = existing.ManuallyAdded || manual
		if addr == "" {
			record.LastKnownIP = existing.LastKnownIP
		}
	}
	if err := d.peers.Upsert(record); err != nil {
		fmt.Fprintf(os.Stderr, "warning: save peer %s: %v\n", pc.PeerNickname(), err)
	}
}

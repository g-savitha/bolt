package transport

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/bolt/bolt/internal/identity"
	"github.com/bolt/bolt/internal/proto"
	"github.com/quic-go/quic-go"
)

// BoltVersion is the current application version, sent during handshake.
// Peers use this for display only — not for protocol compatibility gating.
const BoltVersion = "0.1.0"

// PeerConn represents an authenticated, live QUIC connection to a remote peer.
//
// Every feature in flick (chat, file transfer, control messages) runs over
// streams opened on this connection. PeerConn owns the connection lifecycle —
// when it is closed, all streams on it are also closed.
//
// PeerConn is safe to use from multiple goroutines.
type PeerConn struct {
	conn *quic.Conn

	// These fields are set once during Handshake() and then read-only.
	peerFingerprint string
	peerNickname    string

	// chatStream is the single persistent stream for chat messages.
	// Opened lazily on first chat send.
	chatStreamMu sync.Mutex
	chatStream   *quic.Stream
}

// Handshake performs the bolt application-level identity verification after
// the TLS handshake has already completed inside QUIC.
//
// After Handshake returns without error, PeerConn is ready to use.
func (pc *PeerConn) Handshake(ctx context.Context, local LocalPeer) error {
	// Both sides open a handshake stream simultaneously. We open ours and
	// accept theirs concurrently to avoid deadlock.
	type handshakeResult struct {
		msg proto.HandshakeMsg
		err error
	}

	receivedCh := make(chan handshakeResult, 1)

	// Goroutine: accept and read the peer's handshake stream.
	go func() {
		stream, err := pc.conn.AcceptStream(ctx)
		if err != nil {
			receivedCh <- handshakeResult{err: fmt.Errorf("accept handshake stream: %w", err)}
			return
		}
		defer stream.Close()

		streamType, err := proto.ReadStreamType(stream)
		if err != nil {
			receivedCh <- handshakeResult{err: fmt.Errorf("read stream type: %w", err)}
			return
		}
		if streamType != proto.StreamHandshake {
			receivedCh <- handshakeResult{err: fmt.Errorf("expected handshake stream (0x%02x), got 0x%02x", proto.StreamHandshake, streamType)}
			return
		}

		var msg proto.HandshakeMsg
		if err := proto.ReadFrame(stream, &msg); err != nil {
			receivedCh <- handshakeResult{err: fmt.Errorf("read handshake message: %w", err)}
			return
		}
		receivedCh <- handshakeResult{msg: msg}
	}()

	if err := pc.sendHandshake(ctx, local); err != nil {
		return err
	}

	// Wait for the peer's handshake.
	result := <-receivedCh
	if result.err != nil {
		return result.err
	}

	return pc.validateHandshake(result.msg)
}

func (pc *PeerConn) sendHandshake(ctx context.Context, local LocalPeer) error {
	stream, err := pc.conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("open handshake stream: %w", err)
	}
	defer stream.Close()

	if err := proto.WriteStreamType(stream, proto.StreamHandshake); err != nil {
		return err
	}

	msg := proto.HandshakeMsg{
		V:            proto.WireVersion,
		PublicKeyHex: hex.EncodeToString(local.Identity.PublicKey),
		Nickname:     local.Nickname,
		BoltVersion:  BoltVersion,
	}
	return proto.WriteFrame(stream, msg)
}

// validateHandshake checks that the peer's HandshakeMsg is internally
// consistent and matches the TLS certificate we already verified.
func (pc *PeerConn) validateHandshake(msg proto.HandshakeMsg) error {
	if msg.V != proto.WireVersion {
		return fmt.Errorf(
			"peer speaks protocol version %d, we speak version %d — update bolt to connect",
			msg.V, proto.WireVersion,
		)
	}

	pubKeyBytes, err := hex.DecodeString(msg.PublicKeyHex)
	if err != nil {
		return fmt.Errorf("peer sent invalid public key hex: %w", err)
	}

	handshakeFingerprint := identity.Fingerprint(pubKeyBytes)

	// The fingerprint from the handshake message must match the fingerprint
	// we extracted from the TLS certificate. If they differ, the TLS cert
	// and the claimed Ed25519 identity are from different keys — reject.
	if handshakeFingerprint != pc.peerFingerprint {
		return fmt.Errorf(
			"handshake identity mismatch: TLS cert has fingerprint %s but handshake claims %s — rejecting connection",
			pc.peerFingerprint[:17]+"...",
			handshakeFingerprint[:17]+"...",
		)
	}

	pc.peerNickname = msg.Nickname
	return nil
}

// PeerFingerprint returns the verified fingerprint of the remote peer.
// Only valid after Handshake() has completed successfully.
func (pc *PeerConn) PeerFingerprint() string {
	return pc.peerFingerprint
}

// PeerNickname returns the nickname the remote peer advertised.
// Only valid after Handshake() has completed successfully.
func (pc *PeerConn) PeerNickname() string {
	return pc.peerNickname
}

// OpenStream opens a new QUIC stream to the peer and writes the stream type byte.
// The caller is responsible for writing the appropriate message and closing the stream.
func (pc *PeerConn) OpenStream(ctx context.Context, streamType proto.StreamType) (*quic.Stream, error) {
	stream, err := pc.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("open stream to peer %s: %w", pc.peerNickname, err)
	}

	if err := proto.WriteStreamType(stream, streamType); err != nil {
		stream.Close()
		return nil, err
	}
	return stream, nil
}

// AcceptStream waits for the peer to open a new stream and reads the type byte.
// Used by the daemon's accept loop to route incoming streams to handlers.
func (pc *PeerConn) AcceptStream(ctx context.Context) (*quic.Stream, proto.StreamType, error) {
	stream, err := pc.conn.AcceptStream(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("accept stream from peer %s: %w", pc.peerNickname, err)
	}

	streamType, err := proto.ReadStreamType(stream)
	if err != nil {
		stream.Close()
		return nil, 0, fmt.Errorf("read stream type from peer %s: %w", pc.peerNickname, err)
	}
	return stream, streamType, nil
}

// Close shuts down the QUIC connection to this peer.
// All open streams on this connection will be terminated.
func (pc *PeerConn) Close() error {
	return pc.conn.CloseWithError(0, "connection closed")
}

// --- Registry ---

// PeerRegistry is the in-memory map of currently connected, authenticated peers.
// The daemon populates this as peers connect and removes entries on disconnect.
// All methods are safe to call from multiple goroutines.
type PeerRegistry struct {
	mu    sync.RWMutex
	conns map[string]*PeerConn // keyed by peer fingerprint
}

// NewPeerRegistry creates an empty registry.
func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		conns: make(map[string]*PeerConn),
	}
}

// Add registers a newly authenticated peer connection.
func (r *PeerRegistry) Add(pc *PeerConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[pc.peerFingerprint] = pc
}

// Remove removes a peer from the registry, called on disconnect.
func (r *PeerRegistry) Remove(fingerprint string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, fingerprint)
}

// Get returns the live connection for a peer, or nil if not connected.
func (r *PeerRegistry) Get(fingerprint string) *PeerConn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conns[fingerprint]
}

// GetByNickname returns a connected peer with an exact nickname match, or nil.
func (r *PeerRegistry) GetByNickname(nickname string) *PeerConn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, pc := range r.conns {
		if pc.PeerNickname() == nickname {
			return pc
		}
	}
	return nil
}

// ResolvePeer finds a connected peer by nickname or fingerprint prefix.
func (r *PeerRegistry) ResolvePeer(name string) (*PeerConn, error) {
	if pc := r.GetByNickname(name); pc != nil {
		return pc, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var matches []*PeerConn
	for fp, pc := range r.conns {
		if strings.HasPrefix(fp, name) || strings.HasPrefix(pc.PeerNickname(), name) {
			matches = append(matches, pc)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("peer %q is not connected — run 'bolt connect <ip>' first", name)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("peer name %q is ambiguous (%d matches)", name, len(matches))
	}
}

// Online returns a snapshot of all currently connected peers.
// The returned slice is safe to use without holding the registry lock.
func (r *PeerRegistry) Online() []*PeerConn {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*PeerConn, 0, len(r.conns))
	for _, pc := range r.conns {
		result = append(result, pc)
	}
	return result
}

// NewPeerConn wraps a QUIC connection as a PeerConn.
// peerFingerprint must be the fingerprint extracted during TLS verification —
// it is the ground truth identity before the application handshake runs.
func NewPeerConn(conn *quic.Conn, peerFingerprint string) *PeerConn {
	return &PeerConn{
		conn:            conn,
		peerFingerprint: peerFingerprint,
	}
}

// Package daemon manages the flick background process.
//
// The daemon is responsible for:
//   - Holding open QUIC connections to known peers
//   - Accepting incoming connections from new peers
//   - Running mDNS peer discovery (Phase 3)
//   - Serving the Unix socket IPC interface used by the CLI
//
// The daemon auto-spawns: the first flick CLI command that needs it will start
// the daemon as a detached subprocess. Subsequent commands connect to the
// already-running daemon via the Unix socket.
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/flick/flick/internal/config"
	"github.com/flick/flick/internal/identity"
	"github.com/flick/flick/internal/proto"
	"github.com/flick/flick/internal/transport"
	"github.com/quic-go/quic-go"
)

// Daemon holds all long-running state for a flick node.
// Create one with New() and start it with Run().
type Daemon struct {
	id        *identity.Identity
	cfg       *config.Config
	peers     *config.PeerStore
	registry  *transport.PeerRegistry
	ipcServer *IPCServer
	configDir string
}

// New creates a Daemon from an already-loaded identity and config.
func New(
	id *identity.Identity,
	cfg *config.Config,
	peers *config.PeerStore,
	configDir string,
) *Daemon {
	return &Daemon{
		id:        id,
		cfg:       cfg,
		peers:     peers,
		registry:  transport.NewPeerRegistry(),
		configDir: configDir,
	}
}

// Run starts the daemon and blocks until ctx is cancelled or a shutdown signal
// (SIGTERM, SIGINT) is received. It cleans up all resources before returning.
func (d *Daemon) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()

	// Start the Unix socket IPC server so the CLI can talk to us.
	socketPath := SocketPath(d.configDir)
	ipcServer, err := NewIPCServer(socketPath, d)
	if err != nil {
		return fmt.Errorf("start ipc server: %w", err)
	}
	d.ipcServer = ipcServer
	defer d.cleanupSocket(socketPath)

	// Start the QUIC listener for incoming peer connections.
	listenAddr := fmt.Sprintf(":%d", d.cfg.Port)
	if err := d.listenForPeers(ctx, listenAddr); err != nil {
		return fmt.Errorf("start peer listener: %w", err)
	}

	fmt.Fprintf(os.Stderr, "flick daemon running on port %d\n", d.cfg.Port)

	// Block until shutdown signal.
	<-ctx.Done()

	fmt.Fprintln(os.Stderr, "flick daemon shutting down...")
	d.shutdown()
	return nil
}

// listenForPeers starts the QUIC listener and accepts incoming connections
// in a background goroutine.
func (d *Daemon) listenForPeers(ctx context.Context, addr string) error {
	tlsConf, err := transport.ServerTLSConfig(d.id)
	if err != nil {
		return fmt.Errorf("build tls config: %w", err)
	}

	ln, err := transport.Listen(addr, tlsConf)
	if err != nil {
		return err
	}

	go d.acceptLoop(ctx, ln)
	return nil
}

// acceptLoop runs in a goroutine, accepting QUIC connections from remote peers
// and handing each off to handleIncomingPeer.
func (d *Daemon) acceptLoop(ctx context.Context, ln *quic.Listener) {
	defer ln.Close()

	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			// Context cancelled means we're shutting down — not an error.
			if ctx.Err() != nil {
				return
			}
			fmt.Fprintf(os.Stderr, "accept connection: %v\n", err)
			continue
		}

		go d.handleIncomingPeer(ctx, conn)
	}
}

// handleIncomingPeer authenticates a new incoming QUIC connection and, if
// accepted, adds it to the registry and begins serving its streams.
func (d *Daemon) handleIncomingPeer(ctx context.Context, conn *quic.Conn) {
	pc, err := d.authenticateIncoming(ctx, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "authentication failed for incoming connection: %v\n", err)
		conn.CloseWithError(1, "authentication failed")
		return
	}

	d.registry.Add(pc)
	defer d.registry.Remove(pc.PeerFingerprint())

	fmt.Fprintf(os.Stderr, "peer connected: %s (%s)\n", pc.PeerNickname(), pc.PeerFingerprint()[:17]+"...")

	// Serve streams from this peer until the connection closes.
	d.serveStreams(ctx, pc)

	fmt.Fprintf(os.Stderr, "peer disconnected: %s\n", pc.PeerNickname())
}

// authenticateIncoming performs TOFU verification on an incoming connection.
// It extracts the peer's fingerprint from the TLS certificate and either
// accepts (known or newly approved peer) or rejects (blocked peer) the connection.
func (d *Daemon) authenticateIncoming(ctx context.Context, conn *quic.Conn) (*transport.PeerConn, error) {
	// Extract the peer's fingerprint from their TLS certificate.
	tlsState := conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		return nil, fmt.Errorf("peer presented no TLS certificate")
	}

	peerPub, err := identity.ExtractPublicKeyFromCert(tlsState.PeerCertificates[0])
	if err != nil {
		return nil, fmt.Errorf("extract peer identity from certificate: %w", err)
	}

	fingerprint := identity.Fingerprint(peerPub)

	// Check if this peer is known and how much we trust them.
	existing := d.peers.Get(fingerprint)
	if existing != nil && existing.Trust == config.TrustBlock {
		return nil, fmt.Errorf("peer %s is blocked", fingerprint[:17]+"...")
	}

	pc := transport.NewPeerConn(conn, fingerprint)

	// Perform the application-level handshake to verify the claimed identity
	// matches the TLS certificate identity.
	if err := pc.Handshake(ctx, d.id); err != nil {
		return nil, fmt.Errorf("application handshake failed: %w", err)
	}

	// If this is the first time we've seen this peer, record them.
	if existing == nil {
		record := config.PeerRecord{
			Fingerprint: fingerprint,
			Nickname:    pc.PeerNickname(),
			Trust:       config.TrustAllowOnce,
		}
		if err := d.peers.Upsert(record); err != nil {
			// Log but don't fail — the connection itself is fine.
			fmt.Fprintf(os.Stderr, "warning: could not save new peer to disk: %v\n", err)
		}
	}

	return pc, nil
}

// serveStreams reads incoming streams from a peer and dispatches each to
// the appropriate handler based on its StreamType byte.
// Blocks until the peer disconnects or ctx is cancelled.
func (d *Daemon) serveStreams(ctx context.Context, pc *transport.PeerConn) {
	for {
		stream, streamType, err := pc.AcceptStream(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Connection closed by peer — normal exit.
			return
		}

		go d.routeStream(ctx, pc, stream, streamType)
	}
}

// routeStream dispatches a single accepted stream to the right handler.
// Each stream runs in its own goroutine so slow handlers don't block others.
func (d *Daemon) routeStream(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream, streamType proto.StreamType) {
	defer stream.Close()

	switch streamType {
	case proto.StreamChat:
		// Phase 4: chat handler goes here.
		fmt.Fprintf(os.Stderr, "received chat stream from %s (Phase 4 not yet implemented)\n", pc.PeerNickname())

	case proto.StreamFile:
		// Phase 2: file transfer handler goes here.
		fmt.Fprintf(os.Stderr, "received file stream from %s (Phase 2 not yet implemented)\n", pc.PeerNickname())

	case proto.StreamControl:
		// Phase 3+: control/heartbeat handler goes here.
		fmt.Fprintf(os.Stderr, "received control stream from %s\n", pc.PeerNickname())

	default:
		fmt.Fprintf(os.Stderr, "unknown stream type 0x%02x from peer %s — ignoring\n", streamType, pc.PeerNickname())
	}
}

// shutdown gracefully stops all active connections and flushes state.
func (d *Daemon) shutdown() {
	for _, pc := range d.registry.Online() {
		if err := pc.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "close peer connection: %v\n", err)
		}
	}
	if d.ipcServer != nil {
		d.ipcServer.Close()
	}
}

// cleanupSocket removes the Unix socket file on shutdown so the next daemon
// start doesn't fail trying to bind to an already-existing path.
func (d *Daemon) cleanupSocket(socketPath string) {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "remove socket file: %v\n", err)
	}
}

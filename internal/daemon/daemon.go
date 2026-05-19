// Package daemon manages the bolt background process.
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bolt/bolt/internal/chat"
	"github.com/bolt/bolt/internal/config"
	"github.com/bolt/bolt/internal/identity"
	"github.com/bolt/bolt/internal/proto"
	"github.com/bolt/bolt/internal/transport"
	"github.com/quic-go/quic-go"
)

// Daemon holds all long-running state for a bolt node.
type Daemon struct {
	id               *identity.Identity
	cfg              *config.Config
	peers            *config.PeerStore
	registry         *transport.PeerRegistry
	chat             *chat.Service
	ipcServer        *IPCServer
	configDir        string
	runCtx           context.Context
	streamHandlers   map[proto.StreamType]StreamHandler
	streamHandlersMu sync.RWMutex
}

// New creates a Daemon from an already-loaded identity and config.
func New(
	id *identity.Identity,
	cfg *config.Config,
	peers *config.PeerStore,
	configDir string,
) *Daemon {
	// chat.NewService requires the run context so its reader goroutines share
	// the daemon lifetime. Run() sets d.runCtx before any peers connect, so
	// we pass a background context here and wire the real one in Run().
	d := &Daemon{
		id:        id,
		cfg:       cfg,
		peers:     peers,
		registry:  transport.NewPeerRegistry(),
		configDir: configDir,
	}
	return d
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()

	d.runCtx = ctx

	// Now that the run context exists, create the chat service with it so
	// outbound-side reader goroutines (opened by ensureChatStream) are
	// cancelled on daemon shutdown, not just when an individual Send call ends.
	d.chat = chat.NewService(ctx)
	d.chat.OnMessage(d.onChatMessage)
	d.RegisterStreamHandler(proto.StreamChat, d.chat.HandleIncomingStream)
	d.RegisterStreamHandler(proto.StreamFile, handleStreamFileNotImplemented)

	ipcServer, err := NewIPCServer(d.configDir, d)
	if err != nil {
		return fmt.Errorf("start ipc server: %w", err)
	}
	d.ipcServer = ipcServer
	defer func() {
		d.ipcServer.Close()
		cleanupIPC(d.configDir)
	}()

	listenAddr := fmt.Sprintf(":%d", d.cfg.Port)
	if err := d.listenForPeers(ctx, listenAddr); err != nil {
		return fmt.Errorf("start peer listener: %w", err)
	}

	// daemon.pid is for operator/supervisor introspection; flock on the same
	// path in spawn.go guards against double-spawn races.
	if err := writeDaemonPID(d.configDir); err != nil {
		return fmt.Errorf("write daemon pid file: %w", err)
	}
	defer func() { _ = removeDaemonPID(d.configDir) }()

	fmt.Fprintf(os.Stderr, "bolt daemon running on UDP port %d\n", d.cfg.Port)

	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "bolt daemon shutting down...")
	d.shutdown()
	return nil
}

// SendChat sends a message to a connected peer identified by nickname or fingerprint prefix.
func (d *Daemon) SendChat(ctx context.Context, peerName, body string) error {
	pc, err := d.registry.ResolvePeer(peerName)
	if err != nil {
		return err
	}
	return d.chat.Send(ctx, pc, d.cfg.Nickname, body)
}

func (d *Daemon) onChatMessage(msg chat.IncomingMessage) {
	if d.ipcServer == nil {
		return
	}
	sentAt := msg.SentAt.UTC().Format(time.RFC3339)
	d.ipcServer.PublishChat(ChatEventPayload{
		From:        msg.From,
		Fingerprint: msg.Fingerprint,
		Body:        msg.Body,
		SentAt:      sentAt,
	})
}

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

func (d *Daemon) acceptLoop(ctx context.Context, ln *quic.Listener) {
	defer ln.Close()
	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			fmt.Fprintf(os.Stderr, "accept connection: %v\n", err)
			continue
		}
		go d.handleIncomingPeer(ctx, conn)
	}
}

func (d *Daemon) handleIncomingPeer(ctx context.Context, conn *quic.Conn) {
	pc, err := d.authenticateIncoming(ctx, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "authentication failed: %v\n", err)
		_ = conn.CloseWithError(1, "authentication failed")
		return
	}

	remote := ""
	if conn.RemoteAddr() != nil {
		remote = conn.RemoteAddr().String()
	}
	d.savePeerRecord(pc, remote, false)
	go d.servePeer(ctx, pc)
}

func (d *Daemon) authenticateIncoming(ctx context.Context, conn *quic.Conn) (*transport.PeerConn, error) {
	tlsState := conn.ConnectionState().TLS
	if len(tlsState.PeerCertificates) == 0 {
		return nil, fmt.Errorf("peer presented no TLS certificate")
	}

	peerPub, err := identity.ExtractPublicKeyFromCert(tlsState.PeerCertificates[0])
	if err != nil {
		return nil, fmt.Errorf("extract peer identity: %w", err)
	}

	fingerprint := identity.Fingerprint(peerPub)
	existing := d.peers.Get(fingerprint)
	if existing != nil && existing.Trust == config.TrustBlock {
		return nil, fmt.Errorf("peer %s is blocked", fingerprint[:17]+"...")
	}

	pc := transport.NewPeerConn(conn, fingerprint)
	if err := pc.Handshake(ctx, d.localPeer()); err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	return pc, nil
}

func (d *Daemon) servePeer(ctx context.Context, pc *transport.PeerConn) {
	fp := pc.PeerFingerprint()
	if existing := d.registry.Get(fp); existing != nil && existing != pc {
		_ = pc.Close()
		return
	}

	d.registry.Add(pc)
	defer d.registry.Remove(fp)

	nick := proto.SanitizeDisplay(pc.PeerNickname(), proto.MaxNicknameRunes)
	fmt.Fprintf(os.Stderr, "peer connected: %s (%s)\n", nick, fp[:17]+"...")
	d.serveStreams(ctx, pc)
	fmt.Fprintf(os.Stderr, "peer disconnected: %s\n", nick)
}

func (d *Daemon) serveStreams(ctx context.Context, pc *transport.PeerConn) {
	for {
		stream, streamType, err := pc.AcceptStream(ctx)
		if err != nil {
			return
		}
		go d.routeStream(ctx, pc, stream, streamType)
	}
}

func (d *Daemon) routeStream(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream, streamType proto.StreamType) {
	if streamType == proto.StreamHandshake {
		_ = stream.Close()
		return
	}
	if h, ok := d.streamHandler(streamType); ok {
		h(ctx, pc, stream)
		return
	}
	d.rejectUnknownStream(pc, stream, streamType)
}

func (d *Daemon) shutdown() {
	for _, pc := range d.registry.Online() {
		_ = pc.Close()
	}
}

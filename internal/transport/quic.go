package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	// keepAlivePeriod is how often quic-go sends a PING frame to keep the
	// connection alive through NAT mappings and idle timeouts.
	keepAlivePeriod = 15 * time.Second

	// maxIncomingStreams is the maximum number of peer-initiated streams we
	// accept concurrently per connection. Each file chunk uses one stream,
	// so this needs to be large enough for parallel transfers.
	maxIncomingStreams = 1000

	// handshakeTimeout is how long we wait for the QUIC+TLS handshake to
	// complete before giving up. 10 seconds is generous for a LAN connection
	// but necessary for internet connections with relay coordination.
	handshakeTimeout = 10 * time.Second
)

// quicConfig returns the shared quic.Config used by both listeners and dialers.
// Keeping this in one place ensures both sides use identical settings.
func quicConfig() *quic.Config {
	return &quic.Config{
		KeepAlivePeriod:       keepAlivePeriod,
		MaxIncomingStreams:    maxIncomingStreams,
		MaxIncomingUniStreams: maxIncomingStreams,
		HandshakeIdleTimeout:  handshakeTimeout,
	}
}

// Listen starts a QUIC listener on the given address (e.g. ":7799").
// tlsConf must be a server-side TLS config from ServerTLSConfig().
// The caller is responsible for closing the listener.
func Listen(addr string, tlsConf *tls.Config) (*quic.Listener, error) {
	ln, err := quic.ListenAddr(addr, tlsConf, quicConfig())
	if err != nil {
		return nil, fmt.Errorf("start quic listener on %s: %w", addr, err)
	}
	return ln, nil
}

// Dial opens a QUIC connection to addr (e.g. "192.168.1.42:7799").
// tlsConf must be a client-side TLS config from ClientTLSConfig().
//
// Dial blocks until the QUIC+TLS handshake completes or ctx is cancelled.
// The caller is responsible for closing the connection.
func Dial(ctx context.Context, addr string, tlsConf *tls.Config) (*quic.Conn, error) {
	conn, err := quic.DialAddr(ctx, addr, tlsConf, quicConfig())
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	return conn, nil
}

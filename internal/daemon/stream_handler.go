package daemon

import (
	"context"
	"fmt"
	"os"

	"github.com/bolt/bolt/internal/proto"
	"github.com/bolt/bolt/internal/transport"
	"github.com/quic-go/quic-go"
)

// StreamHandler processes one accepted QUIC stream for a registered StreamType.
type StreamHandler func(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream)

// RegisterStreamHandler wires a handler for the given stream type.
// Safe for concurrent registration during daemon startup.
func (d *Daemon) RegisterStreamHandler(t proto.StreamType, h StreamHandler) {
	d.streamHandlersMu.Lock()
	defer d.streamHandlersMu.Unlock()
	if d.streamHandlers == nil {
		d.streamHandlers = make(map[proto.StreamType]StreamHandler)
	}
	d.streamHandlers[t] = h
}

func (d *Daemon) streamHandler(t proto.StreamType) (StreamHandler, bool) {
	d.streamHandlersMu.RLock()
	defer d.streamHandlersMu.RUnlock()
	h, ok := d.streamHandlers[t]
	return h, ok
}

func (d *Daemon) rejectUnknownStream(pc *transport.PeerConn, stream *quic.Stream, streamType proto.StreamType) {
	code := quic.StreamErrorCode(proto.ErrCodeUnknownStreamType)
	stream.CancelRead(code)
	stream.CancelWrite(code)
	_ = stream.Close()
	fmt.Fprintf(
		os.Stderr,
		"unhandled stream type 0x%02x from %s — protocol violation\n",
		streamType,
		proto.SanitizeDisplay(pc.PeerNickname(), proto.MaxNicknameRunes),
	)
}

func handleStreamFileNotImplemented(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream) {
	_ = ctx
	fmt.Fprintf(
		os.Stderr,
		"file transfer stream from %s — not implemented yet\n",
		proto.SanitizeDisplay(pc.PeerNickname(), proto.MaxNicknameRunes),
	)
	_ = stream.Close()
}

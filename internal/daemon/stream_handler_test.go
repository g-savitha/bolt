package daemon

import (
	"context"
	"testing"

	"github.com/bolt/bolt/internal/config"
	"github.com/bolt/bolt/internal/identity"
	"github.com/bolt/bolt/internal/proto"
	"github.com/bolt/bolt/internal/transport"
	"github.com/quic-go/quic-go"
)

func TestRegisterStreamHandler_lookup(t *testing.T) {
	dir := t.TempDir()
	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Nickname: "test", Port: 7797}
	peers, err := config.LoadPeerStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	d := New(id, cfg, peers, dir)
	var called bool
	d.RegisterStreamHandler(proto.StreamType(0x05), func(context.Context, *transport.PeerConn, *quic.Stream) {
		called = true
	})

	h, ok := d.streamHandler(proto.StreamType(0x05))
	if !ok {
		t.Fatal("handler not registered")
	}
	h(context.Background(), nil, nil)
	if !called {
		t.Fatal("registered handler was not invoked")
	}

	if _, ok := d.streamHandler(proto.StreamType(0xFF)); ok {
		t.Fatal("expected no handler for unknown type")
	}
}

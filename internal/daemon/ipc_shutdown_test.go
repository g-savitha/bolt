//go:build !windows

package daemon

import (
	"os"
	"testing"
	"time"

	"github.com/bolt/bolt/internal/config"
	"github.com/bolt/bolt/internal/identity"
)

func TestIPCServerCloseWithActiveSubscriber(t *testing.T) {
	// Short path: macOS limits unix socket paths to ~104 bytes.
	dir, err := os.MkdirTemp("/tmp", "bolt-ipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	id, err := identity.Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Nickname: "test", Port: 7799}
	peers, err := config.LoadPeerStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	d := New(id, cfg, peers, dir)
	srv, err := NewIPCServer(dir, d)
	if err != nil {
		t.Fatal(err)
	}

	subConn, _, err := Subscribe(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer subConn.Close()

	// Wait until handleSubscribe registered the connection (race with Close).
	deadline := time.Now().Add(2 * time.Second)
	for {
		srv.subsMu.Lock()
		n := len(srv.subs)
		srv.subsMu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("subscribe connection not registered on server")
		}
		time.Sleep(5 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		srv.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("IPCServer.Close hung with an active subscribe connection")
	}
}

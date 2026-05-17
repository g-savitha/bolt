package transport

import "github.com/bolt/bolt/internal/identity"

// LocalPeer is this machine's identity for protocol handshakes.
type LocalPeer struct {
	Identity *identity.Identity
	Nickname string
}

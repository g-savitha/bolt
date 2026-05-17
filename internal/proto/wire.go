// Package proto defines the on-wire protocol between bolt nodes.
//
// Every QUIC stream begins with a single StreamType byte that tells the
// receiver how to interpret the rest of the stream. After that byte, all
// messages are framed as:
//
//	[4-byte big-endian uint32: payload length][JSON payload bytes]
//
// All JSON payloads include a "v" field set to WireVersion. Receivers must
// reject messages with an unknown version rather than attempting to parse them,
// so that protocol evolution never causes silent data corruption.
package proto

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// WireVersion is the current protocol version. Increment this when making
// breaking changes to any message type. Bump WireVersion, not individual messages.
const WireVersion = 1

// StreamType is the single byte written at the start of every QUIC stream.
// It tells the receiver which handler should own this stream.
type StreamType byte

const (
	// StreamHandshake is opened once per connection immediately after TLS
	// completes. Both sides exchange HandshakeMsg to verify Ed25519 identities.
	StreamHandshake StreamType = 0x01

	// StreamChat carries real-time chat messages. One persistent stream per
	// connection, kept open for the lifetime of the connection.
	StreamChat StreamType = 0x02

	// StreamFile carries either a file transfer control message (FileHeader,
	// TransferAck, TransferDone) or raw chunk data. New stream per file transfer.
	StreamFile StreamType = 0x03

	// StreamControl carries housekeeping messages: heartbeats, cancellations.
	StreamControl StreamType = 0x04
)

// --- Handshake ---

// HandshakeMsg is the first message exchanged after a QUIC connection is
// established. It lets both sides verify that the Ed25519 public key embedded
// in the TLS certificate actually belongs to who they claim to be.
//
// Both peers send and receive a HandshakeMsg simultaneously on a StreamHandshake
// stream before any other protocol activity begins.
type HandshakeMsg struct {
	V int `json:"v"`
	// PublicKeyHex is the sender's Ed25519 public key as a hex string.
	// The receiver must verify it matches the SubjectKeyId of the TLS cert.
	PublicKeyHex string `json:"public_key"`
	Nickname     string `json:"nickname"`
	// BoltVersion is the bolt binary version string (e.g. "0.1.0").
	// Used for informational display, not protocol gating (use V for that).
	BoltVersion string `json:"bolt_version"`
}

// --- Chat ---

// ChatMsg carries a single chat message from one peer to another.
// Sent over a persistent StreamChat stream.
type ChatMsg struct {
	V      int       `json:"v"`
	ID     string    `json:"id"`   // UUID, used for deduplication in group chat
	From   string    `json:"from"` // sender's nickname
	Body   string    `json:"body"` // message text
	SentAt time.Time `json:"sent_at"`
	// GroupID is non-empty for group chat messages. Empty means 1:1.
	GroupID string `json:"group_id,omitempty"`
}

// --- File Transfer ---

// FileHeader is the first message on a StreamFile control stream.
// The sender writes it; the receiver responds with TransferAck.
type FileHeader struct {
	V           int    `json:"v"`
	TransferID  string `json:"transfer_id"` // UUID uniquely identifying this transfer
	Filename    string `json:"filename"`    // original filename, no path components
	SizeBytes   int64  `json:"size_bytes"`  // -1 if unknown (stdin pipe)
	TotalChunks int    `json:"total_chunks"`
	// ChunkSize is negotiated per-transfer: large on LAN, small over TURN relay.
	ChunkSize int `json:"chunk_size"`
	// FileHash is the SHA-256 of the complete file, hex-encoded.
	// Receiver verifies this after all chunks arrive.
	FileHash string `json:"file_hash"`
}

// TransferAck is the receiver's response to a FileHeader.
// Accepted=false means the receiver rejected the transfer — check Reason.
// ResumeFromChunks is non-nil when the receiver has a partial download and
// wants only the listed chunk indices retransmitted.
type TransferAck struct {
	V          int    `json:"v"`
	TransferID string `json:"transfer_id"`
	Accepted   bool   `json:"accepted"`
	// Reason is set when Accepted=false. Always a human-readable string.
	Reason string `json:"reason,omitempty"`
	// ResumeFromChunks lists chunk indices the receiver still needs.
	// Nil means "send everything" (new transfer, no partial data).
	ResumeFromChunks []int `json:"resume_from_chunks,omitempty"`
}

// ChunkMsg carries one chunk of file data on a parallel StreamFile stream.
// Multiple ChunkMsg streams run concurrently for a single transfer — one
// goroutine per stream, pulling chunks from a shared work queue.
type ChunkMsg struct {
	V          int    `json:"v"`
	TransferID string `json:"transfer_id"`
	Index      int    `json:"index"`
	// ChunkHash is SHA-256 of Data, hex-encoded. Receiver verifies before writing.
	ChunkHash string `json:"chunk_hash"`
	Data      []byte `json:"data"`
}

// TransferDone signals that the sender has dispatched all chunks.
// The receiver verifies the whole-file hash and renames the temp file.
type TransferDone struct {
	V          int    `json:"v"`
	TransferID string `json:"transfer_id"`
}

// --- Control ---

// HeartbeatMsg is sent periodically on a StreamControl stream to confirm
// the connection is still alive.
type HeartbeatMsg struct {
	V int `json:"v"`
	// SeqNo increments with each heartbeat. The other side echoes it back
	// so we can measure round-trip latency.
	SeqNo int `json:"seq_no"`
}

// CancelMsg requests that a running file transfer be aborted.
type CancelMsg struct {
	V          int    `json:"v"`
	TransferID string `json:"transfer_id"`
	Reason     string `json:"reason"`
}

// --- Framing helpers ---

// WriteFrame writes a length-prefixed JSON message to w.
// The format is: [4-byte big-endian uint32 length][JSON bytes].
// All bolt messages are sent with this framing.
func WriteFrame(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message to json: %w", err)
	}

	if len(data) > maxFrameSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), maxFrameSize)
	}

	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(data)))

	if _, err := w.Write(lengthBuf[:]); err != nil {
		return fmt.Errorf("write frame length: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write frame payload: %w", err)
	}
	return nil
}

// ReadFrame reads a length-prefixed JSON frame from r and unmarshals it into dst.
// dst must be a pointer to the expected message type.
func ReadFrame(r io.Reader, dst any) error {
	var lengthBuf [4]byte
	if _, err := io.ReadFull(r, lengthBuf[:]); err != nil {
		return fmt.Errorf("read frame length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])
	if length > uint32(maxFrameSize) {
		return fmt.Errorf("incoming frame too large: %d bytes (max %d) — possible protocol mismatch or attack", length, maxFrameSize)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("read frame payload: %w", err)
	}

	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("unmarshal frame: %w", err)
	}
	return nil
}

// WriteStreamType writes the stream type byte to w.
// Must be the very first write on any new QUIC stream.
func WriteStreamType(w io.Writer, t StreamType) error {
	if _, err := w.Write([]byte{byte(t)}); err != nil {
		return fmt.Errorf("write stream type byte: %w", err)
	}
	return nil
}

// ReadStreamType reads the stream type byte from r.
// Must be the very first read on any accepted QUIC stream.
func ReadStreamType(r io.Reader) (StreamType, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, fmt.Errorf("read stream type byte: %w", err)
	}
	return StreamType(buf[0]), nil
}

// maxFrameSize is a safety cap on incoming frame sizes.
// Prevents a malicious peer from causing an OOM allocation.
// Chunks are at most 4MB; JSON overhead is minimal.
const maxFrameSize = 8 * 1024 * 1024 // 8 MB

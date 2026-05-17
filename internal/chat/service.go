// Package chat routes real-time messages over authenticated peer connections.
package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/bolt/bolt/internal/proto"
	"github.com/bolt/bolt/internal/transport"
	"github.com/quic-go/quic-go"
)

// IncomingMessage is delivered to local subscribers (CLI, logging, etc.).
type IncomingMessage struct {
	From        string
	Fingerprint string
	Body        string
	SentAt      time.Time
}

// Handler receives chat messages from remote peers.
type Handler func(IncomingMessage)

// Service manages chat streams and message delivery for connected peers.
type Service struct {
	mu       sync.RWMutex
	handlers []Handler
	streams  map[string]*peerChat // fingerprint → chat session
}

type peerChat struct {
	stream  *quic.Stream
	writeMu sync.Mutex
}

// NewService creates an empty chat service.
func NewService() *Service {
	return &Service{
		streams: make(map[string]*peerChat),
	}
}

// OnMessage registers a handler for incoming chat messages.
func (s *Service) OnMessage(h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, h)
}

func (s *Service) emit(msg IncomingMessage) {
	s.mu.RLock()
	handlers := append([]Handler(nil), s.handlers...)
	s.mu.RUnlock()
	for _, h := range handlers {
		h(msg)
	}
}

// AttachStream takes ownership of a bidirectional chat stream for a peer.
// It reads frames until the stream closes or ctx is cancelled.
func (s *Service) AttachStream(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream) {
	fp := pc.PeerFingerprint()

	s.mu.Lock()
	if existing, ok := s.streams[fp]; ok {
		existing.stream.Close()
	}
	s.streams[fp] = &peerChat{stream: stream}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if cur, ok := s.streams[fp]; ok && cur.stream == stream {
			delete(s.streams, fp)
		}
		s.mu.Unlock()
		stream.Close()
	}()

	for {
		var msg proto.ChatMsg
		if err := proto.ReadFrame(stream, &msg); err != nil {
			return
		}
		if msg.V != proto.WireVersion {
			continue
		}
		sentAt := msg.SentAt
		if sentAt.IsZero() {
			sentAt = time.Now().UTC()
		}
		s.emit(IncomingMessage{
			From:        msg.From,
			Fingerprint: fp,
			Body:        msg.Body,
			SentAt:      sentAt,
		})
	}
}

// Send writes a chat message to a connected peer.
func (s *Service) Send(ctx context.Context, pc *transport.PeerConn, localNickname, body string) error {
	if body == "" {
		return fmt.Errorf("message body is empty")
	}

	session, err := s.ensureChatStream(ctx, pc)
	if err != nil {
		return err
	}

	msg := proto.ChatMsg{
		V:      proto.WireVersion,
		ID:     newMessageID(),
		From:   localNickname,
		Body:   body,
		SentAt: time.Now().UTC(),
	}

	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	return proto.WriteFrame(session.stream, msg)
}

func (s *Service) ensureChatStream(ctx context.Context, pc *transport.PeerConn) (*peerChat, error) {
	fp := pc.PeerFingerprint()

	s.mu.RLock()
	if session, ok := s.streams[fp]; ok {
		s.mu.RUnlock()
		return session, nil
	}
	s.mu.RUnlock()

	stream, err := pc.OpenStream(ctx, proto.StreamChat)
	if err != nil {
		return nil, fmt.Errorf("open chat stream to %s: %w", pc.PeerNickname(), err)
	}

	session := &peerChat{stream: stream}
	s.mu.Lock()
	s.streams[fp] = session
	s.mu.Unlock()

	go s.AttachStream(context.Background(), pc, stream)
	return session, nil
}

// HandleIncomingStream is called when the remote peer opened the chat stream.
func (s *Service) HandleIncomingStream(ctx context.Context, pc *transport.PeerConn, stream *quic.Stream) {
	go s.AttachStream(ctx, pc, stream)
}

func newMessageID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

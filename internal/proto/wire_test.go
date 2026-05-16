package proto_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/flick/flick/internal/proto"
)

func TestWriteAndReadFrame_RoundTrip(t *testing.T) {
	original := proto.ChatMsg{
		V:      proto.WireVersion,
		ID:     "test-id-123",
		From:   "alice",
		Body:   "hello world",
		SentAt: time.Now().Round(time.Second), // round to avoid nanosecond diff in JSON
	}

	var buf bytes.Buffer
	if err := proto.WriteFrame(&buf, original); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	var decoded proto.ChatMsg
	if err := proto.ReadFrame(&buf, &decoded); err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.From != original.From {
		t.Errorf("From: got %q, want %q", decoded.From, original.From)
	}
	if decoded.Body != original.Body {
		t.Errorf("Body: got %q, want %q", decoded.Body, original.Body)
	}
}

func TestReadFrame_RejectsOversizeFrame(t *testing.T) {
	// Craft a frame header claiming a 9MB payload (over the 8MB limit).
	buf := bytes.NewBuffer([]byte{0x00, 0x90, 0x00, 0x00}) // 9,437,184 bytes

	var dst proto.ChatMsg
	if err := proto.ReadFrame(buf, &dst); err == nil {
		t.Error("ReadFrame should reject oversized frame but did not")
	}
}

func TestWriteAndReadStreamType(t *testing.T) {
	types := []proto.StreamType{
		proto.StreamHandshake,
		proto.StreamChat,
		proto.StreamFile,
		proto.StreamControl,
	}

	for _, want := range types {
		var buf bytes.Buffer
		if err := proto.WriteStreamType(&buf, want); err != nil {
			t.Fatalf("WriteStreamType(%v): %v", want, err)
		}
		got, err := proto.ReadStreamType(&buf)
		if err != nil {
			t.Fatalf("ReadStreamType: %v", err)
		}
		if got != want {
			t.Errorf("stream type round-trip: got %v, want %v", got, want)
		}
	}
}

func TestWriteFrame_MultipleMessages(t *testing.T) {
	// Verifies that multiple frames written sequentially can be read back
	// in order — important for the chat stream which is long-lived.
	messages := []proto.ChatMsg{
		{V: proto.WireVersion, ID: "1", Body: "first"},
		{V: proto.WireVersion, ID: "2", Body: "second"},
		{V: proto.WireVersion, ID: "3", Body: "third"},
	}

	var buf bytes.Buffer
	for _, msg := range messages {
		if err := proto.WriteFrame(&buf, msg); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}

	for i, want := range messages {
		var got proto.ChatMsg
		if err := proto.ReadFrame(&buf, &got); err != nil {
			t.Fatalf("ReadFrame[%d]: %v", i, err)
		}
		if got.ID != want.ID || got.Body != want.Body {
			t.Errorf("message[%d]: got {%s %s}, want {%s %s}", i, got.ID, got.Body, want.ID, want.Body)
		}
	}
}

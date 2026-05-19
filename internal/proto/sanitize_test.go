package proto

import "testing"

func TestSanitizeDisplayEscapesANSIAndCR(t *testing.T) {
	got := SanitizeDisplay("alice\x1b[2J\x1b[H", MaxNicknameRunes)
	want := `alice\x1b[2J\x1b[H`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSanitizeDisplayPreservesNewlineAndTab(t *testing.T) {
	got := SanitizeDisplay("a\nb\tc", MaxChatBodyRunes)
	if got != "a\nb\tc" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeDisplayEscapesCarriageReturn(t *testing.T) {
	got := SanitizeDisplay("line\rforge", MaxChatBodyRunes)
	if got != `line\x0dforge` {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeDisplayTruncatesRunes(t *testing.T) {
	got := SanitizeDisplay("abcdef", 3)
	if got != "abc" {
		t.Fatalf("got %q", got)
	}
}

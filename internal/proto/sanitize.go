package proto

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxNicknameRunes is the display cap for peer nicknames.
	MaxNicknameRunes = 64
	// MaxChatBodyRunes is the display cap for chat message bodies.
	MaxChatBodyRunes = 4096
)

// SanitizeDisplay strips terminal control bytes from peer-controlled strings
// before they are shown on a TTY or persisted for display. Bytes below 0x20
// (except newline and tab) and DEL (0x7f) are replaced with a literal \xNN
// escape. The result is truncated to maxRunes UTF-8 runes.
func SanitizeDisplay(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}

	var b strings.Builder
	runes := 0
	for i := 0; i < len(s) && runes < maxRunes; {
		c := s[i]
		if (c < 0x20 && c != '\n' && c != '\t') || c == 0x7f {
			fmt.Fprintf(&b, `\x%02x`, c)
			i++
			runes++
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\x%02x`, c)
			i++
			runes++
			continue
		}
		if runes+1 > maxRunes {
			break
		}
		b.WriteString(s[i : i+size])
		i += size
		runes++
	}
	return b.String()
}

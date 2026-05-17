package identity

import (
	"crypto/ed25519"
	"fmt"
	"strings"
)

// Randomart generates an SSH-style "drunken bishop" visual fingerprint from
// a public key. The output looks like this:
//
//	+--[ bolt ]----+
//	|     .o.       |
//	|    o  o       |
//	|   . +  .      |
//	|    o . ..     |
//	|     . oS..    |
//	|      . .=.    |
//	|       . E+    |
//	|          .o   |
//	+---------------+
//
// The same key always produces the same picture. Two different keys almost
// never produce the same picture. This lets users verify a peer's identity
// with a glance rather than comparing 64 hex characters.
func Randomart(pub ed25519.PublicKey) string {
	const (
		width  = 17
		height = 9
	)

	// The bishop starts in the centre of the board.
	const startX, startY = width / 2, height / 2

	// field tracks how many times the bishop has visited each cell.
	field := [height][width]int{}
	x, y := startX, startY

	// Walk the bishop through the key bytes, two bits at a time.
	// Each pair of bits encodes a diagonal move: NW, NE, SW, SE.
	for _, b := range pub {
		for step := 0; step < 4; step++ {
			bits := (b >> (step * 2)) & 0x3

			// Move horizontally.
			if bits&0x1 != 0 {
				x++
			} else {
				x--
			}
			// Move vertically.
			if bits&0x2 != 0 {
				y++
			} else {
				y--
			}

			// Clamp to board boundaries — bishop bounces off the walls.
			if x < 0 {
				x = 0
			} else if x >= width {
				x = width - 1
			}
			if y < 0 {
				y = 0
			} else if y >= height {
				y = height - 1
			}

			field[y][x]++
		}
	}

	// Mark the start and end positions with special symbols.
	field[startY][startX] = 15 // 'S'
	field[y][x] = 16           // 'E'

	// Each visit count maps to a character. More visits = more visually dense.
	symbols := " .o+=*BOX@%&#/^SE"

	var sb strings.Builder
	sb.WriteString("+--[ bolt ]----+\n")

	for row := 0; row < height; row++ {
		sb.WriteByte('|')
		for col := 0; col < width; col++ {
			count := field[row][col]
			if count >= len(symbols) {
				count = len(symbols) - 1
			}
			sb.WriteByte(symbols[count])
		}
		sb.WriteString("|\n")
	}

	sb.WriteString("+-[SHA256]------+")
	return sb.String()
}

// FormatIdentityBlock returns a formatted block showing the fingerprint and
// randomart together, ready to print to the terminal. Example output:
//
//	Fingerprint: ab:cd:ef:12:34:...
//	+--[ bolt ]----+
//	|     .o.       |
//	...
func FormatIdentityBlock(pub ed25519.PublicKey) string {
	fp := Fingerprint(pub)
	art := Randomart(pub)
	return fmt.Sprintf("Fingerprint: %s\n%s", fp, art)
}

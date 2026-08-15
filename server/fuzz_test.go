package server

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func FuzzSanitizeInput(f *testing.F) {
	f.Add("hello \x1b[31mworld\x1b[0m")
	f.Add("a\tb\nc")
	f.Add(strings.Repeat("x", 5000))
	f.Add("\x00\x01\x02\x1b[2J")
	f.Add("emoji 🙂🙃🫠")

	f.Fuzz(func(t *testing.T, in string) {
		out := sanitizeInput(in)

		if strings.Contains(out, "\x1b") {
			t.Fatalf("sanitizeInput(%q) left an escape sequence: %q", in, out)
		}

		if !utf8.ValidString(out) {
			t.Fatalf("sanitizeInput(%q) produced invalid UTF-8: %q", in, out)
		}

		for _, r := range out {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				t.Fatalf("sanitizeInput(%q) left control char %q in %q", in, r, out)
			}
		}
	})
}

func FuzzTruncateRunes(f *testing.F) {
	f.Add("héllo", 4)
	f.Add("😀😀😀", 2)
	f.Add("", 0)
	f.Add("mixed aé😀b", 3)

	f.Fuzz(func(t *testing.T, in string, max int) {
		out := truncateRunes(in, max)

		if !utf8.ValidString(out) {
			t.Fatalf("truncateRunes(%q, %d) produced invalid UTF-8: %q", in, max, out)
		}

		if max > 0 && utf8.RuneCountInString(out) > max {
			t.Fatalf("truncateRunes(%q, %d) = %q exceeds rune limit", in, max, out)
		}
	})
}

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestSanitizeInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"sgr ansi", "\x1b[31mred\x1b[0m", "red"},
		{"bold ansi", " \x1b[1m hi \x1b[0m ", "hi"},
		{"clear screen", "\x1b[2J", ""},
		{"cursor move", "\x1b[A\x1b[B\x1b[C\x1b[D", ""},
		{"cursor position", "\x1b[12;34H", ""},
		{"bell", "a\x07b", "ab"},
		{"null and controls", "\x00\x01\x02abc", "abc"},
		{"tab preserved", "a\tb", "a\tb"},
		{"newline preserved", "line1\nline2", "line1\nline2"},
		{"trimmed", "  spaced  ", "spaced"},
		{"empty", "", ""},
		{"emoji untouched", "hi 🙂", "hi 🙂"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeInput(c.in); got != c.want {
				t.Errorf("sanitizeInput(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"fits exactly", "hello", 5, "hello"},
		{"shorter", "hello", 10, "hello"},
		{"zero", "hello", 0, ""},
		{"negative", "hello", -1, ""},
		{"empty", "", 5, ""},
		{"ascii cut", "hello", 3, "hel"},
		{"multibyte boundary", "héllo", 4, "héll"},
		{"emoji cut", "😀😀😀", 2, "😀😀"},
		{"emoji exact", "😀😀", 2, "😀😀"},
		{"mixed cut", "aé😀b", 2, "aé"},
		{"invalid utf8 passthrough", "\xdb", 34, "\ufffd"},
		{"invalid utf8 truncated", "\xdbhello", 3, "\ufffdhe"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateRunes(c.in, c.max)

			if got != c.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
			}

			if !utf8.ValidString(got) {
				t.Errorf("truncateRunes(%q, %d) produced invalid UTF-8 %q", c.in, c.max, got)
			}

			if c.max >= 0 && utf8.RuneCountInString(got) > c.max {
				t.Errorf("truncateRunes(%q, %d) = %q has %d runes", c.in, c.max, got, utf8.RuneCountInString(got))
			}
		})
	}
}

func TestDefaultColorForNick(t *testing.T) {
	for _, nick := range []string{"alice", "bob", "anonymous", ""} {
		first := defaultColorForNick(nick)
		second := defaultColorForNick(nick)

		if first != second {
			t.Errorf("defaultColorForNick(%q) not deterministic: %q vs %q", nick, first, second)
		}
	}

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[defaultColorForNick(fmt.Sprintf("nick-%d", i))] = true
	}

	if len(seen) < 10 {
		t.Errorf("only %d distinct colors from 200 nicks, palette too small", len(seen))
	}
}

func TestFetchLatestCLIVersion(t *testing.T) {
	t.Cleanup(func() {
		githubAPIBase = "https://api.github.com"
		githubRepo = "ishaan-jindal/termchat"
	})

	t.Run("success", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(`{"tag_name":"cli-v9.9.9"}`))
		}))
		defer srv.Close()

		githubAPIBase = srv.URL
		githubRepo = "acme/termchat"

		if got := fetchLatestCLIVersion(); got != "cli-v9.9.9" {
			t.Errorf("version = %q, want cli-v9.9.9", got)
		}

		if want := "/repos/acme/termchat/releases/latest"; gotPath != want {
			t.Errorf("request path = %q, want %q", gotPath, want)
		}
	})

	t.Run("non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		githubAPIBase = srv.URL

		if got := fetchLatestCLIVersion(); got != "" {
			t.Errorf("version = %q, want empty on 500", got)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("not json"))
		}))
		defer srv.Close()

		githubAPIBase = srv.URL

		if got := fetchLatestCLIVersion(); got != "" {
			t.Errorf("version = %q, want empty on bad json", got)
		}
	})

	t.Run("empty tag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"tag_name":""}`))
		}))
		defer srv.Close()

		githubAPIBase = srv.URL

		if got := fetchLatestCLIVersion(); got != "" {
			t.Errorf("version = %q, want empty for empty tag", got)
		}
	})

	t.Run("network error", func(t *testing.T) {
		githubAPIBase = "http://127.0.0.1:1"

		if got := fetchLatestCLIVersion(); got != "" {
			t.Errorf("version = %q, want empty on network error", got)
		}
	})
}

func TestRefreshCLIVersionOnceDoesNotRegress(t *testing.T) {
	t.Cleanup(func() {
		githubAPIBase = "https://api.github.com"
		versionMu.Lock()
		cachedCLIVersion = "cli-v0.0.0"
		versionMu.Unlock()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"cli-v1.2.3"}`))
	}))
	defer srv.Close()

	githubAPIBase = srv.URL

	refreshCLIVersionOnce()

	if got := latestCLIVersion(); got != "cli-v1.2.3" {
		t.Fatalf("version after refresh = %q, want cli-v1.2.3", got)
	}

	// A failing refresh must not downgrade the cached version.
	githubAPIBase = "http://127.0.0.1:1"

	refreshCLIVersionOnce()

	if got := latestCLIVersion(); got != "cli-v1.2.3" {
		t.Fatalf("version regressed to %q after failed refresh", got)
	}
}

func TestLatestCLIVersionConcurrent(t *testing.T) {
	versionMu.Lock()
	cachedCLIVersion = "cli-v5.0.0"
	versionMu.Unlock()

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if got := latestCLIVersion(); !strings.HasPrefix(got, "cli-v") {
				t.Errorf("unexpected version %q", got)
			}
		}()
	}

	wg.Wait()
}

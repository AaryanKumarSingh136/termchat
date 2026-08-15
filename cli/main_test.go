package main

import "testing"

func TestParseHostModeWithRoomAndPort(t *testing.T) {
	opts, err := parseArgs([]string{"host", "FROG", "--port", "9000"})
	if err != nil {
		t.Fatal(err)
	}

	if !opts.HostMode {
		t.Fatal("expected host mode")
	}
	if opts.Room != "FROG" {
		t.Fatalf("room = %q, want FROG", opts.Room)
	}
	if opts.Port != 9000 {
		t.Fatalf("port = %d, want 9000", opts.Port)
	}
	if got := websocketURL(opts); got != "ws://localhost:9000/ws" {
		t.Fatalf("websocketURL = %q", got)
	}
}

func TestParseLANJoinFlagsAfterRoom(t *testing.T) {
	opts, err := parseArgs([]string{"FROG", "--host", "192.168.1.42", "--port", "9000"})
	if err != nil {
		t.Fatal(err)
	}

	if opts.HostMode {
		t.Fatal("did not expect host mode")
	}
	if opts.Room != "FROG" {
		t.Fatalf("room = %q, want FROG", opts.Room)
	}
	if got := websocketURL(opts); got != "ws://192.168.1.42:9000/ws" {
		t.Fatalf("websocketURL = %q", got)
	}
}

func TestExplicitServerTakesPriority(t *testing.T) {
	opts, err := parseArgs([]string{
		"FROG",
		"--host", "192.168.1.42",
		"--port", "9000",
		"--server", "ws://example.test/ws",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := websocketURL(opts); got != "ws://example.test/ws" {
		t.Fatalf("websocketURL = %q", got)
	}
}

func TestParseHelp(t *testing.T) {
	opts, err := parseArgs([]string{"--help"})
	if err != nil {
		t.Fatal(err)
	}

	if !opts.Help {
		t.Fatal("expected help flag")
	}
}

func TestDiscoverBaseURL(t *testing.T) {
	opts, err := parseArgs([]string{"discover"})
	if err != nil {
		t.Fatal(err)
	}

	if got := discoverBaseURL(opts); got != "https://termchat.sacred99.online" {
		t.Fatalf("default base = %q", got)
	}

	opts, err = parseArgs([]string{"discover", "--server", "ws://example.test/ws"})
	if err != nil {
		t.Fatal(err)
	}

	if got := discoverBaseURL(opts); got != "http://example.test" {
		t.Fatalf("server-derived base = %q", got)
	}

	opts, err = parseArgs([]string{"discover", "--server", "wss://example.test/ws"})
	if err != nil {
		t.Fatal(err)
	}

	if got := discoverBaseURL(opts); got != "https://example.test" {
		t.Fatalf("wss-derived base = %q", got)
	}

	opts, err = parseArgs([]string{"discover", "--host", "192.168.1.42", "--port", "9000"})
	if err != nil {
		t.Fatal(err)
	}

	if got := discoverBaseURL(opts); got != "http://192.168.1.42:9000" {
		t.Fatalf("host-derived base = %q", got)
	}
}

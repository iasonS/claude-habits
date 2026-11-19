package main

import (
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	good := map[string]time.Duration{"": 0, "30d": 30 * 24 * time.Hour, "12h": 12 * time.Hour, "90m": 90 * time.Minute}
	for text, want := range good {
		if got, err := parseSince(text); err != nil || got != want {
			t.Errorf("parseSince(%q) = %v, %v; want %v", text, got, err, want)
		}
	}
	for _, text := range []string{"d", "-3d", "0d", "soon", "-1h"} {
		if _, err := parseSince(text); err == nil {
			t.Errorf("parseSince(%q) gave no error", text)
		}
	}
}

func TestParseArgsTakesFlagsOnEitherSide(t *testing.T) {
	opts, err := parseArgs([]string{"--top", "5", "sequences", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.command != "sequences" || opts.top != 5 || !opts.json {
		t.Fatalf("got %+v", opts)
	}
	if _, err := parseArgs([]string{"sequence"}); err == nil {
		t.Fatal("no error for an unknown command")
	}
}

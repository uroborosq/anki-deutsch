package wiktextract

import (
	"anki/internal/deckbuilder/usecase"
	"anki/internal/lexicon"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Client must satisfy the outbound Dictionary port.
var _ usecase.Dictionary = (*Client)(nil)

// writeCompact builds a compact dump (one lexicon.Word JSON per line) from the
// raw kaikki fixtures, mirroring what cmd/wikt-import produces.
func writeCompact(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "compact.jsonl")

	wf, err := os.Create(out)
	if err != nil {
		t.Fatalf("create compact: %v", err)
	}
	defer func() { _ = wf.Close() }()

	enc := json.NewEncoder(wf)

	for _, fixture := range []string{"haus.jsonl", "lernen.jsonl", "schnell.jsonl", "obst.jsonl"} {
		rf, err := os.Open(filepath.Join("testdata", fixture))
		if err != nil {
			t.Fatalf("open %s: %v", fixture, err)
		}

		sc := bufio.NewScanner(rf)
		sc.Buffer(make([]byte, 0, 1<<20), 1<<24)

		for sc.Scan() {
			if w, ok := ParseEntry(sc.Bytes()); ok {
				if err := enc.Encode(w); err != nil {
					t.Fatalf("encode: %v", err)
				}
			}
		}

		_ = rf.Close()
	}

	return out
}

func TestClientLookup(t *testing.T) {
	c, err := Open(writeCompact(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if c.Len() == 0 {
		t.Fatal("no lemmas loaded")
	}

	w, err := c.Lookup(context.Background(), "Haus")
	if err != nil {
		t.Fatalf("Lookup Haus: %v", err)
	}

	if w.Plural != "Häuser" {
		t.Errorf("Plural = %q, want Häuser", w.Plural)
	}

	// Case-insensitive fallback: lowercase "haus" still resolves.
	if _, err := c.Lookup(context.Background(), "haus"); err != nil {
		t.Errorf("folded Lookup haus: %v", err)
	}

	if _, err := c.Lookup(context.Background(), "Nichtdawort"); !errors.Is(err, lexicon.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

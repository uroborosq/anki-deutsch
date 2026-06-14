package wiktextract

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"anki/internal/lexicon"
)

// Client is an offline lexicon.Dictionary that answers lookups from a compact
// dump produced by cmd/wikt-import: one JSON-encoded lexicon.Word per line.
// The whole (trimmed) set is held in memory keyed by lemma.
type Client struct {
	exact  map[string]*lexicon.Word
	folded map[string]*lexicon.Word // lowercased fallback for casing drift
}

// Open loads a compact dump into memory. The first entry for a lemma wins.
func Open(path string) (*Client, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("wiktextract: open dump: %w", err)
	}
	defer func() { _ = f.Close() }()

	c := &Client{exact: map[string]*lexicon.Word{}, folded: map[string]*lexicon.Word{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)

	for sc.Scan() {
		var w lexicon.Word
		if err := json.Unmarshal(sc.Bytes(), &w); err != nil || w.Lemma == "" {
			continue
		}

		word := w
		if _, ok := c.exact[w.Lemma]; !ok {
			c.exact[w.Lemma] = &word
		}

		fold := strings.ToLower(w.Lemma)
		if _, ok := c.folded[fold]; !ok {
			c.folded[fold] = &word
		}
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("wiktextract: read dump: %w", err)
	}

	return c, nil
}

// Len reports how many lemmas were loaded.
func (c *Client) Len() int { return len(c.exact) }

// Lookup returns the word for lemma, trying an exact match first and a
// case-insensitive fallback second (German nouns are capitalized, but cards in
// the wild often aren't). Returns lexicon.ErrNotFound when absent.
func (c *Client) Lookup(_ context.Context, lemma string) (*lexicon.Word, error) {
	if w, ok := c.exact[lemma]; ok {
		return w, nil
	}

	if w, ok := c.folded[strings.ToLower(lemma)]; ok {
		return w, nil
	}

	return nil, lexicon.ErrNotFound
}

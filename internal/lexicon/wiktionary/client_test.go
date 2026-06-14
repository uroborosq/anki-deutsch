package wiktionary

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"anki/internal/deckbuilder/usecase"
	"anki/internal/lexicon"
)

// Client must satisfy the outbound Dictionary port.
var _ usecase.Dictionary = (*Client)(nil)

// fixtureServer serves the given testdata file for any request.
func fixtureServer(t *testing.T, fixture string) *httptest.Server {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sanity-check the query the client builds.
		q := r.URL.Query()
		if q.Get("action") != "query" || q.Get("titles") == "" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func newTestClient(t *testing.T, fixture string) *Client {
	t.Helper()
	srv := fixtureServer(t, fixture)

	return NewClient(nil, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

func TestLookupNoun(t *testing.T) {
	c := newTestClient(t, "noun_haus.json")

	got, err := c.Lookup(context.Background(), "Haus")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if got.PartOfSpeech != lexicon.Noun {
		t.Errorf("PartOfSpeech = %v, want Noun", got.PartOfSpeech)
	}

	if got.Gender != lexicon.Neuter {
		t.Errorf("Gender = %v, want Neuter", got.Gender)
	}

	if got.Plural != "Häuser" {
		t.Errorf("Plural = %q, want %q", got.Plural, "Häuser")
	}

	if len(got.Definitions) == 0 {
		t.Errorf("Definitions empty, want at least one")
	}

	if len(got.Definitions) > 0 && got.Definitions[0] != "Gebäude zum Wohnen" {
		t.Errorf("Definitions[0] = %q, want %q", got.Definitions[0], "Gebäude zum Wohnen")
	}
}

func TestLookupVerb(t *testing.T) {
	c := newTestClient(t, "verb_lernen.json")

	got, err := c.Lookup(context.Background(), "lernen")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if got.PartOfSpeech != lexicon.Verb {
		t.Errorf("PartOfSpeech = %v, want Verb", got.PartOfSpeech)
	}

	if got.PartizipII != "gelernt" {
		t.Errorf("PartizipII = %q, want %q", got.PartizipII, "gelernt")
	}

	if got.Auxiliary != "haben" {
		t.Errorf("Auxiliary = %q, want %q", got.Auxiliary, "haben")
	}

	if got.Praeteritum != "lernte" {
		t.Errorf("Praeteritum = %q, want %q", got.Praeteritum, "lernte")
	}

	if got.Reflexive {
		t.Errorf("Reflexive = true, want false for %q", got.Lemma)
	}
}

func TestLookupReflexiveLemma(t *testing.T) {
	c := newTestClient(t, "verb_lernen.json")

	got, err := c.Lookup(context.Background(), "sich waschen")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if !got.Reflexive {
		t.Errorf("Reflexive = false, want true for lemma containing 'sich'")
	}
}

func TestLookupAdjective(t *testing.T) {
	c := newTestClient(t, "adjective_schoen.json")

	got, err := c.Lookup(context.Background(), "schön")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if got.PartOfSpeech != lexicon.Adjective {
		t.Errorf("PartOfSpeech = %v, want Adjective", got.PartOfSpeech)
	}

	if got.Comparative != "schöner" {
		t.Errorf("Comparative = %q, want %q", got.Comparative, "schöner")
	}

	if got.Superlative != "am schönsten" {
		t.Errorf("Superlative = %q, want %q", got.Superlative, "am schönsten")
	}
}

func TestLookupMissing(t *testing.T) {
	c := newTestClient(t, "missing.json")

	_, err := c.Lookup(context.Background(), "Nichtexistentwort")
	if !errors.Is(err, lexicon.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

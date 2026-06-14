package wiktextract

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"anki/internal/lexicon"
)

// loadWord scans a fixture dump for the first German lemma entry named lemma.
func loadWord(t *testing.T, fixture, lemma string) *lexicon.Word {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<24) // entries can be large (full inflection tables)

	for sc.Scan() {
		w, ok := ParseEntry(sc.Bytes())
		if ok && w.Lemma == lemma {
			return w
		}
	}

	t.Fatalf("lemma %q not found in %s", lemma, fixture)

	return nil
}

func contains_(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}

	return false
}

func TestParseNoun(t *testing.T) {
	w := loadWord(t, "haus.jsonl", "Haus")
	if w.PartOfSpeech != lexicon.Noun {
		t.Errorf("PartOfSpeech = %v, want Noun", w.PartOfSpeech)
	}

	if w.Gender != lexicon.Neuter {
		t.Errorf("Gender = %v, want Neuter", w.Gender)
	}

	if w.Plural != "Häuser" {
		t.Errorf("Plural = %q, want Häuser", w.Plural)
	}

	if len(w.Definitions) == 0 {
		t.Error("Definitions empty")
	}

	if !contains_(w.Russian, "дом") {
		t.Errorf("Russian = %v, want to contain дом", w.Russian)
	}
}

func TestParseNounUncountable(t *testing.T) {
	w := loadWord(t, "obst.jsonl", "Obst")
	if w.Gender != lexicon.Neuter {
		t.Errorf("Gender = %v, want Neuter", w.Gender)
	}

	if w.Plural != "" {
		t.Errorf("Plural = %q, want empty (uncountable)", w.Plural)
	}
}

func TestParseVerb(t *testing.T) {
	w := loadWord(t, "lernen.jsonl", "lernen")
	if w.PartOfSpeech != lexicon.Verb {
		t.Errorf("PartOfSpeech = %v, want Verb", w.PartOfSpeech)
	}

	if w.PartizipII != "gelernt" {
		t.Errorf("PartizipII = %q, want gelernt", w.PartizipII)
	}

	if w.Auxiliary != "haben" {
		t.Errorf("Auxiliary = %q, want haben", w.Auxiliary)
	}

	if w.Praeteritum != "lernte" {
		t.Errorf("Praeteritum = %q, want lernte", w.Praeteritum)
	}

	if !contains_(w.Russian, "учить") {
		t.Errorf("Russian = %v, want to contain учить", w.Russian)
	}
}

func TestParseAdjective(t *testing.T) {
	w := loadWord(t, "schnell.jsonl", "schnell")
	if w.PartOfSpeech != lexicon.Adjective {
		t.Errorf("PartOfSpeech = %v, want Adjective", w.PartOfSpeech)
	}

	if w.Comparative != "schneller" {
		t.Errorf("Comparative = %q, want schneller", w.Comparative)
	}

	if w.Superlative != "am schnellsten" {
		t.Errorf("Superlative = %q, want am schnellsten", w.Superlative)
	}

	if !contains_(w.Russian, "быстрый") {
		t.Errorf("Russian = %v, want to contain быстрый", w.Russian)
	}
}

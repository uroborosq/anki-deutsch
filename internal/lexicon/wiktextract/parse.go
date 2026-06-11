// Package wiktextract is an offline lexicon Dictionary backed by the
// machine-readable German Wiktionary extraction from kaikki.org (the wiktextract
// project). Each line of the dump is one JSON entry; ParseEntry maps a relevant
// entry to a lexicon.Word, and Client serves lookups from a loaded compact dump.
package wiktextract

import (
	"encoding/json"
	"strings"

	"anki/internal/lexicon"
)

// rawEntry mirrors the subset of the kaikki dewiktionary JSON we consume. The
// dump carries far more (examples, sounds, full inflection tables); we ignore it.
type rawEntry struct {
	Word     string   `json:"word"`
	Pos      string   `json:"pos"`
	LangCode string   `json:"lang_code"`
	Tags     []string `json:"tags"` // gender for nouns; "form-of" marks inflected pages.
	Senses   []struct {
		Glosses []string `json:"glosses"`
		Tags    []string `json:"tags"`
	} `json:"senses"`
	Forms []struct {
		Form     string   `json:"form"`
		Tags     []string `json:"tags"`
		Pronouns []string `json:"pronouns"`
	} `json:"forms"`
	Translations []struct {
		Word     string `json:"word"`
		LangCode string `json:"lang_code"`
	} `json:"translations"`
}

// ParseEntry maps one kaikki JSON line to a lexicon.Word. ok is false when the
// entry is not a German lemma we card (wrong language, or an inflected "form-of"
// page like "Genitiv Singular des Substantivs …").
func ParseEntry(line []byte) (*lexicon.Word, bool) {
	var e rawEntry
	if err := json.Unmarshal(line, &e); err != nil {
		return nil, false
	}
	if e.LangCode != "de" || e.Word == "" || contains(e.Tags, "form-of") {
		return nil, false
	}

	w := &lexicon.Word{
		Lemma:        e.Word,
		PartOfSpeech: partOfSpeech(e.Pos),
		Definitions:  e.definitions(),
		Russian:      e.russian(),
	}

	switch w.PartOfSpeech {
	case lexicon.Noun:
		w.Gender = gender(e.Tags)
		w.Plural = e.form(func(t []string) bool { return contains(t, "nominative") && contains(t, "plural") })
	case lexicon.Verb:
		w.PartizipII = e.form(func(t []string) bool { return contains(t, "participle") && contains(t, "perfect") })
		w.Auxiliary = e.form(func(t []string) bool { return contains(t, "auxiliary") })
		w.Praeteritum = e.form(func(t []string) bool { return len(t) == 1 && t[0] == "past" })
		w.Reflexive = e.reflexive()
	case lexicon.Adjective:
		w.Comparative = e.form(func(t []string) bool { return len(t) == 1 && t[0] == "comparative" })
		w.Superlative = e.form(func(t []string) bool { return len(t) == 1 && t[0] == "superlative" })
	}
	return w, true
}

func partOfSpeech(pos string) lexicon.PartOfSpeech {
	switch pos {
	case "noun":
		return lexicon.Noun
	case "verb":
		return lexicon.Verb
	case "adj":
		return lexicon.Adjective
	default:
		return lexicon.Unknown
	}
}

func gender(tags []string) lexicon.Gender {
	switch {
	case contains(tags, "masculine"):
		return lexicon.Masculine
	case contains(tags, "feminine"):
		return lexicon.Feminine
	case contains(tags, "neuter"):
		return lexicon.Neuter
	default:
		return lexicon.NoGender
	}
}

// Cards stay readable: keep only the leading senses/translations. Wiktionary
// lists up to a dozen+ senses for common words, which would bury the card.
const (
	maxDefinitions = 3
	maxRussian     = 6
)

// definitions returns the German gloss of each real sense (capped), skipping
// inflection senses that are tagged "form-of".
func (e rawEntry) definitions() []string {
	var defs []string
	for _, s := range e.Senses {
		if contains(s.Tags, "form-of") || len(s.Glosses) == 0 {
			continue
		}
		if g := strings.TrimSpace(strings.Join(s.Glosses, " ")); g != "" {
			defs = append(defs, g)
			if len(defs) == maxDefinitions {
				break
			}
		}
	}
	return defs
}

// russian returns distinct Russian translations in dump order (capped).
func (e rawEntry) russian() []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range e.Translations {
		if t.LangCode != "ru" {
			continue
		}
		w := strings.TrimSpace(t.Word)
		if w != "" && !seen[w] {
			seen[w] = true
			out = append(out, w)
			if len(out) == maxRussian {
				break
			}
		}
	}
	return out
}

// form returns the first single-token form whose tags satisfy match, e.g. the
// bare "lernte"/"gelernt"/"Häuser" headword rather than a full clause like
// "ich lernte". Multi-word forms (auxiliary "haben" aside) are skipped.
func (e rawEntry) form(match func(tags []string) bool) string {
	for _, f := range e.Forms {
		if !match(f.Tags) {
			continue
		}
		v := strings.TrimSpace(f.Form)
		// "am schnellsten" is the canonical superlative, so allow a leading "am".
		if v == "" || (strings.Contains(v, " ") && !strings.HasPrefix(v, "am ")) {
			continue
		}
		return v
	}
	return ""
}

// reflexive reports whether any sense is marked reflexive.
func (e rawEntry) reflexive() bool {
	for _, s := range e.Senses {
		if contains(s.Tags, "reflexive") {
			return true
		}
	}
	return false
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

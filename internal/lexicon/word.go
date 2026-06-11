// Package lexicon is the subdomain holding linguistic knowledge of a German
// word. Its root package contains only the model; the Wiktionary adapter lives
// in the lexicon/wiktionary subpackage.
package lexicon

import "errors"

// ErrNotFound is returned by a Dictionary when the word has no entry.
var ErrNotFound = errors.New("lexicon: word not found")

// PartOfSpeech is the grammatical category that decides which fields are filled.
type PartOfSpeech int

const (
	Unknown PartOfSpeech = iota
	Noun
	Verb
	Adjective
)

func (p PartOfSpeech) String() string {
	switch p {
	case Noun:
		return "noun"
	case Verb:
		return "verb"
	case Adjective:
		return "adjective"
	default:
		return "unknown"
	}
}

// Gender is the grammatical gender of a noun.
type Gender int

const (
	NoGender Gender = iota
	Masculine
	Feminine
	Neuter
)

// Article returns the definite article (der/die/das) or "" when unknown.
func (g Gender) Article() string {
	switch g {
	case Masculine:
		return "der"
	case Feminine:
		return "die"
	case Neuter:
		return "das"
	default:
		return ""
	}
}

// Word is the looked-up grammatical information for a German lemma. Which
// of the part-of-speech-specific fields are populated depends on PartOfSpeech.
type Word struct {
	Lemma        string
	PartOfSpeech PartOfSpeech
	Definitions  []string // German meaning lines from {{Bedeutungen}}.
	Russian      []string // Russian translations from the {{Übersetzungen}} section.

	// Noun.
	Gender Gender
	Plural string

	// Verb.
	PartizipII  string
	Auxiliary   string // "haben" or "sein"
	Praeteritum string
	Reflexive   bool

	// Adjective.
	Comparative string
	Superlative string
}

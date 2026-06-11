// Package deckbuilder is the core subdomain: turning bare German words into rich
// Anki cards (single-word add and whole-deck scan/fill). Its root package holds
// only the model; the use-case layer lives in the deckbuilder/usecase
// subpackage.
package deckbuilder

// Suggestion is a proposed enrichment of one existing note produced by a deck
// scan. Fields holds the note fields to overwrite (e.g. {"Back": "..."}). When
// Skipped is true the lookup failed and Reason explains why; Fields is then nil.
type Suggestion struct {
	NoteID  uint64
	Lemma   string
	Fields  map[string]string
	Preview string
	Skipped bool
	Reason  string
}

// Package flashcard is the subdomain for Anki cards and decks. Its root package
// contains only the model and its invariants; the AnkiConnect-backed adapter
// lives in the flashcard/anki subpackage.
package flashcard

import "strings"

// DeckName identifies an Anki deck.
type DeckName string

// Note is an Anki note in the "Front/Back" shape used by this app. ID is zero
// for a not-yet-created note.
type Note struct {
	ID    uint64
	Front string
	Back  string
	Tags  []string
}

// IsComplete reports whether a note already carries its translation/grammar.
// A note is incomplete when its Back is blank — that is what the deck scan
// looks for and fills in.
func (n Note) IsComplete() bool {
	return strings.TrimSpace(n.Back) != ""
}

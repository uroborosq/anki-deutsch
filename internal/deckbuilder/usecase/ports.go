// Package usecase is the application/use-case layer of the deckbuilder core. It
// declares the ports it needs (consumer-defined interfaces) and orchestrates the
// lexicon and flashcard subdomains. Adapters satisfy the outbound ports
// structurally without importing this package.
package usecase

import (
	"context"

	"anki/internal/deckbuilder"
	"anki/internal/flashcard"
	"anki/internal/lexicon"
)

// Dictionary is the outbound port for looking up a German word. Implemented by
// lexicon/wiktionary.
type Dictionary interface {
	Lookup(ctx context.Context, lemma string) (*lexicon.Word, error)
}

// Cards is the outbound port for the Anki card store. Implemented by
// flashcard/anki.
type Cards interface {
	Add(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error)
	Decks(ctx context.Context) ([]flashcard.DeckName, error)
	FindIncomplete(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error)
	Update(ctx context.Context, id uint64, fields map[string]string) error
}

// Service is the inbound port driven by the UI. The concrete implementation is
// the private service struct; the TUI depends on this interface only.
type Service interface {
	// PreviewWord looks up a word and builds the Note that would be added,
	// without writing to Anki, so the UI can show it for confirmation.
	PreviewWord(ctx context.Context, lemma string) (flashcard.Note, error)
	// AddNote stores an already-built (and user-confirmed) Note in the deck.
	AddNote(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error)
	// AddWord previews and adds a word in one step (PreviewWord + AddNote).
	AddWord(ctx context.Context, deck flashcard.DeckName, lemma string) (uint64, error)
	ScanDeck(ctx context.Context, deck flashcard.DeckName) ([]deckbuilder.Suggestion, error)
	Apply(ctx context.Context, suggestions []deckbuilder.Suggestion) (updated, skipped int, err error)
	Decks(ctx context.Context) ([]flashcard.DeckName, error)
}

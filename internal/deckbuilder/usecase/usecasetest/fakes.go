// Package usecasetest provides programmable test doubles for the usecase ports
// and Service, shared by the core tests (agent D) and the TUI tests (agent E) so
// their mocks do not diverge. The fakes satisfy the usecase interfaces
// structurally and therefore import only the model packages, not usecase, which
// keeps them free of import cycles.
package usecasetest

import (
	"anki/internal/deckbuilder"
	"anki/internal/flashcard"
	"anki/internal/lexicon"
	"context"
)

// FakeDictionary is a programmable usecase.Dictionary.
type FakeDictionary struct {
	LookupFunc func(ctx context.Context, lemma string) (*lexicon.Word, error)
}

func (f FakeDictionary) Lookup(ctx context.Context, lemma string) (*lexicon.Word, error) {
	return f.LookupFunc(ctx, lemma)
}

// FakeCards is a programmable usecase.Cards.
type FakeCards struct {
	AddFunc            func(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error)
	DecksFunc          func(ctx context.Context) ([]flashcard.DeckName, error)
	FindIncompleteFunc func(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error)
	UpdateFunc         func(ctx context.Context, id uint64, fields map[string]string) error
}

func (f FakeCards) Add(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
	return f.AddFunc(ctx, deck, note)
}

func (f FakeCards) Decks(ctx context.Context) ([]flashcard.DeckName, error) {
	return f.DecksFunc(ctx)
}

func (f FakeCards) FindIncomplete(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error) {
	return f.FindIncompleteFunc(ctx, deck)
}

func (f FakeCards) Update(ctx context.Context, id uint64, fields map[string]string) error {
	return f.UpdateFunc(ctx, id, fields)
}

// FakeService is a programmable usecase.Service for driving the TUI in tests.
type FakeService struct {
	PreviewWordFunc func(ctx context.Context, lemma string) (flashcard.Note, error)
	AddNoteFunc     func(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error)
	AddWordFunc     func(ctx context.Context, deck flashcard.DeckName, lemma string) (uint64, error)
	ScanDeckFunc    func(ctx context.Context, deck flashcard.DeckName) ([]deckbuilder.Suggestion, error)
	ApplyFunc       func(ctx context.Context, suggestions []deckbuilder.Suggestion) (int, int, error)
	DecksFunc       func(ctx context.Context) ([]flashcard.DeckName, error)
}

func (f FakeService) PreviewWord(ctx context.Context, lemma string) (flashcard.Note, error) {
	return f.PreviewWordFunc(ctx, lemma)
}

func (f FakeService) AddNote(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
	return f.AddNoteFunc(ctx, deck, note)
}

func (f FakeService) AddWord(ctx context.Context, deck flashcard.DeckName, lemma string) (uint64, error) {
	return f.AddWordFunc(ctx, deck, lemma)
}

func (f FakeService) ScanDeck(ctx context.Context, deck flashcard.DeckName) ([]deckbuilder.Suggestion, error) {
	return f.ScanDeckFunc(ctx, deck)
}

func (f FakeService) Apply(ctx context.Context, suggestions []deckbuilder.Suggestion) (int, int, error) {
	return f.ApplyFunc(ctx, suggestions)
}

func (f FakeService) Decks(ctx context.Context) ([]flashcard.DeckName, error) {
	return f.DecksFunc(ctx)
}

package usecase

import (
	"context"
	"errors"

	"anki/internal/deckbuilder"
	"anki/internal/flashcard"
	"anki/internal/lexicon"
)

// service is the concrete Service implementation. It depends only on the ports.
type service struct {
	dict  Dictionary
	cards Cards
}

// NewService wires the use cases to their outbound ports.
func NewService(dict Dictionary, cards Cards) Service {
	return &service{dict: dict, cards: cards}
}

var _ Service = (*service)(nil)

// PreviewWord looks up a word and translates it to a Note without writing to
// Anki, so the UI can show the card and ask for confirmation before AddNote.
func (s *service) PreviewWord(ctx context.Context, lemma string) (flashcard.Note, error) {
	// Users type the word as it appears on a card ("das Obst", "sich freuen"),
	// but Wiktionary pages are titled by the bare lemma. Strip the article/sich
	// before lookup, same as ScanDeck does; translate() re-adds the article from
	// the looked-up gender.
	lemma = extractLemma(lemma)

	w, err := s.dict.Lookup(ctx, lemma)
	if err != nil {
		return flashcard.Note{}, err
	}

	return translate(w), nil
}

// AddNote stores an already-built Note (typically one returned by PreviewWord
// and confirmed by the user) in the deck.
func (s *service) AddNote(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
	return s.cards.Add(ctx, deck, note)
}

// AddWord looks up a word and adds it to a deck in one step, without the
// preview/confirm round-trip. Kept for non-interactive callers.
func (s *service) AddWord(ctx context.Context, deck flashcard.DeckName, lemma string) (uint64, error) {
	note, err := s.PreviewWord(ctx, lemma)
	if err != nil {
		return 0, err
	}

	return s.AddNote(ctx, deck, note)
}

// ScanDeck finds incomplete notes in a deck and proposes enrichments.
func (s *service) ScanDeck(ctx context.Context, deck flashcard.DeckName) ([]deckbuilder.Suggestion, error) {
	notes, err := s.cards.FindIncomplete(ctx, deck)
	if err != nil {
		return nil, err
	}

	suggestions := make([]deckbuilder.Suggestion, 0, len(notes))
	for _, note := range notes {
		lemma := extractLemma(note.Front)

		w, err := s.dict.Lookup(ctx, lemma)
		if err != nil {
			reason := "not found"
			if !errors.Is(err, lexicon.ErrNotFound) {
				reason = err.Error()
			}

			suggestions = append(suggestions, deckbuilder.Suggestion{
				NoteID:  note.ID,
				Lemma:   lemma,
				Skipped: true,
				Reason:  reason,
			})

			continue
		}

		back := translate(w).Back
		suggestions = append(suggestions, deckbuilder.Suggestion{
			NoteID:  note.ID,
			Lemma:   lemma,
			Fields:  map[string]string{"Back": back},
			Preview: back,
		})
	}

	return suggestions, nil
}

// Apply writes the selected suggestions back to their notes.
func (s *service) Apply(ctx context.Context, suggestions []deckbuilder.Suggestion) (int, int, error) {
	var updated, skipped int

	for _, sg := range suggestions {
		if sg.Skipped {
			skipped++
			continue
		}

		if err := s.cards.Update(ctx, sg.NoteID, sg.Fields); err != nil {
			skipped++
			continue
		}

		updated++
	}

	return updated, skipped, nil
}

// Decks lists the available decks (passthrough so the UI need not know Cards).
func (s *service) Decks(ctx context.Context) ([]flashcard.DeckName, error) {
	return s.cards.Decks(ctx)
}

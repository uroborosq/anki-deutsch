package tui

import (
	"anki/internal/deckbuilder"
	"anki/internal/flashcard"
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// Messages flowing back into Update from the asynchronous Service commands. Each
// Service call has a success message and an error message so Update can branch on
// the message type alone.
type (
	decksMsg     struct{ decks []flashcard.DeckName }
	decksErrMsg  struct{ err error }
	lookupErrMsg struct{ err error }

	previewDoneMsg struct{ note flashcard.Note }

	addDoneMsg struct{ id uint64 }
	addErrMsg  struct{ err error }

	scanDoneMsg struct{ suggestions []deckbuilder.Suggestion }
	scanErrMsg  struct{ err error }

	applyDoneMsg struct {
		updated int
		skipped int
	}
	applyErrMsg struct{ err error }
)

// decksCmd fetches the deck list. Fired from Init when no default deck is set.
func decksCmd(svc Service) tea.Cmd {
	return func() tea.Msg {
		decks, err := svc.Decks(context.Background())
		if err != nil {
			return decksErrMsg{err}
		}

		return decksMsg{decks}
	}
}

// previewWordCmd looks up a word and builds its card without writing to Anki,
// so the UI can show it for confirmation.
func previewWordCmd(svc Service, lemma string) tea.Cmd {
	return func() tea.Msg {
		note, err := svc.PreviewWord(context.Background(), lemma)
		if err != nil {
			return lookupErrMsg{err}
		}

		return previewDoneMsg{note}
	}
}

// addNoteCmd stores a previewed, confirmed note in the deck.
func addNoteCmd(svc Service, deck flashcard.DeckName, note flashcard.Note) tea.Cmd {
	return func() tea.Msg {
		id, err := svc.AddNote(context.Background(), deck, note)
		if err != nil {
			return addErrMsg{err}
		}

		return addDoneMsg{id}
	}
}

// scanDeckCmd scans a deck for incomplete notes and returns suggestions.
func scanDeckCmd(svc Service, deck flashcard.DeckName) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := svc.ScanDeck(context.Background(), deck)
		if err != nil {
			return scanErrMsg{err}
		}

		return scanDoneMsg{suggestions}
	}
}

// applyCmd applies the chosen suggestions.
func applyCmd(svc Service, suggestions []deckbuilder.Suggestion) tea.Cmd {
	return func() tea.Msg {
		updated, skipped, err := svc.Apply(context.Background(), suggestions)
		if err != nil {
			return applyErrMsg{err}
		}

		return applyDoneMsg{updated: updated, skipped: skipped}
	}
}

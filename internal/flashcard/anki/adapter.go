// Package anki adapts the low-level AnkiConnect client to the deckbuilder's
// Cards outbound port, translating between the flashcard domain model and the
// AnkiConnect DTOs.
package anki

import (
	"anki/internal/flashcard"
	"anki/pkg/ankiconnect"
	"context"
	"fmt"
)

// Adapter implements the deckbuilder Cards port on top of an AnkiConnect client.
type Adapter struct {
	client *ankiconnect.Client
	model  string
}

// NewAdapter returns an Adapter that creates new notes with the given Anki note
// type (model), e.g. "Basic". Notes use the "Front" and "Back" fields.
func NewAdapter(client *ankiconnect.Client, model string) *Adapter {
	return &Adapter{client: client, model: model}
}

// Add creates a note in deck and returns its new id.
func (a *Adapter) Add(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
	return a.client.AddNote(ctx, ankiconnect.AddNoteParams{
		DeckName:  string(deck),
		ModelName: a.model,
		Fields: map[string]string{
			"Front": note.Front,
			"Back":  note.Back,
		},
		Tags: note.Tags,
	})
}

// Decks lists all deck names.
func (a *Adapter) Decks(ctx context.Context) ([]flashcard.DeckName, error) {
	names, err := a.client.DeckNames(ctx)
	if err != nil {
		return nil, err
	}

	decks := make([]flashcard.DeckName, len(names))
	for i, name := range names {
		decks[i] = flashcard.DeckName(name)
	}

	return decks, nil
}

// FindIncomplete returns the notes in deck whose Back is still blank.
func (a *Adapter) FindIncomplete(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error) {
	ids, err := a.client.FindNotes(ctx, fmt.Sprintf("deck:%q", deck))
	if err != nil {
		return nil, err
	}

	infos, err := a.client.NotesInfo(ctx, ids)
	if err != nil {
		return nil, err
	}

	var incomplete []flashcard.Note

	for _, info := range infos {
		note := flashcard.Note{
			ID:    info.NoteID,
			Front: info.Fields["Front"].Value,
			Back:  info.Fields["Back"].Value,
			Tags:  info.Tags,
		}
		if !note.IsComplete() {
			incomplete = append(incomplete, note)
		}
	}

	return incomplete, nil
}

// Update overwrites the given fields of the note with id.
func (a *Adapter) Update(ctx context.Context, id uint64, fields map[string]string) error {
	return a.client.UpdateNoteFields(ctx, id, fields)
}

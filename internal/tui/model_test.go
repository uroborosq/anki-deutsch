package tui

import (
	"anki/internal/deckbuilder"
	"anki/internal/deckbuilder/usecase/usecasetest"
	"anki/internal/flashcard"
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// update applies a message and returns the concrete *Model for assertions.
func update(t *testing.T, m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	t.Helper()

	next, cmd := m.Update(msg)

	got, ok := next.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *Model", next)
	}

	return got, cmd
}

func TestNewSkipsDeckPickWithDefaultDeck(t *testing.T) {
	m := New(usecasetest.FakeService{}, "Default")
	if m.state != stateModePick {
		t.Fatalf("state = %d, want stateModePick", m.state)
	}

	if m.deck != "Default" {
		t.Fatalf("deck = %q, want Default", m.deck)
	}

	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init with default deck should not fetch decks")
	}
}

func TestNewWithoutDefaultDeckFetchesDecks(t *testing.T) {
	m := New(usecasetest.FakeService{}, "")
	if m.state != stateDeckPick {
		t.Fatalf("state = %d, want stateDeckPick", m.state)
	}

	if m.Init() == nil {
		t.Fatalf("Init without default deck should return a decks command")
	}
}

func TestDecksMsgPopulatesDeckPicker(t *testing.T) {
	m := New(usecasetest.FakeService{}, "")

	m, _ = update(t, m, decksMsg{decks: []flashcard.DeckName{"A", "B"}})
	if m.state != stateDeckPick {
		t.Fatalf("state = %d, want stateDeckPick", m.state)
	}

	if n := len(m.deckList.Items()); n != 2 {
		t.Fatalf("deck list has %d items, want 2", n)
	}

	// Selecting a deck advances to the mode picker.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateModePick {
		t.Fatalf("state = %d, want stateModePick after enter", m.state)
	}

	if m.deck != "A" {
		t.Fatalf("deck = %q, want A", m.deck)
	}
}

func TestDecksErrMsgStaysOnDeckPick(t *testing.T) {
	m := New(usecasetest.FakeService{}, "")

	m, _ = update(t, m, decksErrMsg{err: errors.New("boom")})
	if m.state != stateDeckPick {
		t.Fatalf("state = %d, want stateDeckPick", m.state)
	}

	if m.lastErr == nil {
		t.Fatalf("expected lastErr to be recorded")
	}
}

func TestAddFlowSuccess(t *testing.T) {
	note := flashcard.Note{Front: "der Hund", Back: "собака", Tags: []string{"german", "auto"}}
	svc := usecasetest.FakeService{
		PreviewWordFunc: func(_ context.Context, lemma string) (flashcard.Note, error) {
			return note, nil
		},
		AddNoteFunc: func(_ context.Context, deck flashcard.DeckName, n flashcard.Note) (uint64, error) {
			return 42, nil
		},
	}
	m := New(svc, "Deutsch")

	// Enter the add screen from the mode picker.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateAddInput {
		t.Fatalf("state = %d, want stateAddInput", m.state)
	}

	// Type a word and submit: this only looks the word up.
	m.input.SetValue("Hund")

	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateAddLoading {
		t.Fatalf("state = %d, want stateAddLoading", m.state)
	}

	if cmd == nil {
		t.Fatalf("expected a command to run PreviewWord")
	}

	// The lookup result lands on the preview screen — nothing is added yet.
	m, _ = update(t, m, previewDoneMsg{note: note})
	if m.state != stateAddPreview {
		t.Fatalf("state = %d, want stateAddPreview", m.state)
	}

	if m.previewNote.Front != "der Hund" {
		t.Fatalf("previewNote.Front = %q, want der Hund", m.previewNote.Front)
	}

	// Confirming with enter writes the note to Anki.
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateAddSaving {
		t.Fatalf("state = %d, want stateAddSaving", m.state)
	}

	if cmd == nil {
		t.Fatalf("expected a command to run AddNote")
	}

	// The done message shows the new note id.
	m, _ = update(t, m, addDoneMsg{id: 42})
	if m.state != stateAddResult {
		t.Fatalf("state = %d, want stateAddResult", m.state)
	}

	if m.lastID != 42 || m.lastErr != nil {
		t.Fatalf("lastID=%d lastErr=%v, want 42/nil", m.lastID, m.lastErr)
	}

	// Any key returns to the input for another word.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.state != stateAddInput {
		t.Fatalf("state = %d, want stateAddInput after result", m.state)
	}
}

func TestAddPreviewCancel(t *testing.T) {
	note := flashcard.Note{Front: "der Hund", Back: "собака"}
	svc := usecasetest.FakeService{
		AddNoteFunc: func(_ context.Context, _ flashcard.DeckName, _ flashcard.Note) (uint64, error) {
			t.Fatal("AddNote must not be called when the preview is cancelled")
			return 0, nil
		},
	}
	m := New(svc, "Deutsch")
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> add input
	m.input.SetValue("Hund")
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> loading
	m, _ = update(t, m, previewDoneMsg{note: note})     // -> preview

	// Esc on the preview discards it and returns to the input.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.state != stateAddInput {
		t.Fatalf("state = %d, want stateAddInput after cancel", m.state)
	}
}

func TestAddFlowLookupError(t *testing.T) {
	svc := usecasetest.FakeService{
		PreviewWordFunc: func(_ context.Context, _ string) (flashcard.Note, error) {
			return flashcard.Note{}, errors.New("not found")
		},
	}
	m := New(svc, "Deutsch")
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> add input
	m.input.SetValue("Hund")
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> loading

	// A lookup failure skips the preview and shows the result screen.
	m, _ = update(t, m, lookupErrMsg{err: errors.New("not found")})
	if m.state != stateAddResult {
		t.Fatalf("state = %d, want stateAddResult", m.state)
	}

	if m.lastErr == nil {
		t.Fatalf("expected lastErr after lookupErrMsg")
	}
}

func TestAddFlowSaveError(t *testing.T) {
	m := New(usecasetest.FakeService{}, "Deutsch")
	m.state = stateAddSaving

	m, _ = update(t, m, addErrMsg{err: errors.New("anki offline")})
	if m.state != stateAddResult {
		t.Fatalf("state = %d, want stateAddResult", m.state)
	}

	if m.lastErr == nil {
		t.Fatalf("expected lastErr after addErrMsg")
	}
}

func TestScanFlow(t *testing.T) {
	svc := usecasetest.FakeService{
		ScanDeckFunc: func(_ context.Context, _ flashcard.DeckName) ([]deckbuilder.Suggestion, error) {
			return nil, nil
		},
		ApplyFunc: func(_ context.Context, s []deckbuilder.Suggestion) (int, int, error) {
			return len(s), 0, nil
		},
	}
	m := New(svc, "Deutsch")

	// Move selection to "Scan deck" and start scanning.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyDown})

	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateScanLoading {
		t.Fatalf("state = %d, want stateScanLoading", m.state)
	}

	if cmd == nil {
		t.Fatalf("expected a command to run ScanDeck")
	}

	// Scan results land on the review list with fillable rows preselected.
	suggestions := []deckbuilder.Suggestion{
		{NoteID: 1, Lemma: "Hund", Fields: map[string]string{"Back": "der Hund"}, Preview: "der Hund"},
		{NoteID: 2, Lemma: "xyz", Skipped: true, Reason: "not found"},
	}

	m, _ = update(t, m, scanDoneMsg{suggestions: suggestions})
	if m.state != stateScanList {
		t.Fatalf("state = %d, want stateScanList", m.state)
	}

	if !m.selected[0] {
		t.Fatalf("fillable suggestion should be preselected")
	}

	if m.selected[1] {
		t.Fatalf("skipped suggestion should not be preselected")
	}

	// Only the preselected, non-skipped suggestion is queued for Apply.
	if chosen := m.chosenSuggestions(); len(chosen) != 1 || chosen[0].NoteID != 1 {
		t.Fatalf("chosen = %+v, want exactly note 1", chosen)
	}

	// Apply the selection.
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.state != stateApplyLoading {
		t.Fatalf("state = %d, want stateApplyLoading", m.state)
	}

	if cmd == nil {
		t.Fatalf("expected a command to run Apply")
	}

	// Apply result shows the summary.
	m, _ = update(t, m, applyDoneMsg{updated: 1, skipped: 0})
	if m.state != stateScanSummary {
		t.Fatalf("state = %d, want stateScanSummary", m.state)
	}

	if m.updated != 1 {
		t.Fatalf("updated = %d, want 1", m.updated)
	}

	// Any key returns to the mode picker.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.state != stateModePick {
		t.Fatalf("state = %d, want stateModePick", m.state)
	}
}

// TestCommandsCallService exercises the tea.Cmd wrappers directly, confirming
// they invoke the Service off the Update path and map results to messages.
func TestCommandsCallService(t *testing.T) {
	svc := usecasetest.FakeService{
		DecksFunc: func(_ context.Context) ([]flashcard.DeckName, error) {
			return []flashcard.DeckName{"A"}, nil
		},
		PreviewWordFunc: func(_ context.Context, _ string) (flashcard.Note, error) {
			return flashcard.Note{Front: "der Hund"}, nil
		},
		AddNoteFunc: func(_ context.Context, _ flashcard.DeckName, _ flashcard.Note) (uint64, error) {
			return 7, nil
		},
		ScanDeckFunc: func(_ context.Context, _ flashcard.DeckName) ([]deckbuilder.Suggestion, error) {
			return []deckbuilder.Suggestion{{NoteID: 1}}, nil
		},
		ApplyFunc: func(_ context.Context, s []deckbuilder.Suggestion) (int, int, error) {
			return len(s), 2, nil
		},
	}

	if msg := decksCmd(svc)(); msg.(decksMsg).decks[0] != "A" {
		t.Fatalf("decksCmd returned %#v", msg)
	}

	if msg := previewWordCmd(svc, "Hund")(); msg.(previewDoneMsg).note.Front != "der Hund" {
		t.Fatalf("previewWordCmd returned %#v", msg)
	}

	if msg := addNoteCmd(svc, "D", flashcard.Note{})(); msg.(addDoneMsg).id != 7 {
		t.Fatalf("addNoteCmd returned %#v", msg)
	}

	if msg := scanDeckCmd(svc, "D")(); len(msg.(scanDoneMsg).suggestions) != 1 {
		t.Fatalf("scanDeckCmd returned %#v", msg)
	}

	got := applyCmd(svc, []deckbuilder.Suggestion{{NoteID: 1}})().(applyDoneMsg)
	if got.updated != 1 || got.skipped != 2 {
		t.Fatalf("applyCmd returned %#v", got)
	}

	// Error paths map to the matching error messages.
	errSvc := usecasetest.FakeService{
		DecksFunc: func(_ context.Context) ([]flashcard.DeckName, error) {
			return nil, errors.New("x")
		},
		PreviewWordFunc: func(_ context.Context, _ string) (flashcard.Note, error) {
			return flashcard.Note{}, errors.New("x")
		},
		AddNoteFunc: func(_ context.Context, _ flashcard.DeckName, _ flashcard.Note) (uint64, error) {
			return 0, errors.New("x")
		},
	}
	if _, ok := decksCmd(errSvc)().(decksErrMsg); !ok {
		t.Fatalf("decksCmd error path wrong type")
	}

	if _, ok := previewWordCmd(errSvc, "w")().(lookupErrMsg); !ok {
		t.Fatalf("previewWordCmd error path wrong type")
	}

	if _, ok := addNoteCmd(errSvc, "D", flashcard.Note{})().(addErrMsg); !ok {
		t.Fatalf("addNoteCmd error path wrong type")
	}
}

func TestScanToggleAndApplyEmpty(t *testing.T) {
	m := New(usecasetest.FakeService{
		ApplyFunc: func(_ context.Context, s []deckbuilder.Suggestion) (int, int, error) {
			return 0, 0, nil
		},
	}, "Deutsch")
	m.state = stateScanList
	m.setSuggestions([]deckbuilder.Suggestion{
		{NoteID: 1, Lemma: "Hund", Fields: map[string]string{"Back": "der Hund"}},
	})
	// Space toggles the row off.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if m.selected[0] {
		t.Fatalf("space should have toggled selection off")
	}

	if got := m.chosenSuggestions(); len(got) != 0 {
		t.Fatalf("chosen = %d, want 0 after deselect", len(got))
	}
}

func TestScanErrGoesToSummary(t *testing.T) {
	m := New(usecasetest.FakeService{}, "Deutsch")

	m, _ = update(t, m, scanErrMsg{err: errors.New("offline")})
	if m.state != stateScanSummary {
		t.Fatalf("state = %d, want stateScanSummary", m.state)
	}

	if m.lastErr == nil {
		t.Fatalf("expected lastErr after scanErrMsg")
	}
}

func TestQuitKeys(t *testing.T) {
	m := New(usecasetest.FakeService{}, "Deutsch")
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatalf("ctrl+c should quit")
	}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Fatalf("q should quit from mode pick")
	}
}

func TestWindowSizeDoesNotChangeState(t *testing.T) {
	m := New(usecasetest.FakeService{}, "Deutsch")

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.state != stateModePick {
		t.Fatalf("state = %d, want stateModePick after resize", m.state)
	}

	if m.width != 80 || m.height != 24 {
		t.Fatalf("size = %dx%d, want 80x24", m.width, m.height)
	}
}

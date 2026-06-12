package usecase_test

import (
	"anki/internal/deckbuilder"
	"anki/internal/deckbuilder/usecase"
	"anki/internal/deckbuilder/usecase/usecasetest"
	"anki/internal/flashcard"
	"anki/internal/lexicon"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestTranslate(t *testing.T) {
	tests := []struct {
		name      string
		word      *lexicon.Word
		wantFront string
		wantBack  string
	}{
		{
			name: "noun",
			word: &lexicon.Word{
				Lemma:        "Haus",
				PartOfSpeech: lexicon.Noun,
				Gender:       lexicon.Neuter,
				Plural:       "Häuser",
				Definitions:  []string{"house", "building"},
			},
			wantFront: "das Haus",
			wantBack:  "Plural: die Häuser\n──\nhouse\nbuilding",
		},
		{
			name: "noun with russian translation",
			word: &lexicon.Word{
				Lemma:        "Obst",
				PartOfSpeech: lexicon.Noun,
				Gender:       lexicon.Neuter,
				Russian:      []string{"фрукты", "плоды"},
				Definitions:  []string{"essbare Früchte"},
			},
			wantFront: "das Obst",
			wantBack:  "фрукты, плоды\n──\nessbare Früchte",
		},
		{
			name: "noun drops no-plural placeholder",
			word: &lexicon.Word{
				Lemma:        "Obst",
				PartOfSpeech: lexicon.Noun,
				Gender:       lexicon.Neuter,
				Plural:       "—",
				Russian:      []string{"фрукты"},
			},
			wantFront: "das Obst",
			wantBack:  "фрукты",
		},
		{
			name: "noun without article or plural",
			word: &lexicon.Word{
				Lemma:        "Milch",
				PartOfSpeech: lexicon.Noun,
				Definitions:  []string{"milk"},
			},
			wantFront: "Milch",
			wantBack:  "milk",
		},
		{
			name: "verb reflexive",
			word: &lexicon.Word{
				Lemma:        "freuen",
				PartOfSpeech: lexicon.Verb,
				PartizipII:   "gefreut",
				Auxiliary:    "haben",
				Praeteritum:  "freute",
				Reflexive:    true,
				Definitions:  []string{"to be glad"},
			},
			wantFront: "sich freuen",
			wantBack:  "Partizip II: gefreut · Hilfsverb: haben\nPräteritum: freute\n──\nto be glad",
		},
		{
			name: "verb non-reflexive",
			word: &lexicon.Word{
				Lemma:        "gehen",
				PartOfSpeech: lexicon.Verb,
				PartizipII:   "gegangen",
				Auxiliary:    "sein",
				Definitions:  []string{"to go"},
			},
			wantFront: "gehen",
			wantBack:  "Partizip II: gegangen · Hilfsverb: sein\n──\nto go",
		},
		{
			name: "adjective",
			word: &lexicon.Word{
				Lemma:        "schnell",
				PartOfSpeech: lexicon.Adjective,
				Comparative:  "schneller",
				Superlative:  "am schnellsten",
				Definitions:  []string{"fast"},
			},
			wantFront: "schnell",
			wantBack:  "Komparativ: schneller\nSuperlativ: am schnellsten\n──\nfast",
		},
		{
			name: "unknown",
			word: &lexicon.Word{
				Lemma:       "hallo",
				Definitions: []string{"hello", "hi"},
			},
			wantFront: "hallo",
			wantBack:  "hello\nhi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := usecaseTranslate(t, tt.word)
			if note.Front != tt.wantFront {
				t.Errorf("Front = %q, want %q", note.Front, tt.wantFront)
			}

			if note.Back != tt.wantBack {
				t.Errorf("Back = %q, want %q", note.Back, tt.wantBack)
			}

			wantTags := []string{"german", "auto"}
			if !reflect.DeepEqual(note.Tags, wantTags) {
				t.Errorf("Tags = %v, want %v", note.Tags, wantTags)
			}
		})
	}
}

// usecaseTranslate exercises translate through the public AddWord path, since
// translate itself is unexported. It captures the Note handed to Cards.Add.
func usecaseTranslate(t *testing.T, w *lexicon.Word) flashcard.Note {
	t.Helper()

	var got flashcard.Note

	dict := usecasetest.FakeDictionary{
		LookupFunc: func(ctx context.Context, lemma string) (*lexicon.Word, error) {
			return w, nil
		},
	}
	cards := usecasetest.FakeCards{
		AddFunc: func(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
			got = note
			return 1, nil
		},
	}

	svc := usecase.NewService(dict, cards)
	if _, err := svc.AddWord(context.Background(), "deck", w.Lemma); err != nil {
		t.Fatalf("AddWord returned error: %v", err)
	}

	return got
}

func TestExtractLemma(t *testing.T) {
	tests := []struct {
		front string
		want  string
	}{
		{"das Haus", "Haus"},
		{"der Tisch", "Tisch"},
		{"die Katze", "Katze"},
		{"ein Mann", "Mann"},
		{"eine Frau", "Frau"},
		{"sich freuen", "freuen"},
		{"gehen", "gehen"},
		{"  das Haus  ", "Haus"},
	}
	for _, tt := range tests {
		// extractLemma is unexported; exercise it via ScanDeck's lookup arg.
		got := scanLemma(t, tt.front)
		if got != tt.want {
			t.Errorf("extractLemma(%q) = %q, want %q", tt.front, got, tt.want)
		}
	}
}

// scanLemma recovers the lemma extractLemma produced by observing the value
// ScanDeck passes to Dictionary.Lookup.
func scanLemma(t *testing.T, front string) string {
	t.Helper()

	var seen string

	dict := usecasetest.FakeDictionary{
		LookupFunc: func(ctx context.Context, lemma string) (*lexicon.Word, error) {
			seen = lemma
			return nil, lexicon.ErrNotFound
		},
	}
	cards := usecasetest.FakeCards{
		FindIncompleteFunc: func(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error) {
			return []flashcard.Note{{ID: 1, Front: front}}, nil
		},
	}

	svc := usecase.NewService(dict, cards)
	if _, err := svc.ScanDeck(context.Background(), "deck"); err != nil {
		t.Fatalf("ScanDeck returned error: %v", err)
	}

	return seen
}

func TestAddWordHappyPath(t *testing.T) {
	word := &lexicon.Word{
		Lemma:        "Haus",
		PartOfSpeech: lexicon.Noun,
		Gender:       lexicon.Neuter,
		Plural:       "Häuser",
		Definitions:  []string{"house"},
	}

	var (
		gotDeck flashcard.DeckName
		gotNote flashcard.Note
	)

	dict := usecasetest.FakeDictionary{
		LookupFunc: func(ctx context.Context, lemma string) (*lexicon.Word, error) {
			return word, nil
		},
	}
	cards := usecasetest.FakeCards{
		AddFunc: func(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
			gotDeck = deck
			gotNote = note

			return 42, nil
		},
	}
	svc := usecase.NewService(dict, cards)

	id, err := svc.AddWord(context.Background(), "vocab", "Haus")
	if err != nil {
		t.Fatalf("AddWord error: %v", err)
	}

	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}

	if gotDeck != "vocab" {
		t.Errorf("deck = %q, want vocab", gotDeck)
	}

	want := flashcard.Note{
		Front: "das Haus",
		Back:  "Plural: die Häuser\n──\nhouse",
		Tags:  []string{"german", "auto"},
	}
	if !reflect.DeepEqual(gotNote, want) {
		t.Errorf("note = %+v, want %+v", gotNote, want)
	}
}

func TestAddWordLookupError(t *testing.T) {
	wantErr := errors.New("boom")
	dict := usecasetest.FakeDictionary{
		LookupFunc: func(ctx context.Context, lemma string) (*lexicon.Word, error) {
			return nil, wantErr
		},
	}
	cards := usecasetest.FakeCards{
		AddFunc: func(ctx context.Context, deck flashcard.DeckName, note flashcard.Note) (uint64, error) {
			t.Fatal("Add should not be called on lookup error")
			return 0, nil
		},
	}

	svc := usecase.NewService(dict, cards)
	if _, err := svc.AddWord(context.Background(), "deck", "x"); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

func TestScanDeckFound(t *testing.T) {
	word := &lexicon.Word{
		Lemma:        "Haus",
		PartOfSpeech: lexicon.Noun,
		Gender:       lexicon.Neuter,
		Plural:       "Häuser",
		Definitions:  []string{"house"},
	}
	dict := usecasetest.FakeDictionary{
		LookupFunc: func(ctx context.Context, lemma string) (*lexicon.Word, error) {
			return word, nil
		},
	}
	cards := usecasetest.FakeCards{
		FindIncompleteFunc: func(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error) {
			return []flashcard.Note{{ID: 7, Front: "das Haus"}}, nil
		},
	}
	svc := usecase.NewService(dict, cards)

	got, err := svc.ScanDeck(context.Background(), "deck")
	if err != nil {
		t.Fatalf("ScanDeck error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(got))
	}

	wantBack := "Plural: die Häuser\n──\nhouse"

	want := deckbuilder.Suggestion{
		NoteID:  7,
		Lemma:   "Haus",
		Fields:  map[string]string{"Back": wantBack},
		Preview: wantBack,
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("suggestion = %+v, want %+v", got[0], want)
	}
}

func TestScanDeckNotFound(t *testing.T) {
	dict := usecasetest.FakeDictionary{
		LookupFunc: func(ctx context.Context, lemma string) (*lexicon.Word, error) {
			return nil, lexicon.ErrNotFound
		},
	}
	cards := usecasetest.FakeCards{
		FindIncompleteFunc: func(ctx context.Context, deck flashcard.DeckName) ([]flashcard.Note, error) {
			return []flashcard.Note{{ID: 9, Front: "der Quux"}}, nil
		},
	}
	svc := usecase.NewService(dict, cards)

	got, err := svc.ScanDeck(context.Background(), "deck")
	if err != nil {
		t.Fatalf("ScanDeck error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(got))
	}

	want := deckbuilder.Suggestion{
		NoteID:  9,
		Lemma:   "Quux",
		Skipped: true,
		Reason:  "not found",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("suggestion = %+v, want %+v", got[0], want)
	}
}

func TestApply(t *testing.T) {
	var updates []uint64

	cards := usecasetest.FakeCards{
		UpdateFunc: func(ctx context.Context, id uint64, fields map[string]string) error {
			updates = append(updates, id)
			return nil
		},
	}
	dict := usecasetest.FakeDictionary{}
	svc := usecase.NewService(dict, cards)

	suggestions := []deckbuilder.Suggestion{
		{NoteID: 1, Fields: map[string]string{"Back": "a"}},
		{NoteID: 2, Skipped: true, Reason: "not found"},
		{NoteID: 3, Fields: map[string]string{"Back": "c"}},
	}

	updated, skipped, err := svc.Apply(context.Background(), suggestions)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	if updated != 2 {
		t.Errorf("updated = %d, want 2", updated)
	}

	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}

	if !reflect.DeepEqual(updates, []uint64{1, 3}) {
		t.Errorf("updated ids = %v, want [1 3]", updates)
	}
}

func TestApplyUpdateErrorCountsSkipped(t *testing.T) {
	cards := usecasetest.FakeCards{
		UpdateFunc: func(ctx context.Context, id uint64, fields map[string]string) error {
			if id == 2 {
				return errors.New("update failed")
			}

			return nil
		},
	}
	svc := usecase.NewService(usecasetest.FakeDictionary{}, cards)
	suggestions := []deckbuilder.Suggestion{
		{NoteID: 1, Fields: map[string]string{"Back": "a"}},
		{NoteID: 2, Fields: map[string]string{"Back": "b"}},
	}

	updated, skipped, err := svc.Apply(context.Background(), suggestions)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	if updated != 1 || skipped != 1 {
		t.Errorf("updated=%d skipped=%d, want 1 and 1", updated, skipped)
	}
}

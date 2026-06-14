package anki_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"anki/internal/deckbuilder/usecase"
	"anki/internal/flashcard"
	"anki/internal/flashcard/anki"
	"anki/pkg/ankiconnect"
)

// compile-time assertion that the adapter satisfies the outbound port.
var _ usecase.Cards = (*anki.Adapter)(nil)

// newServer returns an httptest.Server that records the last request envelope and
// replies with the given AnkiConnect body. The captured envelope is returned via
// the pointer for assertions.
func newServer(t *testing.T, response string, captured *map[string]any) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		if captured != nil {
			var env map[string]any
			if err := json.Unmarshal(body, &env); err != nil {
				t.Errorf("decode request envelope: %v", err)
			}

			*captured = env
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func newClient(base string) *ankiconnect.Client {
	return ankiconnect.NewClient(slog.New(slog.DiscardHandler), base)
}

func TestDecks(t *testing.T) {
	srv := newServer(t, `{"result":["Default","DEUTSCH"],"error":null}`, nil)
	a := anki.NewAdapter(newClient(srv.URL), "Basic")

	decks, err := a.Decks(context.Background())
	if err != nil {
		t.Fatalf("Decks: %v", err)
	}

	want := []flashcard.DeckName{"Default", "DEUTSCH"}
	if len(decks) != len(want) {
		t.Fatalf("got %d decks, want %d", len(decks), len(want))
	}

	for i := range want {
		if decks[i] != want[i] {
			t.Errorf("decks[%d] = %q, want %q", i, decks[i], want[i])
		}
	}
}

func TestAdd(t *testing.T) {
	var env map[string]any

	srv := newServer(t, `{"result":1496198395707,"error":null}`, &env)
	a := anki.NewAdapter(newClient(srv.URL), "Basic")

	id, err := a.Add(context.Background(), "DEUTSCH", flashcard.Note{
		Front: "Haus",
		Back:  "house",
		Tags:  []string{"noun"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if id != 1496198395707 {
		t.Errorf("id = %d, want 1496198395707", id)
	}

	if env["action"] != "addNote" {
		t.Errorf("action = %v, want addNote", env["action"])
	}

	if env["version"] != float64(6) {
		t.Errorf("version = %v, want 6", env["version"])
	}

	params, ok := env["params"].(map[string]any)
	if !ok {
		t.Fatalf("params not an object: %v", env["params"])
	}

	note, ok := params["note"].(map[string]any)
	if !ok {
		t.Fatalf("params.note not an object: %v", params["note"])
	}

	if note["deckName"] != "DEUTSCH" {
		t.Errorf("deckName = %v, want DEUTSCH", note["deckName"])
	}

	if note["modelName"] != "Basic" {
		t.Errorf("modelName = %v, want Basic", note["modelName"])
	}

	fields, ok := note["fields"].(map[string]any)
	if !ok {
		t.Fatalf("note.fields not an object: %v", note["fields"])
	}

	if fields["Front"] != "Haus" {
		t.Errorf("fields.Front = %v, want Haus", fields["Front"])
	}

	if fields["Back"] != "house" {
		t.Errorf("fields.Back = %v, want house", fields["Back"])
	}
}

func TestFindIncomplete(t *testing.T) {
	// First call: findNotes -> ids. Second call: notesInfo -> infos.
	responses := []string{
		`{"result":[1,2,3],"error":null}`,
		`{"result":[
			{"noteId":1,"modelName":"Basic","tags":["noun"],"fields":{"Front":{"value":"Haus","order":0},"Back":{"value":"house","order":1}}},
			{"noteId":2,"modelName":"Basic","tags":[],"fields":{"Front":{"value":"Tisch","order":0},"Back":{"value":"","order":1}}},
			{"noteId":3,"modelName":"Basic","tags":["verb"],"fields":{"Front":{"value":"gehen","order":0},"Back":{"value":"   ","order":1}}}
		],"error":null}`,
	}

	var queries []string

	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var env struct {
			Action string         `json:"action"`
			Params map[string]any `json:"params"`
		}

		_ = json.Unmarshal(body, &env)
		if env.Action == "findNotes" {
			queries = append(queries, env.Params["query"].(string))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, responses[i])
		i++
	}))
	t.Cleanup(srv.Close)

	a := anki.NewAdapter(newClient(srv.URL), "Basic")

	notes, err := a.FindIncomplete(context.Background(), "DEUTSCH")
	if err != nil {
		t.Fatalf("FindIncomplete: %v", err)
	}

	if len(queries) != 1 || queries[0] != `deck:"DEUTSCH"` {
		t.Errorf("findNotes query = %v, want [deck:\"DEUTSCH\"]", queries)
	}

	// Only notes 2 and 3 have a blank Back.
	if len(notes) != 2 {
		t.Fatalf("got %d incomplete notes, want 2: %+v", len(notes), notes)
	}

	if notes[0].ID != 2 || notes[0].Front != "Tisch" || notes[0].Back != "" {
		t.Errorf("notes[0] = %+v, want id 2 Tisch/empty", notes[0])
	}

	if notes[1].ID != 3 || notes[1].Front != "gehen" {
		t.Errorf("notes[1] = %+v, want id 3 gehen", notes[1])
	}

	if len(notes[1].Tags) != 1 || notes[1].Tags[0] != "verb" {
		t.Errorf("notes[1].Tags = %v, want [verb]", notes[1].Tags)
	}
}

func TestUpdate(t *testing.T) {
	var env map[string]any

	srv := newServer(t, `{"result":null,"error":null}`, &env)
	a := anki.NewAdapter(newClient(srv.URL), "Basic")

	err := a.Update(context.Background(), 42, map[string]string{"Back": "house"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if env["action"] != "updateNoteFields" {
		t.Errorf("action = %v, want updateNoteFields", env["action"])
	}

	if env["version"] != float64(6) {
		t.Errorf("version = %v, want 6", env["version"])
	}

	params := env["params"].(map[string]any)

	note := params["note"].(map[string]any)
	if note["id"] != float64(42) {
		t.Errorf("note.id = %v, want 42", note["id"])
	}

	fields := note["fields"].(map[string]any)
	if fields["Back"] != "house" {
		t.Errorf("note.fields.Back = %v, want house", fields["Back"])
	}
}

func TestErrorPropagates(t *testing.T) {
	srv := newServer(t, `{"result":null,"error":"deck not found"}`, nil)
	a := anki.NewAdapter(newClient(srv.URL), "Basic")

	_, err := a.Decks(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Package ankiconnect is a low-level client for the AnkiConnect add-on HTTP API.
//
// Every call POSTs a single JSON envelope {"action","version","params"} to one
// base URL; AnkiConnect always answers HTTP 200 with {"result","error"} and
// reports failures via the error field. This package holds no application domain
// types — adapters translate between the domain and these DTOs.
package ankiconnect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Client talks to a single AnkiConnect endpoint.
type Client struct {
	log    *slog.Logger
	client *http.Client
	base   string
}

// NewClient returns a Client posting to base (e.g. http://localhost:8765).
func NewClient(log *slog.Logger, base string) *Client {
	return &Client{log: log, client: &http.Client{}, base: base}
}

// AddNoteParams is the payload of the addNote action (wrapped under "note").
type AddNoteParams struct {
	DeckName  string            `json:"deckName"`
	ModelName string            `json:"modelName"`
	Fields    map[string]string `json:"fields"`
	Tags      []string          `json:"tags"`
	Options   *AddNoteOptions   `json:"options,omitempty"`
}

// AddNoteOptions controls duplicate handling for addNote.
type AddNoteOptions struct {
	AllowDuplicate bool `json:"allowDuplicate"`
}

// NoteInfo is one entry of the notesInfo result.
type NoteInfo struct {
	NoteID    uint64                `json:"noteId"`
	ModelName string                `json:"modelName"`
	Tags      []string              `json:"tags"`
	Fields    map[string]FieldValue `json:"fields"`
}

// FieldValue is a single field of a note as returned by notesInfo.
type FieldValue struct {
	Value string `json:"value"`
	Order int    `json:"order"`
}

// DeckNames returns all deck names (action deckNames).
func (c *Client) DeckNames(ctx context.Context) ([]string, error) {
	var out []string
	if err := c.do(ctx, "deckNames", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddNote creates a note and returns its id (action addNote).
func (c *Client) AddNote(ctx context.Context, p AddNoteParams) (uint64, error) {
	var id uint64
	if err := c.do(ctx, "addNote", map[string]any{"note": p}, &id); err != nil {
		return 0, err
	}
	return id, nil
}

// FindNotes returns note ids matching an Anki search query (action findNotes).
func (c *Client) FindNotes(ctx context.Context, query string) ([]uint64, error) {
	var ids []uint64
	if err := c.do(ctx, "findNotes", map[string]any{"query": query}, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// NotesInfo returns field/model info for the given note ids (action notesInfo).
func (c *Client) NotesInfo(ctx context.Context, ids []uint64) ([]NoteInfo, error) {
	var out []NoteInfo
	if err := c.do(ctx, "notesInfo", map[string]any{"notes": ids}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateNoteFields overwrites the given fields of a note, leaving others intact
// (action updateNoteFields).
func (c *Client) UpdateNoteFields(ctx context.Context, id uint64, fields map[string]string) error {
	note := map[string]any{"id": id, "fields": fields}
	return c.do(ctx, "updateNoteFields", map[string]any{"note": note}, nil)
}

// do builds the AnkiConnect envelope, posts it, and decodes {result,error}.
// AnkiConnect signals failures in the error field on an HTTP 200, so the body
// must always be read. result may be nil for actions without a payload.
func (c *Client) do(ctx context.Context, action string, params, result any) error {
	type envelope struct {
		Action  string `json:"action"`
		Version int    `json:"version"`
		Params  any    `json:"params,omitempty"`
	}
	body, err := json.Marshal(envelope{Action: action, Version: 6, Params: params})
	if err != nil {
		return fmt.Errorf("ankiconnect: encode %s: %w", action, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ankiconnect: build %s: %w", action, err)
	}
	req.Header.Set("Content-Type", "application/json")
	// AnkiConnect rejects requests whose Origin is not in its webCorsOriginList
	// (default ["http://localhost"]) with HTTP 403; a missing Origin also fails.
	req.Header.Set("Origin", "http://localhost")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("ankiconnect: request %s: %w", action, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ankiconnect: read %s: %w", action, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ankiconnect: %s: HTTP %d: %s", action, resp.StatusCode, bytes.TrimSpace(raw))
	}

	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *string         `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("ankiconnect: decode %s response: %w", action, err)
	}
	if env.Error != nil {
		return fmt.Errorf("ankiconnect: %s: %s", action, *env.Error)
	}
	if result != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, result); err != nil {
			return fmt.Errorf("ankiconnect: decode %s result: %w", action, err)
		}
	}
	return nil
}

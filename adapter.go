package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type Adapter struct {
	log    *slog.Logger
	client *http.Client
	base   string
}

func NewAdapter(log *slog.Logger, base string) *Adapter {
	return &Adapter{
		log:    log,
		client: &http.Client{},
		base:   base,
	}
}

func (s *Adapter) GetCards(ctx context.Context, deckName DeckName) ([]Card, error) {
	var 
	s.getRequest(ctx, "getDecks")

}

func (s *Adapter) getRequest(ctx context.Context, action string, req any) error {
	type request struct {
		any

		Version int    `json:"version"`
		Action  string `json:"action"`
	}

	encoded, err := json.Marshal(request{
		any:     req,
		Version: 6,
		Action:  action,
	})
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := s.client.Post(s.base, "application/json", bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

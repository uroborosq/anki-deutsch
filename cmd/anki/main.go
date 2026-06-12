// Command anki is a colorful TUI that enriches German words with grammatical
// information from German Wiktionary and writes them into a local Anki deck via
// the AnkiConnect add-on. It supports adding a single word and scanning a deck
// for incomplete cards to fill in.
//
// This is the composition root: it builds the AnkiConnect client, the
// flashcard/anki and lexicon adapters, the deckbuilder use-case service, and the
// Bubble Tea program, then runs it. Defaults target the user's setup (DEUTSCH
// deck, "Простая" note type, offline kaikki dump) so it runs with no flags.
package main

import (
	"anki/internal/deckbuilder/usecase"
	"anki/internal/lexicon/wiktextract"
	"anki/internal/lexicon/wiktionary"
	"anki/internal/tui"
	"anki/pkg/ankiconnect"
	"flag"
	"fmt"
	"log/slog"
	"os"

	ankiadapter "anki/internal/flashcard/anki"

	tea "github.com/charmbracelet/bubbletea"
)

// Defaults wire the user's usual setup so the TUI runs with no flags: the
// DEUTSCH deck, its "Простая" note type, and the offline kaikki dump produced by
// cmd/wikt-import. Any can be overridden by the matching flag.
const (
	defaultDeck     = "DEUTSCH"
	defaultModel    = "Простая"
	defaultDictPath = "data/de-compact.jsonl"
)

func main() {
	ankiURL := flag.String("anki", "http://localhost:8765", "AnkiConnect base URL")
	deck := flag.String("deck", defaultDeck, "target deck (skips the deck picker when set)")
	model := flag.String("model", defaultModel, "Anki note type for new cards")
	dictPath := flag.String("dict", defaultDictPath, "offline kaikki dump (compact JSONL from wikt-import); falls back to the live Wiktionary API when the file is absent")

	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	client := ankiconnect.NewClient(log, *ankiURL)
	cards := ankiadapter.NewAdapter(client, *model)

	dict, err := newDictionary(log, *dictPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	svc := usecase.NewService(dict, cards)

	program := tea.NewProgram(tui.New(svc, *deck), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// newDictionary picks the lexicon source: the offline kaikki dump when present
// (no network, no rate limits — for bulk adds), otherwise the live Wiktionary
// API. A missing dump is not an error: the tool still works online, just slower
// and rate-limited. Returns an error only when a present dump fails to load.
func newDictionary(log *slog.Logger, dictPath string) (usecase.Dictionary, error) {
	if dictPath == "" {
		return wiktionary.NewClient(log), nil
	}

	if _, err := os.Stat(dictPath); err != nil {
		fmt.Fprintf(os.Stderr, "offline dump %s not found — using live Wiktionary API (run cmd/wikt-import to enable offline)\n", dictPath)
		return wiktionary.NewClient(log), nil
	}

	c, err := wiktextract.Open(dictPath)
	if err != nil {
		return nil, err
	}

	log.Info("loaded offline dictionary", "lemmas", c.Len(), "path", dictPath)

	return c, nil
}

# anki-deutsch

A colorful terminal UI that turns German words into rich Anki flashcards. Type a
word — it looks up the grammar (gender, plural, Partizip II, comparison forms, …)
and a Russian translation in Wiktionary, builds a card, and writes it into your
local Anki deck through the [AnkiConnect](https://git.sr.ht/~foosoft/anki-connect)
add-on.

## Features

- **Add a word** — enter a German word, preview the generated card, confirm, repeat.
- **Scan a deck** — walk existing cards, find incomplete ones (no translation or
  missing the grammar expected for their part of speech) and fill in only the
  missing fields without touching what's already there.
- **Two interchangeable dictionary sources** behind one model:
  - **Live German Wiktionary** (`de.wiktionary.org`) — no key, always fresh, one
    HTTP request per word (rate-limited by Wikimedia on bulk adds).
  - **Offline kaikki.org dump** — a compact JSONL built once from the
    `dewiktionary` dump. Zero network, zero rate limits — the right choice for
    adding many words.
- **Meaningful color** — status (success/error/skipped), gender (der/die/das),
  and UI accents. Degrades to monochrome on non-TTY or under `NO_COLOR`.

## Card format

Note type `Basic` (Front/Back), tags `german`, `auto`. The Back is built top to
bottom: **Russian translation** (the headline) → word forms → the German gloss
under a `──` separator.

| Part of speech | Front      | Back (top → bottom)                                  |
| -------------- | ---------- | ---------------------------------------------------- |
| Noun           | `das Obst` | `фрукты` · `Plural: die …` · `──` · German gloss     |
| Verb           | `lernen`   | translation · `Partizip II: … · Hilfsverb: …` · `──` |
| Adjective      | `schön`    | translation · comparative/superlative · `──` · gloss |

## Requirements

- Go 1.25
- Anki running with the **AnkiConnect** add-on (local HTTP API at
  `http://localhost:8765`).
- Internet access to German Wiktionary (only when using the live source).

## Usage

Build and run the TUI:

```sh
go run ./cmd/anki
```

It runs with no flags, defaulting to the offline dump at `data/de-compact.jsonl`
and falling back to the live Wiktionary API if that file is absent.

### Flags

| Flag     | Default                  | Meaning                                                  |
| -------- | ------------------------ | -------------------------------------------------------- |
| `-anki`  | `http://localhost:8765`  | AnkiConnect base URL                                     |
| `-deck`  | `DEUTSCH`                | Target deck (skips the deck picker when set)            |
| `-model` | `Простая`                | Anki note type for new cards                            |
| `-dict`  | `data/de-compact.jsonl`  | Offline kaikki dump; falls back to the live API if absent |

To force the live source, point `-dict` at nothing:

```sh
go run ./cmd/anki -dict ""
```

## Offline dictionary (optional)

The offline source avoids Wikimedia's `HTTP 429` on bulk adds.

1. Download the kaikki.org `dewiktionary` Deutsch dump (~3.1 GB):
   `https://kaikki.org/dewiktionary/Deutsch/kaikki.org-dictionary-Deutsch.jsonl`
2. Compact it into the form the loader reads (`.jsonl`, `.gz` and `.bz2` inputs
   are accepted):

   ```sh
   go run ./cmd/wikt-import -in kaikki.org-dictionary-Deutsch.jsonl -out data/de-compact.jsonl
   ```

3. Run `cmd/anki` — it picks up `data/de-compact.jsonl` automatically. The raw
   dump can be deleted afterward.

## Project layout

Standard Go layout (`cmd` / `internal` / `pkg`) with a DDD split into bounded
subdomains — each owns its own model; adapters and use-cases live in subpackages.

```
cmd/anki/                 composition root: flags, wiring, run
cmd/wikt-import/          one-shot: raw kaikki dump -> compact JSONL

internal/
  lexicon/                model: Word, PartOfSpeech, Gender, grammar value objects
    wiktionary/           adapter: live German Wiktionary  -> lexicon.Word
    wiktextract/          adapter: offline kaikki dump      -> lexicon.Word
  flashcard/              model: Note, Field, Deck + completeness invariant
    anki/                 adapter: card storage via pkg/ankiconnect
  deckbuilder/            core model: Suggestion
    usecase/              use cases: AddWord, ScanDeck, Apply, Decks + ports
  tui/                    Bubble Tea UI (talks only to deckbuilder/usecase)

pkg/ankiconnect/          reusable low-level AnkiConnect HTTP client
```

See `docs/functional-spec.md` (what it does) and `docs/implementation-plan.md`
(how it's built) for details.

## Development

```sh
go build ./...               # build
go test ./...                # run tests
go vet ./...                 # static checks
golangci-lint run ./... --fix  # lint + gofumpt format
```

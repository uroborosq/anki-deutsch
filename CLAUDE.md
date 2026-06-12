# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go client for the [AnkiConnect](https://git.sr.ht/~foosoft/anki-connect) add-on, which exposes a local HTTP API for the Anki flashcard app at `http://localhost:8765`. The project is an early-stage skeleton — `main.go` is a stub and `adapter.go` has an incomplete `GetCards` method that does not yet compile.

## Design docs — keep in sync

The intended behavior and implementation plan live in `docs/`:

- `docs/functional-spec.md` — *what* the tool does (user scenarios, supported grammar, card format, TUI behavior, error handling).
- `docs/implementation-plan.md` — *how* it is built (file-by-file changes, types, adapter actions, TUI state machine, build order).
- `docs/subagent-plan.md` — how the implementation is split for parallel subagents (contract-freeze phase, per-package fan-out, integration phase).

**Always treat these docs as the source of truth and keep them in sync with the code.** Before implementing or changing a feature, read the relevant doc; after changing behavior, structure, or scope, update the doc in the same change so code and docs never drift. If the code and a doc disagree, reconcile them — don't silently diverge.

## External API reference

AnkiConnect API reference (request envelope + full action list — `findCards`, `cardsInfo`, `addNote`, `deckNames`, `findNotes`, `notesInfo`, `updateNoteFields`, etc.):

- Canonical home: https://git.sr.ht/~foosoft/anki-connect (the old `foosoft.net/projects/anki-connect` and `github.com/FooSoft/anki-connect` URLs are dead/moved).
- Fetchable raw mirror (use this to look up an action's params/result shape): https://raw.githubusercontent.com/amikey/anki-connect/master/README.md

## Commands

```sh
task build
go run .              # run main
task test
go test -run TestName # run a single test
task lint
task fmt
```

Module is `anki`. Requires Go 1.25.

### Shell gotcha — multi-line git commit messages

Default shell is PowerShell, but git/test commands are often run through the Bash tool. **PowerShell here-string syntax (`@'...'@`) is NOT valid in bash** — the `@` characters end up literally inside the commit message and force an `--amend`. When committing via the Bash tool, use repeated `-m` flags instead:

```sh
git commit -m "subject" -m "body paragraph" -m "Co-Authored-By: ..."
```

Reserve `@'...'@` for commands you actually run through the PowerShell tool.

## Project layout

Standard Go layout (`cmd` / `internal` / `pkg`) combined with a **DDD split into bounded subdomains**. Each subdomain owns its own model — do **not** collect all types into one shared `domain` / `models` package. **Layering rule: the root package of a subdomain holds only the model (value objects, entities, invariants). Everything else — adapters, use-case/application layers, ports — lives in subpackages.** (The repo currently still has the original flat `package main` at the root — migrate to this layout as code is implemented.)

```
cmd/anki/                 # composition root: flags, dependency wiring, run

internal/
  lexicon/                # SUBDOMAIN — root: MODEL only
    word.go               #   Word, PartOfSpeech, Gender, grammar value objects
    wiktionary/           #   adapter subpackage: German Wiktionary -> lexicon.Word
  flashcard/              # SUBDOMAIN — root: MODEL only
    note.go               #   Note, Field, Deck, DeckName + completeness invariant
    anki/                 #   adapter subpackage: card storage via pkg/ankiconnect
  deckbuilder/             # CORE SUBDOMAIN — root: MODEL only
    suggestion.go         #   Suggestion
    usecase/              #   use-case layer: AddWord, ScanDeck, Apply, Decks + ports
  tui/                    # interface layer: Bubble Tea UI (depends only on deckbuilder/usecase)

pkg/ankiconnect/          # reusable low-level AnkiConnect HTTP client (Adapter + envelope)
```

Rules of thumb: the entry point lives only under `cmd/`; a subdomain root is pure model, its adapters and application code go in subpackages. **Ports (interfaces) are declared by the consumer**, i.e. the use-case layer `deckbuilder/usecase` (Go idiom "accept interfaces"); adapters in `lexicon/wiktionary` and `flashcard/anki` satisfy them structurally without importing the core. Cross-subdomain mapping (lexicon `Word` → flashcard `Note`) is an explicit translation in `deckbuilder/usecase`, never a shared struct reused across subdomains. The TUI talks only to `deckbuilder/usecase`, never to a subdomain port directly. The AnkiConnect HTTP client is reusable, so it lives under `pkg/ankiconnect`; the `flashcard/anki` adapter translates flashcard model types to/from its DTOs.

## Architecture

`Adapter` (`pkg/ankiconnect`) is the low-level AnkiConnect client. All AnkiConnect calls POST a single JSON envelope to one base URL — there are no REST paths. The envelope wraps every request as `{"action": "...", "version": 6, "params": {...}}`. `getRequest` builds this envelope by embedding the caller's params struct anonymously alongside `Version` and `Action`. When adding new operations (e.g. `AddNote`, `FindNotes`), add a method on `Adapter` that defines its params type and delegates to the shared request helper rather than constructing HTTP requests directly. Keep this package free of app domain types — the `flashcard/anki` adapter maps between the domain and these DTOs.

Each subdomain root defines its own model (`lexicon.Word`, `flashcard.Note`, `deckbuilder.Suggestion`); logic that operates on them sits in subpackages. Testing AnkiConnect interactions requires a running Anki instance with the AnkiConnect add-on, or an `httptest.Server` standing in for `base`.

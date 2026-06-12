// Package wiktionary implements the lexicon Dictionary port against the German
// Wiktionary MediaWiki API (https://de.wiktionary.org/w/api.php). It fetches the
// raw wikitext of a lemma's page and parses the relevant overview templates and
// meaning lines into a lexicon.Word.
package wiktionary

import (
	"anki/internal/lexicon"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// defaultBaseURL is the German Wiktionary MediaWiki API endpoint.
const defaultBaseURL = "https://de.wiktionary.org/w/api.php"

// Client looks up German words via the Wiktionary MediaWiki API.
type Client struct {
	log     *slog.Logger
	baseURL string
	http    *http.Client
}

// Option customises a Client.
type Option func(*Client)

// WithBaseURL overrides the MediaWiki API base URL (e.g. an httptest server).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = baseURL }
}

// WithHTTPClient overrides the HTTP client used for requests.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.http = h
		}
	}
}

// NewClient builds a Client. A nil logger is replaced with a discard logger.
func NewClient(log *slog.Logger, opts ...Option) *Client {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	c := &Client{
		log:     log,
		baseURL: defaultBaseURL,
		http:    http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// apiResponse mirrors the MediaWiki query response shape we care about.
type apiResponse struct {
	Query struct {
		Pages map[string]apiPage `json:"pages"`
	} `json:"query"`
}

type apiPage struct {
	PageID    int         `json:"pageid"`
	Title     string      `json:"title"`
	Missing   *string     `json:"missing"`
	Revisions []apiRevisn `json:"revisions"`
}

type apiRevisn struct {
	Slots struct {
		Main struct {
			Content string `json:"*"`
		} `json:"main"`
	} `json:"slots"`
}

// Lookup queries Wiktionary for lemma and parses the result. It returns
// lexicon.ErrNotFound when the page is missing.
func (c *Client) Lookup(ctx context.Context, lemma string) (*lexicon.Word, error) {
	wikitext, err := c.fetch(ctx, lemma)
	if err != nil {
		return nil, err
	}

	word := parse(lemma, wikitext)

	return word, nil
}

// fetch retrieves the raw wikitext for lemma, or ErrNotFound if absent.
func (c *Client) fetch(ctx context.Context, lemma string) (string, error) {
	endpoint, err := c.requestURL(lemma)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("wiktionary: build request: %w", err)
	}
	// Wikimedia's API rejects requests without a descriptive User-Agent (HTTP
	// 403); see https://meta.wikimedia.org/wiki/User-Agent_policy.
	req.Header.Set("User-Agent", "anki-go/0.1 (https://github.com/; AnkiConnect deck builder)")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("wiktionary: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wiktionary: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("wiktionary: read body: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("wiktionary: decode response: %w", err)
	}

	for _, page := range parsed.Query.Pages {
		if page.Missing != nil || len(page.Revisions) == 0 {
			c.log.Debug("wiktionary lookup missing", "lemma", lemma)
			return "", lexicon.ErrNotFound
		}

		return page.Revisions[0].Slots.Main.Content, nil
	}

	return "", lexicon.ErrNotFound
}

// requestURL builds the MediaWiki query URL for lemma.
func (c *Client) requestURL(lemma string) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("wiktionary: parse base URL: %w", err)
	}

	q := url.Values{}
	q.Set("action", "query")
	q.Set("prop", "revisions")
	q.Set("rvslots", "main")
	q.Set("rvprop", "content")
	q.Set("format", "json")
	q.Set("titles", lemma)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

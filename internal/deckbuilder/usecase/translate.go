package usecase

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"anki/internal/flashcard"
	"anki/internal/lexicon"
)

// translate is the anti-corruption mapping from a lexicon.Word to a
// flashcard.Note. The output is deterministic so golden tests can pin it.
func translate(w *lexicon.Word) flashcard.Note {
	front := w.Lemma

	var forms []string

	switch w.PartOfSpeech {
	case lexicon.Noun:
		if a := w.Gender.Article(); a != "" {
			front = a + " " + w.Lemma
		}

		if p := normalizePlural(w.Plural); p != "" {
			forms = append(forms, "Plural: die "+p)
		}

	case lexicon.Verb:
		if w.Reflexive {
			front = "sich " + w.Lemma
		}

		var parts []string
		if w.PartizipII != "" {
			parts = append(parts, "Partizip II: "+w.PartizipII)
		}

		if w.Auxiliary != "" {
			parts = append(parts, "Hilfsverb: "+w.Auxiliary)
		}

		if len(parts) > 0 {
			forms = append(forms, strings.Join(parts, " · "))
		}

		if w.Praeteritum != "" {
			forms = append(forms, "Präteritum: "+w.Praeteritum)
		}

	case lexicon.Adjective:
		if w.Comparative != "" {
			forms = append(forms, "Komparativ: "+w.Comparative)
		}

		if w.Superlative != "" {
			forms = append(forms, "Superlativ: "+w.Superlative)
		}
	}

	// Back: Russian translation first, then word forms, then the German meaning
	// below a separator. The German block is supplementary context.
	var lines []string
	if ru := strings.Join(w.Russian, ", "); ru != "" {
		lines = append(lines, ru)
	}

	lines = append(lines, forms...)
	if len(w.Definitions) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "──")
		}

		lines = append(lines, w.Definitions...)
	}

	return flashcard.Note{
		Front: front,
		Back:  strings.Join(lines, "\n"),
		Tags:  []string{"german", "auto"},
	}
}

// normalizePlural drops Wiktionary's "no plural" placeholders (em/en dash or
// hyphen) so uncountable nouns don't get a bogus "Plural: die —" line.
func normalizePlural(p string) string {
	switch strings.TrimSpace(p) {
	case "", "—", "–", "-":
		return ""
	}

	return strings.TrimSpace(p)
}

var (
	// htmlNoise matches the markup that creeps into hand-typed Front fields:
	// tags like <br> and entities like &nbsp;.
	htmlNoise = regexp.MustCompile(`<[^>]*>|&[a-zA-Z]+;|&#\d+;`)
	// gluedSuffix matches a plural/feminine inflection hyphenated onto the lemma,
	// e.g. "Kaugummi-s", "Auto-s", "Lehrer-in".
	gluedSuffix = regexp.MustCompile(`(?i)-(s|e|n|en|er|nen|in|innen)$`)
)

// leadingArticles are the articles and the reflexive "sich" stripped from the
// front of a card before lookup.
var leadingArticles = map[string]bool{
	"der": true, "die": true, "das": true,
	"ein": true, "eine": true, "sich": true,
}

// extractLemma recovers the lookup word from an existing card's Front. Real
// decks are noisy: the Front carries the dictionary article (der/die/das), the
// reflexive "sich", and grammar shorthand that is not part of the lemma —
// single-letter or short plural markers ("Bauer n", "Bildschirm e",
// "Jahreszahl en"), bare symbols ("Gewerbe =", "Kloster = ö"), hyphenated
// inflections ("Kaugummi-s"), HTML leftovers ("Job<br>") and comma-separated
// alternatives ("Ursache n, der Grund"). All of that is dropped so only the
// headword is looked up.
func extractLemma(front string) string {
	s := htmlNoise.ReplaceAllString(front, " ")

	// Keep only the first alternative when several are crammed into one field.
	if i := strings.IndexAny(s, ",;/"); i >= 0 {
		s = s[:i]
	}

	fields := strings.Fields(s)

	// Drop leading articles and "sich".
	for len(fields) > 0 && leadingArticles[strings.ToLower(fields[0])] {
		fields = fields[1:]
	}

	// Drop trailing grammar markers and a postfix reflexive "sich"
	// ("entspannen sich"), keeping at least one token as the lemma.
	for len(fields) > 1 {
		last := fields[len(fields)-1]
		if !isGrammarMarker(last) && !strings.EqualFold(last, "sich") {
			break
		}

		fields = fields[:len(fields)-1]
	}

	lemma := strings.Join(fields, " ")

	// Strip an inflection hyphenated directly onto a single headword.
	if !strings.Contains(lemma, " ") {
		lemma = gluedSuffix.ReplaceAllString(lemma, "")
	}

	return strings.TrimSpace(lemma)
}

// isGrammarMarker reports whether a trailing token is grammar shorthand rather
// than part of the lemma: a parenthetical note ("(ab)", "(-e)"), a hyphen-led
// ending ("-en", "-er"), a bare symbol (=, ¨, -), a single letter, or a short
// German plural/feminine ending.
func isGrammarMarker(tok string) bool {
	t := strings.ToLower(strings.TrimSpace(tok))
	if t == "" {
		return true
	}

	if strings.HasPrefix(t, "(") || strings.HasPrefix(t, "-") {
		return true // "(ab)", "(-e)", "-en", "-er".
	}

	if !strings.ContainsFunc(t, unicode.IsLetter) {
		return true // "=", "¨" and friends.
	}

	if utf8.RuneCountInString(t) == 1 {
		return true // "n", "e", "s", "t", "ö".
	}

	switch t {
	case "en", "er", "nen", "in", "innen":
		return true
	}

	return false
}

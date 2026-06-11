package usecase

import (
	"strings"

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

// extractLemma recovers the lookup word from an existing card's Front by
// stripping a leading article (der/die/das/ein/eine) and a leading "sich".
func extractLemma(front string) string {
	s := strings.TrimSpace(front)
	for {
		fields := strings.SplitN(s, " ", 2)
		if len(fields) != 2 {
			break
		}
		switch strings.ToLower(fields[0]) {
		case "der", "die", "das", "ein", "eine", "sich":
			s = strings.TrimSpace(fields[1])
			continue
		}
		break
	}
	return s
}

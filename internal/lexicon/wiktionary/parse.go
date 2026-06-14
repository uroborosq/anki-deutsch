package wiktionary

import (
	"regexp"
	"strings"

	"anki/internal/lexicon"
)

// parse turns raw Wiktionary wikitext into a lexicon.Word. It is best-effort:
// fields that cannot be found are left at their zero value.
func parse(lemma, wikitext string) *lexicon.Word {
	word := &lexicon.Word{
		Lemma:        lemma,
		PartOfSpeech: partOfSpeech(wikitext),
	}

	switch word.PartOfSpeech {
	case lexicon.Noun:
		parseNoun(word, wikitext)
	case lexicon.Verb:
		parseVerb(word, wikitext)
	case lexicon.Adjective:
		parseAdjective(word, wikitext)
	}

	word.Definitions = parseDefinitions(wikitext)
	word.Russian = parseRussian(wikitext)

	return word
}

// ruTranslationRe captures the headword of a Russian translation template,
// matching both {{Ü|ru|слово}} and {{Üt|ru|слово|translit}} forms.
var ruTranslationRe = regexp.MustCompile(`\{\{Üt?\|ru\|([^|}]+)`)

// parseRussian extracts Russian translations from the {{Übersetzungen}} section,
// reading the "*{{ru}}:" bullet(s) of the Ü-Tabelle. Duplicates and stress
// marks are removed; order is preserved.
func parseRussian(wikitext string) []string {
	idx := strings.Index(wikitext, "{{Übersetzungen}}")
	if idx < 0 {
		return nil
	}

	section := wikitext[idx:]

	var out []string

	seen := map[string]bool{}

	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "*{{ru}}") {
			continue
		}

		for _, m := range ruTranslationRe.FindAllStringSubmatch(line, -1) {
			w := cleanRussian(m[1])
			if w != "" && !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}

	return out
}

// cleanRussian strips link markup and the combining acute stress mark (U+0301)
// that Wiktionary uses to mark Russian word stress (e.g. "фру́кт" → "фрукт").
func cleanRussian(s string) string {
	s = stripLinks(s)
	s = strings.ReplaceAll(s, "́", "")

	return strings.TrimSpace(s)
}

// partOfSpeech reads the {{Wortart|...|Deutsch}} marker.
func partOfSpeech(wikitext string) lexicon.PartOfSpeech {
	switch {
	case strings.Contains(wikitext, "{{Wortart|Substantiv|Deutsch}}"):
		return lexicon.Noun
	case strings.Contains(wikitext, "{{Wortart|Verb|Deutsch}}"):
		return lexicon.Verb
	case strings.Contains(wikitext, "{{Wortart|Adjektiv|Deutsch}}"):
		return lexicon.Adjective
	default:
		return lexicon.Unknown
	}
}

func parseNoun(word *lexicon.Word, wikitext string) {
	tmpl, ok := findTemplate(wikitext, "Deutsch Substantiv Übersicht")
	if !ok {
		return
	}

	switch strings.ToLower(strings.TrimSpace(tmpl.params["Genus"])) {
	case "m":
		word.Gender = lexicon.Masculine
	case "f":
		word.Gender = lexicon.Feminine
	case "n":
		word.Gender = lexicon.Neuter
	}

	word.Plural = strings.TrimSpace(tmpl.params["Nominativ Plural"])
}

func parseVerb(word *lexicon.Word, wikitext string) {
	tmpl, ok := findTemplate(wikitext, "Deutsch Verb Übersicht")
	if ok {
		word.PartizipII = strings.TrimSpace(tmpl.params["Partizip II"])
		word.Auxiliary = strings.TrimSpace(tmpl.params["Hilfsverb"])
		word.Praeteritum = strings.TrimSpace(tmpl.params["Präteritum_ich"])
	}

	if reflexiveHeadword.MatchString(word.Lemma) || reflexiveHeadword.MatchString(headwordLine(wikitext)) {
		word.Reflexive = true
	}
}

// headwordLine returns the heading line that carries the {{Wortart|Verb|...}}
// marker, where a reflexive "sich" would appear, or "" if absent.
func headwordLine(wikitext string) string {
	for _, line := range strings.Split(wikitext, "\n") {
		if strings.Contains(line, "{{Wortart|Verb|Deutsch}}") {
			return line
		}
	}

	return ""
}

func parseAdjective(word *lexicon.Word, wikitext string) {
	tmpl, ok := findTemplate(wikitext, "Deutsch Adjektiv Übersicht")
	if !ok {
		return
	}

	word.Comparative = strings.TrimSpace(tmpl.params["Komparativ"])
	word.Superlative = strings.TrimSpace(tmpl.params["Superlativ"])
}

// reflexiveHeadword matches a "sich" token in a headword/lemma line such as
// {{Worttrennung}} or the Hilfsverb line referencing a reflexive verb.
var reflexiveHeadword = regexp.MustCompile(`(?i)\bsich\b`)

// template is a parsed wikitext template: its name plus key=value parameters.
type template struct {
	name   string
	params map[string]string
}

// findTemplate locates the first {{name ...}} block, honouring nested {{ }},
// and splits it into key=value parameters.
func findTemplate(wikitext, name string) (template, bool) {
	start := templateStart(wikitext, name)
	if start < 0 {
		return template{}, false
	}

	// Scan from the opening braces tracking nesting depth.
	depth := 0
	i := start
	contentStart := -1
	end := -1

	for i < len(wikitext)-1 {
		switch {
		case wikitext[i] == '{' && wikitext[i+1] == '{':
			depth++
			if depth == 1 {
				contentStart = i + 2
			}

			i += 2
		case wikitext[i] == '}' && wikitext[i+1] == '}':
			depth--
			if depth == 0 {
				end = i
				i = len(wikitext) // break outer loop

				continue
			}

			i += 2
		default:
			i++
		}
	}

	if contentStart < 0 || end < 0 {
		return template{}, false
	}

	inner := wikitext[contentStart:end]
	parts := splitTopLevel(inner)

	tmpl := template{params: map[string]string{}}

	for idx, part := range parts {
		if idx == 0 {
			tmpl.name = strings.TrimSpace(part)
			continue
		}

		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}

		tmpl.params[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return tmpl, true
}

// templateStart returns the byte index of the "{{" that introduces a template
// whose name equals name (after the braces), or -1.
func templateStart(wikitext, name string) int {
	search := wikitext
	offset := 0

	for {
		idx := strings.Index(search, "{{")
		if idx < 0 {
			return -1
		}

		after := strings.TrimLeft(search[idx+2:], " \t")
		if strings.HasPrefix(after, name) {
			// Ensure the name is delimited by | , whitespace, or closing braces.
			rest := after[len(name):]
			if rest == "" || strings.HasPrefix(rest, "|") ||
				strings.HasPrefix(rest, "}") || strings.HasPrefix(rest, "\n") ||
				strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t") {
				return offset + idx
			}
		}

		offset += idx + 2
		search = wikitext[offset:]
	}
}

// splitTopLevel splits inner template content on "|" that are not nested inside
// another {{ }} or [[ ]].
func splitTopLevel(inner string) []string {
	var (
		parts []string
		buf   strings.Builder
	)

	braceDepth := 0
	bracketDepth := 0

	for i := 0; i < len(inner); i++ {
		if i < len(inner)-1 {
			pair := inner[i : i+2]
			switch pair {
			case "{{":
				braceDepth++

				buf.WriteString(pair)

				i++

				continue
			case "}}":
				if braceDepth > 0 {
					braceDepth--
				}

				buf.WriteString(pair)

				i++

				continue
			case "[[":
				bracketDepth++

				buf.WriteString(pair)

				i++

				continue
			case "]]":
				if bracketDepth > 0 {
					bracketDepth--
				}

				buf.WriteString(pair)

				i++

				continue
			}
		}

		if inner[i] == '|' && braceDepth == 0 && bracketDepth == 0 {
			parts = append(parts, buf.String())
			buf.Reset()

			continue
		}

		buf.WriteByte(inner[i])
	}

	parts = append(parts, buf.String())

	return parts
}

var (
	refTagRe     = regexp.MustCompile(`(?s)<ref[^>]*>.*?</ref>`)
	refSelfRe    = regexp.MustCompile(`<ref[^>]*/>`)
	tagRe        = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	meaningLine  = regexp.MustCompile(`^:\s*\[[0-9a-z, .–-]+\]\s*(.*\S)?\s*$`)
	templateInRe = regexp.MustCompile(`\{\{[^{}]*\}\}`)
)

// parseDefinitions extracts the ":[1] ..." style meaning lines that follow the
// {{Bedeutungen}} heading, stripped of basic wiki markup.
func parseDefinitions(wikitext string) []string {
	idx := strings.Index(wikitext, "{{Bedeutungen}}")
	if idx < 0 {
		return nil
	}

	section := wikitext[idx+len("{{Bedeutungen}}"):]

	var defs []string

	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimRight(line, "\r")

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Stop at the next section heading template ({{...}} on its own line).
		if !strings.HasPrefix(trimmed, ":") {
			if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
				break
			}

			continue
		}

		m := meaningLine.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		text := cleanMarkup(m[1])
		if text != "" {
			defs = append(defs, text)
		}
	}

	if len(defs) == 0 {
		return nil
	}

	return defs
}

// cleanMarkup strips refs, HTML tags, residual templates, and [[link]] syntax,
// keeping the human-readable link text.
func cleanMarkup(s string) string {
	s = refTagRe.ReplaceAllString(s, "")
	s = refSelfRe.ReplaceAllString(s, "")
	s = templateInRe.ReplaceAllString(s, "")
	s = tagRe.ReplaceAllString(s, "")
	s = stripLinks(s)
	s = strings.ReplaceAll(s, "'''", "")
	s = strings.ReplaceAll(s, "''", "")

	return strings.TrimSpace(s)
}

// stripLinks replaces [[target|label]] with label and [[target]] with target.
func stripLinks(s string) string {
	var buf strings.Builder

	for {
		open := strings.Index(s, "[[")
		if open < 0 {
			buf.WriteString(s)
			break
		}

		buf.WriteString(s[:open])

		close := strings.Index(s[open:], "]]")
		if close < 0 {
			buf.WriteString(s[open:])
			break
		}

		inner := s[open+2 : open+close]
		if pipe := strings.LastIndex(inner, "|"); pipe >= 0 {
			inner = inner[pipe+1:]
		}

		buf.WriteString(inner)

		s = s[open+close+2:]
	}

	return buf.String()
}

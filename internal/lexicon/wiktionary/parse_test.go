package wiktionary

import (
	"reflect"
	"testing"
)

func TestParseRussian(t *testing.T) {
	// "фру́кты" carries a combining acute stress mark that must be stripped.
	wikitext := "{{Übersetzungen}}\n" +
		"{{Ü-Tabelle|Ü-links=\n" +
		"*{{ru}}: [1] {{Üt|ru|фру́кты}}, {{Üt|ru|плоды|plody}}; [2] {{Ü|ru|плоды}}\n" +
		"*{{en}}: [1] {{Ü|en|fruit}}\n" +
		"|Ü-rechts=\n" +
		"*{{fr}}: [1] {{Ü|fr|fruit}}\n" +
		"}}\n"

	got := parseRussian(wikitext)

	want := []string{"фрукты", "плоды"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseRussian = %#v, want %#v", got, want)
	}
}

func TestParseRussianAbsent(t *testing.T) {
	if got := parseRussian("no translations here"); got != nil {
		t.Errorf("parseRussian = %#v, want nil", got)
	}
}

package flashcard

import "testing"

func TestNoteIsComplete(t *testing.T) {
	tests := []struct {
		name string
		back string
		want bool
	}{
		{"non-empty back", "house", true},
		{"empty back", "", false},
		{"whitespace back", "   \t\n", false},
		{"leading/trailing space around text", "  house  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := Note{Front: "Haus", Back: tt.back}
			if got := n.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() with Back %q = %v, want %v", tt.back, got, tt.want)
			}
		})
	}
}

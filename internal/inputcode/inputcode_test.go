package inputcode

import "testing"

func TestParseKeyChord(t *testing.T) {
	tests := []struct {
		name  string
		chord string
		want  []string
	}{
		{
			name:  "single key",
			chord: "enter",
			want:  []string{"KEY_ENTER"},
		},
		{
			name:  "modifier chord",
			chord: "CTRL+L",
			want:  []string{"KEY_LEFTCTRL", "KEY_L"},
		},
		{
			name:  "three key chord with spaces",
			chord: " Ctrl + Shift + P ",
			want:  []string{"KEY_LEFTCTRL", "KEY_LEFTSHIFT", "KEY_P"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chord, err := ParseKeyChord(test.chord)
			if err != nil {
				t.Fatalf("ParseKeyChord() error = %v", err)
			}
			keys := chord.Keys()
			if len(keys) != len(test.want) {
				t.Fatalf("len(Keys()) = %d, want %d", len(keys), len(test.want))
			}
			for i, key := range keys {
				if key.String() != test.want[i] {
					t.Fatalf("Keys()[%d] = %s, want %s", i, key.String(), test.want[i])
				}
			}
		})
	}
}

func TestParseKeyChordRejectsInvalidChords(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"ctrl+",
		"+a",
		"ctrl++l",
		"ctrl+nope",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			if _, err := ParseKeyChord(test); err == nil {
				t.Fatal("ParseKeyChord() error = nil")
			}
		})
	}
}

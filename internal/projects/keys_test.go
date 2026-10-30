package projects

import (
	"regexp"
	"strings"
	"testing"
)

// Hashkey generation test

func TestHashKeyKnownVectors(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedHash string
	}{
		{"empty", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"short string", "abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"long string", "The quick brown fox jumps over the lazy dog", "d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HashKey(tc.input)
			if got != tc.expectedHash {
				t.Errorf("HashKey(%q) = %q, want %q", tc.input, got, tc.expectedHash)
			}
		})
	}
}

func TestHashKeyLengthAndCharacters(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]+$`)
	input := "abc"
	t.Run(input, func(t *testing.T) {
		output := HashKey(input)

		if len(output) != 64 {
			t.Errorf("output should be exactly 64 characters, %v", input)
		}
		if !pattern.MatchString(output) {
			t.Errorf("%v characters should all be in 0-9a-f", input)
		}
	})
}

func TestHashKeyDeterminism(t *testing.T) {
	input := "abc"
	t.Run(input, func(t *testing.T) {
		outputOne := HashKey(input)
		outputTwo := HashKey(input)

		if outputOne != outputTwo {
			t.Errorf("the same input should always produce the same output, %v", input)
		}
	})
}

func TestHashKeyWithDifferentInputs(t *testing.T) {
	input := "abc"
	t.Run(input, func(t *testing.T) {
		outputOne := HashKey(input)
		outputTwo := HashKey("abd")

		if outputOne == outputTwo {
			t.Errorf("different outputs should produce different hashes: %v, %v", outputOne, outputTwo)
		}
	})
}

// Key hint tests
func TestKeyHint(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"short key", "obs_abcdefgh", "efgh"},
		{"full-length key", "obs_" + strings.Repeat("a", 39) + "x7Qa", "x7Qa"},
		{"exactly 4 characters", "abcd", "abcd"},
		{"fewer than 4", "ab", "ab"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := KeyHint(tc.input)
			if got != tc.want {
				t.Errorf("HasKey(%q) = %q, want %v", tc.input, got, tc.want)
			}
		})
	}
}

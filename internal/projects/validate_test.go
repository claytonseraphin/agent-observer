package projects

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAgentName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"simple word", "news", nil},
		{"empty", "", ErrInvalidAgentName},
		{"uppercase is rejected, not lowercased", "News", ErrInvalidAgentName},
		{"exactly 32 characters", strings.Repeat("a", 32), nil},
		{"over 32 characters", strings.Repeat("a", 33), ErrInvalidAgentName},
		{"leading dash", "-news", ErrInvalidAgentName},
		{"trailing dash", "news-", nil},
		{"leading  uderscore", "_news", ErrInvalidAgentName},
		{"underscore separated name", "news_agent", nil},
		{"leading period", ".news", ErrInvalidAgentName},
		{"period separated name", "news.agent", ErrInvalidAgentName},
		{"leading slash", "/news", ErrInvalidAgentName},
		{"slash separated name", "news/agent", ErrInvalidAgentName},
		{"space separated names", "news agent", ErrInvalidAgentName},
		{"trail space", "news ", ErrInvalidAgentName},
		{"diacritical marks", "néws", ErrInvalidAgentName}, // accent aigu, grave etc..
		{"one character name", "n", nil},
		{"leading digit name", "0abc", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAgentName(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ValidateAgentName(%q) = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestValidateAgentNameReserved(t *testing.T) {
	reservedWords := []string{
		"api", "debug",
	}
	for _, word := range reservedWords {
		t.Run(word, func(t *testing.T) {
			err := ValidateAgentName(word)
			if !errors.Is(err, ErrReservedAgentName) {
				t.Errorf("ValidateAgentName(%q) = %v, want %v", word, err, ErrReservedAgentName)
			}
		})
	}
}

func TestValidateAgentNameReservedPrefixAllowed(t *testing.T) {
	leadingReservedWords := []string{
		"apis", "api-gateway", "debugger",
	}
	for _, word := range leadingReservedWords {
		t.Run(word, func(t *testing.T) {
			if err := ValidateAgentName(word); err != nil {
				t.Errorf("ValidateAgentName(%q) = %v, want no error", word, err)
			}
		})
	}
}

// Test to Validate Project Name
func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"empty", "", "", ErrInvalidProjectName},
		{"spaces only", " ", "", ErrInvalidProjectName},
		{"65 characters", strings.Repeat("a", 65), "", ErrInvalidProjectName},
		{"65 characters", strings.Repeat("é", 65), "", ErrInvalidProjectName},
		{"control charcacter", "a\nb", "", ErrInvalidProjectName},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateProjectName(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ValidateProjectName(%q) = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestValidateProjectNameWithAcceptedInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trimmed name", " project ultimate  ", "project ultimate"},
		{"trailing new line", "project ultimate\n", "project ultimate"},
		{"plain name", "Research assistant", "Research assistant"},
		{"one character", "a", "a"},
		{"exactly 64 characters", strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"64 accented characters", strings.Repeat("é", 64), strings.Repeat("é", 64)},
		{"64 characters plus padding", "  " + strings.Repeat("a", 64) + "  ", strings.Repeat("a", 64)},
		{"leading tab", "\tproject", "project"},
		{"inner spacing kept", "a  b", "a  b"},
		{"digits", "project 2", "project 2"},
		{"non-english name", "Équipe spéciale", "Équipe spéciale"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateProjectName(tc.input)
			if err != nil {
				t.Errorf("ValidateProjectName(%q) returned error %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ValidateProjectName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

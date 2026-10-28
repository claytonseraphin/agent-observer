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

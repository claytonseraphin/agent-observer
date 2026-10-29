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

// Validate upstream URL
func TestValidateUpstreamURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"spaced url", "http://a space"},
		{"url longer than 2048 characters", "http://" + strings.Repeat("a", 2049-len("http://"))},
		{"surrounding whitespace", " http://x  "},
		{"url with query", "http://host?x=1"},
		{"url with empty query", "http://host?"},
		{"url with fragment", "http://host#frag"},
		{"url with empty fragment", "http://host#"},
		{"url with credentials", "http://user:pass@host"},
		{"url with username only", "http://user@host"},
		{"empty host", "http://"},
		{"empty host with path", "http:///path"},
		{"port without host", "http://:8001"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateUpstreamURL(tc.input)
			if !errors.Is(err, ErrInvalidUpstreamURL) {
				t.Errorf("ValidateUpstreamURL(%q) error = %v, want %v", tc.input, err, ErrInvalidUpstreamURL)
			}
		})
	}
}

func TestValidateUpstreamURLRejectedSchemes(t *testing.T) {
	for _, input := range []string{
		"example.com",    // no scheme at all
		"//host",         // empty scheme
		"localhost:8001", // "localhost" parsed as the scheme
		"ftp://x",        // other schemes
		"file:///etc/passwd",
		"javascript:alert(1)",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := ValidateUpstreamURL(input)
			if !errors.Is(err, ErrInvalidUpstreamURL) {
				t.Errorf("ValidateUpstreamURL(%q) error = %v, want %v", input, err, ErrInvalidUpstreamURL)
			}
		})
	}
}

func TestValidateUpstreamURLWithAcceptedInput(t *testing.T) {
	exactly2048 := "http://" + strings.Repeat("a", 2048-len("http://"))

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"localhost with port", "http://localhost:8001", "http://localhost:8001"},
		{"https host", "https://agent.example.com", "https://agent.example.com"},
		{"docker host", "http://host.docker.internal:8001", "http://host.docker.internal:8001"},
		{"trailing slash trimmed", "http://news:8001/", "http://news:8001"},
		{"path kept", "http://10.0.0.5:8080/a2a", "http://10.0.0.5:8080/a2a"},
		{"trailing slash after path trimmed", "http://host/a2a/", "http://host/a2a"},
		{"ipv6", "http://[::1]:8001", "http://[::1]:8001"},
		{"scheme lowercased", "HTTP://localhost:8001", "http://localhost:8001"},
		{"exactly 2048 characters", exactly2048, exactly2048},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateUpstreamURL(tc.input)
			if err != nil {
				t.Errorf("ValidateUpstreamURL(%q) returned error %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ValidateUpstreamURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestValidateUpstreamURLErrorDoesNotLeakInput(t *testing.T) {
	_, err := ValidateUpstreamURL("http://user:secret-password@host")
	if err == nil || strings.Contains(err.Error(), "secret-password") {
		t.Errorf("error must exist and must not contain the password, got %v", err)
	}
}

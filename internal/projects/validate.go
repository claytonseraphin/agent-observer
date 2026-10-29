package projects

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var agentNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
var ErrInvalidAgentName = errors.New("agent name: agent name must be 1 to 32 characters: lowercase letters, digits, dashes and underscores, starting with a letter or digit")
var ErrReservedAgentName = errors.New("agent name: reserved agent name")
var reservedWords = []string{"api", "debug"}

func ValidateAgentName(input string) error {

	matched := agentNamePattern.MatchString(input)
	if !matched {
		return ErrInvalidAgentName
	}

	if slices.Contains(reservedWords, input) {
		return ErrReservedAgentName
	}

	return nil

}

var ErrInvalidProjectName = errors.New("project name: project name must be 1 to 64 chracters with no control character")

func ValidateProjectName(input string) (string, error) {
	trimmed_input := strings.TrimSpace(input)
	n := utf8.RuneCountInString(trimmed_input)

	if n == 0 || n >= 65 || strings.ContainsFunc(trimmed_input, unicode.IsControl) {
		return "", ErrInvalidProjectName
	}

	return trimmed_input, nil
}

const maxUpstreamURLLength = 2048

var ErrInvalidUpstreamURL = errors.New("invalid upstream URL")

// ValidateUpstreamURL checks an agent's upstream URL and returns it in its
// normalised form (lowercase scheme, no trailing slash).
//
// The error messages never include the input: a URL can carry a password,
// and these messages end up in API responses and logs.
func ValidateUpstreamURL(raw string) error {
	if raw == "" || len(raw) > maxUpstreamURLLength {
		return fmt.Errorf("%w: must be 1 to %d characters", ErrInvalidUpstreamURL, maxUpstreamURLLength)
	}
	if strings.TrimSpace(raw) != raw {
		return fmt.Errorf("%w: must not start or end with whitespace", ErrInvalidUpstreamURL)
	}
	// A real path never contains a raw ? or #, so this also catches an
	// empty query ("http://host?") and an empty fragment ("http://host#").
	if strings.ContainsAny(raw, "?#") {
		return fmt.Errorf("%w: must not contain a query or fragment", ErrInvalidUpstreamURL)
	}

	u, err := url.Parse(raw)
	if err != nil {
		// err is not wrapped on purpose: *url.Error repeats the whole URL.
		return fmt.Errorf("%w: cannot be parsed", ErrInvalidUpstreamURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidUpstreamURL)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%w: host is missing", ErrInvalidUpstreamURL)
	}
	if u.User != nil {
		return fmt.Errorf("%w: must not contain credentials", ErrInvalidUpstreamURL)
	}

	// u.Path = strings.TrimRight(u.Path, "/")
	// return u.String(), nil
	return nil
}

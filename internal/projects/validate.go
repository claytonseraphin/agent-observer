package projects

import (
	"errors"
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

var ErrInvalidProjectName = errors.New("project name: project name must be 1 to 64 chracters with no control character")

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

func ValidateProjectName(input string) (string, error) {
	trimmed_input := strings.TrimSpace(input)
	n := utf8.RuneCountInString(trimmed_input)

	if n == 0 || n >= 65 || strings.ContainsFunc(trimmed_input, unicode.IsControl) {
		return "", ErrInvalidProjectName
	}

	return trimmed_input, nil
}

package projects

import (
	"errors"
	"regexp"
	"slices"
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

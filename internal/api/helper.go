package api

import (
	"errors"
	"strconv"
)

var errInvalidLimit = errors.New("invalid limit")

const (
	defaultLimit = 50
	maxLimit     = 200
)

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, errInvalidLimit
	}

	if limit > maxLimit {
		return maxLimit, nil
	}

	return limit, nil
}

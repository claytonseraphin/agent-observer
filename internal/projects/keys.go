package projects

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashKey(input string) string {
	// HashKey returns the SHA-256 of a project key as lowercase hex.
	// Only the hash is stored.
	hashBytes := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hashBytes[:])
}

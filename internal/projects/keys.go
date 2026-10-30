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

func KeyHint(input string) string {
	// KeyHint returns the last 4 characters of a key so people can tell keys apart.
	// Keys are plain ASCII, so slicing by byte is safe here.
	if len(input) <= 4 {
		return input
	}
	return input[len(input)-4:]
}

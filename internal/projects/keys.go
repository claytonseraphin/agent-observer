package projects

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
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

func NewKey() string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	n := 43
	b := make([]byte, n)
	charsetSize := big.NewInt(int64(len(charset)))

	for i := range b {
		num, err := rand.Int(rand.Reader, charsetSize)
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		b[i] = charset[num.Int64()]
	}
	characters := string(b)
	return "obs_" + characters
}

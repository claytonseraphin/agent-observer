package projects

import (
	"crypto/rand"
	"math/big"
)

func randomString(length int, charset, prefix string) string {
	// helper function to randomize project id and key id
	b := make([]byte, length)
	charsetSize := big.NewInt(int64(len(charset)))

	for i := range b {
		num, err := rand.Int(rand.Reader, charsetSize)
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		b[i] = charset[num.Int64()]
	}
	characters := string(b)
	return prefix + characters
}

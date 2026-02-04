package links

import (
	"crypto/rand"
)

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type CryptoSlugger struct{}

func NewCryptoSlugger() *CryptoSlugger { return &CryptoSlugger{} }

func (s *CryptoSlugger) Generate(length int) (string, error) {
	if length <= 0 {
		length = 6
	}

	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	out := make([]byte, length)
	for i := range buf {
		out[i] = base62Alphabet[int(buf[i])%len(base62Alphabet)]
	}

	return string(out), nil
}


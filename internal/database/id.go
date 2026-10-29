package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewID returns an opaque identifier for user data rows.
func NewID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

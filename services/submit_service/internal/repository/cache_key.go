package repository

import (
	"crypto/sha1"
	"encoding/hex"
)

func hashKey(value string) string {
	if value == "" {
		return ""
	}
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:8])
}

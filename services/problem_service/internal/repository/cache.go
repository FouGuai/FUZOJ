package repository

import (
	"crypto/rand"
	"math/big"
	"time"
)

func jitterTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	maxJitter := int64(ttl / 10)
	if maxJitter <= 0 {
		return ttl
	}
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitter+1))
	if err != nil {
		return ttl
	}
	return ttl - time.Duration(n.Int64())
}

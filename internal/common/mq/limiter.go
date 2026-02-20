package mq

import "context"

// TokenLimiter is a simple counting limiter for fetch control.
type TokenLimiter struct {
	tokens chan struct{}
}

// NewTokenLimiter creates a limiter with a fixed capacity.
func NewTokenLimiter(size int) *TokenLimiter {
	if size <= 0 {
		size = 1
	}
	tokens := make(chan struct{}, size)
	for i := 0; i < size; i++ {
		tokens <- struct{}{}
	}
	return &TokenLimiter{tokens: tokens}
}

// Acquire blocks until a token is available or ctx is canceled.
func (l *TokenLimiter) Acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
		return nil
	}
}

// Release returns a token to the limiter.
func (l *TokenLimiter) Release() {
	select {
	case l.tokens <- struct{}{}:
	default:
	}
}

package kinetic

import (
	"context"
)

// Semaphore is a counting semaphore that limits concurrent access.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a Semaphore with the given capacity.
func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, capacity),
	}
}

// Acquire blocks until a token is available, or the context is cancelled.
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to acquire a token without blocking.
// Returns true if successful, false if no tokens are available.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release returns a token to the semaphore.
func (s *Semaphore) Release() {
	<-s.ch
}

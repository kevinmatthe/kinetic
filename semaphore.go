package kinetic

import (
	"context"
)

// Semaphore is a counting semaphore that limits concurrent access to a resource.
// It uses a buffered channel internally for zero-allocation acquire/release.
//
// Example:
//
//	sem := kinetic.NewSemaphore(5) // at most 5 concurrent accesses
//	sem.Acquire(ctx)
//	defer sem.Release()
//	accessResource()
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a Semaphore with the given capacity.
//
// Example:
//
//	sem := kinetic.NewSemaphore(10) // at most 10 concurrent goroutines
func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, capacity),
	}
}

// Acquire blocks until a token is available, or the context is cancelled.
// Returns nil on success, or the context error if cancelled.
//
// Example:
//
//	err := sem.Acquire(ctx)
//	if err != nil {
//	    return err // context cancelled
//	}
//	defer sem.Release()
//	doWork()
//	// err == nil if acquired successfully
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
//
// Example:
//
//	if sem.TryAcquire() {
//	    defer sem.Release()
//	    doWork()
//	} else {
//	    // too busy, skip or queue
//	}
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release returns a token to the semaphore.
// Must be called exactly once for each successful Acquire or TryAcquire.
//
// Example:
//
//	sem.Acquire(ctx)
//	defer sem.Release()
func (s *Semaphore) Release() {
	<-s.ch
}

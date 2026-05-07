package kinetic

import (
	"sync"
)

// Once executes fn exactly once, no matter how many times Get is called concurrently.
// The result is cached and returned on subsequent calls.
type Once[T any] struct {
	once  sync.Once
	val   T
	err   error
	fn    func() (T, error)
}

// NewOnce creates an Once that will call fn on the first Get().
func NewOnce[T any](fn func() (T, error)) *Once[T] {
	return &Once[T]{fn: fn}
}

// Get returns the value, calling fn exactly once if not yet called.
// Concurrent calls block until fn completes.
func (o *Once[T]) Get() (T, error) {
	o.once.Do(func() {
		o.val, o.err = o.fn()
	})
	return o.val, o.err
}

// Lazy is an alias for Once — the function is deferred until the first Get() call.
type Lazy[T any] = Once[T]

// NewLazy creates a Lazy value that is computed on first access.
func NewLazy[T any](fn func() (T, error)) *Lazy[T] {
	return NewOnce(fn)
}

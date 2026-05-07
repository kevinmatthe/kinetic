package kinetic

import (
	"sync"
)

// Once executes fn exactly once, no matter how many times Get is called concurrently.
// The result is cached and returned on subsequent calls. Similar to sync.Once but with
// return values and error support.
//
// Example:
//
//	config := kinetic.NewOnce(func() (Config, error) {
//	    return loadConfigFromFile("config.yaml")
//	})
//	cfg1, _ := config.Get() // loads config from file
//	cfg2, _ := config.Get() // returns cached result, fn not called again
//	// cfg1 == cfg2
type Once[T any] struct {
	once sync.Once
	val  T
	err  error
	fn   func() (T, error)
}

// NewOnce creates an Once that will call fn on the first Get().
//
// Example:
//
//	o := kinetic.NewOnce(func() (DB, error) { return connectDB() })
func NewOnce[T any](fn func() (T, error)) *Once[T] {
	return &Once[T]{fn: fn}
}

// Get returns the value, calling fn exactly once if not yet called.
// Concurrent calls block until fn completes.
//
// Example:
//
//	o := kinetic.NewOnce(func() (int, error) { return 42, nil })
//	val, err := o.Get()
//	// val == 42, err == nil
//	val2, err2 := o.Get() // same result, fn not called again
//	// val2 == 42, err2 == nil
func (o *Once[T]) Get() (T, error) {
	o.once.Do(func() {
		o.val, o.err = o.fn()
	})
	return o.val, o.err
}

// Lazy is an alias for Once — the function is deferred until the first Get() call.
// Useful for expensive initializations that should only run when needed.
//
// Example:
//
//	conn := kinetic.NewLazy(func() (Connection, error) { return dialRemote() })
//	// dialRemote is NOT called yet
//	c, err := conn.Get() // now dialRemote is called
type Lazy[T any] = Once[T]

// NewLazy creates a Lazy value that is computed on first access.
//
// Example:
//
//	l := kinetic.NewLazy(func() ([]Item, error) {
//	    return expensiveQuery()
//	})
//	// expensiveQuery not yet called
//	items, err := l.Get() // now it runs
func NewLazy[T any](fn func() (T, error)) *Lazy[T] {
	return NewOnce(fn)
}

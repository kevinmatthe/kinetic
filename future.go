package kinetic

import (
	"context"
)

// Awaitable is a non-generic interface for waiting on a Future's completion.
// It allows different Future types to be used as dependencies in Go options.
type Awaitable interface {
	Wait()
}

// Future represents an asynchronous computation that will eventually produce a value of type T or an error.
// It is a lazy handle — creating a Future does not block; value retrieval methods wait as needed.
type Future[T any] struct {
	done  chan struct{}
	value T
	err   error
}

// Go starts an asynchronous computation and returns a Future handle.
// Options can be combined freely: After(deps...), WithContext(ctx), etc.
func Go[T any](fn func() (T, error), opts ...Option) *Future[T] {
	f := &Future[T]{
		done: make(chan struct{}),
	}

	// Fast path: no options, skip allocation.
	if len(opts) == 0 {
		go func() {
			var val T
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = &PanicError{Value: r}
					}
				}()
				val, err = fn()
			}()
			f.finish(val, err)
		}()
		return f
	}

	// Slow path: apply options.
	o := &option{}
	for _, opt := range opts {
		opt.apply(o)
	}

	go func() {
		// Wait for all dependencies to complete, or context cancellation.
		if o.ctx != nil {
			for _, dep := range o.deps {
				if !waitOrCancel(dep, o.ctx) {
					f.finish(zero[T](), o.ctx.Err())
					return
				}
			}
		} else {
			for _, dep := range o.deps {
				dep.Wait()
			}
		}

		// Check context cancellation before executing.
		if o.ctx != nil {
			select {
			case <-o.ctx.Done():
				f.finish(zero[T](), o.ctx.Err())
				return
			default:
			}
		}

		// Execute the computation.
		var (
			val T
			err error
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = &PanicError{Value: r}
				}
			}()
			val, err = fn()
		}()

		f.finish(val, err)
	}()

	return f
}

// finish sets the result and signals completion.
// Must be called exactly once. The write happens-before the channel close,
// ensuring happens-before guarantees for any subsequent Get/Val/Err calls.
func (f *Future[T]) finish(val T, err error) {
	f.value = val
	f.err = err
	close(f.done)
}

// Wait blocks until the Future completes. Implements the Awaitable interface.
func (f *Future[T]) Wait() {
	<-f.done
}

// Get blocks until the Future completes and returns the value and error.
// This is the standard Go-style way to retrieve a Future's result.
func (f *Future[T]) Get() (T, error) {
	<-f.done
	return f.value, f.err
}

// Val blocks until the Future completes and returns the value.
// It panics if the Future resulted in an error.
// Use this when you are certain the computation succeeds (e.g., inside After callbacks).
func (f *Future[T]) Val() T {
	val, err := f.Get()
	if err != nil {
		panic(err)
	}
	return val
}

// Err blocks until the Future completes and returns the error, if any.
func (f *Future[T]) Err() error {
	<-f.done
	return f.err
}

// WaitAll blocks until all given Awaitables have completed.
func WaitAll(awaitables ...Awaitable) {
	for _, a := range awaitables {
		a.Wait()
	}
}

// --- Option pattern ---

// Option configures how a Future is executed.
type Option interface {
	apply(*option)
}

type option struct {
	deps []Awaitable
	ctx  context.Context
}

type optionFunc func(*option)

func (f optionFunc) apply(o *option) { f(o) }

// After specifies dependencies that must complete before the Future's function executes.
// Inside the function, calling .Val() on the dependencies is safe.
func After(deps ...Awaitable) Option {
	return optionFunc(func(o *option) {
		o.deps = append(o.deps, deps...)
	})
}

// WithContext binds a context to the Future.
// If the context is cancelled before the function executes, the Future completes with the context's error.
func WithContext(ctx context.Context) Option {
	return optionFunc(func(o *option) {
		o.ctx = ctx
	})
}

// --- PanicError ---

// PanicError wraps a panic value recovered from a Future's function.
type PanicError struct {
	Value any
}

func (e *PanicError) Error() string {
	return "kinetic: panic in async task"
}

func (e *PanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// --- helpers ---

func zero[T any]() T {
	var z T
	return z
}

// waitOrCancel waits for an Awaitable or returns false if the context is cancelled first.
func waitOrCancel(a Awaitable, ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		a.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

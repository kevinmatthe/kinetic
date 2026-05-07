// Package kinetic provides a high-performance, easy-to-use asynchronous utility library for Go.
//
// Core design principles:
//   - A single Future alone is just a deferred function call; the value of async is in concurrency.
//   - Value retrieval is always safe — methods wait internally, never read zero-values from unfinished Futures.
//   - Launch concurrently, wait in batch, then retrieve values.
//   - Declare dependencies explicitly — After(deps...) ensures .Val() is safe inside callbacks.
//
// Quick start:
//
//	// Launch concurrent tasks
//	users := kinetic.Go(func() ([]User, error) { return fetchUsers() })
//	orders := kinetic.Go(func() ([]Order, error) { return fetchOrders() })
//
//	// Declare dependency — waits for users and orders before executing
//	report := kinetic.Go(func() (Report, error) {
//	    return buildReport(users.Val(), orders.Val()), nil
//	}, kinetic.After(users, orders))
//
//	// Get result
//	r, err := report.Get()
package kinetic

import (
	"context"
)

// Awaitable is a non-generic interface for waiting on a Future's completion.
// It allows different Future types (e.g., *Future[User], *Future[Order]) to be
// used uniformly as dependencies in the After option.
type Awaitable interface {
	Wait()
}

// Future represents an asynchronous computation that will eventually produce a value of type T or an error.
// It is a lazy handle — creating a Future does not block; value retrieval methods (Get, Val, Err) wait as needed.
//
// Internal design: zero-lock, using close(ch) happens-before guarantee. Only 1 channel allocation per Future.
type Future[T any] struct {
	done  chan struct{}
	value T
	err   error
}

// Go starts an asynchronous computation and returns a Future handle immediately.
// The function runs in a goroutine; the returned Future can be used to wait for and retrieve the result.
// Options can be combined freely: After(deps...), WithContext(ctx), etc.
//
// Example (basic):
//
//	f := kinetic.Go(func() (string, error) {
//	    return "hello", nil
//	})
//	val, err := f.Get()
//	// val == "hello", err == nil
//
// Example (with dependencies):
//
//	a := kinetic.Go(func() (int, error) { return 10, nil })
//	b := kinetic.Go(func() (int, error) { return 20, nil })
//	c := kinetic.Go(func() (int, error) {
//	    return a.Val() + b.Val(), nil // safe: a and b are guaranteed complete
//	}, kinetic.After(a, b))
//	val, err := c.Get()
//	// val == 30, err == nil
//
// Example (with context cancellation):
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//	f := kinetic.Go(func() (string, error) {
//	    return fetchData(ctx)
//	}, kinetic.WithContext(ctx))
//	val, err := f.Get()
//	// if ctx cancelled before fn runs: val == "", err == context.DeadlineExceeded
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
//
// Example:
//
//	f := kinetic.Go(func() (int, error) { return 42, nil })
//	f.Wait()
//	// f is now guaranteed complete
func (f *Future[T]) Wait() {
	<-f.done
}

// Get blocks until the Future completes and returns the value and error.
// This is the standard Go-style way to retrieve a Future's result.
//
// Example:
//
//	f := kinetic.Go(func() (int, error) { return 42, nil })
//	val, err := f.Get()
//	// val == 42, err == nil
//
//	f2 := kinetic.Go(func() (int, error) { return 0, errors.New("fail") })
//	val, err = f2.Get()
//	// val == 0, err.Error() == "fail"
func (f *Future[T]) Get() (T, error) {
	<-f.done
	return f.value, f.err
}

// Val blocks until the Future completes and returns the value.
// It panics if the Future resulted in an error.
// Use this when you are certain the computation succeeds (e.g., inside After callbacks where
// dependencies are already verified, or after WaitAll).
//
// Example:
//
//	a := kinetic.Go(func() (string, error) { return "hello", nil })
//	b := kinetic.Go(func() (string, error) { return "world", nil })
//	kinetic.WaitAll(a, b)
//	result := a.Val() + " " + b.Val()
//	// result == "hello world"
func (f *Future[T]) Val() T {
	val, err := f.Get()
	if err != nil {
		panic(err)
	}
	return val
}

// Err blocks until the Future completes and returns the error, if any.
// Returns nil if the computation succeeded.
//
// Example:
//
//	f := kinetic.Go(func() (int, error) { return 0, errors.New("timeout") })
//	err := f.Err()
//	// err.Error() == "timeout"
func (f *Future[T]) Err() error {
	<-f.done
	return f.err
}

// WaitAll blocks until all given Awaitables have completed.
// After WaitAll returns, calling .Val() or .Get() on any of the Futures is safe.
//
// Example:
//
//	a := kinetic.Go(func() (string, error) { return "users", nil })
//	b := kinetic.Go(func() (int, error) { return 42, nil })
//	c := kinetic.Go(func() (bool, error) { return true, nil })
//	kinetic.WaitAll(a, b, c)
//	// a.Val() == "users", b.Val() == 42, c.Val() == true
func WaitAll(awaitables ...Awaitable) {
	for _, a := range awaitables {
		a.Wait()
	}
}

// --- Option pattern ---

// Option configures how a Future is executed. Passed as variadic arguments to Go().
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
// Inside the function, calling .Val() on the dependencies is safe — they are guaranteed to be done.
// Multiple After calls accumulate dependencies.
//
// Example:
//
//	users := kinetic.Go(fetchUsers)
//	orders := kinetic.Go(fetchOrders)
//	report := kinetic.Go(func() (Report, error) {
//	    return buildReport(users.Val(), orders.Val()), nil
//	}, kinetic.After(users, orders))
//	r, _ := report.Get()
//	// r is built from completed users and orders
func After(deps ...Awaitable) Option {
	return optionFunc(func(o *option) {
		o.deps = append(o.deps, deps...)
	})
}

// WithContext binds a context to the Future.
// If the context is cancelled before the function starts executing (e.g., during dependency wait),
// the Future completes with the context's error. If the function is already running, it cannot be
// interrupted — the function itself must respect ctx if it needs cancellability.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
//	defer cancel()
//	slowDep := kinetic.Go(func() (int, error) {
//	    time.Sleep(1 * time.Second)
//	    return 1, nil
//	})
//	f := kinetic.Go(func() (int, error) {
//	    return slowDep.Val() * 2, nil
//	}, kinetic.After(slowDep), kinetic.WithContext(ctx))
//	val, err := f.Get()
//	// val == 0, err == context.DeadlineExceeded (ctx cancelled while waiting for slowDep)
func WithContext(ctx context.Context) Option {
	return optionFunc(func(o *option) {
		o.ctx = ctx
	})
}

// --- PanicError ---

// PanicError wraps a panic value recovered from a Future's function.
// It is returned as the error when a Future's function panics instead of returning an error.
//
// Example:
//
//	f := kinetic.Go(func() (int, error) { panic("oops") })
//	_, err := f.Get()
//	var pe *kinetic.PanicError
//	if errors.As(err, &pe) {
//	    // pe.Value == "oops"
//	}
type PanicError struct {
	Value any
}

func (e *PanicError) Error() string {
	return "kinetic: panic in async task"
}

// Unwrap returns the underlying error if the panic value implements error, otherwise nil.
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

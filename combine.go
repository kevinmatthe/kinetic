package kinetic

import (
	"sync"
)

// Result represents the outcome of a Future, whether successful or failed.
type Result[T any] struct {
	Value T
	Error error
}

// All returns a Future that completes when all given Futures have completed successfully.
// If any Future fails, the returned Future fails immediately with that error.
// The returned Future's value is a slice of results in the same order as the input Futures.
func All[T any](futures ...*Future[T]) *Future[[]T] {
	return Go(func() ([]T, error) {
		results := make([]T, len(futures))
		for i, f := range futures {
			val, err := f.Get()
			if err != nil {
				return nil, err
			}
			results[i] = val
		}
		return results, nil
	})
}

// Race returns a Future that completes with the result of the first Future to complete,
// whether success or failure.
func Race[T any](futures ...*Future[T]) *Future[T] {
	return Go(func() (T, error) {
		done := make(chan Result[T], 1)

		for _, f := range futures {
			f := f
			go func() {
				val, err := f.Get()
				select {
				case done <- Result[T]{Value: val, Error: err}:
				default:
				}
			}()
		}

		r := <-done
		return r.Value, r.Error
	})
}

// AllSettled returns a Future that completes when all given Futures have completed,
// regardless of success or failure. The result contains all outcomes in input order.
func AllSettled[T any](futures ...*Future[T]) *Future[[]Result[T]] {
	return Go(func() ([]Result[T], error) {
		results := make([]Result[T], len(futures))

		var wg sync.WaitGroup
		wg.Add(len(futures))

		for i, f := range futures {
			i, f := i, f
			go func() {
				defer wg.Done()
				val, err := f.Get()
				results[i] = Result[T]{Value: val, Error: err}
			}()
		}

		wg.Wait()
		return results, nil
	})
}

// Any returns a Future that completes with the result of the first Future to succeed.
// If all Futures fail, it returns the last error.
func Any[T any](futures ...*Future[T]) *Future[T] {
	return Go(func() (T, error) {
		type outcome struct {
			val T
			err error
		}

		done := make(chan outcome, len(futures))

		for _, f := range futures {
			f := f
			go func() {
				val, err := f.Get()
				done <- outcome{val: val, err: err}
			}()
		}

		var lastErr error
		for range futures {
			o := <-done
			if o.err == nil {
				return o.val, nil
			}
			lastErr = o.err
		}
		var zero T
		return zero, lastErr
	})
}

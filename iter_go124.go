//go:build go1.24

package kinetic

import (
	"slices"
	"sync"
)

// Map applies fn to each item concurrently and returns results in the same order.
// Go 1.24+ implementation.
func Map[T any, R any](items []T, fn func(T) (R, error), opts ...IterOption) ([]R, error) {
	n := len(items)
	if n == 0 {
		return nil, nil
	}

	cfg := applyIterOpts(opts, n)
	results := make([]R, n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.concurrency)

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		i, item := i, item
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i], errs[i] = fn(item)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// ForEach applies fn to each item concurrently.
// Go 1.24+ implementation.
func ForEach[T any](items []T, fn func(T) error, opts ...IterOption) error {
	n := len(items)
	if n == 0 {
		return nil
	}

	cfg := applyIterOpts(opts, n)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	sem := make(chan struct{}, cfg.concurrency)

	for _, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		item := item
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(item); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Filter returns items for which fn returns true, concurrently. Order is preserved.
// Go 1.24+ implementation uses slices.Collect for efficient result building.
func Filter[T any](items []T, fn func(T) (bool, error), opts ...IterOption) ([]T, error) {
	n := len(items)
	if n == 0 {
		return nil, nil
	}

	cfg := applyIterOpts(opts, n)
	keep := make([]bool, n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.concurrency)

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		i, item := i, item
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			keep[i], errs[i] = fn(item)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	result := slices.Collect(func(yield func(T) bool) {
		for i, ok := range keep {
			if ok && !yield(items[i]) {
				return
			}
		}
	})
	return result, nil
}

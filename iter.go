//go:build !go1.24

package kinetic

import (
	"sync"
)

// Map applies fn to each item concurrently and returns results in the same order.
func Map[T any, R any](items []T, fn func(T) (R, error), opts ...IterOption) ([]R, error) {
	cfg := applyIterOpts(opts, len(items))

	results := make([]R, len(items))
	errs := make([]error, len(items))

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
func ForEach[T any](items []T, fn func(T) error, opts ...IterOption) error {
	cfg := applyIterOpts(opts, len(items))

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
func Filter[T any](items []T, fn func(T) (bool, error), opts ...IterOption) ([]T, error) {
	cfg := applyIterOpts(opts, len(items))

	keep := make([]bool, len(items))
	errs := make([]error, len(items))

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

	var result []T
	for i, ok := range keep {
		if ok {
			result = append(result, items[i])
		}
	}
	return result, nil
}

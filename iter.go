//go:build !go1.24

package kinetic

import (
	"sync"
)

// Map applies fn to each item concurrently and returns results in the same order.
// Default concurrency equals the number of items. Use WithConcurrency to limit.
//
// Example:
//
//	results, err := kinetic.Map([]int{1, 2, 3}, func(n int) (int, error) {
//	    return n * 2, nil
//	})
//	// results == []int{2, 4, 6}, err == nil
//
// Example (with concurrency limit):
//
//	results, err := kinetic.Map(urls, func(u string) (Page, error) {
//	    return fetch(u)
//	}, kinetic.WithConcurrency(10))
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

// ForEach applies fn to each item concurrently. Returns the first error encountered.
//
// Example:
//
//	err := kinetic.ForEach([]string{"a", "b", "c"}, func(s string) error {
//	    return process(s)
//	})
//	// err == nil if all succeed
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
//
// Example:
//
//	evens, err := kinetic.Filter([]int{1, 2, 3, 4, 5}, func(n int) (bool, error) {
//	    return n%2 == 0, nil
//	})
//	// evens == []int{2, 4}, err == nil
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

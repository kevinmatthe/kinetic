package kinetic

import (
	"sync"
)

// Stream processes items concurrently while preserving input order in the output.
// Submit items with Go, then call Wait to get ordered results.
//
// Example:
//
//	s := kinetic.NewStream[string, int](4)
//	for _, url := range urls {
//	    url := url
//	    s.Go(url, func(u string) (int, error) {
//	        resp, err := http.Get(u)
//	        return len(resp.Body), err
//	    })
//	}
//	sizes, err := s.Wait()
//	// sizes[i] corresponds to urls[i], even though processing was concurrent
type Stream[T any, R any] struct {
	concurrency int
	items       []T
	fns         []func(T) (R, error)
}

// NewStream creates a Stream with the given maximum concurrency.
//
// Example:
//
//	s := kinetic.NewStream[Input, Output](10) // at most 10 concurrent workers
func NewStream[T any, R any](maxConcurrency int) *Stream[T, R] {
	return &Stream[T, R]{
		concurrency: maxConcurrency,
	}
}

// Go adds an item and its processing function to the stream.
// Items are processed concurrently, but results are returned in submission order by Wait.
//
// Example:
//
//	s := kinetic.NewStream[int, int](4)
//	s.Go(1, func(n int) (int, error) { return n * 2, nil })
//	s.Go(2, func(n int) (int, error) { return n * 3, nil })
func (s *Stream[T, R]) Go(item T, fn func(T) (R, error)) {
	s.items = append(s.items, item)
	s.fns = append(s.fns, fn)
}

// Wait blocks until all items are processed and returns results in submission order.
// Returns the first error encountered, if any.
//
// Example:
//
//	s := kinetic.NewStream[int, int](4)
//	for i := 0; i < 5; i++ {
//	    i := i
//	    s.Go(i, func(n int) (int, error) { return n * 2, nil })
//	}
//	results, err := s.Wait()
//	// results == []int{0, 2, 4, 6, 8}, err == nil
func (s *Stream[T, R]) Wait() ([]R, error) {
	n := len(s.items)
	if n == 0 {
		return nil, nil
	}

	results := make([]R, n)
	errs := make([]error, n)

	conc := s.concurrency
	if conc < 1 {
		conc = 1
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, conc)

	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		i := i
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i], errs[i] = s.fns[i](s.items[i])
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

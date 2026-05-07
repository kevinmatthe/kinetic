package kinetic

import (
	"sync"
)

// Stream processes items concurrently while preserving input order in the output.
type Stream[T any, R any] struct {
	concurrency int
	items       []T
	fns         []func(T) (R, error)
}

// NewStream creates a Stream with the given maximum concurrency.
func NewStream[T any, R any](maxConcurrency int) *Stream[T, R] {
	return &Stream[T, R]{
		concurrency: maxConcurrency,
	}
}

// Go adds an item and its processing function to the stream.
func (s *Stream[T, R]) Go(item T, fn func(T) (R, error)) {
	s.items = append(s.items, item)
	s.fns = append(s.fns, fn)
}

// Wait blocks until all items are processed and returns results in submission order.
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

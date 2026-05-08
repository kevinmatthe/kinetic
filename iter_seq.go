//go:build go1.24

package kinetic

import (
	"iter"
	"sync"
)

// MapSeq applies fn to each element yielded by seq concurrently and returns results
// in iteration order.
func MapSeq[T any, R any](seq iter.Seq[T], fn func(T) (R, error), opts ...IterOption) ([]R, error) {
	var items []T
	for v := range seq {
		items = append(items, v)
	}
	return Map(items, fn, opts...)
}

// ForEachSeq applies fn to each element yielded by seq concurrently.
func ForEachSeq[T any](seq iter.Seq[T], fn func(T) error, opts ...IterOption) error {
	var items []T
	for v := range seq {
		items = append(items, v)
	}
	return ForEach(items, fn, opts...)
}

// FilterSeq returns elements from seq for which fn returns true, concurrently.
// Order is preserved.
func FilterSeq[T any](seq iter.Seq[T], fn func(T) (bool, error), opts ...IterOption) ([]T, error) {
	var items []T
	for v := range seq {
		items = append(items, v)
	}
	return Filter(items, fn, opts...)
}

// MapSeqIter applies fn concurrently and returns an iter.Seq2 of results (lazy).
// Processing starts when the returned iterator is consumed.
func MapSeqIter[T any, R any](seq iter.Seq[T], fn func(T) (R, error), opts ...IterOption) iter.Seq2[R, error] {
	return func(yield func(R, error) bool) {
		cfg := applyIterOpts(opts, 16)

		type indexed struct {
			val R
			err error
		}

		var items []T
		for v := range seq {
			items = append(items, v)
		}

		out := make([]indexed, len(items))
		var wg sync.WaitGroup
		sem := make(chan struct{}, cfg.concurrency)

		for i, item := range items {
			wg.Add(1)
			sem <- struct{}{}
			i, item := i, item
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				out[i].val, out[i].err = fn(item)
			}()
		}
		wg.Wait()

		for _, o := range out {
			if !yield(o.val, o.err) {
				return
			}
		}
	}
}

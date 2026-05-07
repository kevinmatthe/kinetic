//go:build go1.24

package kinetic

import (
	"errors"
	"iter"
	"testing"
)

func TestMapSeq(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 5; i++ {
			if !yield(i) {
				return
			}
		}
	}

	results, err := MapSeq(seq, func(n int) (int, error) {
		return n * 2, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{2, 4, 6, 8, 10}
	for i, v := range results {
		if v != expected[i] {
			t.Fatalf("results[%d]: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestForEachSeq(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}

	sum := 0
	err := ForEachSeq(seq, func(n int) error {
		sum += n
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum != 6 {
		t.Fatalf("expected 6, got %d", sum)
	}
}

func TestFilterSeq(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 5; i++ {
			if !yield(i) {
				return
			}
		}
	}

	result, err := FilterSeq(seq, func(n int) (bool, error) {
		return n%2 == 0, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || result[0] != 2 || result[1] != 4 {
		t.Fatalf("expected [2,4], got %v", result)
	}
}

func TestMapSeqIter(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}

	var results []int
	var errs []error
	for v, err := range MapSeqIter(seq, func(n int) (int, error) {
		return n * 10, nil
	}) {
		results = append(results, v)
		errs = append(errs, err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, v := range results {
		if v != (i+1)*10 {
			t.Fatalf("results[%d]: expected %d, got %d", i, (i+1)*10, v)
		}
	}
	for _, err := range errs {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestMapSeqIter_WithError(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				return
			}
		}
	}

	var hasError bool
	for _, err := range MapSeqIter(seq, func(n int) (int, error) {
		if n == 2 {
			return 0, errors.New("fail at 2")
		}
		return n, nil
	}) {
		if err != nil {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("expected error")
	}
}

// Verify iter.Seq types exist at compile time.
var _ iter.Seq[int] = nil
var _ iter.Seq2[int, error] = nil

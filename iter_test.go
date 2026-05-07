//go:build !go1.24

package kinetic

import (
	"errors"
	"testing"
)

func TestMap(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results, err := Map(items, func(n int) (int, error) {
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

func TestMap_WithConcurrency(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results, err := Map(items, func(n int) (int, error) {
		return n * 2, nil
	}, WithConcurrency(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestMap_Error(t *testing.T) {
	items := []int{1, 2, 3}
	_, err := Map(items, func(n int) (int, error) {
		if n == 2 {
			return 0, errors.New("fail at 2")
		}
		return n, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMap_Empty(t *testing.T) {
	results, err := Map[int, int](nil, func(n int) (int, error) {
		return n, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestForEach(t *testing.T) {
	items := []int{1, 2, 3}
	sum := 0
	err := ForEach(items, func(n int) error {
		sum += n
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum != 6 {
		t.Fatalf("expected sum 6, got %d", sum)
	}
}

func TestForEach_Error(t *testing.T) {
	items := []int{1, 2, 3}
	err := ForEach(items, func(n int) error {
		if n == 2 {
			return errors.New("fail")
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilter(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	result, err := Filter(items, func(n int) (bool, error) {
		return n%2 == 0, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || result[0] != 2 || result[1] != 4 {
		t.Fatalf("expected [2,4], got %v", result)
	}
}

func TestFilter_Empty(t *testing.T) {
	result, err := Filter[int](nil, func(n int) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty, got %v", result)
	}
}

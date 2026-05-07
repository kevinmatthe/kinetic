package kinetic

import (
	"errors"
	"testing"
	"time"
)

func TestStream_Basic(t *testing.T) {
	s := NewStream[int, int](4)
	for i := 0; i < 5; i++ {
		i := i
		s.Go(i, func(n int) (int, error) {
			return n * 2, nil
		})
	}
	results, err := s.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{0, 2, 4, 6, 8}
	for i, v := range results {
		if v != expected[i] {
			t.Fatalf("results[%d]: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestStream_Error(t *testing.T) {
	s := NewStream[int, int](4)
	for i := 0; i < 3; i++ {
		i := i
		s.Go(i, func(n int) (int, error) {
			if n == 1 {
				return 0, errors.New("fail")
			}
			return n, nil
		})
	}
	_, err := s.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStream_Empty(t *testing.T) {
	s := NewStream[int, int](4)
	results, err := s.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty, got %v", results)
	}
}

func TestStream_OrderPreserved(t *testing.T) {
	s := NewStream[int, int](4)
	for i := 0; i < 10; i++ {
		i := i
		s.Go(i, func(n int) (int, error) {
			time.Sleep(time.Duration(10-i%3) * time.Millisecond) // varying durations
			return n, nil
		})
	}
	results, err := s.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range results {
		if v != i {
			t.Fatalf("order not preserved: results[%d] = %d", i, v)
		}
	}
}

func TestFanIn(t *testing.T) {
	ch1 := make(chan int, 3)
	ch2 := make(chan int, 3)

	ch1 <- 1
	ch1 <- 2
	ch2 <- 3
	close(ch1)
	close(ch2)

	merged := FanIn(ch1, ch2)
	var results []int
	for v := range merged {
		results = append(results, v)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 items, got %d", len(results))
	}
}

func TestFanOut(t *testing.T) {
	ch := make(chan int, 10)
	for i := 0; i < 10; i++ {
		ch <- i
	}
	close(ch)

	var sum int
	err := FanOut(ch, 3, func(n int) error {
		sum += n
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum != 45 { // 0+1+...+9
		t.Fatalf("expected sum 45, got %d", sum)
	}
}

func TestPipe(t *testing.T) {
	in := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		in <- i
	}
	close(in)

	out := Pipe(in, 3, func(n int) (int, error) {
		return n * 10, nil
	})

	var results []int
	for v := range out {
		results = append(results, v)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestPipe_WithError(t *testing.T) {
	in := make(chan int, 3)
	in <- 1
	in <- 2
	in <- 3
	close(in)

	out := Pipe(in, 2, func(n int) (int, error) {
		if n == 2 {
			return 0, errors.New("skip")
		}
		return n, nil
	})

	var results []int
	for v := range out {
		results = append(results, v)
	}
	// Item 2 should be skipped (error), only 1 and 3 should pass
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

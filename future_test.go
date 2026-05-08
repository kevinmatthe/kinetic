package kinetic

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestGo_Basic(t *testing.T) {
	f := Go(func() (int, error) {
		return 42, nil
	})

	val, err := f.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}

func TestGo_Error(t *testing.T) {
	f := Go(func() (int, error) {
		return 0, errors.New("fail")
	})

	val, err := f.Get()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "fail" {
		t.Fatalf("expected 'fail', got '%s'", err.Error())
	}
	if val != 0 {
		t.Fatalf("expected zero value, got %d", val)
	}
}

func TestGo_Val(t *testing.T) {
	f := Go(func() (string, error) {
		return "hello", nil
	})

	if f.Val() != "hello" {
		t.Fatalf("expected 'hello', got '%s'", f.Val())
	}
}

func TestGo_ValPanicsOnError(t *testing.T) {
	f := Go(func() (int, error) {
		return 0, errors.New("boom")
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	f.Val()
}

func TestGo_Err(t *testing.T) {
	f := Go(func() (int, error) {
		return 0, errors.New("fail")
	})

	if f.Err() == nil {
		t.Fatal("expected error")
	}
}

func TestGo_ErrNil(t *testing.T) {
	f := Go(func() (int, error) {
		return 1, nil
	})

	if f.Err() != nil {
		t.Fatal("expected nil error")
	}
}

func TestGo_Panic(t *testing.T) {
	f := Go(func() (int, error) {
		panic("oops")
	})

	_, err := f.Get()
	if err == nil {
		t.Fatal("expected error from panic")
	}

	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got %T", err)
	}
	if pe.Value != "oops" {
		t.Fatalf("expected panic value 'oops', got %v", pe.Value)
	}
}

func TestGo_After(t *testing.T) {
	order := []string{}

	a := Go(func() (string, error) {
		time.Sleep(50 * time.Millisecond)
		order = append(order, "a")
		return "a", nil
	})

	b := Go(func() (string, error) {
		time.Sleep(30 * time.Millisecond)
		order = append(order, "b")
		return "b", nil
	})

	// c depends on a and b — they must complete before c runs
	c := Go(func() (string, error) {
		order = append(order, "c")
		return a.Val() + b.Val(), nil
	}, After(a, b))

	val, err := c.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "ab" {
		t.Fatalf("expected 'ab', got '%s'", val)
	}

	// a and b must both appear before c in the order
	aIdx, bIdx, cIdx := -1, -1, -1
	for i, s := range order {
		switch s {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		}
	}
	if cIdx < aIdx || cIdx < bIdx {
		t.Fatalf("c must run after a and b, order: %v", order)
	}
}

func TestGo_WithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	f := Go(func() (int, error) {
		return 42, nil
	}, WithContext(ctx))

	_, err := f.Get()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGo_WithContextTimeout(t *testing.T) {
	// Context cancels during dependency wait — Future should fail without running fn.
	slowDep := Go(func() (int, error) {
		time.Sleep(1 * time.Second)
		return 1, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	f := Go(func() (int, error) {
		return slowDep.Val() * 2, nil
	}, After(slowDep), WithContext(ctx))

	_, err := f.Get()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestGo_AfterAndContext(t *testing.T) {
	a := Go(func() (int, error) {
		return 10, nil
	})

	f := Go(func() (int, error) {
		return a.Val() * 2, nil
	}, After(a), WithContext(context.Background()))

	val, err := f.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 20 {
		t.Fatalf("expected 20, got %d", val)
	}
}

func TestWaitAll(t *testing.T) {
	a := Go(func() (int, error) {
		time.Sleep(50 * time.Millisecond)
		return 1, nil
	})
	b := Go(func() (string, error) {
		time.Sleep(30 * time.Millisecond)
		return "x", nil
	})

	WaitAll(a, b)

	// After WaitAll, both should be safe to read
	if a.Val() != 1 {
		t.Fatalf("expected 1")
	}
	if b.Val() != "x" {
		t.Fatalf("expected 'x'")
	}
}

func TestGo_Concurrent(t *testing.T) {
	const n = 100
	futures := make([]*Future[int], n)
	for i := 0; i < n; i++ {
		i := i
		futures[i] = Go(func() (int, error) {
			return i, nil
		})
	}

	WaitAll(castToAwaitable(futures)...)

	for i, f := range futures {
		if f.Val() != i {
			t.Fatalf("futures[%d]: expected %d, got %d", i, i, f.Val())
		}
	}
}

// --- helpers for tests ---

func castToAwaitable[T any](futures []*Future[T]) []Awaitable {
	result := make([]Awaitable, len(futures))
	for i, f := range futures {
		result[i] = f
	}
	return result
}

func TestMain(m *testing.M) {
	// Run all benchmarks as tests to verify correctness
	fmt.Println("Running tests...")
	m.Run()
}

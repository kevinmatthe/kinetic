package kinetic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebounce(t *testing.T) {
	var count atomic.Int32
	debounced := Debounce(func(s string) {
		count.Add(1)
	}, 50*time.Millisecond)

	for i := 0; i < 10; i++ {
		debounced("hello")
	}

	time.Sleep(100 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("expected 1 call (debounced), got %d", count.Load())
	}
}

func TestThrottle(t *testing.T) {
	var count atomic.Int32
	throttled := Throttle(func(s string) {
		count.Add(1)
	}, 50*time.Millisecond)

	throttled("a") // immediate
	throttled("b") // dropped
	throttled("c") // dropped

	time.Sleep(60 * time.Millisecond)
	throttled("d") // allowed now

	time.Sleep(20 * time.Millisecond)
	if count.Load() != 2 {
		t.Fatalf("expected 2 calls (throttled), got %d", count.Load())
	}
}

func TestOnce(t *testing.T) {
	var calls atomic.Int32
	o := NewOnce(func() (int, error) {
		calls.Add(1)
		return 42, nil
	})

	for i := 0; i < 10; i++ {
		val, err := o.Get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 42 {
			t.Fatalf("expected 42, got %d", val)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", calls.Load())
	}
}

func TestOnce_Error(t *testing.T) {
	o := NewOnce(func() (int, error) {
		return 0, errors.New("fail")
	})

	_, err := o.Get()
	if err == nil {
		t.Fatal("expected error")
	}

	// Second call should return same error
	_, err2 := o.Get()
	if err2 == nil {
		t.Fatal("expected same error")
	}
}

func TestLazy(t *testing.T) {
	var computed atomic.Int32
	l := NewLazy(func() (string, error) {
		computed.Add(1)
		return "lazy", nil
	})

	if computed.Load() != 0 {
		t.Fatal("lazy should not compute until Get()")
	}

	val, err := l.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "lazy" {
		t.Fatalf("expected 'lazy', got '%s'", val)
	}
	if computed.Load() != 1 {
		t.Fatalf("expected 1 computation, got %d", computed.Load())
	}
}

func TestSemaphore(t *testing.T) {
	sem := NewSemaphore(3)

	if !sem.TryAcquire() {
		t.Fatal("expected acquire to succeed")
	}
	if !sem.TryAcquire() {
		t.Fatal("expected acquire to succeed")
	}
	if !sem.TryAcquire() {
		t.Fatal("expected acquire to succeed")
	}
	if sem.TryAcquire() {
		t.Fatal("expected acquire to fail (full)")
	}

	sem.Release()
	if !sem.TryAcquire() {
		t.Fatal("expected acquire to succeed after release")
	}
}

func TestSemaphore_ContextCancel(t *testing.T) {
	sem := NewSemaphore(1)
	sem.TryAcquire() // fill it

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := sem.Acquire(ctx)
	if err == nil {
		t.Fatal("expected context deadline exceeded")
	}
}

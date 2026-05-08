package kinetic

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_Basic(t *testing.T) {
	p := NewPool(4)
	var count atomic.Int32
	for i := 0; i < 100; i++ {
		p.Go(func() {
			count.Add(1)
		})
	}
	p.Wait()
	if count.Load() != 100 {
		t.Fatalf("expected 100, got %d", count.Load())
	}
}

func TestPool_ConcurrencyLimit(t *testing.T) {
	const max = 3
	p := NewPool(max)
	var running atomic.Int32
	var maxRunning atomic.Int32

	for i := 0; i < 100; i++ {
		p.Go(func() {
			cur := running.Add(1)
			for {
				old := maxRunning.Load()
				if cur <= old || maxRunning.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			running.Add(-1)
		})
	}
	p.Wait()

	if maxRunning.Load() > max {
		t.Fatalf("max concurrent exceeded: %d > %d", maxRunning.Load(), max)
	}
}

func TestPool_Panic(t *testing.T) {
	p := NewPool(4)
	p.Go(func() { panic("boom") })

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate")
		}
	}()
	p.Wait()
}

func TestResultPool(t *testing.T) {
	rp := NewResultPool[int](4)
	for i := 0; i < 10; i++ {
		i := i
		rp.Go(func() int { return i * 2 })
	}
	results := rp.Wait()
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	sum := 0
	for _, v := range results {
		sum += v
	}
	if sum != 90 { // 0+2+4+...+18 = 90
		t.Fatalf("expected sum 90, got %d", sum)
	}
}

func TestErrorPool_NoErrors(t *testing.T) {
	ep := NewErrorPool(4)
	for i := 0; i < 10; i++ {
		ep.Go(func() error { return nil })
	}
	if err := ep.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestErrorPool_WithErrors(t *testing.T) {
	ep := NewErrorPool(4)
	ep.Go(func() error { return errors.New("err1") })
	ep.Go(func() error { return errors.New("err2") })
	ep.Go(func() error { return nil })

	err := ep.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestContextPool_CancelOnError(t *testing.T) {
	ctx := context.Background()
	cp := NewContextPool(ctx, 4)

	var cancelled atomic.Int32

	cp.Go(func(ctx context.Context) error {
		return errors.New("fail fast")
	})
	cp.Go(func(ctx context.Context) error {
		<-ctx.Done()
		cancelled.Add(1)
		return ctx.Err()
	})

	err := cp.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
}

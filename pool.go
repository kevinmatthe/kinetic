package kinetic

import (
	"context"
	"errors"
	"sync"
)

// Pool is a concurrency-limited task runner. At most maxGoroutines run concurrently.
// Panics in tasks are recovered and re-panicked on Wait().
//
// Example:
//
//	p := kinetic.NewPool(4)
//	for i := 0; i < 100; i++ {
//	    i := i
//	    p.Go(func() { process(i) })
//	}
//	p.Wait() // blocks until all 100 tasks complete
type Pool struct {
	maxGoroutines int
	tokens        chan struct{}
	wg            sync.WaitGroup

	panicMu sync.Mutex
	panics  []recoveredPanic
}

type recoveredPanic struct {
	value interface{}
}

// NewPool creates a Pool with the given maximum concurrency.
//
// Example:
//
//	p := kinetic.NewPool(10) // at most 10 concurrent goroutines
func NewPool(maxGoroutines int) *Pool {
	p := &Pool{
		maxGoroutines: maxGoroutines,
		tokens:        make(chan struct{}, maxGoroutines),
	}
	for i := 0; i < maxGoroutines; i++ {
		p.tokens <- struct{}{}
	}
	return p
}

// Go submits a function to be executed in the pool.
// The function runs when a concurrency slot is available.
//
// Example:
//
//	p := kinetic.NewPool(4)
//	p.Go(func() { fmt.Println("task 1") })
//	p.Go(func() { fmt.Println("task 2") })
func (p *Pool) Go(fn func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		<-p.tokens        // acquire token
		defer p.release() // release token

		defer func() {
			if r := recover(); r != nil {
				p.panicMu.Lock()
				p.panics = append(p.panics, recoveredPanic{value: r})
				p.panicMu.Unlock()
			}
		}()
		fn()
	}()
}

func (p *Pool) release() {
	p.tokens <- struct{}{}
}

// Wait blocks until all submitted tasks have completed.
// If any task panicked, Wait re-panics with the first panic value.
//
// Example:
//
//	p := kinetic.NewPool(4)
//	p.Go(func() { doWork() })
//	p.Wait() // blocks until doWork finishes
func (p *Pool) Wait() {
	p.wg.Wait()
	p.panicMu.Lock()
	panics := p.panics
	p.panics = nil
	p.panicMu.Unlock()

	if len(panics) > 0 {
		panic(panics[0].value)
	}
}

// ResultPool is a Pool that collects results from tasks into a slice.
//
// Example:
//
//	rp := kinetic.NewResultPool[string](4)
//	for _, url := range urls {
//	    url := url
//	    rp.Go(func() string { return fetch(url) })
//	}
//	results := rp.Wait() // []string, e.g. ["body1", "body2", "body3"]
type ResultPool[T any] struct {
	pool     Pool
	results  []T
	resultMu sync.Mutex
}

// NewResultPool creates a ResultPool with the given maximum concurrency.
//
// Example:
//
//	rp := kinetic.NewResultPool[int](10)
func NewResultPool[T any](maxGoroutines int) *ResultPool[T] {
	return &ResultPool[T]{
		pool: *NewPool(maxGoroutines),
	}
}

// Go submits a function that returns a result. Results are appended to the internal slice.
//
// Example:
//
//	rp := kinetic.NewResultPool[int](4)
//	for i := 0; i < 5; i++ {
//	    i := i
//	    rp.Go(func() int { return i * 2 })
//	}
//	results := rp.Wait() // e.g. [0, 2, 4, 6, 8] (order not guaranteed)
func (rp *ResultPool[T]) Go(fn func() T) {
	rp.pool.Go(func() {
		val := fn()
		rp.resultMu.Lock()
		rp.results = append(rp.results, val)
		rp.resultMu.Unlock()
	})
}

// Wait blocks until all tasks complete and returns the collected results.
// Order is not guaranteed across concurrent tasks.
func (rp *ResultPool[T]) Wait() []T {
	rp.pool.Wait()
	return rp.results
}

// ErrorPool is a Pool that collects errors from tasks.
//
// Example:
//
//	ep := kinetic.NewErrorPool(4)
//	for _, item := range items {
//	    item := item
//	    ep.Go(func() error { return process(item) })
//	}
//	err := ep.Wait() // nil if all succeeded, joined error if any failed
type ErrorPool struct {
	pool  Pool
	errs  []error
	errMu sync.Mutex
}

// NewErrorPool creates an ErrorPool with the given maximum concurrency.
func NewErrorPool(maxGoroutines int) *ErrorPool {
	return &ErrorPool{
		pool: *NewPool(maxGoroutines),
	}
}

// Go submits a function that may return an error. Non-nil errors are collected.
//
// Example:
//
//	ep := kinetic.NewErrorPool(4)
//	ep.Go(func() error { return validate(input1) })
//	ep.Go(func() error { return validate(input2) })
//	err := ep.Wait() // nil or joined errors
func (ep *ErrorPool) Go(fn func() error) {
	ep.pool.Go(func() {
		err := fn()
		if err != nil {
			ep.errMu.Lock()
			ep.errs = append(ep.errs, err)
			ep.errMu.Unlock()
		}
	})
}

// Wait blocks until all tasks complete and returns the combined error (if any).
// Multiple errors are joined using errors.Join.
func (ep *ErrorPool) Wait() error {
	ep.pool.Wait()
	if len(ep.errs) == 0 {
		return nil
	}
	return joinErrors(ep.errs...)
}

// ContextPool is a Pool that cancels remaining tasks when any task returns an error.
// Tasks receive the pool's derived context and should respect ctx.Done().
//
// Example:
//
//	ctx := context.Background()
//	cp := kinetic.NewContextPool(ctx, 4)
//	cp.Go(func(ctx context.Context) error {
//	    return fetch(ctx, url1)
//	})
//	cp.Go(func(ctx context.Context) error {
//	    select {
//	    case <-ctx.Done():
//	        return ctx.Err() // cancelled because url1 failed
//	    default:
//	        return fetch(ctx, url2)
//	    }
//	})
//	err := cp.Wait() // error from url1
type ContextPool struct {
	pool   Pool
	ctx    context.Context
	cancel context.CancelFunc
	errMu  sync.Mutex
	errs   []error
}

// NewContextPool creates a ContextPool with the given parent context and maximum concurrency.
// If any task returns an error, the pool's context is cancelled to signal other tasks.
func NewContextPool(ctx context.Context, maxGoroutines int) *ContextPool {
	ctx, cancel := context.WithCancel(ctx)
	return &ContextPool{
		pool:   *NewPool(maxGoroutines),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Go submits a function that receives the pool's context.
// If the function returns an error, the context is cancelled.
//
// Example:
//
//	cp.Go(func(ctx context.Context) error {
//	    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
//	    _, err := client.Do(req)
//	    return err
//	})
func (cp *ContextPool) Go(fn func(ctx context.Context) error) {
	cp.pool.Go(func() {
		err := fn(cp.ctx)
		if err != nil {
			cp.errMu.Lock()
			cp.errs = append(cp.errs, err)
			cp.errMu.Unlock()
			cp.cancel()
		}
	})
}

// Wait blocks until all tasks complete and returns the combined error.
func (cp *ContextPool) Wait() error {
	cp.pool.Wait()
	cp.cancel()
	if len(cp.errs) == 0 {
		return nil
	}
	return joinErrors(cp.errs...)
}

// --- helpers ---

func joinErrors(errs ...error) error {
	return errors.Join(errs...)
}

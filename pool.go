package kinetic

import (
	"context"
	"errors"
	"sync"
)

// Pool is a concurrency-limited task runner. All tasks are executed in goroutines,
// but at most maxGoroutines run concurrently.
// Panics in tasks are recovered and re-panicked with full stack traces on Wait().
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
func (p *Pool) Go(fn func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		<-p.tokens         // acquire token
		defer p.release()  // release token

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

// ResultPool is a Pool that collects results from tasks.
type ResultPool[T any] struct {
	pool     Pool
	results  []T
	resultMu sync.Mutex
}

// NewResultPool creates a ResultPool with the given maximum concurrency.
func NewResultPool[T any](maxGoroutines int) *ResultPool[T] {
	return &ResultPool[T]{
		pool: *NewPool(maxGoroutines),
	}
}

// Go submits a function that returns a result.
func (rp *ResultPool[T]) Go(fn func() T) {
	rp.pool.Go(func() {
		val := fn()
		rp.resultMu.Lock()
		rp.results = append(rp.results, val)
		rp.resultMu.Unlock()
	})
}

// Wait blocks until all tasks complete and returns the collected results.
// Results are in submission order if there is no concurrency race,
// but order is not guaranteed across concurrent tasks.
func (rp *ResultPool[T]) Wait() []T {
	rp.pool.Wait()
	return rp.results
}

// ErrorPool is a Pool that collects errors from tasks.
type ErrorPool struct {
	pool   Pool
	errs   []error
	errMu  sync.Mutex
}

// NewErrorPool creates an ErrorPool with the given maximum concurrency.
func NewErrorPool(maxGoroutines int) *ErrorPool {
	return &ErrorPool{
		pool: *NewPool(maxGoroutines),
	}
}

// Go submits a function that may return an error.
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
type ContextPool struct {
	pool Pool
	ctx  context.Context
	cancel context.CancelFunc
	errMu  sync.Mutex
	errs   []error
}

// NewContextPool creates a ContextPool with the given parent context and maximum concurrency.
// If any task returns an error, the context is cancelled.
func NewContextPool(ctx context.Context, maxGoroutines int) *ContextPool {
	ctx, cancel := context.WithCancel(ctx)
	return &ContextPool{
		pool:   *NewPool(maxGoroutines),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Go submits a function that may return an error. The function receives the pool's context.
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

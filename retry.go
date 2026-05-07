package kinetic

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryOption configures retry behavior. Passed as variadic arguments to Retry().
type RetryOption interface {
	applyRetry(*retryConfig)
}

type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	multiplier  float64
	jitter      bool
	retryIf     func(error) bool
	ctx         context.Context
}

type retryOptionFunc func(*retryConfig)

func (f retryOptionFunc) applyRetry(c *retryConfig) { f(c) }

// Attempts sets the maximum number of attempts (including the first call).
// Default is 3.
//
// Example:
//
//	f := kinetic.Retry(fn, kinetic.Attempts(5))
//	// fn will be called up to 5 times
func Attempts(n int) RetryOption {
	return retryOptionFunc(func(c *retryConfig) { c.maxAttempts = n })
}

// Backoff sets exponential backoff with the given base delay and multiplier.
// Default: no backoff (immediate retry).
//
// Example:
//
//	f := kinetic.Retry(fn, kinetic.Backoff(100*time.Millisecond, 2.0))
//	// delays: 100ms, 200ms, 400ms, ...
func Backoff(baseDelay time.Duration, multiplier float64) RetryOption {
	return retryOptionFunc(func(c *retryConfig) {
		c.baseDelay = baseDelay
		c.multiplier = multiplier
	})
}

// MaxDelay caps the delay between retries.
//
// Example:
//
//	f := kinetic.Retry(fn,
//	    kinetic.Backoff(100*time.Millisecond, 3.0),
//	    kinetic.MaxDelay(5*time.Second),
//	)
//	// delays: 100ms, 300ms, 900ms, 2700ms, 5000ms(capped), 5000ms(capped), ...
func MaxDelay(d time.Duration) RetryOption {
	return retryOptionFunc(func(c *retryConfig) { c.maxDelay = d })
}

// Jitter enables randomized jitter on backoff delays to avoid thundering herd.
// Adds 50-100% randomness to each delay.
//
// Example:
//
//	f := kinetic.Retry(fn,
//	    kinetic.Backoff(100*time.Millisecond, 2.0),
//	    kinetic.Jitter(),
//	)
//	// delays: ~50-100ms, ~100-200ms, ~200-400ms, ...
func Jitter() RetryOption {
	return retryOptionFunc(func(c *retryConfig) { c.jitter = true })
}

// RetryIf limits retries to errors that match the predicate.
// By default, all errors are retried.
//
// Example:
//
//	f := kinetic.Retry(fn, kinetic.RetryIf(func(err error) bool {
//	    return isRetryable(err) // only retry on transient errors
//	}))
func RetryIf(fn func(error) bool) RetryOption {
	return retryOptionFunc(func(c *retryConfig) { c.retryIf = fn })
}

// RetryContext sets the context for cancellation. If the context is cancelled
// during a retry wait, the retry stops and returns the context error.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	f := kinetic.Retry(fn, kinetic.Backoff(1*time.Second, 2.0), kinetic.RetryContext(ctx))
//	// retries will stop after 5 seconds total
func RetryContext(ctx context.Context) RetryOption {
	return retryOptionFunc(func(c *retryConfig) { c.ctx = ctx })
}

// Retry retries a function up to the configured number of attempts with optional backoff.
// It returns a Future that resolves to the function's result on success,
// or the last error if all attempts fail.
//
// Example (basic):
//
//	f := kinetic.Retry(func() (string, error) {
//	    return unreliableAPI()
//	}, kinetic.Attempts(3))
//	val, err := f.Get()
//	// val == "ok" if succeeds within 3 attempts
//	// err != nil if all 3 attempts fail
//
// Example (full configuration):
//
//	f := kinetic.Retry(fn,
//	    kinetic.Attempts(5),
//	    kinetic.Backoff(100*time.Millisecond, 2.0),
//	    kinetic.MaxDelay(10*time.Second),
//	    kinetic.Jitter(),
//	    kinetic.RetryIf(isRetryable),
//	    kinetic.RetryContext(ctx),
//	)
func Retry[T any](fn func() (T, error), opts ...RetryOption) *Future[T] {
	cfg := retryConfig{
		maxAttempts: 3,
		multiplier:  2.0,
		retryIf:     func(error) bool { return true },
	}
	for _, opt := range opts {
		opt.applyRetry(&cfg)
	}

	return Go(func() (T, error) {
		var (
			val   T
			err   error
			delay time.Duration
		)

		for attempt := 0; attempt < cfg.maxAttempts; attempt++ {
			if attempt > 0 && delay > 0 {
				timer := time.NewTimer(delay)
				if cfg.ctx != nil {
					select {
					case <-timer.C:
					case <-cfg.ctx.Done():
						timer.Stop()
						var zero T
						return zero, cfg.ctx.Err()
					}
				} else {
					<-timer.C
				}
			}

			val, err = fn()
			if err == nil {
				return val, nil
			}
			if !cfg.retryIf(err) {
				var zero T
				return zero, err
			}

			// Calculate next delay.
			if cfg.baseDelay > 0 {
				delay = time.Duration(float64(cfg.baseDelay) * math.Pow(cfg.multiplier, float64(attempt)))
				if cfg.maxDelay > 0 && delay > cfg.maxDelay {
					delay = cfg.maxDelay
				}
				if cfg.jitter {
					delay = time.Duration(float64(delay) * (0.5 + rand.Float64()*0.5))
				}
			}
		}

		var zero T
		return zero, err
	}, contextOption(cfg.ctx)...)
}

func contextOption(ctx context.Context) []Option {
	if ctx == nil {
		return nil
	}
	return []Option{WithContext(ctx)}
}

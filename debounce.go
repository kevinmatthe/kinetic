package kinetic

import (
	"sync"
	"time"
)

// Debounce delays fn execution until after wait has elapsed since the last call.
// Only the last call's arguments are used. Returns a cancel function to flush immediately.
func Debounce[T any](fn func(T), wait time.Duration) func(T) {
	var (
		mu    sync.Mutex
		timer *time.Timer
		arg   T
	)

	return func(v T) {
		mu.Lock()
		arg = v
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(wait, func() {
			mu.Lock()
			val := arg
			mu.Unlock()
			fn(val)
		})
		mu.Unlock()
	}
}

// Throttle ensures fn is called at most once per interval.
// The first call executes immediately; subsequent calls within the interval are dropped.
func Throttle[T any](fn func(T), interval time.Duration) func(T) {
	var (
		mu       sync.Mutex
		lastCall time.Time
	)

	return func(v T) {
		mu.Lock()
		now := time.Now()
		if now.Sub(lastCall) >= interval {
			lastCall = now
			mu.Unlock()
			fn(v)
			return
		}
		mu.Unlock()
	}
}

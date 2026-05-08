package kinetic

import (
	"sync"
	"time"
)

// Debounce delays fn execution until after wait has elapsed since the last call.
// Only the last call's arguments are used. Returns a function that can be called
// to trigger the debounced operation.
//
// Example:
//
//	save := kinetic.Debounce(func(content string) {
//	    writeToFile(content)
//	}, 300*time.Millisecond)
//	save("hello")  // starts 300ms timer
//	save("world")  // resets timer, "hello" is discarded
//	// after 300ms with no further calls: writeToFile("world") is called
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
//
// Example:
//
//	log := kinetic.Throttle(func(event string) {
//	    fmt.Println(event)
//	}, 1*time.Second)
//	log("click1") // prints "click1" immediately
//	log("click2") // dropped (within 1s)
//	log("click3") // dropped (within 1s)
//	time.Sleep(1 * time.Second)
//	log("click4") // prints "click4"
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

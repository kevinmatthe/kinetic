package kinetic

import (
	"sync"
)

// FanIn merges multiple channels into a single output channel.
// The output channel is closed when all input channels are closed.
//
// Example:
//
//	ch1 := produceUsers()
//	ch2 := produceOrders()
//	merged := kinetic.FanIn(ch1, ch2) // <-chan any
//	for item := range merged {
//	    // receives from both ch1 and ch2
//	}
func FanIn[T any](channels ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup

	for _, ch := range channels {
		wg.Add(1)
		ch := ch
		go func() {
			defer wg.Done()
			for v := range ch {
				out <- v
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// FanOut distributes items from a source channel to N workers running concurrently.
// It blocks until the source channel is closed and all workers finish.
// Returns the first error encountered, if any.
//
// Example:
//
//	ch := produceJobs()
//	err := kinetic.FanOut(ch, 5, func(job Job) error {
//	    return process(job)
//	})
//	// all jobs processed by 5 concurrent workers
func FanOut[T any](src <-chan T, workers int, fn func(T) error) error {
	if workers < 1 {
		workers = 1
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range src {
				if err := fn(item); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Pipe transforms items from an input channel through fn with bounded concurrency,
// producing an output channel. Items that produce errors are dropped.
// The output channel is closed when the input channel is closed and all processing is done.
//
// Example:
//
//	in := produceURLs()
//	out := kinetic.Pipe(in, 10, func(url string) (string, error) {
//	    resp, err := http.Get(url)
//	    return string(resp.Body), err
//	})
//	for body := range out {
//	    fmt.Println(body)
//	}
func Pipe[T any, R any](in <-chan T, maxConcurrency int, fn func(T) (R, error)) <-chan R {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}

	out := make(chan R)

	go func() {
		defer close(out)

		var wg sync.WaitGroup
		sem := make(chan struct{}, maxConcurrency)

		for item := range in {
			wg.Add(1)
			sem <- struct{}{}
			item := item
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				result, err := fn(item)
				if err == nil {
					out <- result
				}
			}()
		}
		wg.Wait()
	}()

	return out
}

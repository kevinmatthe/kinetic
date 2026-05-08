package kinetic

// IterOption configures concurrent iteration.
// Passed as variadic arguments to Map, ForEach, and Filter.
type IterOption interface {
	applyIter(*iterOption)
}

type iterOption struct {
	concurrency int
}

type iterOptionFunc func(*iterOption)

func (f iterOptionFunc) applyIter(o *iterOption) { f(o) }

// WithConcurrency sets the maximum number of concurrent workers for iteration functions.
// Default concurrency equals the number of items (unlimited parallelism).
//
// Example:
//
//	results, err := kinetic.Map(items, fn, kinetic.WithConcurrency(10))
//	// at most 10 goroutines process items concurrently
func WithConcurrency(n int) IterOption {
	return iterOptionFunc(func(o *iterOption) { o.concurrency = n })
}

func applyIterOpts(opts []IterOption, defaultConcurrency int) iterOption {
	cfg := iterOption{concurrency: defaultConcurrency}
	for _, opt := range opts {
		opt.applyIter(&cfg)
	}
	if cfg.concurrency < 1 {
		cfg.concurrency = 1
	}
	return cfg
}

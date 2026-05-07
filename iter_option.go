package kinetic

// iterOption configures concurrent iteration behavior.
type iterOption struct {
	concurrency int
}

// IterOption configures concurrent iteration.
type IterOption interface {
	applyIter(*iterOption)
}

type iterOptionFunc func(*iterOption)

func (f iterOptionFunc) applyIter(o *iterOption) { f(o) }

// WithConcurrency sets the maximum number of concurrent workers for iteration.
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

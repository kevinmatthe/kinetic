package kinetic

import (
	"context"
	"errors"
	"testing"
)

// --- Future benchmarks ---

func BenchmarkGo_Get(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f := Go(func() (int, error) { return 42, nil })
		f.Get()
	}
}

func BenchmarkGo_Get_WithDeps(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a := Go(func() (int, error) { return 1, nil })
		f := Go(func() (int, error) { return a.Val() + 1, nil }, After(a))
		f.Get()
	}
}

func BenchmarkGo_Get_WithContext(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f := Go(func() (int, error) { return 42, nil }, WithContext(ctx))
		f.Get()
	}
}

func BenchmarkWaitAll_N100(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		futures := make([]*Future[int], 100)
		for j := 0; j < 100; j++ {
			futures[j] = Go(func() (int, error) { return 1, nil })
		}
		WaitAll(castToAwaitable(futures)...)
	}
}

func BenchmarkGo_Val_AlreadyCompleted(b *testing.B) {
	f := Go(func() (int, error) { return 42, nil })
	f.Wait()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Val()
	}
}

func BenchmarkGo_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f := Go(func() (int, error) { return 42, nil })
			f.Get()
		}
	})
}

// --- Combine benchmarks ---

func BenchmarkAll_N100(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		futures := make([]*Future[int], 100)
		for j := 0; j < 100; j++ {
			futures[j] = Go(func() (int, error) { return 1, nil })
		}
		All(futures...).Get()
	}
}

func BenchmarkRace_N10(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		futures := make([]*Future[int], 10)
		for j := 0; j < 10; j++ {
			futures[j] = Go(func() (int, error) { return 1, nil })
		}
		Race(futures...).Get()
	}
}

// --- Pool benchmarks ---

func BenchmarkPool_Go_Wait(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := NewPool(10)
		for j := 0; j < 100; j++ {
			p.Go(func() {})
		}
		p.Wait()
	}
}

func BenchmarkResultPool_Go_Wait(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := NewResultPool[int](10)
		for j := 0; j < 100; j++ {
			p.Go(func() int { return j })
		}
		p.Wait()
	}
}

// --- Iter benchmarks ---

func BenchmarkMap_N1000_Conc10(b *testing.B) {
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Map(items, func(n int) (int, error) { return n * 2, nil }, WithConcurrency(10))
	}
}

func BenchmarkFilter_N1000_Conc10(b *testing.B) {
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Filter(items, func(n int) (bool, error) { return n%2 == 0, nil }, WithConcurrency(10))
	}
}

// --- Retry benchmarks ---

func BenchmarkRetry_ImmediateSuccess(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Retry(func() (int, error) { return 42, nil }, Attempts(3)).Get()
	}
}

func BenchmarkRetry_OneRetry(b *testing.B) {
	var calls int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calls = 0
		Retry(func() (int, error) {
			calls++
			if calls == 1 {
				return 0, errors.New("retry")
			}
			return 42, nil
		}, Attempts(3)).Get()
	}
}

// --- Stream benchmarks ---

func BenchmarkStream_N100_Conc10(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := NewStream[int, int](10)
		for j := 0; j < 100; j++ {
			j := j
			s.Go(j, func(n int) (int, error) { return n * 2, nil })
		}
		s.Wait()
	}
}

// --- Pipe benchmarks ---

func BenchmarkPipe_N100_Conc10(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		in := make(chan int, 100)
		for j := 0; j < 100; j++ {
			in <- j
		}
		close(in)
		out := Pipe(in, 10, func(n int) (int, error) { return n * 2, nil })
		for range out {
		}
	}
}

// --- Semaphore benchmarks ---

func BenchmarkSemaphore_AcquireRelease(b *testing.B) {
	sem := NewSemaphore(10)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sem.Acquire(context.Background())
		sem.Release()
	}
}

// --- Once/Lazy benchmarks ---

func BenchmarkOnce_Get(b *testing.B) {
	o := NewOnce(func() (int, error) { return 42, nil })
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		o.Get()
	}
}

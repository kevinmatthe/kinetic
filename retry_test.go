package kinetic

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetry_Success(t *testing.T) {
	f := Retry(func() (string, error) {
		return "ok", nil
	})

	val, err := f.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "ok" {
		t.Fatalf("expected 'ok', got '%s'", val)
	}
}

func TestRetry_EventualSuccess(t *testing.T) {
	var attempts int
	f := Retry(func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("not yet")
		}
		return "finally", nil
	}, Attempts(5))

	val, err := f.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "finally" {
		t.Fatalf("expected 'finally', got '%s'", val)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_AllFail(t *testing.T) {
	f := Retry(func() (int, error) {
		return 0, errors.New("always fail")
	}, Attempts(2))

	_, err := f.Get()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRetry_RetryIf(t *testing.T) {
	var attempts int
	f := Retry(func() (int, error) {
		attempts++
		if attempts == 1 {
			return 0, errors.New("retryable")
		}
		return 0, errors.New("permanent")
	}, Attempts(5), RetryIf(func(err error) bool {
		return err.Error() == "retryable"
	}))

	_, err := f.Get()
	if err == nil || err.Error() != "permanent" {
		t.Fatalf("expected 'permanent' error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetry_Backoff(t *testing.T) {
	var attempts int
	start := time.Now()

	f := Retry(func() (string, error) {
		attempts++
		return "", errors.New("fail")
	}, Attempts(4), Backoff(10*time.Millisecond, 2.0))

	_, _ = f.Get()
	elapsed := time.Since(start)

	if attempts != 4 {
		t.Fatalf("expected 4 attempts, got %d", attempts)
	}
	// Delays: 10ms, 20ms, 40ms = ~70ms minimum
	if elapsed < 60*time.Millisecond {
		t.Fatalf("expected at least 60ms with backoff, got %v", elapsed)
	}
}

func TestRetry_MaxDelay(t *testing.T) {
	var attempts int
	start := time.Now()

	f := Retry(func() (string, error) {
		attempts++
		return "", errors.New("fail")
	}, Attempts(5), Backoff(10*time.Millisecond, 3.0), MaxDelay(20*time.Millisecond))

	_, _ = f.Get()
	elapsed := time.Since(start)

	// Delays capped at 20ms: 10ms, 20ms, 20ms, 20ms = ~70ms
	if elapsed < 60*time.Millisecond {
		t.Fatalf("expected delays capped, got %v", elapsed)
	}
}

func TestRetry_WithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	var attempts int
	f := Retry(func() (string, error) {
		attempts++
		return "", errors.New("fail")
	}, Attempts(100), Backoff(10*time.Millisecond, 2.0), RetryContext(ctx))

	_, err := f.Get()
	if err == nil {
		t.Fatal("expected error")
	}
	// Should not have retried all 100 times
	if attempts >= 100 {
		t.Fatalf("context should have cancelled retries, got %d attempts", attempts)
	}
}

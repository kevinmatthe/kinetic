package kinetic

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestAll_Success(t *testing.T) {
	a := Go(func() (int, error) { return 1, nil })
	b := Go(func() (int, error) { return 2, nil })
	c := Go(func() (int, error) { return 3, nil })

	results, err := All(a, b, c).Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, v := range results {
		if v != i+1 {
			t.Fatalf("results[%d]: expected %d, got %d", i, i+1, v)
		}
	}
}

func TestAll_Failure(t *testing.T) {
	a := Go(func() (int, error) {
		time.Sleep(50 * time.Millisecond)
		return 1, nil
	})
	b := Go(func() (int, error) {
		return 0, errors.New("fail")
	})

	_, err := All(a, b).Get()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAll_Empty(t *testing.T) {
	results, err := All[int]().Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty slice, got %d", len(results))
	}
}

func TestRace_FirstWins(t *testing.T) {
	a := Go(func() (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 1, nil
	})
	b := Go(func() (int, error) {
		return 2, nil
	})

	val, err := Race(a, b).Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 2 {
		t.Fatalf("expected 2 (faster), got %d", val)
	}
}

func TestRace_FirstError(t *testing.T) {
	a := Go(func() (int, error) {
		return 0, errors.New("immediate error")
	})
	b := Go(func() (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 2, nil
	})

	_, err := Race(a, b).Get()
	if err == nil {
		t.Fatal("expected error from race")
	}
}

func TestAllSettled_Mixed(t *testing.T) {
	a := Go(func() (int, error) { return 1, nil })
	b := Go(func() (int, error) { return 0, errors.New("fail") })
	c := Go(func() (int, error) { return 3, nil })

	results, err := AllSettled(a, b, c).Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Error != nil || results[0].Value != 1 {
		t.Fatalf("results[0]: expected success with 1")
	}
	if results[1].Error == nil {
		t.Fatalf("results[1]: expected error")
	}
	if results[2].Error != nil || results[2].Value != 3 {
		t.Fatalf("results[2]: expected success with 3")
	}
}

func TestAny_FirstSuccess(t *testing.T) {
	a := Go(func() (int, error) {
		return 0, errors.New("fail")
	})
	b := Go(func() (int, error) {
		return 42, nil
	})

	val, err := Any(a, b).Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}

func TestAny_AllFail(t *testing.T) {
	a := Go(func() (int, error) { return 0, errors.New("fail1") })
	b := Go(func() (int, error) { return 0, errors.New("fail2") })

	_, err := Any(a, b).Get()
	if err == nil {
		t.Fatal("expected error when all fail")
	}
}

func TestCombine_WithAfter(t *testing.T) {
	// Combine All with After for complex dependency graphs
	users := Go(func() ([]string, error) {
		return []string{"alice", "bob"}, nil
	})
	orders := Go(func() (int, error) {
		return 10, nil
	})

	report := Go(func() (string, error) {
		us := users.Val()
		oc := orders.Val()
		return fmt.Sprintf("%d users, %d orders", len(us), oc), nil
	}, After(users, orders))

	val, err := report.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "2 users, 10 orders" {
		t.Fatalf("unexpected result: %s", val)
	}
}

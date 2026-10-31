package session

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForTimesOutAtTheConfiguredTimeout(t *testing.T) {
	// covers: R-SESS-033
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		err := waitFor(func() (bool, error) { return false, nil }, 60*time.Second, 250*time.Millisecond)
		if !errors.Is(err, ErrStalled) || time.Since(start) != 60*time.Second {
			t.Fatalf("err %v after %v", err, time.Since(start))
		}
		start = time.Now()
		err = waitFor(func() (bool, error) { return false, nil }, 120*time.Second, 250*time.Millisecond)
		if !errors.Is(err, ErrStalled) || time.Since(start) != 120*time.Second {
			t.Fatalf("120 s: err %v after %v", err, time.Since(start))
		}
	})
}

func TestWaitForReturnsOnTheMark(t *testing.T) {
	// covers: R-SESS-032
	synctest.Test(t, func(t *testing.T) {
		start, calls := time.Now(), 0
		err := waitFor(func() (bool, error) { calls++; return calls == 4, nil }, 60*time.Second, 250*time.Millisecond)
		if err != nil || time.Since(start) != 750*time.Millisecond {
			t.Fatalf("err %v after %v", err, time.Since(start))
		}
	})
}

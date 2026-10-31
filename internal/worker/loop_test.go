package worker_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

func TestLoopPassesEvery5sAndRestartsOnABinaryChange(t *testing.T) {
	// Passes at 0, 5, 10, … s of fake time; the binary is re-resolved every 60 s; a new version at
	// t=61 s makes the check at t=120 s return nil (exit 0 for systemd) with one restart line.
	// covers: R-WRK-024, R-WRK-009
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var stamps []time.Duration
		lg, path := newLog(t)
		l := &worker.Loop{
			Pass: func() error { stamps = append(stamps, time.Since(start)); return nil },
			Binary: func() (string, error) {
				if time.Since(start) >= 61*time.Second {
					return "0.2.0", nil
				}
				return "0.1.0", nil
			},
			Version: "0.1.0", Log: lg, Interval: worker.Interval, Check: worker.CheckEvery,
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute) // fake time: a loop that never returns fails fast
		defer cancel()
		if err := l.Run(ctx); err != nil {
			t.Fatal(err)
		}
		if since := time.Since(start); since != 120*time.Second {
			t.Fatalf("returned after %s, want 120 s", since)
		}
		// At t=120 s the tick and the check fire together; either order is right.
		if (len(stamps) != 24 && len(stamps) != 25) || stamps[1] != 5*time.Second || stamps[2] != 10*time.Second ||
			stamps[3] != 15*time.Second {
			t.Fatalf("passes at %v", stamps)
		}
		got := lines(t, path)
		if len(got) != 1 || !strings.Contains(got[0], " restart - binary is now 0.2.0") {
			t.Fatalf("log: %v", got)
		}
	})
}

func TestLoopKeepsRunningOnTheSameVersion(t *testing.T) {
	// covers: R-WRK-009
	synctest.Test(t, func(t *testing.T) {
		n := 0
		lg, path := newLog(t)
		l := &worker.Loop{
			Pass: func() error { n++; return nil }, Binary: func() (string, error) { return "0.1.0", nil },
			Version: "0.1.0", Log: lg, Interval: worker.Interval, Check: worker.CheckEvery,
		}
		ctx, cancel := context.WithTimeout(t.Context(), 202*time.Second) // off the 5 s grid: no tie at the end
		defer cancel()
		if err := l.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run = %v", err)
		}
		if n != 41 || len(lines(t, path)) != 0 {
			t.Fatalf("%d passes, log %v", n, lines(t, path))
		}
	})
}

func TestLoopSurvivesFailingPasses(t *testing.T) {
	// Review Focus 5: a failing pass is logged once and the cadence holds; a recovery clears it.
	// covers: R-WRK-022, R-WRK-024
	synctest.Test(t, func(t *testing.T) {
		n := 0
		lg, path := newLog(t)
		l := &worker.Loop{
			Pass: func() error {
				n++
				if n <= 10 {
					return errors.New("database is locked")
				}
				return nil
			},
			Binary: func() (string, error) { return "", errors.New("exec: not found") },
			Version: "0.1.0", Log: lg, Interval: worker.Interval, Check: worker.CheckEvery,
		}
		ctx, cancel := context.WithTimeout(t.Context(), 132*time.Second)
		defer cancel()
		if err := l.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Run = %v", err)
		}
		got := lines(t, path)
		if n != 27 || len(got) != 2 || !strings.Contains(got[0], " error pass database is locked") ||
			!strings.Contains(got[1], " error binary exec: not found") {
			t.Fatalf("%d passes, log %v", n, got)
		}
	})
}

func TestLoopLogsARecurringPassErrorAgainAfterARecovery(t *testing.T) {
	// covers: R-WRK-022
	synctest.Test(t, func(t *testing.T) {
		n := 0
		lg, path := newLog(t)
		l := &worker.Loop{
			Pass: func() error {
				n++
				if n == 2 || n == 4 {
					return errors.New("database is locked")
				}
				return nil
			},
			Binary:  func() (string, error) { return "0.1.0", nil },
			Version: "0.1.0", Log: lg, Interval: worker.Interval, Check: worker.CheckEvery,
		}
		ctx, cancel := context.WithTimeout(t.Context(), 32*time.Second)
		defer cancel()
		_ = l.Run(ctx)
		if got := lines(t, path); len(got) != 2 {
			t.Fatalf("log %v", got)
		}
	})
}

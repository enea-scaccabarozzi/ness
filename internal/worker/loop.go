package worker

import (
	"context"
	"time"
)

// CheckEvery is how often the loop re-resolves its binary (R-WRK-009).
const CheckEvery = time.Minute

// Loop runs Pass every Interval and asks Binary for the installed binary's version every Check.
type Loop struct {
	Pass     func() error
	Binary   func() (string, error)
	Version  string
	Log      *Log
	Interval time.Duration
	Check    time.Duration
}

// Run passes until ctx ends (its error) or the resolved binary's version differs from Version
// (nil: exit 0, and systemd's Restart=always brings the new binary up, R-WRK-009). A failing pass
// or check is logged once per text and never stops the loop (R-WRK-022). The first pass is immediate.
func (l *Loop) Run(ctx context.Context) error {
	tick := time.NewTicker(l.Interval)
	defer tick.Stop()
	check := time.NewTicker(l.Check)
	defer check.Stop()
	l.pass()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			l.pass()
		case <-check.C:
			v, err := l.Binary()
			if err != nil {
				l.Log.Error("binary", err)
				continue
			}
			l.Log.Clear("binary")
			if v != l.Version {
				l.Log.Action("restart", "-", "binary is now "+v+", running "+l.Version)
				return nil
			}
		}
	}
}

func (l *Loop) pass() {
	if err := l.Pass(); err != nil {
		l.Log.Error("pass", err)
		return
	}
	l.Log.Clear("pass")
}

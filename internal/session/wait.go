package session

import (
	"errors"
	"time"
)

// ErrStalled is returned when the ready mark does not arrive within the timeout (R-SESS-033).
var ErrStalled = errors.New("the session did not report ready")

// waitFor polls ready every tick until it is true or timeout passes. Timers only, so synctest
// drives it.
func waitFor(ready func() (bool, error), timeout, tick time.Duration) error {
	deadline := time.After(timeout)
	for {
		ok, err := ready()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-deadline:
			return ErrStalled
		case <-time.After(tick):
		}
	}
}

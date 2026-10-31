package update

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

// Failure is upgrade-failed.json: an unattended upgrade that failed after the idle point
// (R-UPD-081).
type Failure struct {
	Version string    `json:"version"`
	Cause   string    `json:"cause"`
	At      time.Time `json:"at"`
}

// ReadFailure reads the record at path; ok is false when none is stored.
func ReadFailure(path string) (Failure, bool, error) {
	var f Failure
	data, err := os.ReadFile(path) //nolint:gosec // our own record
	if errors.Is(err, os.ErrNotExist) {
		return f, false, nil
	}
	if err != nil {
		return f, false, err
	}
	err = json.Unmarshal(data, &f)
	return f, true, err
}

// RecordFailure stores f; changed reports a cause other than the stored one (or none stored), so
// the worker logs a failure once per cause across processes. A corrupt or unreadable record is
// overwritten, as none stored, so it cannot disable the back-off that reads it.
func RecordFailure(path string, f Failure) (changed bool, err error) {
	old, ok, rerr := ReadFailure(path)
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return false, err
	}
	return rerr != nil || !ok || old.Cause != f.Cause, writeAtomic(path, append(data, '\n'))
}

// ClearFailure removes the record; a missing one is not an error.
func ClearFailure(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

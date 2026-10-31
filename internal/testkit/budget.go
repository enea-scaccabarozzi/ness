package testkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// maxSnapshotAge is how old the usage snapshot may be: Claude Code rewrites it only when it renders
// its statusline, so an old file can show low usage while the real number is high.
const maxSnapshotAge = 30 * time.Minute

// UsageGuard is the D31 stop rule (R-QA-027, R-QA-042): the real-client runs abort when the
// client's usage snapshot shows weekly usage at or above limit percent. The snapshot is the
// statusline input Claude Code last wrote (~/.claude/usage-latest.json). It fails closed: a
// missing, unreadable, malformed, incomplete or stale snapshot (file older than 30 minutes, or
// seven_day.resets_at already past, so the number belongs to a window that has rolled) stops the run too.
func UsageGuard(path string, limit int) error {
	data, err := os.ReadFile(path) //nolint:gosec // the user's own usage snapshot
	if err != nil {
		return fmt.Errorf("no usage data at %s (run one Claude Code session first): %w", path, err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("usage data at %s: %w", path, err)
	}
	if age := time.Since(fi.ModTime()); age > maxSnapshotAge {
		return fmt.Errorf("usage data at %s is stale (%s old, max %s): run any Claude Code session to refresh it",
			path, age.Round(time.Minute), maxSnapshotAge)
	}
	var u struct {
		RateLimits struct {
			SevenDay *struct {
				UsedPercentage *float64 `json:"used_percentage"`
				ResetsAt       *float64 `json:"resets_at"` // unix seconds
			} `json:"seven_day"`
		} `json:"rate_limits"`
	}
	if err = json.Unmarshal(data, &u); err != nil {
		return fmt.Errorf("usage data at %s: %w", path, err)
	}
	if u.RateLimits.SevenDay == nil || u.RateLimits.SevenDay.UsedPercentage == nil {
		return fmt.Errorf("usage data at %s has no seven_day used_percentage", path)
	}
	if r := u.RateLimits.SevenDay.ResetsAt; r != nil && float64(time.Now().Unix()) >= *r {
		return fmt.Errorf("usage data at %s is stale: its seven_day window reset at %s", path,
			time.Unix(int64(*r), 0).Format(time.RFC3339))
	}
	if used := *u.RateLimits.SevenDay.UsedPercentage; used >= float64(limit) {
		return fmt.Errorf("weekly usage is at %g%% (stop rule: %d%%); the real-client runs wait for the reset",
			used, limit)
	}
	return nil
}

// RequireBudget gates a real-client test: it runs only with NESS_E2E=1 and under the usage
// guard, so a default `go test` and CI never spend budget.
func RequireBudget(t *testing.T) {
	t.Helper()
	if os.Getenv("NESS_E2E") != "1" {
		t.Skip("real-client run: set NESS_E2E=1")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := UsageGuard(filepath.Join(home, ".claude", "usage-latest.json"), 80); err != nil {
		t.Fatalf("budget: %v", err)
	}
}

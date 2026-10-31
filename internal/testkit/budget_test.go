package testkit_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func usageFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "usage-latest.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func sevenDay(pct string) string {
	return `{"rate_limits":{"five_hour":{"used_percentage":6},"seven_day":{"used_percentage":` + pct + `}}}`
}

func TestUsageGuardStopsAtTheLimit(t *testing.T) {
	// covers: R-QA-027, R-QA-042
	for _, tc := range []struct {
		name, body string
		wantErr    string // empty: the guard passes
	}{
		{"below", sevenDay("79"), ""},
		{"fractional below", sevenDay("79.9"), ""},
		{"zero", sevenDay("0"), ""},
		{"at the limit", sevenDay("80"), "80%"},
		{"fractional at the limit", sevenDay("80.0"), "80%"},
		{"fractional above", sevenDay("85.5"), "85.5%"},
		{"above", sevenDay("100"), "100%"},
		{"garbage", "not json at all", "usage data"},
		{"empty file", "", "usage data"},
		{"no rate_limits", `{"session_id":"x"}`, "no seven_day"},
		{"no seven_day", `{"rate_limits":{"five_hour":{"used_percentage":6}}}`, "no seven_day"},
		{"seven_day without a percentage", `{"rate_limits":{"seven_day":{}}}`, "no seven_day"},
		{"percentage is a string", sevenDay(`"85"`), "usage data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := testkit.UsageGuard(usageFile(t, tc.body), 80)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("guard stopped: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want an error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestUsageGuardMissingFile(t *testing.T) {
	// covers: R-QA-027, R-QA-042
	err := testkit.UsageGuard(filepath.Join(t.TempDir(), "missing.json"), 80)
	if err == nil || !strings.Contains(err.Error(), "no usage data") {
		t.Fatalf("a missing usage file must stop the run: %v", err)
	}
}

func TestRequireBudgetSkipsWithoutOptIn(t *testing.T) {
	// covers: R-QA-027
	t.Setenv("NESS_E2E", "")
	t.Run("inner", func(t *testing.T) {
		testkit.RequireBudget(t)
		t.Fatal("RequireBudget did not skip without NESS_E2E=1")
	})
}

func TestRequireBudgetStopsOverTheLimit(t *testing.T) {
	// covers: R-QA-042
	if os.Getenv("NESS_BUDGET_CHILD") == "1" {
		testkit.RequireBudget(t)
		return // reached only when the guard let the run through
	}
	run := func(pct string) (string, error) {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".claude", "usage-latest.json"), []byte(sevenDay(pct)), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRequireBudgetStopsOverTheLimit$", "-test.v")
		cmd.Env = append(os.Environ(), "NESS_BUDGET_CHILD=1", "NESS_E2E=1", "HOME="+home)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("86"); err == nil || !strings.Contains(out, "budget: weekly usage is at 86%") {
		t.Fatalf("86%% must fail the test fast: %v\n%s", err, out)
	}
	if out, err := run("10"); err != nil || strings.Contains(out, "SKIP") {
		t.Fatalf("10%% must pass through: %v\n%s", err, out)
	}
}

func TestUsageGuardRefusesAStaleSnapshot(t *testing.T) {
	// covers: R-QA-027, R-QA-042
	future := strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10)
	past := strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	withReset := func(reset string) string {
		return `{"rate_limits":{"seven_day":{"used_percentage":10,"resets_at":` + reset + `}}}`
	}
	aged := func(body string, age time.Duration) string {
		p := usageFile(t, body)
		old := time.Now().Add(-age)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := testkit.UsageGuard(aged(withReset(future), 5*time.Minute), 80); err != nil {
		t.Fatalf("a fresh snapshot with a future reset must pass: %v", err)
	}
	if err := testkit.UsageGuard(aged(withReset(future), 31*time.Minute), 80); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("a 31 minute old snapshot must stop the run: %v", err)
	}
	if err := testkit.UsageGuard(aged(withReset(past), time.Minute), 80); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("a snapshot whose seven_day window already reset must stop the run: %v", err)
	}
}

package stats_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestCostRoundsToCentsByExactModel(t *testing.T) {
	// covers: R-STAT-001
	u := stats.Usage{Input: 1000000, Output: 100000, CacheWrite: 0, CacheRead: 1000000}
	for model, want := range map[string]int{
		"claude-opus-5-5":          620,  // 4.00 + 2.00 + 0.20
		"claude-opus-5-5-20260401": 620,  // a -YYYYMMDD snapshot suffix is stripped
		"claude-sonnet-5-5":        310,  // 2.00 + 1.00 + 0.10
		"claude-fable-5-1":         1525, // 10.00 + 5.00 + 0.25
		"claude-haiku-4-5":         160,  // 1.00 + 0.50 + 0.10
	} {
		if cents, ok := stats.Cost(model, u); !ok || cents != want {
			t.Errorf("Cost(%s) = %d, %v; want %d", model, cents, ok, want)
		}
	}
	for _, m := range []string{"<synthetic>", "claude-opus-5-6", "claude-opus-5-5-x", "claude-opus-4-20250514", "claude-opus-5-5-2026"} {
		if cents, ok := stats.Cost(m, u); ok || cents != 0 {
			t.Errorf("%s is unpriced, got %d, %v", m, cents, ok)
		}
	}
	if cents, _ := stats.Cost("claude-opus-5-5", stats.Usage{Input: 1250}); cents != 1 {
		t.Errorf("0.005 USD rounds to 1 cent, got %d", cents)
	}
	// One million written tokens at Opus 5.5 (input 4.00): 5.00 at 5 minutes, 8.00 for 1 hour.
	// A usage without the split (CacheWrite1h 0) is all 5-minute.
	for _, c := range []struct{ w1h, want int }{{0, 500}, {1000000, 800}, {400000, 620}} {
		if cents, _ := stats.Cost("claude-opus-5-5", stats.Usage{CacheWrite: 1000000, CacheWrite1h: c.w1h}); cents != c.want {
			t.Errorf("1h share %d: %d cents, want %d", c.w1h, cents, c.want)
		}
	}
	if stats.PriceDate != "2026-09-25" {
		t.Errorf("PriceDate = %q", stats.PriceDate)
	}
}

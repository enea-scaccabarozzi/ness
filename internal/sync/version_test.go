package sync_test

import (
	"testing"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

func TestCompareNumericPrerelease(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"0.2.0-rc.10", "0.2.0-rc.9", 1}, {"0.2.0-rc.1", "0.2.0", -1}, {"0.2.0+abc", "0.2.0", 0},
		{"v0.10.0", "0.9.9", 1},
	} {
		if got, err := nsync.Compare(c.a, c.b); err != nil || got != c.want {
			t.Errorf("Compare(%s,%s) = %d, %v", c.a, c.b, got, err)
		}
	}
	if _, err := nsync.Compare("main", "0.1.0"); err == nil {
		t.Error("non-semver: want error")
	}
}

package update_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestCeiling(t *testing.T) {
	// covers: R-UPD-046
	pins := []update.Pin{{Path: "/a/.ness/ness.toml", Pin: "0.2", Primary: "/a"}, {Path: "/a/.worktrees/x/.ness/ness.toml", Pin: "0.1", Primary: "/a"}}
	// rejects: requiring an exact pin match (0.2.5 runs a 0.2 pin)
	if err := update.Ceiling("0.2.5", pins); err != nil {
		t.Fatal(err)
	}
	// rejects: checking only the base pin, or a message without the path, pin and fix
	err := update.Ceiling("0.3.0", pins)
	if err == nil || !strings.Contains(err.Error(), "/a/.worktrees/x/.ness/ness.toml") || !strings.Contains(err.Error(), `ness = "0.1"): run ness upgrade in /a; open sessions then merge their base`) {
		t.Fatal(err)
	}
	// rejects: naming every pin rather than the ones that cannot run
	if strings.Contains(err.Error(), "/a/.ness/ness.toml (") {
		t.Errorf("names the passing pin too: %v", err)
	}
}

func TestCeilingDowngrade(t *testing.T) {
	// covers: R-UPD-046 — a target older than a pin: revert, or a release the pin runs
	pins := []update.Pin{{Path: "/a/.ness/ness.toml", Pin: "0.3", Primary: "/a"}}
	err := update.Ceiling("0.2.0", pins)
	// rejects: the upgrade fix for a downgrade (ness upgrade cannot lower a pin)
	want := `ness 0.2.0 cannot run /a/.ness/ness.toml (ness = "0.3"): revert the upgrade commit in /a first, ` +
		`or install a release it runs: ness self-update --version 0.3.0`
	if err == nil || err.Error() != want {
		t.Fatalf("got %v\nwant %s", err, want)
	}
}

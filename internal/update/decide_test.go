package update_test

import (
	"errors"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestDecide(t *testing.T) {
	// covers: R-UPD-080, R-UPD-081, R-UPD-082 — table
	all := map[string]string{"/a": "install", "/b": "install"}
	mixed := map[string]string{"/a": "install", "/b": "notify"}
	for _, c := range []struct {
		name string
		in   update.Inputs
		want update.Plan
	}{
		// rejects: no install at all
		{"install", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: all}, update.Plan{Install: "0.2.0"}},
		// rejects: deciding from the current project's auto alone
		{"veto", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: mixed}, update.Plan{Note: `auto-install held: /b has auto = "notify"`}},
		// rejects: veto note naming a map-order project; two vetoes, the first sorted root is named
		{"veto order", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: map[string]string{"/z": "off", "/c": "notify", "/a": "install"}}, update.Plan{Note: `auto-install held: /c has auto = "notify"`}},
		// rejects: writing a binary another tool manages
		{"managed", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: all, Managed: true}, update.Plan{Note: "auto-install held: the binary is managed by another tool"}},
		// rejects: ignoring the ceiling
		{"ceiling", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.3.0", Auto: "install", Autos: all, Ceiling: errors.New("x")}, update.Plan{Note: "auto-install held: x"}},
		// rejects: install not evaluated before upgrade when both apply
		{"install wins", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Offer: "0.3.0", Auto: "install", Autos: all, Idle: true}, update.Plan{Install: "0.3.0"}},
		{"upgrade idle", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "install", Autos: all, Idle: true}, update.Plan{Upgrade: true}},
		// rejects: upgrading, or flagging Behind, before the idle point
		{"upgrade busy", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "install", Autos: all}, update.Plan{}},
		// rejects: the project's own upgrade gated on the machine-wide veto (R-UPD-081 is per project)
		{"upgrade idle mixed", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "install", Autos: mixed, Idle: true}, update.Plan{Upgrade: true}},
		{"behind notify", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "notify", Autos: mixed}, update.Plan{Behind: true}},
		// rejects: auto = "off" silencing the behind fyi (R-UPD-082: the same one fyi)
		{"behind off", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "off", Autos: mixed}, update.Plan{Behind: true}},
		// rejects: a notify project installing
		{"notify offer", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "notify", Autos: mixed}, update.Plan{}},
		// rejects: Behind when the lock equals the binary
		{"current", update.Inputs{Installed: "0.2.0", Lock: "0.2.0", Auto: "notify"}, update.Plan{}},
	} {
		if got := update.Decide(c.in); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

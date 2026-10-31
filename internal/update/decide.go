package update

import (
	"fmt"
	"maps"
	"slices"
)

// Inputs is what the worker knows on one pass: the running binary's version, the base lock's
// version, the version the cache offers (or ""), this project's update.auto, every registered
// project's auto by root, whether another tool manages the binary, the ceiling's verdict on
// Offer, and whether the base checkout is at the idle point.
type Inputs struct {
	Installed, Lock, Offer, Auto string
	Autos                        map[string]string
	Managed                      bool
	Ceiling                      error
	Idle                         bool
}

// Plan is what the pass does: install that version, upgrade this project, tell the open roots
// the project is behind the binary, or log Note.
type Plan struct {
	Install         string
	Upgrade, Behind bool
	Note            string
}

// Decide plans one unattended pass (R-UPD-080..082). The machine-wide install needs every
// registered project at auto = "install" (the first other one, in sorted root order, is named),
// an unmanaged binary and a passing ceiling; it wins over this pass's upgrade, which follows on
// the restarted worker. A project's own upgrade depends only on its own auto and the idle point;
// a project that does not upgrade itself is flagged Behind.
func Decide(in Inputs) Plan {
	var p Plan
	if in.Auto == "install" && in.Offer != "" {
		p.Note = installHeld(in)
		if p.Note == "" {
			p.Install = in.Offer
			return p
		}
	}
	if !newer(in.Installed, in.Lock) {
		return p
	}
	if in.Auto == "install" {
		p.Upgrade = in.Idle
	} else {
		p.Behind = true
	}
	return p
}

// installHeld is why the machine-wide install may not run, or "".
func installHeld(in Inputs) string {
	for _, root := range slices.Sorted(maps.Keys(in.Autos)) {
		if a := in.Autos[root]; a != "install" {
			return fmt.Sprintf("auto-install held: %s has auto = %q", root, a)
		}
	}
	switch {
	case in.Managed:
		return "auto-install held: the binary is managed by another tool"
	case in.Ceiling != nil:
		return "auto-install held: " + in.Ceiling.Error()
	}
	return ""
}

package update

import (
	"errors"
	"fmt"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// Pin is one ness.toml's version pin: Path is the file, Primary the registered checkout whose
// ness upgrade fixes it.
type Pin struct{ Path, Pin, Primary string }

// Ceiling refuses version when any pin cannot run it (R-UPD-046), naming each such ness.toml,
// its pin and the fix: an older pin is upgraded; a newer one (a downgrade) is reverted first, or
// the target raised to a release the pin runs.
func Ceiling(version string, pins []Pin) error {
	var errs []error
	for _, p := range pins {
		err := config.CheckPin(p.Pin, version)
		if err == nil {
			continue
		}
		fix := fmt.Sprintf("run ness upgrade in %s; open sessions then merge their base", p.Primary)
		if pe := (*config.PinError)(nil); errors.As(err, &pe) && pe.Older {
			fix = fmt.Sprintf("revert the upgrade commit in %s first, or install a release it runs: "+
				"ness self-update --version %s.0", p.Primary, p.Pin)
		}
		errs = append(errs, fmt.Errorf("ness %s cannot run %s (ness = %q): %s", version, p.Path, p.Pin, fix))
	}
	return errors.Join(errs...)
}

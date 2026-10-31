package config

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// PinOf returns the "<major>.<minor>" pin ness init writes for a binary version (R-CFG-010).
func PinOf(version string) (string, error) {
	maj, minor, err := majorMinor(version)
	if err != nil {
		return "", fmt.Errorf("binary version %q is not a release version, so ness init cannot pin it", version)
	}
	return fmt.Sprintf("%d.%d", maj, minor), nil
}

// CheckPin refuses a binary outside the project's pin with a *PinError: for 0.N the binary may
// be 0.N.x or 0.(N+1).x; for M.N it must have major M and a minor of at least N (R-CFG-011,
// R-CFG-012).
func CheckPin(pin, version string) error {
	pm, pn, err := majorMinor(pin)
	if err != nil || strings.Count(pin, ".") != 1 {
		return fmt.Errorf(`version pin ness = %q in %s: want "<major>.<minor>"`, pin, Path)
	}
	vm, vn, err := majorMinor(version)
	if err != nil {
		return fmt.Errorf("binary version %q is not a release version", version)
	}
	if vm == pm && pn <= vn && vn <= maxMinor(pm, pn) {
		return nil
	}
	return &PinError{Pin: pin, Version: version, Older: vm < pm || (vm == pm && vn < pn)}
}

// maxMinor is the newest minor a pin admits: one past it for 0.x, any for M >= 1.
func maxMinor(major, minor int) int {
	if major == 0 {
		return minor + 1
	}
	return math.MaxInt
}

// PinError is a binary outside the project's pin. Older means the binary is older than the pin.
type PinError struct {
	Pin, Version string
	Older        bool
}

// Error names the pinned range, the pin and the running binary's version (R-CFG-013).
func (e *PinError) Error() string {
	pm, pn, _ := majorMinor(e.Pin) //nolint:errcheck // CheckPin parsed the pin before building e
	want := fmt.Sprintf("%d.%d.x or %d.%d.x", pm, pn, pm, pn+1)
	if pm >= 1 {
		want = fmt.Sprintf("%d.%d or a later %d.x", pm, pn, pm)
	}
	return fmt.Sprintf("this project pins ness %s (ness = %q); the running binary is %s", want, e.Pin, e.Version)
}

// Fix is the one command that clears the refusal (R-CFG-013): an older binary updates itself,
// an older pin is upgraded in the base checkout. loadProject and the hooks both print it.
func (e *PinError) Fix() string {
	if e.Older {
		return "ness self-update"
	}
	return "run ness upgrade in the base checkout; open sessions then merge their base"
}

var (
	pinLineRe = regexp.MustCompile(`(?m)^(ness[ \t]*=[ \t]*")[^"]*"`)
	tableRe   = regexp.MustCompile(`(?m)^[ \t]*\[`)
)

// SetPin rewrites the value of the top-level ness = "…" line and leaves every other byte
// unchanged. A file whose first such line is missing or sits under a [table] header is an error.
func SetPin(data []byte, pin string) ([]byte, error) {
	top := data
	if i := tableRe.FindIndex(data); i != nil {
		top = data[:i[0]]
	}
	loc := pinLineRe.FindSubmatchIndex(top)
	if loc == nil {
		return nil, fmt.Errorf("%s has no top-level ness = \"…\" line to set", Path)
	}
	out := append([]byte{}, data[:loc[3]]...)
	out = append(out, pin...)
	return append(out, data[loc[1]-1:]...), nil
}

func majorMinor(v string) (major, minor int, err error) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, fmt.Errorf("version %q", v)
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, err
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

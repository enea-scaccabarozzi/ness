package config

import (
	"fmt"
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

// CheckPin refuses a binary outside the project's pin: for 0.N exactly that minor; for M.N the
// same major and a minor of at least N (R-CFG-011, R-CFG-012).
func CheckPin(pin, version string) error {
	pm, pn, err := majorMinor(pin)
	if err != nil || strings.Count(pin, ".") != 1 {
		return fmt.Errorf(`version pin ness = %q in %s: want "<major>.<minor>"`, pin, Path)
	}
	vm, vn, err := majorMinor(version)
	if err != nil {
		return fmt.Errorf("binary version %q is not a release version", version)
	}
	if vm == pm && (vn == pn || (pm >= 1 && vn > pn)) {
		return nil
	}
	want := fmt.Sprintf("%d.%d.x", pm, pn)
	if pm >= 1 {
		want = fmt.Sprintf("%d.%d or a later %d.x", pm, pn, pm)
	}
	return fmt.Errorf("this project pins ness %s (ness = %q); the running binary is %s", want, pin, version)
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

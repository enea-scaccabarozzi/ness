package config_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

func TestCheckPin(t *testing.T) {
	// covers: R-CFG-011, R-CFG-012, R-CFG-013
	cases := []struct {
		pin, version string
		ok           bool
	}{
		{"0.3", "0.3.0", true},
		{"0.3", "0.3.7", true},
		{"0.3", "0.2.9", false},
		{"0.3", "0.4.0", false},
		{"1.2", "1.2.0", true},
		{"1.2", "1.5.1", true},
		{"1.2", "1.1.9", false},
		{"1.2", "2.0.0", false},
		{"0.1", "0.1.0-dev", true},
		{"x", "0.1.0", false},
		{"0.1", "garbage", false},
	}
	for _, c := range cases {
		err := config.CheckPin(c.pin, c.version)
		if (err == nil) != c.ok {
			t.Errorf("CheckPin(%q, %q) = %v, want ok=%v", c.pin, c.version, err, c.ok)
		}
		if err != nil && c.pin != "x" && c.version != "garbage" &&
			(!strings.Contains(err.Error(), c.pin) || !strings.Contains(err.Error(), c.version)) {
			t.Errorf("refusal %q must name the pin and the binary version", err)
		}
	}
}

func TestPinOf(t *testing.T) {
	// covers: R-CFG-010
	if got, err := config.PinOf("0.3.7-rc1"); err != nil || got != "0.3" {
		t.Fatalf("PinOf = %q, %v", got, err)
	}
	for _, v := range []string{"dev", "1", "1.x.0", ""} {
		if got, err := config.PinOf(v); err == nil {
			t.Errorf("PinOf(%q) = %q, want an error", v, got)
		}
	}
}

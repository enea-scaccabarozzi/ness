package config_test

import (
	"errors"
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
		{"0.3", "0.4.0", true},
		{"0.3", "0.5.0", false},
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

func TestCheckPinWidened(t *testing.T) {
	// covers: R-CFG-011
	for _, v := range []string{"0.3.0", "0.3.7", "0.4.0", "0.4.2-rc.1"} {
		if err := config.CheckPin("0.3", v); err != nil {
			t.Errorf("pin 0.3, binary %s: %v", v, err)
		}
	}
	for v, older := range map[string]bool{"0.2.9": true, "0.5.0": false, "1.0.0": false} {
		var pe *config.PinError
		if err := config.CheckPin("0.3", v); !errors.As(err, &pe) {
			t.Fatalf("pin 0.3, binary %s: %v, want *PinError", v, err)
		}
		if pe.Older != older {
			t.Errorf("%s: Older = %v", v, pe.Older)
		}
	}
}

func TestPinErrorTextAndFix(t *testing.T) {
	// covers: R-CFG-013
	var pe *config.PinError
	if !errors.As(config.CheckPin("0.3", "0.2.9"), &pe) {
		t.Fatal("want *PinError")
	}
	if got := pe.Error(); got != `this project pins ness 0.3.x or 0.4.x (ness = "0.3"); the running binary is 0.2.9` {
		t.Errorf("Error = %q", got)
	}
	if pe.Fix() != "ness self-update" {
		t.Errorf("Fix = %q", pe.Fix())
	}
	if !errors.As(config.CheckPin("0.1", "0.3.0"), &pe) {
		t.Fatal("want *PinError")
	}
	if pe.Fix() != "run ness upgrade in the base checkout; open sessions then merge their base" {
		t.Errorf("Fix = %q", pe.Fix())
	}
}

func TestParseErrorWraps(t *testing.T) {
	_, err := config.Parse([]byte("ness = \"0.1\"\nbogus = 1\n"))
	var pe *config.ParseError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("Parse = %v", err)
	}
}

func TestSetPinKeepsTheRest(t *testing.T) {
	in := []byte("# top\nness = \"0.1\"   # pinned\nproject = \"x\"\n[update]\nauto = \"install\"\n")
	got, err := config.SetPin(in, "0.2")
	want := "# top\nness = \"0.2\"   # pinned\nproject = \"x\"\n[update]\nauto = \"install\"\n"
	if err != nil || string(got) != want {
		t.Fatalf("SetPin = %q, %v", got, err)
	}
	if _, err := config.SetPin([]byte("project = \"x\"\n"), "0.2"); err == nil {
		t.Fatal("no pin line: want error")
	}
	if _, err := config.SetPin([]byte("[x]\nness = \"0.1\"\n"), "0.2"); err == nil {
		t.Fatal("table key: want error")
	}
}

package config_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

func TestUpdateDefaults(t *testing.T) {
	// covers: R-UPD-030
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"true\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Update != (config.Update{Check: true, Channel: "stable", Auto: "notify"}) {
		t.Fatalf("defaults = %+v", c.Update)
	}
}

func TestReadHeadIgnoresUnknownKeys(t *testing.T) {
	// covers: R-UPD-046 — another binary's ness.toml is still readable for the ceiling and the veto
	h, err := config.ReadHead([]byte("ness = \"0.4\"\nproject = \"p\"\nfuture = 1\n[update]\nauto = \"install\"\n[newtable]\nx = 1\n"))
	if err != nil || h.Ness != "0.4" || h.Project != "p" || h.Update.Auto != "install" || !h.Update.Check || h.Update.Channel != "stable" {
		t.Fatalf("%+v, %v", h, err)
	}
}

package sync

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestTOMLKeysSetGetDelete(t *testing.T) {
	// Line-level edits: a top-level key, a key in an existing table, a new table appended; the
	// user's other lines and comments are untouched; the result parses.
	// covers: R-SYNC-068, R-SYNC-086, R-MOD-050
	in := []byte("# mine\nmodel = \"gpt-6-luna\"\n\n[mcp_servers.x]\ncommand = \"npx\"\n")
	out, err := tomlSet(in, "model_auto_compact_token_limit", []byte("235000"))
	if err != nil {
		t.Fatal(err)
	}
	out, err = tomlSet(out, "agents.default_subagent_model", []byte(`"gpt-6-luna"`))
	if err != nil {
		t.Fatal(err)
	}
	out, err = tomlSet(out, "agents.default_subagent_reasoning_effort", []byte(`"low"`))
	if err != nil {
		t.Fatal(err)
	}
	want := "# mine\nmodel = \"gpt-6-luna\"\nmodel_auto_compact_token_limit = 235000\n\n[mcp_servers.x]\n" +
		"command = \"npx\"\n\n[agents]\ndefault_subagent_model = \"gpt-6-luna\"\n" +
		"default_subagent_reasoning_effort = \"low\"\n"
	if string(out) != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if v, ok, err := tomlGet(out, "agents.default_subagent_model"); err != nil || !ok || string(v) != `"gpt-6-luna"` {
		t.Fatalf("tomlGet = %q, %v, %v", v, ok, err)
	}
	if _, ok, _ := tomlGet(out, "agents.nope"); ok {
		t.Fatal("missing key found")
	}
	out, err = tomlSet(out, "agents.default_subagent_model", []byte(`"astra"`))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if _, err := toml.Decode(string(out), &m); err != nil {
		t.Fatalf("result does not parse: %v\n%s", err, out)
	}
	if m["agents"].(map[string]any)["default_subagent_model"] != "astra" {
		t.Fatalf("set did not replace: %s", out)
	}
	out, err = tomlDelete(out, "agents.default_subagent_model")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := tomlGet(out, "agents.default_subagent_model"); ok {
		t.Fatalf("delete left the key: %s", out)
	}
	if !strings.Contains(string(out), "# mine\n") || !strings.Contains(string(out), "[mcp_servers.x]") {
		t.Fatalf("delete touched the user's lines: %s", out)
	}
	if _, _, err := tomlGet([]byte("a = = 1\n"), "a"); err == nil {
		t.Fatal("invalid TOML accepted")
	}
}

func TestTOMLSetRespectsArraysOfTables(t *testing.T) {
	// A root key and an [agents] key land before the first [[...]] that follows them, never in
	// an array-of-tables element; comments survive an update; CRLF files keep CRLF.
	// covers: R-SYNC-068, R-SYNC-086, R-MOD-050
	for _, c := range []struct{ name, in, key string }{
		{"root before array", "[[skills.config]]\npath = \"x\"\n", "model_auto_compact_token_limit"},
		{"agents before array", "[agents]\nmax_depth = 2\n\n[[skills.config]]\npath = \"x\"\n", "agents.default_subagent_model"},
	} {
		out, err := tomlSet([]byte(c.in), c.key, []byte("1"))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		table, name := splitKey(c.key)
		var m map[string]any
		if _, err := toml.Decode(string(out), &m); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		tbl := m
		if table != "" {
			tbl = m[table].(map[string]any)
		}
		if tbl[name] != int64(1) {
			t.Fatalf("%s: key not in %q:\n%s", c.name, table, out)
		}
		if elems := m["skills"].(map[string]any)["config"].([]map[string]any); len(elems) != 1 || elems[0]["path"] != "x" || len(elems[0]) != 1 {
			t.Fatalf("%s: array element changed:\n%s", c.name, out)
		}
	}
	out, err := tomlSet([]byte("[agents]\r\nx = 1 # mine\r\n"), "agents.x", []byte("2"))
	if err != nil || string(out) != "[agents]\r\nx = 2 # mine\r\n" {
		t.Fatalf("comment/CRLF update = %q, %v", out, err)
	}
	out, err = tomlSet([]byte("a = 1\r\n"), "agents.y", []byte("3"))
	if err != nil || string(out) != "a = 1\r\n\r\n[agents]\r\ny = 3\r\n" {
		t.Fatalf("CRLF append = %q, %v", out, err)
	}
	if _, err := tomlDelete([]byte("a = 1\n"), "a"); err != nil {
		t.Fatal(err)
	}
}

var (
	fuzzKey = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)?$`)
	fuzzVal = regexp.MustCompile(`^[^\n\r#]*$`)
)

func FuzzTOMLKeys(f *testing.F) {
	// covers: R-QA-022
	f.Add([]byte("a = 1\n[t]\nb = 2\n"), "t.b", "3")
	f.Add([]byte(""), "agents.default_subagent_model", `"x"`)
	f.Add([]byte("[agents]\ndefault_subagent_model = \"y\"\n"), "agents.default_subagent_model", `"z"`)
	f.Fuzz(func(t *testing.T, data []byte, key, val string) {
		// ness writes plain keys and one-line literals (rawTOML); anything else is out of domain.
		var probe map[string]any
		if !fuzzKey.MatchString(key) || !fuzzVal.MatchString(val) {
			t.Skip()
		}
		if _, err := toml.Decode("v = "+val, &probe); err != nil {
			t.Skip()
		}
		out, err := tomlSet(data, key, []byte(val))
		if err != nil {
			return
		}
		if v, ok, gerr := tomlGet(out, key); gerr != nil || !ok || strings.TrimSpace(string(v)) != strings.TrimSpace(val) {
			t.Fatalf("after set, get %q = %q, %v, %v", key, v, ok, gerr)
		}
		// The key sits in the intended table with the intended value.
		var got, want map[string]any
		table, name := splitKey(key)
		if _, err := toml.Decode(string(out), &got); err != nil {
			t.Fatalf("output is not TOML: %v", err)
		}
		_, _ = toml.Decode("v = "+val, &want)
		tbl := got
		if table != "" {
			var ok bool
			if tbl, ok = got[table].(map[string]any); !ok {
				t.Fatalf("table %q missing in\n%s", table, out)
			}
		}
		if !reflect.DeepEqual(tbl[name], want["v"]) {
			t.Fatalf("%s = %#v, want %#v in\n%s", key, tbl[name], want["v"], out)
		}
		if _, err := tomlDelete(out, key); err != nil {
			t.Fatalf("delete after set: %v", err)
		}
	})
}

func TestTOMLArrayOfTablesIsNotATable(t *testing.T) {
	// A [[x]] element is not table x: get finds nothing; delete of a multi-line value is refused
	// rather than leaving a broken file (the re-parse).
	// covers: R-SYNC-068
	in := []byte("[[x]]\nk = 1\n")
	if _, found, err := tomlGet(in, "x.k"); err != nil || found {
		t.Fatalf("get x.k under [[x]] = %v, %v", found, err)
	}
	if _, err := tomlDelete([]byte("k = [\n  1,\n]\n"), "k"); err == nil {
		t.Fatal("deleting the first line of a multi-line value was not refused")
	}
}

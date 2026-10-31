package sync

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

const nessHook = `{"hooks":[{"type":"command","command":"command -v ness >/dev/null || exit 0; ness hook session-start"}]}`

func TestJSONKeepsUnownedBytes(t *testing.T) {
	// covers: R-SYNC-050, R-SYNC-053
	user := `{"hooks": [{"type": "command", "command": "echo mine"}]}`
	in := []byte("{\n    \"model\":   \"x\",\n  \"hooks\": {\"SessionStart\": [" + user + "]}\n}\n")
	key := hookKeyFor("SessionStart", "ness hook session-start")
	out := in
	for range 2 {
		var err error
		if out, err = jsonSet(out, "autoCompactWindow", []byte("265000")); err != nil {
			t.Fatal(err)
		}
		if out, err = jsonSet(out, key, []byte(nessHook)); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.HasPrefix(out, []byte("{\n    \"model\":   \"x\",\n  \"hooks\": {\"SessionStart\": ["+user)) {
		t.Fatalf("unowned bytes changed:\n%s", out)
	}
	if n := gjson.GetBytes(out, "hooks.SessionStart.#").Int(); n != 2 {
		t.Fatalf("%d SessionStart entries, want the user's and one ness entry:\n%s", n, out)
	}
	got, ok, err := jsonGet(out, key)
	if err != nil || !ok || string(got) != nessHook {
		t.Fatalf("jsonGet hook = %s %v %v", got, ok, err)
	}
}

func TestJSONDeleteHookKeepsUsers(t *testing.T) {
	in := []byte(`{"hooks":{"SessionStart":[{"hooks":[{"command":"echo mine"}]},` + nessHook + `]}}`)
	out, err := jsonDelete(in, hookKeyFor("SessionStart", "ness hook session-start"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"hooks":{"SessionStart":[{"hooks":[{"command":"echo mine"}]}]}}` {
		t.Fatalf("got %s", out)
	}
}

func TestJSONInvalid(t *testing.T) {
	// covers: R-SYNC-055
	for _, in := range []string{"{nope", "[1]", `"s"`} {
		if _, _, err := jsonGet([]byte(in), "a"); err == nil {
			t.Errorf("jsonGet(%q) accepted", in)
		}
		if _, err := jsonSet([]byte(in), "a", []byte("1")); err == nil {
			t.Errorf("jsonSet(%q) accepted", in)
		}
	}
}

func TestJSONRefusesNonObjectAncestor(t *testing.T) {
	// covers: R-SYNC-050, R-SYNC-055
	hook := hookKeyFor("SessionStart", "ness hook session-start")
	for _, c := range []struct{ in, key, name string }{
		{`{"env": "x"}`, "env.CLAUDE_CODE_SUBAGENT_MODEL", "env"},
		{`{"hooks": 3}`, hook, "hooks"},
		{`{"hooks":{"SessionStart":{"a":1}}}`, hook, "hooks.SessionStart"},
	} {
		in := []byte(c.in)
		out, err := jsonSet(in, c.key, []byte(nessHook))
		if err == nil || !strings.Contains(err.Error(), c.name) {
			t.Errorf("jsonSet(%s, %s) = %s, %v; want an error naming %s", c.in, c.key, out, err, c.name)
		}
		if string(in) != c.in || (out != nil && string(out) != c.in) {
			t.Errorf("jsonSet(%s) changed the input: in %s out %s", c.in, in, out)
		}
	}
	if i := hookIndex([]byte(`{"hooks":{"SessionStart":{"a":{"hooks":[{"command":"ness hook session-start"}]}}}}`),
		"SessionStart", "ness hook session-start"); i != -1 {
		t.Errorf("hookIndex on an object = %d, want -1", i)
	}
}

func TestJSONCreatedFileIsIndented(t *testing.T) {
	out, err := jsonSet(nil, "env.CLAUDE_CODE_SUBAGENT_MODEL", []byte(`"sonnet"`))
	if err != nil {
		t.Fatal(err)
	}
	if string(indentJSON(out)) != "{\n  \"env\": {\n    \"CLAUDE_CODE_SUBAGENT_MODEL\": \"sonnet\"\n  }\n}\n" {
		t.Fatalf("got %q", indentJSON(out))
	}
}

func FuzzJSONKeys(f *testing.F) {
	// covers: R-SYNC-055, R-SYNC-050
	f.Add([]byte(`{"a": 1, "hooks": {"SessionStart": []}}`))
	f.Add([]byte(`{"hooks": 3}`))
	f.Add([]byte(`{`))
	f.Add([]byte(`{"env": "x", "b": [1]}`))
	f.Add([]byte(`{"hooks":{"SessionStart":{"a":1}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		keys := []string{"autoCompactWindow", "env.CLAUDE_CODE_SUBAGENT_MODEL", hookKeyFor("PreToolUse", "ness hook pre-tool")}
		for _, key := range keys {
			orig := bytes.Clone(data)
			out, err := jsonSet(data, key, []byte(`[1,2]`))
			if !bytes.Equal(data, orig) {
				t.Fatalf("jsonSet mutated its input %q", orig)
			}
			if err != nil {
				if out != nil && !bytes.Equal(out, orig) {
					t.Fatalf("jsonSet failed but returned changed bytes %q from %q", out, orig)
				}
				continue
			}
			first, _, _ := strings.Cut(key, ".")
			before, after := topLevel(data), topLevel(out)
			if v, ok := before[first]; ok && key != first && !gjson.Parse(v).IsObject() {
				t.Fatalf("setting %s overwrote the non-object %s = %s", key, first, v)
			}
			for k, v := range before {
				if k != first && after[k] != v {
					t.Fatalf("setting %s changed top-level key %q: %s -> %s", key, k, v, after[k])
				}
			}
			got, ok, err := jsonGet(out, key)
			if key == "autoCompactWindow" && (err != nil || !ok || string(got) != "[1,2]") {
				t.Fatalf("round trip %q: %s %v %v", data, got, ok, err)
			}
		}
	})
}

// topLevel maps each top-level key to the raw bytes of its first occurrence.
func topLevel(data []byte) map[string]string {
	m := map[string]string{}
	gjson.ParseBytes(data).ForEach(func(k, v gjson.Result) bool {
		if _, ok := m[k.String()]; !ok {
			m[k.String()] = v.Raw
		}
		return true
	})
	return m
}

package testkit_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// conforms checks v against the herdr 0.8.2 schema node: required fields, enums and nested refs.
func conforms(t *testing.T, root map[string]any, node map[string]any, v any, path string) {
	t.Helper()
	if ref, ok := node["$ref"].(string); ok {
		target := any(root)
		for _, k := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			target = target.(map[string]any)[k]
		}
		conforms(t, root, target.(map[string]any), v, path)
		return
	}
	if alts, ok := node["anyOf"].([]any); ok {
		if v == nil {
			return
		}
		for _, a := range alts {
			if m := a.(map[string]any); m["type"] != "null" {
				conforms(t, root, m, v, path)
			}
		}
		return
	}
	if enum, ok := node["enum"].([]any); ok {
		for _, e := range enum {
			if e == v {
				return
			}
		}
		t.Errorf("%s = %v, not in %v", path, v, enum)
		return
	}
	switch val := v.(type) {
	case []any:
		if items, ok := node["items"].(map[string]any); ok {
			for i, e := range val {
				conforms(t, root, items, e, path+"[]"+string(rune('0'+i%10)))
			}
		}
	case map[string]any:
		props, _ := node["properties"].(map[string]any)
		req, _ := node["required"].([]any)
		for _, r := range req {
			if _, ok := val[r.(string)]; !ok {
				t.Errorf("%s misses required %q", path, r)
			}
		}
		for k, e := range val {
			if p, ok := props[k].(map[string]any); ok {
				conforms(t, root, p, e, path+"."+k)
			}
		}
	}
}

func TestFakeHerdrResponsesMatchSchema(t *testing.T) {
	// covers: R-SESS-016
	data, err := os.ReadFile("testdata/herdr-schema-0.8.2.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	success := doc["schemas"].(map[string]any)["success_response"].(map[string]any)
	variants := map[string]map[string]any{}
	for _, v := range success["$defs"].(map[string]any)["ResponseResult"].(map[string]any)["oneOf"].([]any) {
		m := v.(map[string]any)
		variants[m["properties"].(map[string]any)["type"].(map[string]any)["const"].(string)] = m
	}

	dir := t.TempDir()
	if err = testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	f := testkit.FakeExec{Dir: dir}
	calls := [][]string{
		{"tab", "create", "--cwd", "/tmp/x", "--label", "root"},
		{"tab", "list"},
		{"pane", "list"},
		{"agent", "start", "p-root", "--kind", "claude", "--pane", "wF:p1", "--", "hi"},
		{"agent", "list"},
		{"agent", "get", "p-root"},
		{"pane", "read", "wF:p1"},
		{"pane", "send-text", "wF:p1", "x"},
		{"pane", "run", "wF:p1", "x"},
		{"tab", "close", "wF:t1"},
	}
	// The CLI differs from the socket API schema here: `pane read` prints raw terminal text, and
	// `pane send-text`, `pane run` and `agent start` print nothing, so there is no envelope to check.
	notJSON := map[string]bool{"pane read": true, "pane send-text": true, "pane run": true, "agent start": true}
	for _, c := range calls {
		out, err := f.Run("", "herdr", c...)
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		if notJSON[strings.Join(c[:2], " ")] {
			if c[1] != "read" && len(out) != 0 {
				t.Errorf("%v printed %q, want nothing", c, out)
			}
			if c[1] == "read" && json.Valid(out) && len(out) > 0 {
				t.Errorf("pane read printed JSON %q, want raw text", out)
			}
			continue
		}
		var env struct {
			Result map[string]any `json:"result"`
		}
		if err = json.Unmarshal(out, &env); err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		typ, _ := env.Result["type"].(string)
		variant, ok := variants[typ]
		if !ok {
			t.Errorf("%v answered type %q, not in the schema", c, typ)
			continue
		}
		conforms(t, doc, variant, env.Result, strings.Join(c[:2], " "))
	}
}

func TestFakeHerdrReportsEveryAgentStatus(t *testing.T) {
	// covers: R-SESS-016
	// The fake can say every status herdr's schema allows, in the three places the atoms read it;
	// `done` is idle in a tab the operator has not focused since, and the worker treats it as idle.
	data, err := os.ReadFile("testdata/herdr-schema-0.8.2.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schemas struct {
			Success struct {
				Defs struct {
					AgentStatus struct {
						Enum []string `json:"enum"`
					}
				} `json:"$defs"`
			} `json:"success_response"`
		} `json:"schemas"`
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	enum := doc.Schemas.Success.Defs.AgentStatus.Enum
	if len(enum) != 5 {
		t.Fatalf("the schema's AgentStatus is %v", enum)
	}
	dir := t.TempDir()
	if err = testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	f := testkit.FakeExec{Dir: dir}
	for _, c := range [][]string{
		{"tab", "create", "--cwd", "/tmp/x", "--label", "root"},
		{"agent", "start", "p-root", "--kind", "claude", "--pane", "wF:p1", "--", "hi"},
	} {
		if _, err = f.Run("", "herdr", c...); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range enum {
		if err = testkit.SetHerdr(dir, "status", "p-root,"+status); err != nil {
			t.Fatal(err)
		}
		for _, c := range [][]string{{"agent", "get", "p-root"}, {"tab", "list"}, {"pane", "list"}} {
			out, rerr := f.Run("", "herdr", c...)
			if rerr != nil || !strings.Contains(string(out), `"agent_status":"`+status+`"`) {
				t.Errorf("%v with status %s: %s, %v", c, status, out, rerr)
			}
		}
	}
}

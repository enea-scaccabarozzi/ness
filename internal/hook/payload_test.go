package hook_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func TestParse(t *testing.T) {
	// covers: R-HOOK-015, R-HOOK-017
	p, err := hook.Parse(strings.NewReader(`{"hook_event_name":"SessionStart","source":"compact","session_id":"x"}`))
	if err != nil || p.Source != "compact" {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
	p, err = hook.Parse(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`))
	if err != nil || p.ToolName != "Bash" || string(p.ToolInput) != `{"command":"ls"}` {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
	for _, bad := range []string{"", "{not json", "[1,2]", "null"} {
		if _, err := hook.Parse(strings.NewReader(bad)); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}

func TestLargeWritePayload(t *testing.T) {
	// A Write of a big file must parse whole and must not be blocked (no stdin cap).
	// covers: R-HOOK-015, R-HOOK-011, R-HOOK-013
	content := strings.Repeat("while true; do ness inbox; done /tool-results/x.txt\n", 30000)
	if len(content) <= 1<<20 {
		t.Fatalf("fixture is %d bytes, want over 1 MiB", len(content))
	}
	p, err := hook.Parse(strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"/x/big.txt","content":` + string(mustJSON(content)) + `}}`))
	if err != nil || p.ToolName != "Write" {
		t.Fatalf("Parse = %v", err)
	}
	if reason := hook.PreTool(p); reason != "" {
		t.Errorf("large Write blocked: %q", reason)
	}
}

func FuzzParse(f *testing.F) {
	// The parser and the guards never panic on any input (R-HOOK-015, R-QA-022).
	// covers: R-HOOK-015, R-HOOK-032
	f.Add([]byte(`{"source":"compact"}`))
	f.Add([]byte(`{"tool_name":"Bash","tool_input":{"command":"while true; do ness inbox; done"}}`))
	f.Add([]byte(`{"tool_name":"Read","tool_input":{"file_path":"/x/tool-results/a.txt","offset":1}}`))
	f.Add([]byte(`{"tool_name":"Monitor","tool_input":"ness inbox"}`))
	f.Add([]byte(`{"tool_name":"apply_patch","cwd":"/w","tool_input":{"input":"*** Update File: a\n"}}`))
	f.Add([]byte("{not json"))
	// Rejects: a ParseSpawn that panics on a prompt or model that is not a string (R-HOOK-032).
	f.Add([]byte(`{"tool_name":"Agent","tool_input":{"prompt":"kind: search\nescalate: x","model":"opus","effort":"low"}}`))
	f.Add([]byte(`{"tool_name":"Task","tool_input":{"prompt":7,"model":["x"]}}`))
	f.Fuzz(func(_ *testing.T, data []byte) {
		p, err := hook.Parse(bytes.NewReader(data))
		if err != nil {
			return
		}
		_ = hook.PreTool(p)
		_ = hook.EditedFiles(p.ToolName, p.ToolInput)
		_, _ = hook.ParseSpawn(p)
	})
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

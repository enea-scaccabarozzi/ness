package hook_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func TestEditedFilesPerTool(t *testing.T) {
	// The guard sees the file an edit tool names, the files a Codex patch updates or deletes,
	// and nothing for Bash or Read (R-CTR-009, R-CTR-021).
	// covers: R-CTR-009, R-CTR-021
	patch := "*** Begin Patch\n*** Update File: a/b.go\n*** Move to: e/f.go\n@@\n-x\n+y\n" +
		"*** Delete File: c.txt\n*** Add File: d.txt\n+hi\n*** End Patch"
	crlf := "*** Begin Patch\r\n*** Update File: a.feature\r\n@@\r\n-a\r\n+b\r\n*** End Patch\r\n"
	for _, tc := range []struct {
		tool  string
		input any
		want  []string
	}{
		{"Edit", map[string]any{"file_path": "x.go", "old_string": "a", "new_string": "b"}, []string{"x.go"}},
		{"Write", map[string]any{"file_path": "/abs/y.md", "content": "..."}, []string{"/abs/y.md"}},
		{"MultiEdit", map[string]any{"file_path": "z.go", "edits": []any{}}, []string{"z.go"}},
		{"NotebookEdit", map[string]any{"notebook_path": "n.ipynb", "new_source": "x"}, []string{"n.ipynb"}},
		{"apply_patch", map[string]any{"input": patch}, []string{"a/b.go", "e/f.go", "c.txt", "d.txt"}},
		{"apply_patch", map[string]any{"input": crlf}, []string{"a.feature"}},
		{"Bash", map[string]any{"command": "sed -i s/a/b/ x.go"}, nil},
		{"Read", map[string]any{"file_path": "x.go"}, nil},
		{"Edit", "not an object", nil},
	} {
		raw, _ := json.Marshal(tc.input)
		if got := hook.EditedFiles(tc.tool, raw); !slices.Equal(got, tc.want) {
			t.Errorf("EditedFiles(%s, %s) = %v, want %v", tc.tool, raw, got, tc.want)
		}
	}
}

func TestEditedFilesOrderIsStable(t *testing.T) {
	// A patch spread over several input fields lists its files in field order, every time.
	// covers: R-CTR-009
	in := map[string]any{}
	want := []string{}
	for _, k := range strings.Split("abcdefgh", "") {
		in[k] = "*** Update File: " + k + ".txt"
		want = append(want, k+".txt")
	}
	raw, _ := json.Marshal(in)
	for range 20 {
		if got := hook.EditedFiles("apply_patch", raw); !slices.Equal(got, want) {
			t.Fatalf("EditedFiles = %v, want %v", got, want)
		}
	}
}

func TestParseReadsCwd(t *testing.T) {
	// covers: R-CTR-010
	p, err := hook.Parse(strings.NewReader(`{"tool_name":"Edit","cwd":"/w/x","tool_input":{"file_path":"a.go"}}`))
	if err != nil || p.Cwd != "/w/x" {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
}

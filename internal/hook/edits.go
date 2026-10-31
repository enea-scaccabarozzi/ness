package hook

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

var (
	editTools = []string{"Edit", "Write", "MultiEdit"}
	// every path a Codex patch touches: updated, deleted, added, and the destination of a move.
	patchFile = regexp.MustCompile(`(?m)^\*{3} (?:Update File|Delete File|Add File|Move to): (.+)$`)
)

// EditedFiles lists the files a tool call writes: the file_path of Claude Code's Edit, Write
// and MultiEdit, the notebook_path of NotebookEdit, every file a Codex apply_patch updates,
// deletes, adds or moves to. Bash is never inspected (R-CTR-021), so a redirect, tee, sed -i, mv or cp onto
// a frozen path passes the hook; that is a known limit and `ness contract check` at the gate
// is the backstop. Every other tool edits nothing.
func EditedFiles(tool string, input json.RawMessage) []string {
	switch {
	case slices.Contains(editTools, tool), tool == "NotebookEdit":
		var in struct {
			FilePath     string `json:"file_path"`
			NotebookPath string `json:"notebook_path"`
		}
		if json.Unmarshal(input, &in) != nil {
			return nil
		}
		return slices.DeleteFunc([]string{in.FilePath, in.NotebookPath}, func(s string) bool { return s == "" })
	case tool == "apply_patch":
		var out []string
		for _, m := range patchFile.FindAllStringSubmatch(inputText(input), -1) {
			if f := strings.TrimSpace(m[1]); f != "" { // CRLF patches leave a \r on the path
				out = append(out, f)
			}
		}
		return out
	}
	return nil
}

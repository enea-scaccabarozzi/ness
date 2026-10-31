// Package hook parses client hook payloads and decides the pre-tool guards as pure functions:
// no store, no files, so the CLI can wire them and the fuzz target covers every input
// (R-HOOK-011..015).
package hook

import (
	"encoding/json"
	"fmt"
	"io"
)

// Payload is the part of a client hook's JSON stdin that ness reads.
type Payload struct {
	Source    string          `json:"source"`     // session-start: startup, resume, clear, compact
	ToolName  string          `json:"tool_name"`  // pre-tool
	ToolInput json.RawMessage `json:"tool_input"` // pre-tool; kept raw, the guards decode what they need
	Cwd       string          `json:"cwd"`        // both: the client's working directory, where relative paths resolve
}

// Parse decodes one JSON object from r. It reads all of r: a Write payload carries a whole file.
// It never panics (R-HOOK-015); anything that is not an
// object is an error (R-HOOK-017).
func Parse(r io.Reader) (Payload, error) {
	var p Payload
	data, err := io.ReadAll(r)
	if err != nil {
		return Payload{}, fmt.Errorf("hook payload: %w", err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, fmt.Errorf("hook payload: %w", err)
	}
	if len(data) == 0 || data[firstNonSpace(data)] != '{' {
		return Payload{}, fmt.Errorf("hook payload: not a JSON object")
	}
	return p, nil
}

func firstNonSpace(b []byte) int {
	for i, c := range b {
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			return i
		}
	}
	return len(b) - 1
}

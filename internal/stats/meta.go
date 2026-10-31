package stats

import (
	"encoding/json"
	"io"
)

// Meta is what Claude Code records beside a subagent's transcript (agent-*.meta.json): its
// agent type and the model and effort it was launched with, empty when it took the defaults
// (R-STAT-021).
type Meta struct {
	AgentType string `json:"agentType"` //nolint:tagliatelle // Claude Code's key
	Model     string `json:"model"`
	Effort    string `json:"effort"`
}

// ReadMeta decodes one subagent meta object; anything that is not an object is an error.
func ReadMeta(r io.Reader) (Meta, error) {
	var m Meta
	err := json.NewDecoder(r).Decode(&m)
	return m, err
}

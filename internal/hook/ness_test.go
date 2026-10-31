package hook_test

import (
	"encoding/json"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func TestNessCommand(t *testing.T) {
	// While a skew blocks the hooks, only one plain ness command passes (R-UPD-021).
	// covers: R-UPD-021
	for cmd, want := range map[string]bool{
		"ness doctor": true, "  ness self-update --check": true, "ness": true, "ness inbox --show i-1": true,
		"nessy x": false, "cd x && ness sync": false, "rm -rf ness": false, "": false,
		"ness doctor && rm -rf x": false, "ness doctor; rm x": false, "ness doctor || rm x": false,
		"ness doctor | sh": false, "ness `rm x`": false, "ness $(rm x)": false, "ness doctor > f": false,
		"ness doctor < f": false, "ness doctor\nrm x": false, "ness doctor & rm -rf x": false,
		"ness (x)": false, "ness a \\\nb": false,
	} {
		in, err := json.Marshal(map[string]string{"command": cmd})
		if err != nil {
			t.Fatal(err)
		}
		if got := hook.NessCommand(hook.Payload{ToolName: "Bash", ToolInput: in}); got != want {
			t.Errorf("%q = %v, want %v", cmd, got, want)
		}
	}
	edit, err := json.Marshal(map[string]string{"command": "ness doctor"})
	if err != nil {
		t.Fatal(err)
	}
	if hook.NessCommand(hook.Payload{ToolName: "Edit", ToolInput: edit}) {
		t.Error("non-Bash tool passed")
	}
	if hook.NessCommand(hook.Payload{ToolName: "Bash", ToolInput: json.RawMessage(`"ness"`)}) {
		t.Error("non-object input passed")
	}
}

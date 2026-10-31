package hook_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func payload(tool string, input any) hook.Payload {
	raw, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	return hook.Payload{ToolName: tool, ToolInput: raw}
}

const saved = "/home/u/.claude/projects/-home-u-p/9ed8d0d6/tool-results/b3d68f2ny.txt"

func TestPreToolPollingGuard(t *testing.T) {
	// covers: R-HOOK-011, R-HOOK-012
	blocked := []hook.Payload{
		payload("Monitor", map[string]any{"command": "ness inbox", "timeout_sec": 300}),
		payload("ScheduleWakeup", map[string]any{"prompt": "run ness status and report"}),
		payload("CronCreate", map[string]any{"cron": "*/5 * * * *", "prompt": "ness inbox"}),
		payload("Bash", map[string]any{"command": "while true; do ness inbox; sleep 30; done"}),
		payload("Bash", map[string]any{"command": "until ness status | grep -q done; do sleep 5; done"}),
		payload("Bash", map[string]any{"command": "bash -c 'while true; do ness inbox; done'"}),
		payload("Bash", map[string]any{"command": "sh -c \"until ness status|grep d; do sleep 1; done\""}),
		payload("Bash", map[string]any{"command": "while true; do ness  inbox; done"}),
		payload("Bash", map[string]any{"command": "while true; do ness\tstatus; done"}),
		payload("Monitor", map[string]any{"command": "ness  inbox"}),
		payload("Monitor", map[string]any{"command": "ness\tinbox"}),
		payload("CronCreate", map[string]any{"prompt": "check\nness inbox"}),
		payload("Bash", map[string]any{"command": "for i in 1 2 3; do ness inbox; sleep 10; done"}),
		payload("Bash", map[string]any{"command": "watch -n 5 ness status"}),
		payload("Bash", map[string]any{"command": "for ((;;)); do ness status; done"}),
		payload("Bash", map[string]any{"command": "watch -n5 ness inbox"}),
		payload("Bash", map[string]any{"command": "sudo watch ness inbox"}),
		payload("Bash", map[string]any{"command": "sleep 60; while ! ness inbox | grep m-; do :; done"}),
	}
	for _, p := range blocked {
		reason := hook.PreTool(p)
		if reason == "" || !strings.Contains(reason, "end your turn") {
			t.Errorf("%s %s: allowed or no waiting rule: %q", p.ToolName, p.ToolInput, reason)
		}
	}
	allowed := []hook.Payload{
		payload("Bash", map[string]any{"command": "ness inbox"}),
		payload("Bash", map[string]any{"command": "ness inbox --show m-abcdefgh"}),
		payload("Bash", map[string]any{"command": "ness status --json"}),
		payload("Bash", map[string]any{"command": "while read l; do echo $l; done < f"}),
		payload("Bash", map[string]any{"command": "ness send --body \"ready for review\" && ness inbox"}),
		payload("Bash", map[string]any{"command": "echo \"wait for it\"; ness inbox"}),
		payload("Bash", map[string]any{"command": "grep watch log; ness status"}),
		payload("Bash", map[string]any{"command": "for f in *.md; do wc -l $f; done"}),
		payload("Monitor", map[string]any{"command": "tail -f build.log"}),
		payload("ScheduleWakeup", map[string]any{"prompt": "check the deploy"}),
		payload("CronCreate", map[string]any{"prompt": "rotate logs"}),
		payload("Edit", map[string]any{"file_path": "x.go"}),
		payload("Bash", map[string]any{"command": "make test"}),
		payload("Bash", map[string]any{"command": "echo 'sleep 5'"}),
		payload("Bash", map[string]any{"command": "for i in 1 2 3; do ness inbox; done"}),
	}
	for _, p := range allowed {
		if reason := hook.PreTool(p); reason != "" {
			t.Errorf("%s %s: blocked: %q", p.ToolName, p.ToolInput, reason)
		}
	}
}

func TestPreToolOutputGuard(t *testing.T) {
	// covers: R-HOOK-013
	blocked := []hook.Payload{
		payload("Read", map[string]any{"file_path": saved}),
		payload("Bash", map[string]any{"command": "cat " + saved}),
		payload("Bash", map[string]any{"command": "cat " + saved + " | head"}),
		payload("Bash", map[string]any{"command": "sed 's/a/b/' " + saved}),
	}
	for _, p := range blocked {
		if reason := hook.PreTool(p); reason == "" {
			t.Errorf("%s %s: allowed", p.ToolName, p.ToolInput)
		}
	}
	allowed := []hook.Payload{
		payload("Read", map[string]any{"file_path": saved, "offset": 1, "limit": 200}),
		payload("Read", map[string]any{"file_path": saved, "limit": 50}),
		payload("Read", map[string]any{"file_path": "/home/u/p/main.go"}),
		payload("Bash", map[string]any{"command": "sed -n '1,40p' " + saved}),
		payload("Bash", map[string]any{"command": "grep -n error " + saved}),
		payload("Bash", map[string]any{"command": "head -c 2000 " + saved}),
		payload("Bash", map[string]any{"command": "cat main.go"}),
	}
	for _, p := range allowed {
		if reason := hook.PreTool(p); reason != "" {
			t.Errorf("%s %s: blocked: %q", p.ToolName, p.ToolInput, reason)
		}
	}
	// A tool_input that is not an object is not a guard's business.
	if reason := hook.PreTool(hook.Payload{ToolName: "Read", ToolInput: json.RawMessage(`"x"`)}); reason != "" {
		t.Errorf("non-object tool_input blocked: %q", reason)
	}
}

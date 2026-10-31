package stats_test

import (
	"bytes"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "fixture.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReadTheFixture(t *testing.T) {
	// Tokens per class (one count per message.id although its content blocks are several
	// lines), the context of each turn from the last call, two compactions from the markers,
	// tool_result bytes per tool, the unparsable line counted, the synthetic call ignored
	// (R-STAT-001, R-STAT-007, R-STAT-010, R-STAT-011).
	// covers: R-STAT-001, R-STAT-007, R-STAT-010, R-STAT-011
	tr, err := stats.Read(bytes.NewReader(fixture(t)), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := stats.Usage{Input: 218000, Output: 71100, CacheWrite: 1000000, CacheRead: 2415000, CacheWrite1h: 400000}
	if tr.Usage != want || tr.Model != "claude-opus-5-5" {
		t.Fatalf("Usage = %+v, Model = %q", tr.Usage, tr.Model)
	}
	if !slices.Equal(tr.Turns, []int{1210000, 1220000, 3000}) || tr.Context() != 3000 {
		t.Fatalf("Turns = %v, Context = %d", tr.Turns, tr.Context())
	}
	if tr.Compactions != 2 || tr.Skipped != 1 {
		t.Fatalf("Compactions = %d, Skipped = %d", tr.Compactions, tr.Skipped)
	}
	if tr.ToolBytes["Read"] != 100 || tr.ToolBytes["Bash"] != 29 || len(tr.ToolBytes) != 2 {
		t.Fatalf("ToolBytes = %v", tr.ToolBytes)
	}
	if cents, ok := stats.Cost(tr.Model, tr.Usage); !ok || cents != 898 {
		t.Fatalf("Cost = %d, %v; want 898 cents", cents, ok)
	}
	// 7.777 USD with every write at 1.25x, plus 0.4M tokens of 1-hour writes at 2x instead: +1.2.
	if cents, unpriced := tr.Cost(); cents != 898 || unpriced {
		t.Fatalf("Transcript.Cost = %d, unpriced %v; want 898, false", cents, unpriced)
	}
}

func TestUsageIsCountedPerMessageIDNotPerLine(t *testing.T) {
	// One API call written as three lines (one per content block) repeats its usage and its
	// output grows line by line: the call counts once, at the last line (not the first, not the
	// sum). A line with no message.id counts once each.
	// covers: R-STAT-001
	call := func(id string, out int) string {
		m := `"id":"` + id + `",`
		if id == "" {
			m = ""
		}
		return `{"type":"assistant","message":{` + m + `"model":"claude-opus-5-5",` +
			`"usage":{"input_tokens":100,"output_tokens":` + strconv.Itoa(out) + `,"cache_read_input_tokens":900},` +
			`"content":[{"type":"text","text":"x"}]}}`
	}
	in := strings.Join([]string{call("msg_a", 3), call("msg_a", 7), call("msg_a", 30), call("", 4), call("", 5)}, "\n")
	tr, err := stats.Read(strings.NewReader(in), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if want := (stats.Usage{Input: 300, Output: 39, CacheRead: 2700}); tr.Usage != want {
		t.Fatalf("Usage = %+v, want %+v (msg_a once at its last line, two anonymous lines once each)", tr.Usage, want)
	}
}

func TestBlankAndNullLinesAreNotSkipped(t *testing.T) {
	// A blank or whitespace line and a JSON null are empty, not bad; other non-objects are bad.
	// covers: R-STAT-007
	tr, err := stats.Read(strings.NewReader("\n  \nnull\n[1]\n{\n"), time.Time{})
	if err != nil || tr.Skipped != 2 {
		t.Fatalf("Skipped = %d, %v; want 2 ([1] and {)", tr.Skipped, err)
	}
}

func TestEachCallIsPricedAtItsOwnModel(t *testing.T) {
	// Two models in one transcript are priced separately; an unknown model is flagged, its
	// tokens still counted, and the known model's cost still reported.
	// covers: R-STAT-001
	line := func(id, model string) string {
		return `{"type":"assistant","message":{"id":"` + id + `","model":"` + model + `",` +
			`"usage":{"input_tokens":1000000},"content":[{"type":"text","text":"x"}]}}`
	}
	in := strings.Join([]string{line("a", "claude-opus-5-5"), line("b", "claude-haiku-4-5"), line("c", "claude-opus-5-5")}, "\n")
	tr, err := stats.Read(strings.NewReader(in), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if cents, unpriced := tr.Cost(); cents != 900 || unpriced { // 2 x 4.00 + 1.00
		t.Fatalf("Cost = %d, unpriced %v; want 900, false", cents, unpriced)
	}
	in += "\n" + line("d", "claude-opus-5-6")
	tr, _ = stats.Read(strings.NewReader(in), time.Time{})
	if cents, unpriced := tr.Cost(); cents != 900 || !unpriced || tr.Usage.Input != 4000000 {
		t.Fatalf("Cost = %d, unpriced %v, input %d; want 900, true, 4000000", cents, unpriced, tr.Usage.Input)
	}
}

func TestReadSince(t *testing.T) {
	// covers: R-STAT-004
	since := time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)
	tr, err := stats.Read(bytes.NewReader(fixture(t)), since)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Usage != (stats.Usage{Input: 8000, Output: 1100, CacheRead: 1215000}) {
		t.Fatalf("Usage since 11:00 = %+v", tr.Usage)
	}
	if !slices.Equal(tr.Turns, []int{1220000, 3000}) || tr.Compactions != 2 || tr.Skipped != 1 {
		t.Fatalf("since 11:00: Turns %v, Compactions %d, Skipped %d", tr.Turns, tr.Compactions, tr.Skipped)
	}
	if cents, _ := stats.Cost(tr.Model, tr.Usage); cents != 30 {
		t.Fatalf("Cost since 11:00 = %d, want 30", cents)
	}
	future, err := stats.Read(bytes.NewReader(fixture(t)), time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if future.Usage != (stats.Usage{}) || len(future.Turns) != 0 {
		t.Fatalf("a future since keeps figures: %+v", future)
	}
}

func TestSkippedCountsEveryBadLine(t *testing.T) {
	// covers: R-STAT-007
	lines := strings.Split(strings.TrimSpace(string(fixture(t))), "\n")
	r := rand.New(rand.NewPCG(1, 2))
	bad := 0
	var out []string
	for _, l := range lines {
		if l == "not json at all" {
			continue
		}
		out = append(out, l)
		for range r.IntN(3) {
			out = append(out, []string{"{", "[1,2]", "\"str\"", "{\"type\":}", "\x00\xff"}[r.IntN(5)])
			bad++
		}
	}
	tr, err := stats.Read(strings.NewReader(strings.Join(out, "\n")), time.Time{})
	if err != nil || tr.Skipped != bad {
		t.Fatalf("Skipped = %d, %v; injected %d", tr.Skipped, err, bad)
	}
}

func FuzzRead(f *testing.F) {
	// covers: R-STAT-007
	b, err := os.ReadFile(filepath.Join("testdata", "fixture.jsonl"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(b)
	f.Add([]byte(`{"type":"assistant","message":{"id":"m","content":"plain","usage":{"input_tokens":1}}}`))
	f.Add([]byte("[1]\nnull\n{}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		tr, err := stats.Read(bytes.NewReader(data), time.Time{})
		if err != nil {
			return // a line over the scanner limit
		}
		n := bytes.Count(data, []byte("\n")) + 1
		var sum stats.Usage
		for _, u := range tr.Models {
			sum = sum.Add(u)
		}
		if tr.Skipped > n || len(tr.Turns) > n || tr.Compactions > n || sum != tr.Usage {
			t.Fatalf("lines %d: Skipped %d, Turns %d, Compactions %d, Usage %+v vs models %+v",
				n, tr.Skipped, len(tr.Turns), tr.Compactions, tr.Usage, sum)
		}
	})
}

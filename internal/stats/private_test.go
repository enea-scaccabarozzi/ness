package stats_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestPrivateDiscoveryFigures(t *testing.T) {
	// The saved discovery transcripts reproduce the discovery figures (R-STAT-008). The fixture
	// is .private/stats/*.jsonl with expected.json {"cost_cents","input_tokens","output_tokens"};
	// gitignored, absent in CI, where TestReadTheFixture is the synthetic golden.
	// covers: R-STAT-008
	dir := filepath.Join("..", "..", ".private", "stats")
	data, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip(".private/stats is absent: the discovery figures are checked locally only")
	}
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		CostCents    int `json:"cost_cents"`
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
	if err = json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	var total stats.Usage
	cents := 0
	for _, f := range files {
		r, oerr := os.Open(f) //nolint:gosec // the local fixture
		if oerr != nil {
			t.Fatal(oerr)
		}
		tr, rerr := stats.Read(r, time.Time{})
		_ = r.Close()
		if rerr != nil {
			t.Fatal(rerr)
		}
		c, _ := tr.Cost()
		total, cents = total.Add(tr.Usage), cents+c
	}
	slack := len(files)
	if total.Input != want.InputTokens || total.Output != want.OutputTokens ||
		cents < want.CostCents-slack || cents > want.CostCents+slack {
		t.Fatalf("got in %d out %d %d cents; want in %d out %d %d cents (±%d)", total.Input, total.Output, cents,
			want.InputTokens, want.OutputTokens, want.CostCents, slack)
	}
}

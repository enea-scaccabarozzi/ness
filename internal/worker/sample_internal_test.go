package worker

import (
	"math/rand/v2"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
)

func TestSampledGreenFraction(t *testing.T) {
	// sample 0.1 over 10,000 GREEN runs wakes 10% +- 1%; RED never counts as a sampled GREEN
	// (R-PRB-036).
	// covers: R-PRB-036
	w := &Worker{Probes: &Probes{Rand: rand.New(rand.NewPCG(42, 42))}}
	pf := probe.Probe{Sample: 0.1}
	n := 0
	for range 10000 {
		if w.sampledGreen(pf, probe.Green) {
			n++
		}
	}
	if n < 900 || n > 1100 {
		t.Fatalf("sampled %d of 10,000 GREEN runs; want 1,000 +- 100", n)
	}
	if w.sampledGreen(pf, probe.Red) {
		t.Fatal("a RED run was sampled")
	}
}

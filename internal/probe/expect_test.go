package probe_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
)

func TestExpectGrammarAndEvaluation(t *testing.T) {
	// expect is a comma-separated list of key op number; all must hold on the last line's
	// key=value pairs; a missing key or a non-number fails (R-PRB-023, R-PRB-025).
	// covers: R-PRB-023, R-PRB-025
	conds, err := probe.ParseExpect("n>=1, lag<50")
	if err != nil || len(conds) != 2 || conds[1] != (probe.Cond{Key: "lag", Op: "<", Value: 50}) {
		t.Fatalf("ParseExpect = %v, %v", conds, err)
	}
	for line, want := range map[string]bool{
		"n=2 lag=10": true, "n=2 lag=60": false, "lag=10": false, "n=0 lag=10": false, "n=abc lag=10": false, "": false,
	} {
		if got := probe.Satisfied(conds, probe.ParseKV(line)); got != want {
			t.Errorf("Satisfied(%q) = %v, want %v", line, got, want)
		}
	}
	for _, bad := range []string{"n=>1", "n", ">=1", "n>=", "n>=x", "n>=1,", "n ~ 1", "n==1 lag<5"} {
		if _, err = probe.ParseExpect(bad); err == nil {
			t.Errorf("ParseExpect(%q) accepted", bad)
		}
	}
	for _, op := range []string{">=", "<=", ">", "<", "==", "!="} {
		if _, err = probe.ParseExpect("k " + op + " 1.5"); err != nil {
			t.Errorf("op %s refused: %v", op, err)
		}
	}
	if c, _ := probe.ParseExpect("k!=1"); probe.Satisfied(c, probe.ParseKV("k=1")) || !probe.Satisfied(c, probe.ParseKV("k=2")) {
		t.Error("!= is wrong")
	}
}

func TestParseKVIgnoresBareTokens(t *testing.T) {
	// Review Focus 2: a last line mixing pairs with other words keeps the pairs; a repeated key
	// takes the last value; a value with a unit is kept as text and fails a numeric condition.
	// covers: R-PRB-023, R-PRB-024
	kv := probe.ParseKV("done n=3 ok lag=10ms n=4 = x= =y")
	if kv["n"] != "4" || kv["lag"] != "10ms" || len(kv) != 2 || kv["x"] != "" {
		t.Fatalf("ParseKV = %v", kv)
	}
	conds, _ := probe.ParseExpect("lag<50")
	if probe.Satisfied(conds, kv) {
		t.Fatal("10ms is not a number: the condition must fail")
	}
}

func FuzzExpect(f *testing.F) {
	// covers: R-PRB-024
	f.Add("n>=1, lag<50", "n=2 lag=10")
	f.Add("k!=0", "k=0 k=1")
	f.Fuzz(func(_ *testing.T, expr, line string) {
		conds, err := probe.ParseExpect(expr)
		if err != nil {
			return
		}
		_ = probe.Satisfied(conds, probe.ParseKV(line))
	})
}

func TestSatisfiedRefusesNaNAndInf(t *testing.T) {
	// covers: R-PRB-023
	conds, _ := probe.ParseExpect("n!=1")
	for _, v := range []string{"NaN", "Inf", "+Inf", "-Inf"} {
		if probe.Satisfied(conds, probe.ParseKV("n="+v)) {
			t.Errorf("n=%s satisfied", v)
		}
	}
}

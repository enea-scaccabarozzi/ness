package probe

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Cond is one key op number condition of expect (R-PRB-025).
type Cond struct {
	Key, Op string
	Value   float64
}

var condRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*(>=|<=|==|!=|>|<)\s*(-?\d+(?:\.\d+)?)$`)

// ParseExpect reads "key op number, key op number"; any other form is refused.
func ParseExpect(s string) ([]Cond, error) {
	var out []Cond
	for _, part := range strings.Split(s, ",") {
		m := condRe.FindStringSubmatch(strings.TrimSpace(part))
		if m == nil {
			return nil, fmt.Errorf("expect: %q is not \"key op number\" with op in >= <= > < == !=",
				strings.TrimSpace(part))
		}
		v, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			return nil, fmt.Errorf("expect: %q: %w", m[3], err)
		}
		out = append(out, Cond{Key: m[1], Op: m[2], Value: v})
	}
	return out, nil
}

// ParseKV reads the key=value pairs of a line; other words are ignored, a repeated key takes
// the last value, an empty key or value is dropped (R-PRB-024).
func ParseKV(line string) map[string]string {
	out := map[string]string{}
	for _, tok := range strings.Fields(line) {
		k, v, ok := strings.Cut(tok, "=")
		if ok && k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

// Satisfied reports whether every condition holds on kv; a missing key or a value that is not
// a number fails (R-PRB-023).
func Satisfied(conds []Cond, kv map[string]string) bool {
	for _, c := range conds {
		raw, ok := kv[c.Key]
		if !ok {
			return false
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || !holds(c.Op, v, c.Value) {
			return false
		}
	}
	return true
}

func holds(op string, v, want float64) bool {
	switch op {
	case ">=":
		return v >= want
	case "<=":
		return v <= want
	case ">":
		return v > want
	case "<":
		return v < want
	case "==":
		return v == want
	}
	return v != want
}

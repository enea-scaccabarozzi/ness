// Package probe is the probes feature (design §8): one Markdown file with frontmatter per
// probe, an optional floor command whose exit code and last line give a state, and the scope
// the woken agent judges. The worker schedules; the owner answers.
package probe

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Accepted values (R-PRB-004, R-PRB-005) and fields (R-PRB-002).
var (
	Wakes    = []string{"never", "change", "red", "not-green", "always"}
	Notifies = []string{"none", "auto", "judged"}
	Fields   = []string{
		"every", "jitter", "once", "window", "floor", "expect", "confirm", "timeout",
		"wake", "sample", "notify", "channel",
	}
)

// Probe is one parsed probe file with the defaults applied (R-PRB-080).
type Probe struct {
	Path                           string
	Every, Jitter, Window, Timeout time.Duration
	Once                           bool
	Floor                          string
	Expect                         []Cond
	Confirm                        int
	Wake, Notify, Channel          string
	Sample                         float64
	Scope                          string
	Hash                           string
}

// Floorless reports a probe with no floor: a pure semantic check that wakes on every run.
func (p Probe) Floorless() bool { return p.Floor == "" }

// Hash identifies the file's content, for reloads and the needs-edit flag (R-PRB-061, R-PRB-065).
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var kvLine = regexp.MustCompile(`^([a-z]+):\s*(.*)$`)

// Parse reads a probe file: flat key: value frontmatter between --- lines, the body as the
// scope. Every refusal names the file (R-PRB-003, R-PRB-008); nothing panics (R-PRB-009).
func Parse(path string, data []byte) (Probe, error) {
	fields, body, err := frontmatter(data)
	if err != nil {
		return Probe{}, fmt.Errorf("%s: %w", path, err)
	}
	p := Probe{
		Path: path, Confirm: 1, Timeout: 60 * time.Second, Wake: "not-green", Notify: "none",
		Scope: strings.TrimSpace(body), Hash: Hash(data),
	}
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		if !slices.Contains(Fields, k) {
			return Probe{}, fmt.Errorf("%s: unknown field %s", path, k)
		}
		if err = p.set(k, fields[k]); err != nil {
			return Probe{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	if err = p.check(); err != nil {
		return Probe{}, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// frontmatter splits the --- block into key: value pairs (comments after # dropped, quotes
// stripped) and the body; a line that is not a flat pair is refused by number.
func frontmatter(data []byte) (fields map[string]string, scope string, ferr error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if !firstIsFence(sc) {
		return nil, "", errors.New("no frontmatter: the file must start with ---")
	}
	fields = map[string]string{}
	for n := 2; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			scope, err := readBody(sc)
			return fields, scope, err
		}
		if err := add(fields, n, line); err != nil {
			return nil, "", err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	return nil, "", errors.New("frontmatter is not closed by ---")
}

func firstIsFence(sc *bufio.Scanner) bool {
	return sc.Scan() && strings.TrimSpace(sc.Text()) == "---"
}

func readBody(sc *bufio.Scanner) (string, error) {
	var body strings.Builder
	for sc.Scan() {
		body.WriteString(sc.Text())
		body.WriteByte('\n')
	}
	return body.String(), sc.Err()
}

// add stores the pair on line n in fields; a repeated key is refused.
func add(fields map[string]string, n int, line string) error {
	k, v, err := pair(n, line)
	if err != nil || k == "" {
		return err
	}
	if _, dup := fields[k]; dup {
		return fmt.Errorf("line %d: duplicate field %s", n, k)
	}
	fields[k] = v
	return nil
}

// pair reads one frontmatter line; a blank or comment line gives an empty key.
func pair(n int, line string) (key, val string, err error) {
	if t := strings.TrimSpace(line); t == "" || strings.HasPrefix(t, "#") {
		return "", "", nil
	}
	m := kvLine.FindStringSubmatch(line)
	if m == nil {
		return "", "", fmt.Errorf("line %d: %q is not a flat \"key: value\" line", n, line)
	}
	if val, err = value(m[2]); err != nil {
		return "", "", fmt.Errorf("line %d: %w", n, err)
	}
	return m[1], val, nil
}

// value drops the quotes of a quoted value, keeping any # inside them; only a comment may
// follow the closing quote. An unquoted value ends at the first " #", which starts a comment (P-30).
func value(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v != "" && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			if rest := strings.TrimSpace(v[end+2:]); rest != "" && rest[0] != '#' {
				return "", fmt.Errorf("text after the closing quote: %q", rest)
			}
			return v[1 : end+1], nil
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v), nil
}

func dur(f func(*Probe) *time.Duration) func(*Probe, string) error {
	return func(p *Probe, v string) (err error) {
		*f(p), err = ParseDuration(v)
		return err
	}
}

func str(f func(*Probe) *string) func(*Probe, string) error {
	return func(p *Probe, v string) error { *f(p) = v; return nil }
}

// setters assigns each frontmatter field to its Probe member.
var setters = map[string]func(*Probe, string) error{
	"every":   dur(func(p *Probe) *time.Duration { return &p.Every }),
	"jitter":  dur(func(p *Probe) *time.Duration { return &p.Jitter }),
	"window":  dur(func(p *Probe) *time.Duration { return &p.Window }),
	"timeout": dur(func(p *Probe) *time.Duration { return &p.Timeout }),
	"floor":   str(func(p *Probe) *string { return &p.Floor }),
	"wake":    str(func(p *Probe) *string { return &p.Wake }),
	"notify":  str(func(p *Probe) *string { return &p.Notify }),
	"channel": str(func(p *Probe) *string { return &p.Channel }),
	"once": func(p *Probe, v string) (err error) {
		p.Once, err = strconv.ParseBool(v)
		return err
	},
	"expect": func(p *Probe, v string) (err error) {
		p.Expect, err = ParseExpect(v)
		return err
	},
	"confirm": func(p *Probe, v string) (err error) {
		p.Confirm, err = strconv.Atoi(v)
		return err
	},
	"sample": func(p *Probe, v string) (err error) {
		p.Sample, err = strconv.ParseFloat(v, 64)
		return err
	},
}

func (p *Probe) set(k, v string) error {
	if err := setters[k](p, v); err != nil {
		return fmt.Errorf("%s: %s: %w", k, v, err)
	}
	return nil
}

func (p *Probe) check() error {
	switch {
	case p.Every == 0 && !p.Once:
		return errors.New("neither every nor once is set")
	case !slices.Contains(Wakes, p.Wake):
		return fmt.Errorf("wake: %s: accepted values are %s", p.Wake, strings.Join(Wakes, ", "))
	case !slices.Contains(Notifies, p.Notify):
		return fmt.Errorf("notify: %s: accepted values are %s", p.Notify, strings.Join(Notifies, ", "))
	case !(p.Sample >= 0 && p.Sample <= 1): // also refuses NaN
		return fmt.Errorf("sample: %g: must be between 0 and 1", p.Sample)
	case p.Confirm < 1:
		return fmt.Errorf("confirm: %d: must be at least 1", p.Confirm)
	case p.Notify != "none" && p.Channel == "":
		return fmt.Errorf("notify: %s needs a channel", p.Notify)
	}
	return nil
}

const maxDays = 100000

var dayRe = regexp.MustCompile(`^(\d+)d(.*)$`)

// ParseDuration accepts Go duration syntax plus a leading Nd (24 h) unit; the result must be
// positive (R-PRB-081).
func ParseDuration(s string) (time.Duration, error) {
	days, rest, err := splitDays(s)
	if err != nil {
		return 0, err
	}
	var d time.Duration
	if rest != "" {
		if d, err = time.ParseDuration(rest); err != nil {
			return 0, err
		}
	} else if days == 0 {
		return 0, errors.New("empty duration")
	}
	if d > math.MaxInt64-time.Duration(days)*24*time.Hour {
		return 0, errors.New("duration is out of range")
	}
	d += time.Duration(days) * 24 * time.Hour
	if d <= 0 {
		return 0, fmt.Errorf("%v is not a positive duration", d)
	}
	return d, nil
}

// splitDays cuts a leading Nd off s; no sign may follow it.
func splitDays(s string) (days int, rest string, err error) {
	m := dayRe.FindStringSubmatch(s)
	if m == nil {
		return 0, s, nil
	}
	if days, err = strconv.Atoi(m[1]); err != nil || days > maxDays {
		return 0, "", fmt.Errorf("%s days is out of range", m[1])
	}
	if m[2] != "" && (m[2][0] == '-' || m[2][0] == '+') {
		return 0, "", fmt.Errorf("%q: a sign after d is not accepted", m[2])
	}
	return days, m[2], nil
}

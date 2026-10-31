// Package config loads and validates .ness/ness.toml, the only place ness reads project
// settings from (R-CFG-001), and resolves the references its values may hold into the user's
// secrets file or the environment (R-CFG-080).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Path is the configuration file, relative to the repository root.
const Path = ".ness/ness.toml"

// Config is the validated content of .ness/ness.toml with defaults applied.
type Config struct {
	Ness         string          `toml:"ness"`
	Project      string          `toml:"project"`
	Base         string          `toml:"base"`
	Gate         string          `toml:"gate"`
	Sessions     string          `toml:"sessions"`
	Client       string          `toml:"client"`
	Profiles     []string        `toml:"profiles"`
	Features     []string        `toml:"features"`
	Experimental []string        `toml:"experimental"`
	Worktree     Worktree        `toml:"worktree"`
	Integrate    Integrate       `toml:"integrate"`
	Contracts    Contracts       `toml:"contracts"`
	Probes       Probes          `toml:"probes"`
	Notify       Notify          `toml:"notify"`
	Tiers        map[string]Tier `toml:"tiers"`
	Models       Models          `toml:"models"`
	Launch       Launch          `toml:"launch"`
	Worker       Worker          `toml:"worker"`
	Guards       []Guard         `toml:"guards"`
}

// Guard is one [[guards]] entry: a pre-tool refusal for a tool call (R-CFG-086, R-HOOK-022).
type Guard struct {
	Tool    string `toml:"tool"`    // regexp on the tool name
	Match   string `toml:"match"`   // regexp on the Bash command or the edited file path
	Content string `toml:"content"` // optional regexp on the new text of an edit
	Message string `toml:"message"`
	Skill   string `toml:"skill"`
	Section string `toml:"section"`
}

// Worktree lists files copied or linked from the primary checkout into each session worktree.
type Worktree struct {
	Copy []string `toml:"copy"`
	Link []string `toml:"link"`
}

// Integrate configures how ness integrate folds a branch.
type Integrate struct {
	Mode string `toml:"mode"`
	Push bool   `toml:"push"`
	// Setup is an optional shell command run in the gate worktree before the gate (R-CFG-061).
	Setup string `toml:"setup"`
}

// Contracts lists project-wide frozen paths and the globs of append-only prefix files (R-CTR-033).
type Contracts struct {
	// Frozen are repo-relative paths or path.Match globs (no **) of files held to their text at the
	// base branch, in the mode a session lock would give them: a .feature file scenario by
	// scenario, a .md file matching Prefix as an append-only prefix, anything else whole-file
	// (R-CTR-040). A glob freezes only files that exist at base.
	Frozen []string `toml:"frozen"`
	// Prefix are path.Match globs (no **) of Markdown files that freeze as an append-only prefix.
	Prefix []string `toml:"prefix"`
	// Formats are project-defined contract formats, matched in the order the file writes them,
	// before the builtin modes (R-CTR-047).
	Formats map[string]Format `toml:"formats"`
	// FormatOrder lists the Formats names in file order; Config fills it.
	FormatOrder []string `toml:"-"`
}

// Format is one [contracts.formats.NAME]: files matching Match freeze unit by unit, the units being
// what the Units command prints for the file (R-CTR-047).
type Format struct {
	// Match are path.Match globs (no **) of the files in this format.
	Match []string `toml:"match"`
	// Units is a shell command run at the repo root with the file's text on stdin and its path as
	// the last argument; it prints one {"id": ..., "body": ...} JSON line per unit.
	Units string `toml:"units"`
}

// FormatOf is the first format, in file order, whose globs match rel.
func (c Contracts) FormatOf(rel string) (name string, f Format, ok bool) {
	for _, n := range c.FormatOrder {
		for _, g := range c.Formats[n].Match {
			if m, err := path.Match(g, rel); err == nil && m {
				return n, c.Formats[n], true
			}
		}
	}
	return "", Format{}, false
}

// Probes configures the probes feature.
type Probes struct {
	RedactEnv []string `toml:"redact_env"`
}

// Notify holds the named notification channels and, in Operator, the one the worker tells the
// operator on when a session needs them (CX-16, R-CFG-055); "" means nobody is told.
type Notify struct {
	Operator string             `toml:"operator"`
	Channels map[string]Channel `toml:"channels"`
}

// Channel is one notification channel; which keys are required depends on Kind. Every key but
// Kind may hold a reference (env:NAME, secret:NAME) that Resolve turns into its value (R-CFG-080).
type Channel struct {
	Kind      string `toml:"kind"`
	Token     string `toml:"token"`
	ChannelID string `toml:"channel_id"`
	ChatID    string `toml:"chat_id"`
	Repo      string `toml:"repo"`
	Command   string `toml:"command"`
}

// Tier is one row of the tier table: "<model>/<effort>" per client.
type Tier struct {
	Claude string `toml:"claude"`
	Codex  string `toml:"codex"`
}

// Models configures the model ladder and the selector that picks a rung on it (R-MOD-005..008).
type Models struct {
	Policy string `toml:"policy"`
	// Base is the rung a session gets when the selector has no answer; offsets count from it.
	Base string `toml:"base"`
	// Ceiling is the highest rung any selector may reach; "" is the top of the ladder.
	Ceiling string `toml:"ceiling"`
	// Ladder is the rung names, cheapest first.
	Ladder []string `toml:"ladder"`
	// Kinds maps a work kind to its offset from Base, for the declared selector.
	Kinds map[string]int `toml:"kinds"`
	// Command is the command selector's program; CommandTimeout bounds one run of it.
	Command         string   `toml:"command"`
	CommandTimeout  Duration `toml:"command_timeout"`
	RouterThreshold float64  `toml:"router_threshold"`
	FallbackModel   string   `toml:"fallback_model"`
	// Enforce is what the pre-tool hook does with a subagent call above the recommended rung:
	// "refuse" (the default) or "log", which records the verdict and allows the call (R-MOD-060).
	Enforce string `toml:"enforce"`
	// Jev configures the jev selector.
	Jev Jev `toml:"jev"`
}

// Jev configures the jev selector: the pinned model, the key reference, the endpoint and the
// bound on one request.
type Jev struct {
	Model    string   `toml:"model"`
	Key      string   `toml:"key"`
	Endpoint string   `toml:"endpoint"`
	Timeout  Duration `toml:"timeout"`
}

// Launch configures session launches.
type Launch struct {
	ReadyTimeout Duration `toml:"ready_timeout"`
	// Permissions is "skip" (the client's skip-permissions mode) or "client" (its own settings).
	Permissions string `toml:"permissions"`
	// MaxChildren caps the children open at once across the project; 0 is half the cores.
	MaxChildren int `toml:"max_children"`
}

// Worker configures the worker.
type Worker struct {
	StallMinutes int `toml:"stall_minutes"`
	QuietSeconds int `toml:"quiet_seconds"`
}

// Duration is a time.Duration written as a Go duration string such as "60s".
type Duration struct{ time.Duration }

// UnmarshalText parses a Go duration string.
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// DefaultTiers is the tier table for every row or client the file leaves out (R-MOD-001).
func DefaultTiers() map[string]Tier {
	return map[string]Tier{
		"bulk":     {Claude: "haiku/medium", Codex: "gpt-6-luna/low"},
		"economy":  {Claude: "sonnet/medium", Codex: "gpt-6-luna/medium"},
		"lean":     {Claude: "opus/low", Codex: "gpt-6.1-sol/low"},
		"standard": {Claude: "opus/medium", Codex: "gpt-6.1-sol/medium"},
		"deep":     {Claude: "opus/high", Codex: "gpt-6.1-sol/high"},
		"frontier": {Claude: "fable/medium", Codex: "astra/medium"},
		"apex":     {Claude: "fable/high", Codex: "astra/high"},
	}
}

// DefaultLadder is the default rung order, cheapest first (R-MOD-005).
func DefaultLadder() []string {
	return []string{"bulk", "economy", "lean", "standard", "deep", "frontier", "apex"}
}

// DefaultKinds is the default offset from base of each work kind (R-MOD-008).
func DefaultKinds() map[string]int {
	return map[string]int{
		"search": -2, "run": -2, "implement": 0, "debug": 0, "research": 1, "review": 1, "verify": 1, "design": 1,
	}
}

// SplitTier splits "<model>/<effort>".
func SplitTier(spec string) (model, effort string) {
	model, effort, _ = strings.Cut(spec, "/")
	return model, effort
}

// Enable adds the features a profile enables (R-PROF-015); a name already on is kept once.
func (c *Config) Enable(names ...string) {
	for _, n := range names {
		if !slices.Contains(c.Features, n) {
			c.Features = append(c.Features, n)
		}
	}
}

// On reports whether the feature is enabled.
func (c *Config) On(feature string) bool { return slices.Contains(c.Features, feature) }

func defaults() Config {
	return Config{
		Sessions:  "docs/sessions",
		Client:    "claude",
		Integrate: Integrate{Mode: "squash"},
		Models: Models{
			Policy: "declared", Base: "standard", Ladder: DefaultLadder(),
			CommandTimeout: Duration{10 * time.Second}, RouterThreshold: 0.8, Enforce: "refuse",
			Jev: Jev{
				Model: "jev-1.14.0", Key: "env:TYPESAFE_API_KEY", Endpoint: "https://api.typesafe.ai",
				Timeout: Duration{5 * time.Second},
			},
		},
		Launch: Launch{ReadyTimeout: Duration{time.Minute}, Permissions: "skip"},
		Worker: Worker{StallMinutes: 30, QuietSeconds: 60},
	}
}

// File is a parsed .ness/ness.toml: its text and which keys it sets.
type File struct {
	data string
	meta toml.MetaData
	raw  map[string]any
}

// Parse checks syntax (errors carry line and column, R-CFG-003) and validates the file with no
// profile defaults, so a broken file is refused before anything else runs.
func Parse(data []byte) (*File, error) {
	f := &File{data: string(data)}
	meta, err := toml.Decode(f.data, &f.raw)
	if err != nil {
		return nil, posError(err)
	}
	f.meta = meta
	if _, err := f.Config(nil); err != nil {
		return nil, err
	}
	return f, nil
}

// Pin returns the version pin, the value of the ness key.
func (f *File) Pin() string {
	if s, ok := f.raw["ness"].(string); ok {
		return s
	}
	return ""
}

// lookup is the raw value at key, nil when absent.
func (f *File) lookup(key toml.Key) any {
	var v any = f.raw
	for _, p := range key {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}

// Defined reports whether the file itself sets the dotted key, e.g. "integrate.mode".
func (f *File) Defined(key string) bool { return f.meta.IsDefined(strings.Split(key, ".")...) }

// Config decodes the file over the built-in defaults and the given profile defaults (dotted key
// to value; the file's own values win, R-PROF-008), then validates the result.
func (f *File) Config(profileDefaults map[string]any) (*Config, error) {
	c := defaults()
	extra := map[string]any{}
	for _, k := range slices.Sorted(maps.Keys(profileDefaults)) {
		if !f.Defined(k) {
			if err := setDotted(extra, k, profileDefaults[k]); err != nil {
				return nil, err
			}
		}
	}
	if err := decodeMap(extra, &c); err != nil {
		return nil, fmt.Errorf("%s: profile default: %w", Path, err)
	}
	meta, err := toml.Decode(f.data, &c)
	if err != nil {
		return nil, typeError(err)
	}
	if und := meta.Undecoded(); len(und) > 0 {
		if und[0][len(und[0])-1] == "token_env" {
			return nil, fmt.Errorf(`%s: unknown key %s: write token = "env:%v" instead (R-CFG-084)`,
				Path, und[0], f.lookup(und[0]))
		}
		return nil, fmt.Errorf("%s: unknown key %s", Path, und[0])
	}
	c.Contracts.FormatOrder = formatOrder(meta, c.Contracts.Formats)
	fillTiers(&c)
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	return &c, nil
}

// ValidKey reports whether a profile may set the dotted key to value (R-PROF-008).
func ValidKey(key string, value any) error {
	m := map[string]any{}
	if err := setDotted(m, key, value); err != nil {
		return err
	}
	c := defaults()
	c.Ness, c.Project, c.Base, c.Gate = "x", "x", "x", "x" // required keys are not the profile's business
	c.Experimental = []string{"codex"}
	if err := decodeMap(m, &c); err != nil {
		return err
	}
	fillTiers(&c)
	return c.validate()
}

var typeMismatch = regexp.MustCompile(
	`line (\d+) \(last key "([^"]+)"\): incompatible types: ` +
		`TOML value has type (\w+); destination has type (\w+)`)

// typeError rewrites the library's type-mismatch message to name the key path, the expected type
// and, for enum keys, the accepted values (R-CFG-004).
func typeError(err error) error {
	m := typeMismatch.FindStringSubmatch(err.Error())
	if m == nil {
		return posError(err)
	}
	msg := fmt.Sprintf("%s:%s: %s must be %s, not %s", Path, m[1], m[2], m[4], m[3])
	if vals, ok := enumKeys[m[2]]; ok {
		msg += " (accepted values are " + vals + ")"
	}
	return errors.New(msg)
}

var enumKeys = map[string]string{
	"client":         strings.Join(Clients, ", "),
	"integrate.mode": strings.Join(IntegrateModes, ", "),
	"models.policy":  strings.Join(ModelPolicies, ", "),
}

func decodeMap(m map[string]any, c *Config) error {
	if len(m) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	meta, err := toml.Decode(buf.String(), c)
	if err != nil {
		return err
	}
	if und := meta.Undecoded(); len(und) > 0 {
		return fmt.Errorf("unknown key %s", und[0])
	}
	return nil
}

func setDotted(m map[string]any, key string, v any) error {
	parts := strings.Split(key, ".")
	for _, p := range parts[:len(parts)-1] {
		next, ok := m[p]
		if !ok {
			next = map[string]any{}
			m[p] = next
		}
		sub, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("key %s: %s is not a table", key, p)
		}
		m = sub
	}
	m[parts[len(parts)-1]] = v
	return nil
}

func posError(err error) error {
	var pe toml.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s:%d:%d: %s", Path, pe.Position.Line, pe.Position.Col, pe.Message)
	}
	return fmt.Errorf("%s: %w", Path, err)
}

func fillTiers(c *Config) {
	if c.Tiers == nil {
		c.Tiers = map[string]Tier{}
	}
	for name, d := range DefaultTiers() {
		t := c.Tiers[name]
		if t.Claude == "" {
			t.Claude = d.Claude
		}
		if t.Codex == "" {
			t.Codex = d.Codex
		}
		c.Tiers[name] = t
	}
	kinds := DefaultKinds()
	maps.Copy(kinds, c.Models.Kinds)
	c.Models.Kinds = kinds
}

// The one list of each set of names (R-CFG-070): validate, the type errors, the profile seams
// and the atoms read these.
var (
	// Clients are the client atoms.
	Clients = []string{"claude", "codex"}
	// IntegrateModes are the integrator atoms.
	IntegrateModes = []string{"merge", "squash", "rebase", "pr"}
	// ModelPolicies are the model policy atoms.
	ModelPolicies = []string{"declared", "command", "jev"}
	// FeatureNames are the features a project or a profile may enable.
	FeatureNames = []string{"probes", "notify", "contracts"}
	// Experiments are the experimental opt-ins.
	Experiments = []string{"codex"}
)

var (
	// ClaudeEfforts are the effort values Claude Code accepts (R-SESS-017); the atom aliases them.
	ClaudeEfforts = []string{"low", "medium", "high", "xhigh", "max"}
	// CodexEfforts are the reasoning efforts Codex accepts (R-MOD-033); the atom aliases them.
	CodexEfforts = []string{"minimal", "low", "medium", "high", "xhigh"}
	channelKeys  = map[string][]string{
		"slack":    {"token", "channel_id"},
		"telegram": {"token", "chat_id"},
		"github":   {"token", "repo"},
		"command":  {"command"},
	}
)

func oneOf(key, v string, allowed ...string) error {
	if slices.Contains(allowed, v) {
		return nil
	}
	return fmt.Errorf("%s = %q: accepted values are %s", key, v, strings.Join(allowed, ", "))
}

func (c *Config) validate() error {
	var errs []error
	for _, r := range [][2]string{{"ness", c.Ness}, {"project", c.Project}, {"base", c.Base}, {"gate", c.Gate}} {
		if r[1] == "" {
			errs = append(errs, fmt.Errorf("%s is required", r[0]))
		}
	}
	errs = append(errs,
		oneOf("client", c.Client, Clients...),
		oneOf("integrate.mode", c.Integrate.Mode, IntegrateModes...),
		c.validateModels(),
		oneOf("launch.permissions", c.Launch.Permissions, "skip", "client"))
	if c.Client == "codex" && !slices.Contains(c.Experimental, "codex") {
		errs = append(errs, errors.New(`client = "codex" needs experimental = ["codex"]`))
	}
	for _, f := range c.Features {
		errs = append(errs, oneOf("features", f, FeatureNames...))
	}
	for _, e := range c.Experimental {
		errs = append(errs, oneOf("experimental", e, Experiments...))
	}
	for _, name := range slices.Sorted(maps.Keys(c.Tiers)) {
		t := c.Tiers[name]
		errs = append(errs,
			tierSpec("tiers."+name+".claude", t.Claude, ClaudeEfforts),
			tierSpec("tiers."+name+".codex", t.Codex, CodexEfforts))
	}
	for _, name := range slices.Sorted(maps.Keys(c.Notify.Channels)) {
		errs = append(errs, c.Notify.Channels[name].validate("notify.channels."+name))
	}
	errs = append(errs, c.validateOperator(), c.validateNumbers(), c.validateGuards(), c.validateFormats())
	return errors.Join(errs...)
}

// validateGuards wants every [[guards]] entry to carry its required keys and valid regexps,
// naming guards[N] and the key (R-CFG-087).
func (c *Config) validateGuards() error {
	var errs []error
	for i, g := range c.Guards {
		at := func(key string) string { return fmt.Sprintf("guards[%d].%s", i, key) }
		required := [][2]string{
			{"tool", g.Tool},
			{"match", g.Match},
			{"message", g.Message},
			{"skill", g.Skill},
			{"section", g.Section},
		}
		for _, k := range required {
			if k[1] == "" {
				errs = append(errs, fmt.Errorf("%s is required", at(k[0])))
			}
		}
		for _, k := range [][2]string{{"tool", g.Tool}, {"match", g.Match}, {"content", g.Content}} {
			if _, err := regexp.Compile(k[1]); err != nil {
				errs = append(errs, fmt.Errorf("%s is not a valid regexp: %w", at(k[0]), err))
			}
		}
	}
	return errors.Join(errs...)
}

// formatOrder is the names under [contracts.formats], in the order the file writes them; a format
// that came from a profile default, not the file, goes last, sorted.
func formatOrder(meta toml.MetaData, formats map[string]Format) []string {
	var names []string
	for _, k := range meta.Keys() {
		if len(k) >= 3 && k[0] == "contracts" && k[1] == "formats" && !slices.Contains(names, k[2]) {
			names = append(names, k[2])
		}
	}
	for _, n := range slices.Sorted(maps.Keys(formats)) {
		if !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	return names
}

var formatName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// validateFormats wants every [contracts.formats.NAME] to have a lowercase name, valid globs
// without ** and a units command (R-CTR-047).
func (c *Config) validateFormats() error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(c.Contracts.Formats)) {
		f, at := c.Contracts.Formats[name], "contracts.formats."+name
		if !formatName.MatchString(name) {
			errs = append(errs, fmt.Errorf("%s: the name must match [a-z][a-z0-9-]*", at))
		}
		if len(f.Match) == 0 {
			errs = append(errs, fmt.Errorf("%s.match is required: globs of the files in the format", at))
		}
		for _, g := range f.Match {
			if _, err := path.Match(g, ""); err != nil {
				errs = append(errs, fmt.Errorf("%s.match: %q is not a valid glob", at, g))
			} else if strings.Contains(g, "**") {
				errs = append(errs, fmt.Errorf("%s.match: %q: ** is not supported, use path.Match globs", at, g))
			}
		}
		if strings.TrimSpace(f.Units) == "" {
			errs = append(errs, fmt.Errorf("%s.units is required: the command that prints the units", at))
		}
	}
	return errors.Join(errs...)
}

// validateOperator wants notify.operator to name a channel and the notify feature on (R-CFG-055).
func (c *Config) validateOperator() error {
	op := c.Notify.Operator
	if op == "" {
		return nil
	}
	ch, ok := c.Notify.Channels[op]
	if !ok {
		return oneOf("notify.operator", op, slices.Sorted(maps.Keys(c.Notify.Channels))...)
	}
	if ch.Kind == "github" {
		return errors.New("notify.operator: a github channel needs a thread; the operator line has none")
	}
	if !c.On("notify") {
		return errors.New(`notify.operator needs the notify feature: add "notify" to features`)
	}
	return nil
}

// ladderErrs refuses a ladder rung that repeats or is not in the tier table.
func (c *Config) ladderErrs() []error {
	var errs []error
	for i, r := range c.Models.Ladder {
		if slices.Contains(c.Models.Ladder[:i], r) {
			errs = append(errs, fmt.Errorf("models.ladder: %q appears twice", r))
		}
		if _, ok := c.Tiers[r]; !ok {
			errs = append(errs, fmt.Errorf("models.ladder: %q is not in the tier table", r))
		}
	}
	return errs
}

// validateModels wants the ladder's rungs in the tier table, base and ceiling on the ladder with
// the ceiling not below base, and a command for the command selector (R-MOD-005..007).
func (c *Config) validateModels() error {
	m := c.Models
	if m.Policy == "text" {
		return errors.New(`models.policy "text" was replaced by "declared" (ladder and kinds; see ness pick)`)
	}
	errs := append([]error{
		oneOf("models.policy", m.Policy, ModelPolicies...),
		oneOf("models.enforce", m.Enforce, "refuse", "log"),
	}, c.ladderErrs()...)
	base := slices.Index(m.Ladder, m.Base)
	if base < 0 {
		errs = append(errs, oneOf("models.base", m.Base, m.Ladder...))
	}
	if m.Ceiling != "" {
		if ceil := slices.Index(m.Ladder, m.Ceiling); ceil < 0 {
			errs = append(errs, oneOf("models.ceiling", m.Ceiling, m.Ladder...))
		} else if ceil < base {
			errs = append(errs, fmt.Errorf("models.ceiling = %q is below models.base = %q", m.Ceiling, m.Base))
		}
	}
	if m.Policy == "command" && strings.TrimSpace(m.Command) == "" {
		errs = append(errs, errors.New(`models.command is required with models.policy = "command"`))
	}
	return errors.Join(append(errs, m.Jev.validate())...)
}

// pinnedJev is a versioned Jev model id; aliases such as jev-latest are refused.
var pinnedJev = regexp.MustCompile(`^jev-\d+\.\d+\.\d+$`)

// validate wants a reference for the key (never a literal, R-CFG-081), a pinned model (an alias
// moves and changes answers), a timeout in (0, 10s] and an https (or loopback http) endpoint.
func (j Jev) validate() error {
	var errs []error
	if _, name, ref := SecretRef(j.Key); !ref {
		errs = append(errs, errors.New(`models.jev.key: a key is never written in ness.toml; `+
			`use key = "secret:NAME" (ness secret set NAME) or key = "env:NAME" (R-CFG-081)`))
	} else if err := CheckSecretName(name); err != nil {
		errs = append(errs, fmt.Errorf("models.jev.key: %w", err))
	}
	if !pinnedJev.MatchString(j.Model) {
		errs = append(errs, fmt.Errorf("models.jev.model = %q: name a pinned version such as jev-1.14.0", j.Model))
	}
	if j.Timeout.Duration <= 0 || j.Timeout.Duration > maxJevTimeout {
		errs = append(errs, fmt.Errorf("models.jev.timeout = %v: must be positive and at most %v "+
			"(the hook waits for it)", j.Timeout.Duration, maxJevTimeout))
	}
	if !safeEndpoint(j.Endpoint) {
		errs = append(errs, fmt.Errorf("models.jev.endpoint = %q: must be https:// (http:// only to "+
			"a loopback host); the key is sent to it", j.Endpoint))
	}
	return errors.Join(errs...)
}

// maxJevTimeout bounds models.jev.timeout inside the pre-tool hook's budget.
const maxJevTimeout = 10 * time.Second

// safeEndpoint reports an https URL, or an http one to a loopback host (127.0.0.0/8, ::1, localhost).
func safeEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	h := u.Hostname()
	ip := net.ParseIP(h)
	return u.Scheme == "http" && (h == "localhost" || (ip != nil && ip.IsLoopback()))
}

func (c *Config) validateNumbers() error {
	var errs []error
	if c.Models.CommandTimeout.Duration <= 0 {
		errs = append(errs, errors.New("models.command_timeout must be positive"))
	}
	if c.Models.RouterThreshold < 0 || c.Models.RouterThreshold > 1 {
		errs = append(errs, errors.New("models.router_threshold must be between 0 and 1"))
	}
	if c.Launch.MaxChildren < 0 {
		errs = append(errs, errors.New("launch.max_children must not be negative"))
	}
	if c.Launch.ReadyTimeout.Duration <= 0 {
		errs = append(errs, errors.New("launch.ready_timeout must be positive"))
	}
	if c.Worker.StallMinutes <= 0 || c.Worker.QuietSeconds < 0 {
		errs = append(errs, errors.New("worker.stall_minutes must be positive and worker.quiet_seconds not negative"))
	}
	return errors.Join(errs...)
}

func tierSpec(key, spec string, efforts []string) error {
	model, effort, ok := strings.Cut(spec, "/")
	if !ok || model == "" {
		return fmt.Errorf("%s = %q: want \"<model>/<effort>\"", key, spec)
	}
	return oneOf(key+" effort", effort, efforts...)
}

func (ch Channel) validate(key string) error {
	need, ok := channelKeys[ch.Kind]
	if !ok {
		return oneOf(key+".kind", ch.Kind, slices.Sorted(maps.Keys(channelKeys))...)
	}
	have := ch.values()
	var errs []error
	for _, k := range need {
		if *have[k] == "" {
			errs = append(errs, fmt.Errorf("%s.%s is required for kind %s", key, k, ch.Kind))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(have)) {
		_, name, ref := SecretRef(*have[k])
		switch {
		case ref:
			if err := CheckSecretName(name); err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: %w", key, k, err))
			}
		case k == "token" && *have[k] != "":
			errs = append(errs, fmt.Errorf(`%s.token: a token is never written in ness.toml; `+
				`use token = "secret:NAME" (ness secret set NAME) or token = "env:NAME" (R-CFG-081)`, key))
		}
	}
	return errors.Join(errs...)
}

// values points at every key of the channel but kind.
func (ch *Channel) values() map[string]*string {
	return map[string]*string{
		"token": &ch.Token, "channel_id": &ch.ChannelID, "chat_id": &ch.ChatID,
		"repo": &ch.Repo, "command": &ch.Command,
	}
}

// Resolve returns the channel with every set key passed through fn, which turns a reference
// into its value (R-NTF-060); the errors of all keys are joined, each naming its key.
func (ch Channel) Resolve(fn func(value string) (string, error)) (Channel, error) {
	var errs []error
	vals := ch.values()
	for _, k := range slices.Sorted(maps.Keys(vals)) {
		p := vals[k]
		if *p == "" {
			continue
		}
		v, err := fn(*p)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
		}
		*p = v
	}
	return ch, errors.Join(errs...)
}

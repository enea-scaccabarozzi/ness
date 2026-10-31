package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/atom/declared"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/ladder"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

const (
	agentsPath   = "AGENTS.md"
	settingsPath = ".claude/settings.json"
	skillsDir    = ".agents/skills/"
	coreOwner    = "core"
)

// Entry is one desired target: a whole file, a seed, a block, a JSON key or a link.
type Entry struct {
	ID      string
	Path    string
	Kind    Kind
	Owner   string
	Name    string // block name or JSON key; empty for whole files and links
	Content []byte // file bytes, block content, compact JSON value or link target
}

var (
	slotRe = regexp.MustCompile(`(?m)^<!-- ness:slot (?:list|single) [a-z0-9-]+ -->\n?`)
	// HookEvents are the client events ness handles, as the client names them and as `ness hook`
	// takes them (R-CFG-070): sync wires them, the hook command accepts them.
	HookEvents = []HookEvent{{"SessionStart", "session-start"}, {"PreToolUse", "pre-tool"}}
)

// HookEvent is one client event: Event as the client names it, Name as `ness hook` takes it.
type HookEvent struct{ Event, Name string }

// Render turns the resolved profiles and the final config into the desired targets, sorted by
// path. Within a path, entries keep render order, so AGENTS.md blocks come core first, then
// each profile in configured order (R-SYNC-001). It reads the repository only to decide how
// CLAUDE.md is managed.
func Render(res *profile.Resolved, cfg *config.Config, root *os.Root, features fs.FS) ([]Entry, error) {
	files, err := fillSlots(res)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(files)+len(res.Blocks)+12)
	for _, f := range files {
		kind := KindFile
		if f.Seed {
			kind = KindSeed
		}
		out = append(out, Entry{Path: f.Path, Kind: kind, Owner: f.Owner, Content: f.Content})
	}
	for _, b := range res.Blocks {
		out = append(out, Entry{Path: agentsPath, Kind: KindBlock, Owner: b.Owner, Name: b.Name, Content: b.Text})
	}
	core, err := coreEntries(cfg, root)
	if err != nil {
		return nil, err
	}
	out = append(out, core...)
	feats, err := featureEntries(cfg, features)
	if err != nil {
		return nil, err
	}
	out = append(out, feats...)
	for i := range out {
		out[i].ID = entryID(out[i])
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	return out, checkTargets(out)
}

// featureEntries renders each enabled feature's skill text (design §2); an unknown feature has
// no text and renders nothing.
func featureEntries(cfg *config.Config, features fs.FS) ([]Entry, error) {
	out := make([]Entry, 0, len(cfg.Features))
	for _, f := range cfg.Features {
		text, err := fs.ReadFile(features, f+"/SKILL.md")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Path: skillsDir + "ness-" + f + "/SKILL.md", Kind: KindFile, Owner: "feature/" + f, Content: text,
		})
		extra, err := featureSkills(features, f)
		if err != nil {
			return nil, err
		}
		out = append(out, extra...)
	}
	return out, nil
}

// featureSkills renders the further skills a feature ships under <feature>/skills/, each file at
// the same path under the skills directory.
func featureSkills(features fs.FS, f string) ([]Entry, error) {
	var out []Entry
	err := fs.WalkDir(features, f+"/skills", func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.SkipAll
		}
		if err != nil || d.IsDir() {
			return err
		}
		text, err := fs.ReadFile(features, p)
		if err != nil {
			return err
		}
		out = append(out, Entry{
			Path: skillsDir + strings.TrimPrefix(p, f+"/skills/"), Kind: KindFile, Owner: "feature/" + f, Content: text,
		})
		return nil
	})
	return out, err
}

func entryID(e Entry) string {
	switch e.Kind {
	case KindBlock:
		return e.Path + "#" + e.Name
	case KindKey:
		return e.Path + ":" + e.Name
	default:
		return e.Path
	}
}

// checkTargets refuses two entries for one id, and a whole file sharing a path with parts.
func checkTargets(entries []Entry) error {
	var errs []error
	whole := map[string]string{}
	parts := map[string]string{}
	seen := map[string]string{}
	for _, e := range entries {
		if prev, ok := seen[e.ID]; ok {
			errs = append(errs, fmt.Errorf("%s is rendered by both %s and %s", e.ID, prev, e.Owner))
		}
		seen[e.ID] = e.Owner
		if e.Kind == KindBlock || e.Kind == KindKey {
			parts[e.Path] = e.Owner
		} else {
			whole[e.Path] = e.Owner
		}
	}
	for _, p := range slices.Sorted(maps.Keys(whole)) {
		if owner, ok := parts[p]; ok {
			errs = append(errs, fmt.Errorf("%s is owned whole by %s and edited in part by %s", p, whole[p], owner))
		}
	}
	return errors.Join(errs...)
}

// fillSlots inserts slot contributions above their markers, then drops every marker line.
func fillSlots(res *profile.Resolved) ([]profile.OwnedFile, error) {
	files := slices.Clone(res.Files)
	index := map[string]int{}
	for i, f := range files {
		index[f.Path] = i
	}
	errs := make([]error, 0, len(res.Lists)+len(res.Singles))
	for _, key := range slices.Sorted(maps.Keys(res.Lists)) {
		errs = append(errs, fill(files, index, key, "list", res.Lists[key]))
	}
	for _, key := range slices.Sorted(maps.Keys(res.Singles)) {
		errs = append(errs, fill(files, index, key, "single", []profile.Part{res.Singles[key]}))
	}
	for i := range files {
		files[i].Content = slotRe.ReplaceAll(files[i].Content, nil)
	}
	return files, errors.Join(errs...)
}

func fill(files []profile.OwnedFile, index map[string]int, key, kind string, parts []profile.Part) error {
	target, name, _ := strings.Cut(key, "#")
	path := target
	if _, ok := index[path]; !ok {
		path = skillsDir + target + "/SKILL.md"
	}
	i, ok := index[path]
	if !ok {
		return fmt.Errorf("profile %s fills slot %s, but no rendered skill or file is named %s",
			parts[0].Owner, key, target)
	}
	marker := []byte(fmt.Sprintf("<!-- ness:slot %s %s -->", kind, name))
	c := files[i].Content
	at := bytes.Index(c, marker)
	if at < 0 {
		return fmt.Errorf("profile %s fills %s slot %s, which %s does not have", parts[0].Owner, kind, key, path)
	}
	var text []byte
	for _, p := range parts {
		text = append(text, normInner(p.Text)...)
	}
	files[i].Content = slices.Concat(c[:at], text, c[at:])
	return nil
}

func coreEntries(cfg *config.Config, root *os.Root) ([]Entry, error) {
	model, err := rawJSON(searchRung(cfg, "claude").Model)
	if err != nil {
		return nil, err
	}
	out := []Entry{
		{
			Path: ".gitignore", Kind: KindBlock, Owner: coreOwner, Name: "core/ignore",
			Content: []byte(".worktrees/\n*.ness-new\n"),
		},
		{Path: ".claude/skills", Kind: KindLink, Owner: coreOwner, Content: []byte("../.agents/skills")},
		{Path: settingsPath, Kind: KindKey, Owner: coreOwner, Name: "autoCompactWindow", Content: []byte("265000")},
		{Path: settingsPath, Kind: KindKey, Owner: coreOwner, Name: "env.CLAUDE_CODE_SUBAGENT_MODEL", Content: model},
		claudeMD(root),
	}
	for _, ev := range HookEvents {
		cmd := "command -v ness >/dev/null || exit 0; ness hook " + ev.Name
		entry, err := rawJSON(map[string]any{"hooks": []map[string]string{{"type": "command", "command": cmd}}})
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Path: settingsPath, Kind: KindKey, Owner: coreOwner,
			Name: hookKeyFor(ev.Event, "ness hook "+ev.Name), Content: entry,
		})
	}
	out = append(out, subagents(cfg)...)
	if cfg.Client == "codex" {
		out = append(out, codexEntries(cfg)...)
	}
	return out, nil
}

// codexEntries are the keys sync owns in .codex/config.toml for the codex client: the
// compaction limit (R-SYNC-086) and the [agents] subagent defaults from the search kind's rung
// (R-MOD-050), never a named agent (R-SYNC-068). The key names are Codex 0.157's
// (default_subagent_model, default_subagent_reasoning_effort), not the plan's agents.model.
func codexEntries(cfg *config.Config) []Entry {
	r := searchRung(cfg, "codex")
	model, effort := r.Model, r.Effort
	const p = ".codex/config.toml"
	key := func(name, v string) Entry {
		return Entry{Path: p, Kind: KindKey, Owner: coreOwner, Name: name, Content: []byte(v)}
	}
	return []Entry{
		key("model_auto_compact_token_limit", "235000"),
		key("agents.default_subagent_model", strconv.Quote(model)),
		key("agents.default_subagent_reasoning_effort", strconv.Quote(effort)),
	}
}

// claudeMD links CLAUDE.md to AGENTS.md unless it is the user's file, which gets an import
// block instead (R-SYNC-062, R-SYNC-063).
func claudeMD(root *os.Root) Entry {
	if info, err := root.Lstat("CLAUDE.md"); err == nil && info.Mode().IsRegular() {
		return Entry{
			Path: "CLAUDE.md", Kind: KindBlock, Owner: coreOwner, Name: "core/import",
			Content: []byte("@AGENTS.md\n"),
		}
	}
	return Entry{Path: "CLAUDE.md", Kind: KindLink, Owner: coreOwner, Content: []byte(agentsPath)}
}

// searchRung is the client's rung for the search kind: the default subagent model (R-MOD-020).
func searchRung(cfg *config.Config, client string) ladder.Rung {
	r, _ := ladder.New(cfg, client).At(cfg.Models.Kinds["search"])
	return r
}

// subagents generates the named subagents, each on its kind's rung from base (R-MOD-020).
func subagents(cfg *config.Config) []Entry {
	agents := []struct{ name, desc, body string }{
		{
			"ness-sweep",
			"Read-only search. Finds files and facts across the repository and returns paths with one-line findings.",
			"Search for what the caller asks and do not edit files. Write long results to a file, " +
				"then return its path and a summary of at most 8,000 characters.",
		},
		{
			"ness-review",
			"Reviews a change against its plan and the project's rules, and reports findings ranked by severity.",
			"Review the change you are given. Report each finding with its file and line, ranked by severity. " +
				"Check intent too: does the change do, product-wise, what the brief and the operator's words ask? " +
				"Flag every product call (what users see, what the product does or promises) made without an " +
				"operator ruling, above all one a test, a contract or a frozen file locks in.",
		},
		{
			"ness-verify",
			"Adversarially verifies one claim or finding and returns a verdict with evidence.",
			"Try to prove the claim wrong. Return CONFIRMED or REJECTED with the evidence that decides it.",
		},
		{
			"ness-design",
			"Designs an interface or plan for a hard problem and returns the options with a recommendation.",
			"Lay out the viable designs, their costs and the one you recommend, with the reason.",
		},
	}
	l, named := ladder.New(cfg, "claude"), declared.NamedKinds()
	out := make([]Entry, 0, len(agents))
	for _, a := range agents {
		r, _ := l.At(cfg.Models.Kinds[named[a.name]])
		content := fmt.Sprintf("---\nname: %s\ndescription: %s\nmodel: %s\neffort: %s\n---\n\n%s\n",
			a.name, a.desc, r.Model, r.Effort, a.body)
		out = append(out, Entry{
			Path: ".claude/agents/" + a.name + ".md", Kind: KindFile, Owner: coreOwner, Content: []byte(content),
		})
	}
	return out
}

// rawJSON encodes v compactly without HTML escaping, so "&&" and ">" stay readable.
func rawJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

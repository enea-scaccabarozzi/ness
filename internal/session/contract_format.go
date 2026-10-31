package session

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

const (
	// ExtractTimeout is how long a format's units command may run on one file (R-CTR-048).
	ExtractTimeout = 30 * time.Second
	maxUnitID      = 1024
	maxUnitsOutput = 8 << 20
)

var errOutput = errors.New("output exceeds 8 MiB")

// capWriter keeps at most max bytes; past that it fails (strict) or drops the rest.
type capWriter struct {
	buf    bytes.Buffer
	max    int
	strict bool
	over   bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) <= w.max {
		return w.buf.Write(p)
	}
	w.over = true
	if w.strict {
		return 0, errOutput
	}
	return len(p), nil
}

// ExtractUnits runs f.Units for the file rel whose text is given (R-CTR-048): sh -c with the path
// appended as the last positional argument (a label, not a file to open), in dir, the text on
// stdin, a minimal environment. It reads one {"id","body"} JSON object per line and returns each id
// with the digest of its body. A command that fails, prints more than 8 MiB, a line that is not
// exactly those two string keys, an empty, tabbed or over-1-KiB id, or an id twice fails closed.
func ExtractUnits(ctx context.Context, dir string, f config.Format, rel, text string) (map[string]string, error) {
	out, err := runUnits(ctx, dir, f, rel, text)
	if err != nil {
		return nil, err
	}
	units := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		id, body, perr := parseUnit(line)
		if perr != nil {
			return nil, fmt.Errorf("%s: invalid units output: %w", rel, perr)
		}
		if _, dup := units[id]; dup {
			return nil, fmt.Errorf("%s: units output repeats id %q", rel, id)
		}
		units[id] = sum(body)
	}
	return units, nil
}

// parseUnit reads one strict {"id","body"} line: exactly those keys, both strings.
func parseUnit(line string) (id, body string, err error) {
	var raw map[string]json.RawMessage
	if jerr := json.Unmarshal([]byte(line), &raw); jerr != nil {
		return "", "", jerr
	}
	idRaw, body2 := raw["id"], raw["body"]
	if len(raw) != 2 || !bytes.HasPrefix(idRaw, []byte(`"`)) || !bytes.HasPrefix(body2, []byte(`"`)) {
		return "", "", errors.New(`a line needs exactly a string "id" and a string "body"`)
	}
	if err = json.Unmarshal(idRaw, &id); err == nil {
		err = json.Unmarshal(body2, &body)
	}
	switch {
	case err != nil:
		return "", "", err
	case id == "" || len(id) > maxUnitID || strings.ContainsAny(id, "\r\n\t"):
		return "", "", fmt.Errorf("an id must be 1 to %d bytes on one line without tabs", maxUnitID)
	}
	return id, body, nil
}

// runUnits runs the command and returns its stdout; a failure carries the start of its stderr.
// ponytail: the kill takes the process group; a command that setsids away outlives the timeout.
func runUnits(ctx context.Context, dir string, f config.Format, rel, text string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", f.Units+` "$@"`, "sh", rel) //nolint:gosec // the configured command
	cmd.Dir, cmd.Stdin = dir, strings.NewReader(text)
	// Nothing of the caller's environment: what it writes lands in dir, which is a throwaway.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=" + os.Getenv("LANG"), "HOME=" + dir, "TMPDIR=" + dir}
	out, errOut := &capWriter{max: maxUnitsOutput, strict: true}, &capWriter{max: 200}
	cmd.Stdout, cmd.Stderr = out, errOut
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// The group, not just sh: a child holding the pipes must die with it.
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil || out.over {
		why := errOutput.Error()
		if !out.over {
			why = err.Error()
			if ctx.Err() != nil {
				why = "timed out"
			} else if msg := strings.TrimSpace(errOut.buf.String()); msg != "" {
				why += ": " + msg
			}
		}
		return "", fmt.Errorf("%s: units command %q failed: %s", rel, f.Units, why)
	}
	return out.buf.String(), nil
}

// Extractor runs units commands the way R-CTR-048 and R-CTR-052 say: in an export of the merge-base
// tree, so the judge and everything it loads from the repo are the base's, whatever the branch did.
// The export is made on first use and removed by Close.
type Extractor struct {
	g          Git
	root, base string
	tmp, dir   string
	probed     map[string]bool
}

// NewExtractor is an Extractor for the repo at root, whose merge-base with base it exports.
func NewExtractor(g Git, root, base string) *Extractor {
	return &Extractor{g: g, root: root, base: base, probed: map[string]bool{}}
}

// Close removes the export.
func (x *Extractor) Close() error {
	if x == nil || x.tmp == "" {
		return nil
	}
	return os.RemoveAll(x.tmp)
}

// export unpacks the merge-base tree once. git archive honors export-ignore attributes: a file the
// repo marks that way is not in the export, which is the repo's own choice.
func (x *Extractor) export() (string, error) {
	if x.dir != "" {
		return x.dir, nil
	}
	mb, err := MergeBase(x.g, x.root, x.base)
	if err != nil {
		return "", err
	}
	if x.tmp, err = os.MkdirTemp("", "ness-units-"); err != nil {
		return "", err
	}
	tarball := filepath.Join(x.tmp, "base.tar")
	if _, err = x.g.Exec(x.root, "git", "archive", "--format=tar", "--output="+tarball, mb); err != nil {
		return "", fmt.Errorf("git archive %s: %w", mb, err)
	}
	dir := filepath.Join(x.tmp, "tree")
	if err = untar(tarball, dir); err != nil {
		return "", fmt.Errorf("unpack %s: %w", mb, err)
	}
	x.dir = dir
	return dir, nil
}

// untar unpacks directories, regular files (keeping the exec bit) and symlinks of a git archive.
func untar(tarball, dst string) error {
	f, err := os.Open(tarball) //nolint:gosec // our own temp file
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read only
	for tr := tar.NewReader(f); ; {
		h, nerr := tr.Next()
		if errors.Is(nerr, io.EOF) {
			return nil
		} else if nerr != nil {
			return nerr
		}
		if uerr := unpackEntry(tr, h, dst); uerr != nil {
			return uerr
		}
	}
}

// unpackEntry writes one archive entry under dst; the pax header and other types are skipped.
func unpackEntry(r io.Reader, h *tar.Header, dst string) error {
	if h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeSymlink {
		return nil
	}
	if !filepath.IsLocal(h.Name) {
		return fmt.Errorf("unsafe path %q", h.Name)
	}
	p := filepath.Join(dst, h.Name) //nolint:gosec // IsLocal above
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	switch h.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(p, 0o700)
	case tar.TypeSymlink:
		return os.Symlink(h.Linkname, p)
	}
	return writeExported(p, r, h.Mode&0o111 != 0)
}

func writeExported(p string, r io.Reader, executable bool) error {
	mode := os.FileMode(0o600)
	if executable {
		mode = 0o700
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode) //nolint:gosec // under our temp dir
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r) //nolint:gosec // a git archive of our own repo
	return errors.Join(err, f.Close())
}

// FromRepo reports whether the first word of f.Units is a file of the merge-base: when it is not
// (a tool on PATH, a quoted or absolute path), the extractor comes from the environment and the
// branch cannot be stopped from changing it (R-CTR-052).
func (x *Extractor) FromRepo(f config.Format) bool {
	words := strings.Fields(f.Units)
	dir, err := x.export()
	if err != nil || len(words) == 0 || !filepath.IsLocal(words[0]) {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, words[0]))
	return err == nil && st.Mode().IsRegular()
}

// Units is the units of the file rel, whose text is given, under format name (R-CTR-048). The file
// itself is not in the export, so a command that reads its argument instead of stdin fails; one
// that prints units from empty stdin is refused too, once per format.
func (x *Extractor) Units(name string, f config.Format, rel, text string) (map[string]string, error) {
	dir, err := x.export()
	if err != nil {
		return nil, err
	}
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("%s: not a path inside the repo", rel)
	}
	if err = os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !x.probed[name] {
		x.probed[name] = true
		if got, perr := extract(dir, f, rel, ""); perr == nil && len(got) > 0 {
			return nil, fmt.Errorf("%s: units command %q printed units from empty input: "+
				"it must read the file from stdin", rel, f.Units)
		}
	}
	return extract(dir, f, rel, text)
}

// extract is ExtractUnits under ExtractTimeout.
func extract(dir string, f config.Format, rel, text string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ExtractTimeout)
	defer cancel()
	return ExtractUnits(ctx, dir, f, rel, text)
}

// RecordUnits locks path unit by unit under the format name (R-CTR-049), from its text in the
// worktree at root. The units are kept as the lock's scenario lines, bound to the "fmt:" entry by
// the same digest. A file with no units is refused: there is nothing to lock.
func (l *Lock) RecordUnits(x *Extractor, root, path, name string, f config.Format) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // under the repo
	if err != nil {
		return err
	}
	units, err := x.Units(name, f, path, string(data))
	if err != nil {
		return err
	}
	if len(units) == 0 {
		return fmt.Errorf("no units to lock in %s: the units command printed none", path)
	}
	l.init()
	l.Files[path], l.Scenarios[path] = FormatTag+name+":"+scenarioDigest(units), units
	return nil
}

// formatEntry is the project-wide frozen state of a format file, its units read from its text at
// base (untrimmed, as the worktree file is). A file with no units at base is still a format lock,
// with none locked, so that adding units passes. A path absent at base (CheckContracts reports it)
// stays whole-file; a units command that fails is an error.
func formatEntry(g Git, root, base, p, name string, f config.Format, x *Extractor) (Frozen, error) {
	raw, err := g.Exec(root, "git", "cat-file", "blob", base+":"+p)
	if err != nil {
		return Frozen{Path: p}, nil //nolint:nilerr // absent at base: whole-file, check reports it
	}
	units, err := x.Units(name, f, p, string(raw))
	if err != nil {
		return Frozen{}, err
	}
	return Frozen{Path: p, Sha: FormatTag + name + ":" + scenarioDigest(units), Scenarios: units}, nil
}

// formatDiff reports how the file f.Path, whose current text is given, breaks its format lock:
// the format is gone or another one matches now, the units command fails, or a locked unit is
// changed or removed. "" when it holds; new units are free (R-CTR-050, R-CTR-051).
func formatDiff(x *Extractor, f Frozen, text string, c config.Contracts) string {
	locked, _, _ := strings.Cut(strings.TrimPrefix(f.Sha, FormatTag), ":")
	name, fm, ok := c.FormatOf(f.Path)
	switch {
	case !ok:
		return fmt.Sprintf("locked under format %q, which no longer matches this file", locked)
	case name != locked:
		return fmt.Sprintf("locked under format %q but now matches format %q", locked, name)
	}
	now, err := x.Units(name, fm, f.Path, text)
	if err != nil {
		return err.Error()
	}
	var bad []string
	for _, id := range slices.Sorted(maps.Keys(f.Scenarios)) {
		switch got, ok := now[id]; {
		case !ok:
			bad = append(bad, fmt.Sprintf("unit %q removed", id))
		case got != f.Scenarios[id]:
			bad = append(bad, fmt.Sprintf("unit %q changed", id))
		}
	}
	return strings.Join(bad, "; ")
}

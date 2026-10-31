package sync

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// TOML keys are edited line by line (OP-SYNC-10): a dotted key "t.k" is the line "k = v" under
// the header "[t]"; a plain key sits before the first header. Values are one-line TOML
// literals. The result is parsed to prove it is still TOML. Arrays of tables and dotted keys
// inside a line are not handled: ness only writes the keys of R-SYNC-068.
// ponytail: headers must be exactly "[t]"; a quoted or spaced header is treated as another table.

var tomlKeyLine = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+)\s*=`)

func splitKey(key string) (table, name string) {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

func checkTOML(data []byte) error {
	var m map[string]any
	_, err := toml.Decode(string(data), &m)
	return err
}

// header returns the table named by a "[t]" line. A "[[t]]" line also ends the current table;
// its name is "[t]", which no key's table can equal. A trailing comment ("[t] # note") is not
// part of the header.
func header(l string) (string, bool) {
	t, _, _ := strings.Cut(l, "#")
	t = strings.TrimSpace(t)
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") {
		return "", false
	}
	return strings.TrimSpace(t[1 : len(t)-1]), true
}

// splitValue splits the text after "=" into the value and its trailing comment ("" if none).
// ponytail: only a leading quoted string is skipped over; a "#" inside an array of strings is
// read as a comment start (the re-parse in tomlSet refuses the result if that breaks the file).
func splitValue(rest string) (val, comment string) {
	rest = strings.TrimSpace(rest)
	from := 0
	if q := rest[:min(1, len(rest))]; q == `"` || q == "'" {
		for i := 1; i < len(rest); i++ {
			if rest[i] == '\\' && q == `"` {
				i++
			} else if rest[i:i+1] == q {
				from = i
				break
			}
		}
	}
	if j := strings.Index(rest[from:], "#"); j >= 0 {
		return strings.TrimSpace(rest[:from+j]), rest[from+j:]
	}
	return rest, ""
}

// inTable reports whether table (dotted path, "" for the root) holds name, by decoding data.
func inTable(data []byte, table, name string) bool {
	var m map[string]any
	if _, err := toml.Decode(string(data), &m); err != nil {
		return false
	}
	if table != "" {
		for _, part := range strings.Split(table, ".") {
			next, ok := m[part].(map[string]any)
			if !ok {
				return false
			}
			m = next
		}
	}
	_, ok := m[name]
	return ok
}

// locate returns the line of key within its table (-1 if absent), the line where a new key of
// that table goes (the next header, or the end), and whether the table exists.
func locate(lines []string, table, name string) (line, end int, tableFound bool) {
	cur := ""
	tableFound = table == ""
	end = len(lines)
	for i, l := range lines {
		if h, ok := header(l); ok {
			if cur == table && tableFound {
				return -1, i, true
			}
			cur = h
			tableFound = tableFound || cur == table
			continue
		}
		if m := tomlKeyLine.FindStringSubmatch(l); cur == table && m != nil && m[1] == name {
			return i, i, true
		}
	}
	return -1, end, tableFound
}

func tomlGet(data []byte, key string) (val []byte, found bool, err error) {
	if err := checkTOML(data); err != nil {
		return nil, false, err
	}
	table, name := splitKey(key)
	lines := strings.Split(string(data), "\n")
	i, _, _ := locate(lines, table, name)
	if i < 0 {
		return nil, false, nil
	}
	_, rest, _ := strings.Cut(lines[i], "=")
	v, _ := splitValue(rest)
	return []byte(v), true, nil
}

// insertAt puts line before the next header, after the table's last non-blank line.
func insertAt(lines []string, end int, line string) []string {
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return append(lines[:end], append([]string{line}, lines[end:]...)...)
}

// withComment appends old's trailing comment, if any, to line (the user's note survives).
func withComment(line, old string) string {
	_, rest, _ := strings.Cut(old, "=")
	if _, c := splitValue(rest); c != "" {
		return line + " " + strings.TrimRight(c, "\r")
	}
	return line
}

// verifySet proves the edited file is TOML and holds the key in its table.
func verifySet(out []byte, key, table, name string) error {
	if err := checkTOML(out); err != nil {
		return fmt.Errorf("setting %s: %w", key, err)
	}
	if !inTable(out, table, name) {
		return fmt.Errorf("setting %s: cannot place the key in table %q", key, table)
	}
	return nil
}

func tomlSet(data []byte, key string, v []byte) ([]byte, error) {
	if err := checkTOML(data); err != nil {
		return nil, err
	}
	table, name := splitKey(key)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	eol := ""
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r"
	}
	i, end, found := locate(lines, table, name)
	line := name + " = " + string(v)
	switch {
	case i >= 0:
		lines[i] = withComment(line, lines[i]) + eol
	case found:
		lines = insertAt(lines, end, line+eol)
	default:
		if len(lines) > 0 {
			lines = append(lines, eol)
		}
		lines = append(lines, "["+table+"]"+eol, line+eol)
	}
	out := []byte(strings.Join(lines, "\n") + "\n")
	if err := verifySet(out, key, table, name); err != nil {
		return nil, err
	}
	return out, nil
}

func tomlDelete(data []byte, key string) ([]byte, error) {
	if err := checkTOML(data); err != nil {
		return nil, err
	}
	table, name := splitKey(key)
	lines := strings.Split(string(data), "\n")
	i, _, _ := locate(lines, table, name)
	if i < 0 {
		return data, nil
	}
	out := []byte(strings.Join(append(lines[:i], lines[i+1:]...), "\n"))
	if err := checkTOML(out); err != nil {
		return nil, fmt.Errorf("deleting %s: %w", key, err)
	}
	return out, nil
}

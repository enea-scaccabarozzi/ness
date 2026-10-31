package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const hookPrefix = "hooks."

var errNotObject = errors.New("not a valid JSON object")

// hookKeyFor names the hook entry of event whose command contains match.
func hookKeyFor(event, match string) string { return hookPrefix + event + "[" + match + "]" }

func hookKey(key string) (event, match string, ok bool) {
	rest, ok := strings.CutPrefix(key, hookPrefix)
	if !ok || !strings.HasSuffix(rest, "]") {
		return "", "", false
	}
	return strings.Cut(strings.TrimSuffix(rest, "]"), "[")
}

// hookIndex finds the entry recognized by its command, not its position (R-SYNC-053).
func hookIndex(data []byte, event, match string) int {
	idx := -1
	arr := gjson.GetBytes(data, hookPrefix+event)
	if !arr.IsArray() {
		return idx
	}
	arr.ForEach(func(k, v gjson.Result) bool {
		for _, h := range v.Get("hooks").Array() {
			if strings.Contains(h.Get("command").String(), match) {
				idx = int(k.Int())
				return false
			}
		}
		return true
	})
	return idx
}

// jsonPath resolves key to a gjson/sjson path; found is false for an absent hook entry, whose
// path then appends to the event's array.
func jsonPath(data []byte, key string) (path string, found bool) {
	event, match, ok := hookKey(key)
	if !ok {
		return key, true
	}
	if i := hookIndex(data, event, match); i >= 0 {
		return fmt.Sprintf("%s%s.%d", hookPrefix, event, i), true
	}
	return hookPrefix + event + ".-1", false
}

func checkObject(data []byte) error {
	t := bytes.TrimSpace(data)
	if len(t) == 0 {
		return nil
	}
	if t[0] != '{' || !gjson.ValidBytes(t) {
		return errNotObject
	}
	return nil
}

// jsonGet returns the compact JSON value at key and whether it exists.
func jsonGet(data []byte, key string) (val []byte, ok bool, err error) {
	if err := checkObject(data); err != nil {
		return nil, false, err
	}
	path, found := jsonPath(data, key)
	if !found {
		return nil, false, nil
	}
	r := gjson.GetBytes(data, path)
	if !r.Exists() {
		return nil, false, nil
	}
	return compact([]byte(r.Raw)), true, nil
}

// jsonSet sets key to the raw JSON value, leaving every other byte as it was (R-SYNC-050). On
// error it returns data unchanged.
func jsonSet(data []byte, key string, value []byte) ([]byte, error) {
	if err := checkObject(data); err != nil {
		return data, err
	}
	if err := checkAncestors(data, key); err != nil {
		return data, err
	}
	in := data
	if len(bytes.TrimSpace(in)) == 0 {
		in = []byte("{}")
	}
	path, _ := jsonPath(in, key)
	out, err := sjson.SetRawBytes(slices.Clone(in), path, value)
	if err != nil {
		return data, err
	}
	return out, nil
}

// checkAncestors refuses a key whose parent holds a user's non-object value, which setting would
// overwrite (D4); a hook's event must be absent or an array. The engine then leaves the file
// alone, as it does an unparsable one.
func checkAncestors(data []byte, key string) error {
	segs := strings.Split(key, ".")
	event, _, hook := hookKey(key)
	if hook {
		segs = []string{strings.TrimSuffix(hookPrefix, "."), event}
		if r := gjson.GetBytes(data, hookPrefix+event); r.Exists() && !r.IsArray() {
			return fmt.Errorf("%s%s is not an array; ness will not overwrite it", hookPrefix, event)
		}
	}
	for i := 1; i < len(segs); i++ {
		p := strings.Join(segs[:i], ".")
		if r := gjson.GetBytes(data, p); r.Exists() && !r.IsObject() {
			return fmt.Errorf("%s is not an object; ness will not overwrite it to set %s", p, key)
		}
	}
	return nil
}

// jsonDelete removes key if present.
func jsonDelete(data []byte, key string) ([]byte, error) {
	if err := checkObject(data); err != nil {
		return nil, err
	}
	path, found := jsonPath(data, key)
	if !found || !gjson.GetBytes(data, path).Exists() {
		return data, nil
	}
	return sjson.DeleteBytes(slices.Clone(data), path)
}

func compact(raw []byte) []byte {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return raw
	}
	return b.Bytes()
}

// indentJSON pretty-prints a file ness created from nothing.
func indentJSON(data []byte) []byte {
	var b bytes.Buffer
	if err := json.Indent(&b, data, "", "  "); err != nil {
		return data
	}
	b.WriteByte('\n')
	return b.Bytes()
}

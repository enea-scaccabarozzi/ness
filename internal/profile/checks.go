package profile

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// kindArgs maps each built-in check kind to its string arguments, all required. section-matches
// additionally takes the require list.
var kindArgs = map[string][]string{
	"section-matches":          {"file", "heading"},
	"branch-forbids":           {"path", "pattern"},
	"match-last":               {"file", "pattern", "against_file", "against_pattern"},
	"checkpoint-since-landing": nil,
}

// argRegexp and argFile are the kind arguments that must compile as a regexp or be a clean
// repository-relative file.
var (
	argRegexp = []string{"path", "pattern", "against_pattern"}
	argFile   = []string{"file", "against_file"}
	// checkFields are the keys common to every entry; the rest are kind arguments or require.
	checkFields = []string{
		"id", "on", "for", "skill", "section", "message", "kind", "run", "timeout", "require",
		"size",
	}
)

// parseChecks validates the raw [[checks]] tables (R-PROF-045). Each error names the field as
// checks[N].<field>; the caller prefixes the file.
func parseChecks(raw []map[string]any) ([]Check, error) {
	var errs []error
	out := make([]Check, 0, len(raw))
	ids := map[string]bool{}
	for i, m := range raw {
		c, err := parseCheck(i, m)
		if err == nil && ids[c.ID] {
			err = fmt.Errorf("checks[%d].id: duplicate id %q", i, c.ID)
		}
		ids[c.ID] = true
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, c)
	}
	return out, errors.Join(errs...)
}

// checkDecoder reads typed fields of one table, collecting errors under checks[i].
type checkDecoder struct {
	i    int
	m    map[string]any
	errs []error
}

func (d *checkDecoder) fail(field, format string, a ...any) {
	d.errs = append(d.errs, fmt.Errorf("checks[%d].%s: %s", d.i, field, fmt.Sprintf(format, a...)))
}

// str returns the string at key; a wrong type is an error, an absent key is "".
func (d *checkDecoder) str(key string) string {
	v, ok := d.m[key]
	if !ok {
		return ""
	}
	s, isStr := v.(string)
	if !isStr {
		d.fail(key, "want a string")
	}
	return s
}

func (d *checkDecoder) required(key string) string {
	s := d.str(key)
	if _, present := d.m[key]; !present || s == "" {
		d.fail(key, "required")
	}
	return s
}

func parseCheck(i int, m map[string]any) (Check, error) {
	d := &checkDecoder{i: i, m: m}
	c := Check{
		ID: d.required("id"), On: d.required("on"), For: d.str("for"),
		Skill: d.required("skill"), Section: d.required("section"),
		Message: d.str("message"), Kind: d.str("kind"), Run: d.str("run"),
	}
	d.enums(c)
	c.Size = d.size()
	switch {
	case c.Kind != "" && c.Run != "":
		d.fail("run", "set exactly one of kind and run")
	case c.Kind == "" && c.Run == "":
		d.fail("kind", "set exactly one of kind and run")
	case c.Kind != "":
		d.kind(&c)
	default:
		d.script(&c)
	}
	for _, key := range slices.Sorted(maps.Keys(m)) {
		if !slices.Contains(checkFields, key) && !isKindArg(key) {
			d.fail(key, "unknown field")
		}
	}
	return c, errors.Join(d.errs...)
}

func (d *checkDecoder) enums(c Check) {
	if c.On != "" && !slices.Contains([]string{"open", "integrate", "close"}, c.On) {
		d.fail("on", "%q is not open, integrate or close", c.On)
	}
	if c.For != "" && c.For != "child" && c.For != "root" {
		d.fail("for", "%q is not child or root", c.For)
	}
}

// size reads the optional size list: non-empty strings, absent for every size.
func (d *checkDecoder) size() []string {
	raw, present := d.m["size"]
	if !present {
		return nil
	}
	list, isList := raw.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, isStr := e.(string)
		if !isStr || s == "" {
			isList = false
			break
		}
		out = append(out, s)
	}
	if !isList || len(out) == 0 {
		d.fail("size", "want a non-empty list of size names")
		return nil
	}
	return out
}

func isKindArg(key string) bool {
	return slices.Contains(argRegexp, key) || slices.Contains(argFile, key) || key == "heading"
}

func (d *checkDecoder) script(c *Check) {
	for _, key := range slices.Sorted(maps.Keys(d.m)) {
		if isKindArg(key) || key == "require" {
			d.fail(key, "only a kind takes %s", key)
		}
	}
	if _, ok := d.m["timeout"]; !ok {
		return
	}
	dur, err := time.ParseDuration(d.str("timeout"))
	if err != nil || dur <= 0 {
		d.fail("timeout", "want a positive duration such as 30s")
		return
	}
	c.Timeout = dur
}

func (d *checkDecoder) kind(c *Check) {
	if _, ok := d.m["timeout"]; ok {
		d.fail("timeout", "only run takes a timeout")
	}
	want, known := kindArgs[c.Kind]
	if !known {
		d.fail("kind", "unknown kind %q (%s)", c.Kind, strings.Join(slices.Sorted(maps.Keys(kindArgs)), ", "))
		return
	}
	if c.Message == "" {
		d.fail("message", "required with a kind")
	}
	if len(want) > 0 {
		c.Args = map[string]string{}
	}
	for _, key := range slices.Sorted(maps.Keys(d.m)) {
		if isKindArg(key) && !slices.Contains(want, key) {
			d.fail(key, "kind %s takes no %s", c.Kind, key)
		}
	}
	for _, key := range want {
		c.Args[key] = d.required(key)
		d.validArg(key, c.Args[key])
	}
	d.require(c)
}

// validArg checks a kind argument that names a file or a regexp.
func (d *checkDecoder) validArg(key, v string) {
	if v == "" {
		return
	}
	if slices.Contains(argFile, key) && (path.IsAbs(v) || slices.Contains(strings.Split(v, "/"), "..")) {
		d.fail(key, "%q must be relative to the repository and free of ..", v)
	}
	if slices.Contains(argRegexp, key) {
		d.regexp(key, v)
	}
}

func (d *checkDecoder) regexp(field, v string) {
	if _, err := regexp.Compile(v); err != nil {
		d.fail(field, "regexp does not compile: %v", err)
	}
}

func (d *checkDecoder) require(c *Check) {
	raw, present := d.m["require"]
	if c.Kind != "section-matches" {
		if present {
			d.fail("require", "kind %s takes no require", c.Kind)
		}
		return
	}
	list, isList := raw.([]any)
	if !isList || len(list) == 0 {
		d.fail("require", "want a non-empty list of regexps")
	}
	for _, e := range list {
		s, isStr := e.(string)
		if !isStr {
			d.fail("require", "want strings")
			continue
		}
		d.regexp("require", s)
		c.Require = append(c.Require, s)
	}
}

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

var secretName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// SecretRef splits a reference into its source ("env" or "secret") and name; ok is false for a
// literal. The name is not checked: CheckSecretName does that.
func SecretRef(v string) (source, name string, ok bool) {
	for _, s := range []string{"env", "secret"} {
		if n, found := strings.CutPrefix(v, s+":"); found {
			return s, n, true
		}
	}
	return "", "", false
}

// CheckSecretName refuses a name a reference or the secrets file cannot hold.
func CheckSecretName(n string) error {
	if !secretName.MatchString(n) {
		return fmt.Errorf("secret name %q: want letters, digits, _ or -", n)
	}
	return nil
}

// SecretsPath is the secrets file of project under the user's configuration directory.
func SecretsPath(configDir, project string) string {
	return filepath.Join(configDir, "ness", "secrets", project+".toml")
}

// ReadSecrets returns the entries of the secrets file; a missing file has none. A file readable by
// group or others is refused, like ssh does with a private key.
func ReadSecrets(path string) (map[string]string, error) {
	st, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s is readable by others: chmod 600 %s", path, path)
	}
	m := map[string]string{}
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// SetSecret writes name = value into the secrets file, creating it 0600 in a 0700 directory.
func SetSecret(path, n, value string) error {
	if err := CheckSecretName(n); err != nil {
		return err
	}
	return updateSecrets(path, func(m map[string]string) { m[n] = value })
}

// UnsetSecret removes name from the secrets file; false when it was not there.
func UnsetSecret(path, n string) (bool, error) {
	m, err := ReadSecrets(path)
	if err != nil {
		return false, err
	}
	if _, ok := m[n]; !ok {
		return false, nil
	}
	return true, updateSecrets(path, func(m map[string]string) { delete(m, n) })
}

// SecretNames lists the names in the secrets file, sorted.
func SecretNames(path string) ([]string, error) {
	m, err := ReadSecrets(path)
	if err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(m)), nil
}

// updateSecrets rewrites the file through a temporary file and a rename, so a reader never sees half.
func updateSecrets(path string, change func(map[string]string)) error {
	m, err := ReadSecrets(path)
	if err != nil {
		return err
	}
	change(m)
	var buf bytes.Buffer
	if eerr := toml.NewEncoder(&buf).Encode(m); eerr != nil {
		return eerr
	}
	dir := filepath.Dir(path)
	if merr := os.MkdirAll(dir, 0o700); merr != nil {
		return merr
	}
	f, err := os.CreateTemp(dir, ".secrets-*") // created 0600
	if err != nil {
		return err
	}
	_, werr := f.Write(buf.Bytes())
	err = errors.Join(werr, f.Close())
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		err = errors.Join(err, os.Remove(f.Name()))
	}
	return err
}

// SecretResolver turns a configuration value into what it stands for. The secrets file is read once,
// at the first secret: reference.
type SecretResolver struct {
	Path   string
	Getenv func(string) string

	entries map[string]string
	err     error
	read    bool
}

// Resolve returns v itself for a literal, else the referenced value with ref true. A missing or
// empty value is an error naming the reference and never a value.
func (r *SecretResolver) Resolve(v string) (value string, ref bool, err error) {
	source, n, ok := SecretRef(v)
	if !ok {
		return v, false, nil
	}
	if source == "env" {
		if value = r.Getenv(n); value == "" {
			return "", true, fmt.Errorf("%s is not set", n)
		}
		return value, true, nil
	}
	if !r.read {
		r.read = true
		r.entries, r.err = ReadSecrets(r.Path)
	}
	if r.err != nil {
		return "", true, r.err
	}
	if value = r.entries[n]; value == "" {
		return "", true, fmt.Errorf("secret %s is not set in %s", n, r.Path)
	}
	return value, true, nil
}

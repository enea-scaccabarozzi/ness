package cli

import (
	"errors"
	"testing"
)

func TestBinaryVersionParsesTheVersionLineOfTheRealPath(t *testing.T) {
	// covers: R-WRK-009
	var gotName string
	var gotArgs []string
	d := Deps{Executable: "/opt/ness", Exec: func(_, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		return []byte("ness version 0.2.0\n"), nil
	}}
	v, err := binaryVersion(d)
	if err != nil || v != "0.2.0" || gotName != realPath("/opt/ness") || len(gotArgs) != 1 || gotArgs[0] != "--version" {
		t.Fatalf("v=%q err=%v name=%q args=%v", v, err, gotName, gotArgs)
	}
	d.Exec = func(string, string, ...string) ([]byte, error) { return []byte("\n"), nil }
	if _, err = binaryVersion(d); err == nil {
		t.Fatal("empty output must be an error")
	}
	d.Exec = func(string, string, ...string) ([]byte, error) { return nil, errors.New("gone") }
	if _, err = binaryVersion(d); err == nil {
		t.Fatal("exec error must be returned")
	}
}

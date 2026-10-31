package cli

import (
	"bytes"
	"testing"
)

func TestOtherOSRefusesEverythingButVersion(t *testing.T) {
	// covers: R-SESS-015
	var out, errb bytes.Buffer
	d := Deps{Stdout: &out, Stderr: &errb, Version: "0.1.0", GOOS: "darwin"}
	if code := Main([]string{"inbox"}, d); code != ExitRefused || !bytes.Contains(errb.Bytes(), []byte("Linux only")) {
		t.Fatalf("inbox on darwin: exit %d, stderr %q", code, errb.String())
	}
	if code := Main([]string{"--version"}, d); code != ExitOK {
		t.Fatalf("--version on darwin: exit %d", code)
	}
}

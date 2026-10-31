//go:build e2e

package herdr_test

import (
	"os"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestRuntimeContractOverRealHerdr(t *testing.T) {
	// Runs only inside herdr (HERDR_ENV=1): opens one tab, types into it, closes it. No client
	// is started, so it costs nothing (R-SESS-110 real half).
	// covers: R-SESS-110
	if os.Getenv("HERDR_ENV") != "1" {
		t.Skip("not inside herdr")
	}
	testkit.RuntimeContract(t, herdr.Runtime{Exec: cli.SystemExec}, false)
}

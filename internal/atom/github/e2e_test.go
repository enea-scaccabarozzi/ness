//go:build e2e

package github_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/github"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestGitHubContractOverRealGitHub(t *testing.T) {
	// The real half of contract(fake+real): opt-in, needs the operator's credentials in the
	// environment; it posts two short messages. Never runs in CI or make validate.
	// covers: R-NTF-028
	if os.Getenv("NESS_E2E") != "1" || os.Getenv("NESS_E2E_GH_TOKEN") == "" || os.Getenv("NESS_E2E_GH_REPO") == "" ||
		os.Getenv("NESS_E2E_GH_ISSUE") == "" {
		t.Skip("set NESS_E2E=1 and NESS_E2E_GH_TOKEN, NESS_E2E_GH_REPO and NESS_E2E_GH_ISSUE to run")
	}
	client := notify.Hardened(&http.Client{Timeout: 30 * time.Second})
	c := github.Channel{Token: os.Getenv("NESS_E2E_GH_TOKEN"), Repo: os.Getenv("NESS_E2E_GH_REPO"), HTTP: client}
	testkit.ChannelContract(t, c, os.Getenv("NESS_E2E_GH_ISSUE"), nil)
}

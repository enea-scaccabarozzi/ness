//go:build e2e

package slack_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/slack"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSlackContractOverRealSlack(t *testing.T) {
	// The real half of contract(fake+real): opt-in, needs the operator's credentials in the
	// environment; it posts two short messages. Never runs in CI or make validate.
	// covers: R-NTF-023
	if os.Getenv("NESS_E2E") != "1" || os.Getenv("NESS_E2E_SLACK_TOKEN") == "" || os.Getenv("NESS_E2E_SLACK_CHANNEL") == "" {
		t.Skip("set NESS_E2E=1 and NESS_E2E_SLACK_TOKEN and NESS_E2E_SLACK_CHANNEL to run")
	}
	client := notify.Hardened(&http.Client{Timeout: 30 * time.Second})
	c := slack.Channel{Token: os.Getenv("NESS_E2E_SLACK_TOKEN"), ChannelID: os.Getenv("NESS_E2E_SLACK_CHANNEL"), HTTP: client}
	testkit.ChannelContract(t, c, "", nil)
}

//go:build e2e

package telegram_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/telegram"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestTelegramContractOverRealTelegram(t *testing.T) {
	// The real half of contract(fake+real): opt-in, needs the operator's credentials in the
	// environment; it posts two short messages. Never runs in CI or make validate.
	// covers: R-NTF-027
	if os.Getenv("NESS_E2E") != "1" || os.Getenv("NESS_E2E_TG_TOKEN") == "" || os.Getenv("NESS_E2E_TG_CHAT") == "" {
		t.Skip("set NESS_E2E=1 and NESS_E2E_TG_TOKEN and NESS_E2E_TG_CHAT to run")
	}
	client := notify.Hardened(&http.Client{Timeout: 30 * time.Second})
	c := telegram.Channel{Token: os.Getenv("NESS_E2E_TG_TOKEN"), ChatID: os.Getenv("NESS_E2E_TG_CHAT"), HTTP: client}
	testkit.ChannelContract(t, c, "", nil)
}

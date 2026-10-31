package worker_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

func TestHintGolden(t *testing.T) {
	// The hint is short and a pure function of the count and the intent mix (sorted), urgent when
	// any item is; frozen here.
	// covers: R-WRK-025
	item := func(intent string, urgent bool) store.Item { return store.Item{Intent: intent, Urgent: urgent} }
	for want, items := range map[string][]store.Item{
		"ness: 1 unread (request); run ness inbox":                  {item("request", false)},
		"ness: 1 unread (fyi); run ness inbox":                      {item("fyi", false)},
		"ness: 1 unread (reply); run ness inbox":                    {item("reply", false)},
		"ness: 3 unread (fyi, request); run ness inbox":             {item("request", false), item("fyi", false), item("request", false)},
		"ness: 2 unread (decision, handoff); run ness inbox":        {item("handoff", false), item("decision", false)},
		"ness: 2 unread, urgent (fyi, request); run ness inbox now": {item("fyi", false), item("request", true)},
	} {
		if got := worker.Hint(items); got != want {
			t.Errorf("Hint(%v) = %q, want %q", items, got, want)
		}
	}
}

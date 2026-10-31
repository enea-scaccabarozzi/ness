package worker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Hint is the text typed to wake a session (R-WRK-025): the unread count, the intent mix sorted,
// the command to run; "now" when any item is urgent. Frozen by TestHintGolden.
func Hint(items []store.Item) string {
	var intents []string
	urgent := false
	for _, it := range items {
		if !slices.Contains(intents, it.Intent) {
			intents = append(intents, it.Intent)
		}
		urgent = urgent || it.Urgent
	}
	slices.Sort(intents)
	mix := strings.Join(intents, ", ")
	if urgent {
		return fmt.Sprintf("ness: %d unread, urgent (%s); run ness inbox now", len(items), mix)
	}
	return fmt.Sprintf("ness: %d unread (%s); run ness inbox", len(items), mix)
}

package cli

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestEveryClientNameBuildsItsAtom(t *testing.T) {
	// The client list is written once; each name builds the atom of that name.
	// covers: R-CFG-070
	for _, name := range config.Clients {
		if got := buildClient(Deps{}, name).Name(); got != name {
			t.Errorf("buildClient(%q).Name() = %q", name, got)
		}
	}
}

func TestWorkerDeliversThroughTheClientSeam(t *testing.T) {
	// The worker's clients are the atoms that read their screen: claude, not codex.
	// covers: R-WRK-090
	got := workerClients(Deps{})
	if names := slices.Sorted(maps.Keys(got)); !slices.Equal(names, []string{"claude"}) {
		t.Fatalf("worker clients = %v", names)
	}
	c := got["claude"]
	if c.Read == nil || c.Resume != "continue" || c.Rename("p/t") != "/rename p/t" {
		t.Fatalf("claude worker client = %+v", c)
	}
}

func TestAuthorizedRefusesAClientThatCannotCheckWords(t *testing.T) {
	// A codex root closing itself with --authorized is refused naming the client.
	// covers: R-WRK-092
	err := authorized(Deps{}, t.TempDir(), store.Session{ID: "s-1", Client: "codex", ClientSession: "x"}, "close it")
	if err == nil || !strings.Contains(err.Error(), "client codex cannot check") {
		t.Fatalf("err = %v", err)
	}
}

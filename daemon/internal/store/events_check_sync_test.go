package store_test

import (
	"sort"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// The events table CHECK constraint hardcodes the global-category set
// (store cannot import events without a cycle). This test enforces
// lockstep so adding a new global category here-or-there can't drift
// the schema gate from the publish-side gate.
func TestEventsGlobalCheckCategoriesMatch(t *testing.T) {
	got := store.EventsGlobalCheckCategories()
	sort.Strings(got)

	var want []string
	for k := range events.GlobalCategories {
		want = append(want, k)
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("count mismatch: store=%v events=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("mismatch at %d: store=%q events=%q", i, got[i], want[i])
		}
	}
}

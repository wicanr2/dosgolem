package buckrogers

import "testing"

func TestLiveBodyIconWatcherFollowsProvenOrders(t *testing.T) {
	c := bodyTestCatalog()
	groups := func(w *LiveBodyIconWatcher) []string {
		var out []string
		for _, tr := range w.Transitions() {
			out = append(out, tr.Group)
		}
		return out
	}
	feed := func(w *LiveBodyIconWatcher, keys []string, step uint64) {
		for i, k := range keys {
			w.Observe(bodyTestEvent(c, k, step+uint64(i)*10))
		}
	}
	w := NewLiveBodyIconWatcher(c)
	feed(w, bodyInitialKeys, 10)
	feed(w, []string{"body.icon.selection.instruction", "body.icon.confirmation"}, 100)
	feed(w, bodyInitialKeys, 200) // refuse returns to the selection screen
	feed(w, []string{"body.icon.confirmation"}, 300)
	w.Observe(bodyTestSaveEvent(t, "BUCK", 400))
	want := []string{"body-selection", "selection-redraw", "confirmation", "body-selection", "confirmation", "save_prompt"}
	got := groups(w)
	if len(got) != len(want) {
		t.Fatalf("groups=%v", got)
	}
	for i := range want {
		if got[i] != want[i] || w.Transitions()[i].Generation != uint64(i+1) {
			t.Fatalf("groups=%v", got)
		}
	}
	// Out-of-order events reset quietly; the next proven screen still works.
	q := NewLiveBodyIconWatcher(c)
	feed(q, []string{"body.icon.confirmation", "body.icon.selection.instruction"}, 10)
	feed(q, bodyInitialKeys[:3], 100)
	feed(q, bodyInitialKeys, 200)
	if g := groups(q); len(g) != 1 || g[0] != "body-selection" {
		t.Fatalf("groups after out-of-order input=%v", g)
	}
}

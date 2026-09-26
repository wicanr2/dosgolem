package buckrogers

// liveBodyIconKeys is the proven event list of each body-icon group.
func liveBodyIconKeys(group string) ([]string, bool) {
	switch group {
	case "body-selection":
		return append([]string(nil), bodyInitialKeys...), true
	case "selection-redraw":
		return []string{"body.icon.selection.instruction"}, true
	case "confirmation":
		return []string{"body.icon.confirmation"}, true
	case "save_prompt":
		return []string{bodySaveKey}, true
	}
	return nil, false
}

// LiveBodyIconWatcher turns exact body-icon events into transitions without a
// preselected route (spec 009 routes move／refuse／confirm are the proven
// orders it follows).  Events outside the proven orders reset it quietly: in a
// live game the screen simply keeps its original English there.
type LiveBodyIconWatcher struct {
	catalog    *BodyIconCatalog
	generation uint64
	collecting []BodyIconEvent
	state      string // "", "body-selection", "confirmation", "save_prompt"
	out        []BodyIconTransition
}

func NewLiveBodyIconWatcher(c *BodyIconCatalog) *LiveBodyIconWatcher {
	return &LiveBodyIconWatcher{catalog: c}
}

// Observe consumes one completed dispatcher event and returns the
// transitions it completed (zero or one).
func (w *LiveBodyIconWatcher) Observe(e TextEvent) []BodyIconTransition {
	key, slot, ok := w.catalog.matchEvent(e)
	if !ok || e.PostCallStep <= e.EntryStep {
		return nil
	}
	item := BodyIconEvent{EventKey: key, EntryStep: e.EntryStep, PostCallStep: e.PostCallStep, SlotLength: slot}
	if len(w.collecting) > 0 {
		if key == bodyInitialKeys[len(w.collecting)] {
			w.collecting = append(w.collecting, item)
			if len(w.collecting) == len(bodyInitialKeys) {
				events := w.collecting
				w.collecting = nil
				w.state = "body-selection"
				return w.emit("body-selection", events)
			}
			return nil
		}
		w.collecting = nil
	}
	switch {
	case key == bodyInitialKeys[0]:
		w.collecting = []BodyIconEvent{item}
	case key == "body.icon.selection.instruction" && w.state == "body-selection":
		return w.emit("selection-redraw", []BodyIconEvent{item})
	case key == "body.icon.confirmation" && w.state == "body-selection":
		w.state = "confirmation"
		return w.emit("confirmation", []BodyIconEvent{item})
	case key == bodySaveKey && w.state == "confirmation":
		w.state = "save_prompt"
		return w.emit("save_prompt", []BodyIconEvent{item})
	default:
		w.state = ""
	}
	return nil
}

func (w *LiveBodyIconWatcher) emit(group string, events []BodyIconEvent) []BodyIconTransition {
	w.generation++
	for i := range events {
		events[i].Generation, events[i].Group = w.generation, group
	}
	t := BodyIconTransition{Generation: w.generation, Group: group, Events: events}
	w.out = append(w.out, t)
	return []BodyIconTransition{t}
}

// Transitions returns every transition produced so far.
func (w *LiveBodyIconWatcher) Transitions() []BodyIconTransition {
	return append([]BodyIconTransition(nil), w.out...)
}

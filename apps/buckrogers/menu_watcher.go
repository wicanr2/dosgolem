package buckrogers

// MenuRequestWatcher joins the proven text recorder to the exact menu catalog.
// It emits display-only requests and has no rendering or machine capability.
type MenuRequestWatcher struct {
	recorder TextRecorder
	catalog  *MenuCatalog
	requests []DisplayRequest
	misses   int
}

func NewMenuRequestWatcher(catalog *MenuCatalog) *MenuRequestWatcher {
	return &MenuRequestWatcher{catalog: catalog}
}

func (w *MenuRequestWatcher) ObserveDispatchEntry(caller Address, ss, sp uint16, args [6]uint16, original []byte, step uint64) {
	w.recorder.ObserveDispatchEntry(caller, ss, sp, args, original, step)
}

func (w *MenuRequestWatcher) ObserveInstruction(at Address, ss, sp uint16, step uint64) {
	before := len(w.recorder.events)
	w.recorder.ObserveInstruction(at, ss, sp, step)
	if len(w.recorder.events) == before {
		return
	}
	event := w.recorder.events[len(w.recorder.events)-1]
	if w.catalog == nil {
		return
	}
	request, ok := w.catalog.Resolve(event)
	if !ok {
		w.misses++
		return
	}
	w.requests = append(w.requests, request)
}

func (w *MenuRequestWatcher) Events() []TextEvent { return w.recorder.Events() }
func (w *MenuRequestWatcher) Requests() []DisplayRequest {
	return append([]DisplayRequest(nil), w.requests...)
}
func (w *MenuRequestWatcher) Drops() int    { return w.recorder.Drops() }
func (w *MenuRequestWatcher) Pending() bool { return w.recorder.Pending() }
func (w *MenuRequestWatcher) Misses() int   { return w.misses }

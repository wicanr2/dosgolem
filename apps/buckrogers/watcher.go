package buckrogers

import "github.com/wicanr2/dosgolem/oracle"

var manualDispatcher = Address{Segment: 0x0763, Offset: 0x0424}

const dispatcherStackDelta = uint16(0x10)

// Observation is answer-free runtime metadata suitable for deterministic receipts.
type Observation struct {
	Step             uint64  `json:"step"`
	Kind             string  `json:"kind"`
	Caller           Address `json:"caller"`
	EventKey         string  `json:"event_key,omitempty"`
	TextKey          string  `json:"text_key,omitempty"`
	TranslationRunes int     `json:"translation_runes,omitempty"`
}

// ManualPresentationKind identifies a lifecycle boundary suitable for an
// output-only manual presenter. It never represents DOS input or game state.
type ManualPresentationKind string

const (
	ManualPresentationBegin   ManualPresentationKind = "begin"
	ManualPresentationClear   ManualPresentationKind = "clear"
	ManualPresentationRequest ManualPresentationKind = "request"
)

// ManualPresentationEvent is an answer-free, output-side lifecycle event.
// Request is populated only for ManualPresentationRequest and has the same
// generation as the event. It holds values only, so callers cannot retain a
// reference into the watcher or machine.
type ManualPresentationEvent struct {
	Step       uint64
	Kind       ManualPresentationKind
	Generation uint64
	Request    DisplayRequest
}

type dispatchFrame struct {
	caller     Address
	returnTo   Address
	ss         uint16
	entrySP    uint16
	text       string
	generation uint64
	begin      bool
}

// Watcher observes the original output path and emits catalog-backed requests.
// It has no input, memory-write, or rendering capability.
type Watcher struct {
	collector    Collector
	catalog      *Catalog
	pending      *dispatchFrame
	installed    map[uint32]bool
	requests     []DisplayRequest
	events       []Observation
	presentation []ManualPresentationEvent
}

func NewWatcher(catalog *Catalog) *Watcher {
	return &Watcher{catalog: catalog, installed: make(map[uint32]bool)}
}

func (w *Watcher) Requests() []DisplayRequest {
	return append([]DisplayRequest(nil), w.requests...)
}

func (w *Watcher) Observations() []Observation {
	return append([]Observation(nil), w.events...)
}

// PresentationEvents returns a defensive copy of output-only lifecycle data.
func (w *Watcher) PresentationEvents() []ManualPresentationEvent {
	return append([]ManualPresentationEvent(nil), w.presentation...)
}

// Install binds the watcher to the proven original runtime addresses.
func (w *Watcher) Install(o *oracle.Oracle) {
	o.OnCall(toOracle(manualDispatcher), func(o *oracle.Oracle) { w.dispatchEntry(o) })
	o.OnCall(toOracle(manualClear), func(o *oracle.Oracle) {
		w.ObserveClear(o.Steps())
	})
}

func (w *Watcher) dispatchEntry(o *oracle.Oracle) {
	regs := o.Regs()
	caller := fromOracle(o.Caller())
	lengthAt := oracle.Far(o.Arg(1), o.Arg(0))
	n := int(o.Byte(lengthAt))
	textAt := oracle.Far(lengthAt.Seg, lengthAt.Off+1)
	text := string(o.Bytes(textAt, n))
	w.ObserveDispatchEntry(caller, regs.SS, regs.SP, text, o.Steps())

	if w.pending == nil {
		return
	}
	f := w.pending
	lin := f.returnTo.linear()
	if !w.installed[lin] {
		w.installed[lin] = true
		returnAt := toOracle(f.returnTo)
		o.OnCall(returnAt, func(o *oracle.Oracle) { w.dispatchReturn(o, fromOracle(returnAt)) })
	}
}

// ObserveDispatchEntry accepts a dispatcher-entry snapshot from an in-module
// replay tool. Production callers normally use Install.
func (w *Watcher) ObserveDispatchEntry(caller Address, ss, sp uint16, text string, step uint64) {
	if w.pending != nil {
		w.dropNested(step, caller)
		return
	}
	f := &dispatchFrame{
		caller: caller, returnTo: caller, ss: ss, entrySP: sp,
		text: text, generation: w.collector.Generation(),
	}
	if generation, ok := w.collector.BeginEntry(caller, text); ok {
		f.generation, f.begin = generation, true
		w.events = append(w.events, Observation{Step: step, Kind: "begin", Caller: caller})
		w.presentation = append(w.presentation, ManualPresentationEvent{
			Step: step, Kind: ManualPresentationBegin, Generation: generation,
		})
	}
	w.pending = f
}

func (w *Watcher) dropNested(step uint64, caller Address) {
	w.pending = nil
	w.events = append(w.events, Observation{Step: step, Kind: "nested-drop", Caller: caller})
}

func (w *Watcher) dispatchReturn(o *oracle.Oracle, at Address) {
	regs := o.Regs()
	w.ObserveInstruction(at, regs.SS, regs.SP, o.Steps())
}

// ObserveInstruction checks whether one instruction is the guarded post-call.
func (w *Watcher) ObserveInstruction(at Address, ss, sp uint16, step uint64) {
	f := w.pending
	if f == nil {
		return
	}
	if at != f.returnTo || ss != f.ss || sp != f.entrySP+dispatcherStackDelta {
		if at == f.returnTo {
			w.pending = nil
			w.events = append(w.events, Observation{Step: step, Kind: "guard-drop", Caller: f.caller})
		}
		return
	}
	w.pending = nil
	if f.begin {
		return
	}
	q, complete := w.collector.PostCall(f.generation, f.caller, f.text)
	w.events = append(w.events, Observation{Step: step, Kind: "post-call", Caller: f.caller})
	if !complete || w.catalog == nil {
		return
	}
	request, ok := w.catalog.Resolve(f.generation, w.collector.Generation(), q)
	if !ok {
		w.events = append(w.events, Observation{Step: step, Kind: "catalog-miss", Caller: f.caller})
		return
	}
	w.requests = append(w.requests, request)
	w.events = append(w.events, Observation{
		Step: step, Kind: "request", Caller: f.caller, EventKey: request.EventKey,
		TextKey: request.TextKey, TranslationRunes: len([]rune(request.Translation)),
	})
	w.presentation = append(w.presentation, ManualPresentationEvent{
		Step: step, Kind: ManualPresentationRequest, Generation: request.Generation, Request: request,
	})
}

// ObserveClear records the proven clear entry without mutating the original.
func (w *Watcher) ObserveClear(step uint64) {
	active, generation := w.collector.active(), w.collector.Generation()
	w.collector.ClearEntry(manualClear)
	w.events = append(w.events, Observation{Step: step, Kind: "clear", Caller: manualClear})
	if active {
		w.presentation = append(w.presentation, ManualPresentationEvent{
			Step: step, Kind: ManualPresentationClear, Generation: generation,
		})
	}
}

func (a Address) linear() uint32       { return uint32(a.Segment)*16 + uint32(a.Offset) }
func toOracle(a Address) oracle.Addr   { return oracle.Far(a.Segment, a.Offset) }
func fromOracle(a oracle.Addr) Address { return Address{Segment: a.Seg, Offset: a.Off} }

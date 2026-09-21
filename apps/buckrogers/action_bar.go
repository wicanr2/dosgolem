package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const actionGlyphStackDelta = uint16(0x12)

var actionBarHeader = []string{
	"screen", "key", "original_length", "original_sha256", "row", "column",
	"x0", "y0", "x1", "y1", "normal_first_caller", "normal_rest_caller",
	"normal_bg", "normal_first_fg", "normal_rest_fg", "focus_caller",
	"focus_bg", "focus_fg", "evidence_level", "disabled_state",
}

type actionBarEntry struct {
	screen, key                    string
	length                         uint8
	hash                           [32]byte
	row, column                    uint8
	x0, y0, x1, y1                 uint8
	normalFirst, normalRest, focus Address
	normalBG, normalFirstFG        uint8
	normalRestFG, focusBG, focusFG uint8
}

// ActionBarCatalog is a content-safe exact catalog for the career and
// technical skill action bars. It retains hashes and geometry, not source text.
type ActionBarCatalog struct {
	byScreen map[string][]actionBarEntry
}

type actionRequestIdentity struct {
	screen, eventKey, variant string
	length                    uint8
	hash                      [32]byte
	row, column               uint8
	x0, y0, x1, y1            uint8
}

// ActionBarRequestCatalog resolves only completed, exact action-bar events.
type ActionBarRequestCatalog struct {
	events     *ActionBarCatalog
	byIdentity map[actionRequestIdentity]DisplayRequest
}

// LoadActionBarRequestCatalog validates event and Traditional Chinese text
// coverage, then expands each compact event row into normal/focus identities.
func LoadActionBarRequestCatalog(events, translations []byte) (*ActionBarRequestCatalog, error) {
	eventCatalog, err := LoadActionBarCatalog(events)
	if err != nil {
		return nil, err
	}
	rows, err := readTSV("skill-action-bar.zh-TW.tsv", translations, textHeader)
	if err != nil {
		return nil, err
	}
	texts := make(map[string]string, len(rows))
	for _, row := range rows {
		if row[2] != "runtime-interface" {
			return nil, fmt.Errorf("skill-action-bar.zh-TW.tsv: source 漂移 for %q", row[0])
		}
		if _, exists := texts[row[0]]; exists {
			return nil, fmt.Errorf("skill-action-bar.zh-TW.tsv: 重複文字鍵 %q", row[0])
		}
		texts[row[0]] = row[1]
	}
	c := &ActionBarRequestCatalog{events: eventCatalog, byIdentity: make(map[actionRequestIdentity]DisplayRequest, 16)}
	used := make(map[string]bool)
	for _, screen := range []string{"career", "technical"} {
		for _, entry := range eventCatalog.byScreen[screen] {
			translation, ok := texts[entry.key]
			if !ok {
				return nil, fmt.Errorf("skill-action-bar.zh-TW.tsv: 缺少文字鍵 %q", entry.key)
			}
			used[entry.key] = true
			for _, variant := range []string{"normal", "focus"} {
				eventKey := entry.screen + "." + entry.key + "." + variant
				id := actionRequestIdentity{entry.screen, eventKey, variant, entry.length, entry.hash,
					entry.row, entry.column, entry.x0, entry.y0, entry.x1, entry.y1}
				c.byIdentity[id] = DisplayRequest{EventKey: eventKey, TextKey: entry.key, Translation: translation}
			}
		}
	}
	for key := range texts {
		if !used[key] {
			return nil, fmt.Errorf("skill-action-bar.zh-TW.tsv: 孤兒文字鍵 %q", key)
		}
	}
	return c, nil
}

func (c *ActionBarRequestCatalog) Resolve(event ActionBarEvent) (DisplayRequest, bool) {
	if c == nil || event.PostCallStep <= event.EntryStep {
		return DisplayRequest{}, false
	}
	id := actionRequestIdentity{event.Screen, event.EventKey, event.Variant, event.OriginalLength,
		event.OriginalSHA256, event.Row, event.Column, event.X0, event.Y0, event.X1, event.Y1}
	request, ok := c.byIdentity[id]
	return request, ok
}

// LoadActionBarCatalog validates the formal project inventory.
func LoadActionBarCatalog(data []byte) (*ActionBarCatalog, error) {
	rows, err := readTSV("skill-action-bar-events.tsv", data, actionBarHeader)
	if err != nil {
		return nil, err
	}
	c := &ActionBarCatalog{byScreen: map[string][]actionBarEntry{"career": {}, "technical": {}}}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row[0] != "career" && row[0] != "technical" {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: unknown screen %q", row[0])
		}
		if row[1] != "action.add" && row[1] != "action.subtract" && row[1] != "action.prev" && row[1] != "action.next" && row[1] != "action.done" {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: unknown key %q", row[1])
		}
		identity := row[0] + "/" + row[1]
		if seen[identity] {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: duplicate identity %q", identity)
		}
		seen[identity] = true
		length, err := menuByte(row[2])
		if err != nil || length == 0 {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: invalid length %q", row[2])
		}
		hash, err := menuHash(row[3])
		if err != nil {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: invalid hash %q", row[3])
		}
		values := [11]uint8{}
		for i, field := range []int{4, 5, 6, 7, 8, 9, 12, 13, 14, 16, 17} {
			values[i], err = menuByte(row[field])
			if err != nil {
				return nil, fmt.Errorf("skill-action-bar-events.tsv: invalid numeric field %q", row[field])
			}
		}
		first, err := menuAddress(row[10])
		if err != nil {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: invalid normal first caller")
		}
		rest, err := menuAddress(row[11])
		if err != nil {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: invalid normal rest caller")
		}
		focus, err := menuAddress(row[15])
		if err != nil {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: invalid focus caller")
		}
		if row[18] != "confirmed" || row[19] != "unknown" {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: evidence boundary drift")
		}
		if values[4]-values[2] != length*8 || values[5]-values[3] != 8 || values[2] != values[1]*8 {
			return nil, fmt.Errorf("skill-action-bar-events.tsv: geometry drift for %s", identity)
		}
		entry := actionBarEntry{
			row[0], row[1], length, hash, values[0], values[1], values[2], values[3], values[4], values[5],
			first, rest, focus, values[6], values[7], values[8], values[9], values[10],
		}
		c.byScreen[row[0]] = append(c.byScreen[row[0]], entry)
	}
	if len(c.byScreen["career"]) != 3 || len(c.byScreen["technical"]) != 5 {
		return nil, fmt.Errorf("skill-action-bar-events.tsv: screen coverage must be career=3 technical=5")
	}
	return c, nil
}

// ActionBarGlyphCall is one guarded 0763:026B call. Visual parameters use
// their proven low byte; Original is never included in the output event.
type ActionBarGlyphCall struct {
	EntryStep, PostCallStep             uint64
	Caller                              Address
	Mode, Glyph, Repeat                 uint8
	Background, Foreground, Row, Column uint8
}

// ActionBarEvent is content-safe metadata for one exact action label.
type ActionBarEvent struct {
	EntryStep      uint64   `json:"entry_step"`
	PostCallStep   uint64   `json:"post_call_step"`
	Screen         string   `json:"screen"`
	EventKey       string   `json:"event_key"`
	Variant        string   `json:"variant"`
	OriginalLength uint8    `json:"original_length"`
	OriginalSHA256 [32]byte `json:"original_sha256"`
	Row            uint8    `json:"row"`
	Column         uint8    `json:"column"`
	X0             uint8    `json:"x0"`
	Y0             uint8    `json:"y0"`
	X1             uint8    `json:"x1"`
	Y1             uint8    `json:"y1"`
}

type actionCandidate struct {
	entry   actionBarEntry
	variant string
	bytes   []byte
	first   uint64
}

type actionBarCollector struct {
	catalog   *ActionBarCatalog
	screen    string
	candidate *actionCandidate
	events    []ActionBarEvent
	misses    int
	drops     int
}

func (c *actionBarCollector) anchor(screen string) bool {
	if c.catalog == nil || len(c.catalog.byScreen[screen]) == 0 {
		return false
	}
	c.screen, c.candidate = screen, nil
	return true
}

func (c *actionBarCollector) clear() { c.screen, c.candidate = "", nil }

func (c *actionBarCollector) observe(call ActionBarGlyphCall) {
	if c.screen == "" || c.catalog == nil {
		return
	}
	if c.candidate != nil {
		if c.matches(c.candidate.entry, c.candidate.variant, len(c.candidate.bytes), call) {
			c.candidate.bytes = append(c.candidate.bytes, call.Glyph)
			if len(c.candidate.bytes) == int(c.candidate.entry.length) {
				c.finish(call.PostCallStep)
			}
			return
		}
		c.candidate = nil
		c.drops++
	}
	c.start(call)
}

func (c *actionBarCollector) start(call ActionBarGlyphCall) {
	for _, entry := range c.catalog.byScreen[c.screen] {
		if call.Column != entry.column || call.Row != entry.row {
			continue
		}
		for _, variant := range []string{"normal", "focus"} {
			if c.matches(entry, variant, 0, call) {
				c.candidate = &actionCandidate{entry: entry, variant: variant, bytes: []byte{call.Glyph}, first: call.EntryStep}
				if entry.length == 1 {
					c.finish(call.PostCallStep)
				}
				return
			}
		}
	}
}

func (c *actionBarCollector) matches(entry actionBarEntry, variant string, index int, call ActionBarGlyphCall) bool {
	if call.Mode != 1 || call.Repeat != 1 || call.Row != entry.row || int(call.Column) != int(entry.column)+index {
		return false
	}
	if variant == "focus" {
		return call.Caller == entry.focus && call.Background == entry.focusBG && call.Foreground == entry.focusFG
	}
	caller, foreground := entry.normalRest, entry.normalRestFG
	if index == 0 {
		caller, foreground = entry.normalFirst, entry.normalFirstFG
	}
	return call.Caller == caller && call.Background == entry.normalBG && call.Foreground == foreground
}

func (c *actionBarCollector) finish(post uint64) {
	p := c.candidate
	c.candidate = nil
	hash := sha256.Sum256(p.bytes)
	if hash != p.entry.hash {
		c.misses++
		return
	}
	c.events = append(c.events, ActionBarEvent{
		p.first, post, p.entry.screen, p.entry.screen + "." + p.entry.key + "." + p.variant,
		p.variant, p.entry.length, hash, p.entry.row, p.entry.column,
		p.entry.x0, p.entry.y0, p.entry.x1, p.entry.y1,
	})
}

type actionGlyphFrame struct {
	call   ActionBarGlyphCall
	ss, sp uint16
}

// ActionBarWatcher records only guarded glyph calls and has no input, memory,
// VRAM, or rendering capability.
type ActionBarWatcher struct {
	collector     actionBarCollector
	resolver      *ActionBarRequestCatalog
	requests      []DisplayRequest
	requestMisses int
	pending       *actionGlyphFrame
	drops         int
}

func NewActionBarWatcher(catalog *ActionBarCatalog) *ActionBarWatcher {
	return &ActionBarWatcher{collector: actionBarCollector{catalog: catalog}}
}

func NewActionBarRequestWatcher(catalog *ActionBarRequestCatalog) *ActionBarWatcher {
	if catalog == nil {
		return NewActionBarWatcher(nil)
	}
	return &ActionBarWatcher{collector: actionBarCollector{catalog: catalog.events}, resolver: catalog}
}

// ObserveAnchorEvent accepts only the two proven exact heading event keys.
func (w *ActionBarWatcher) ObserveAnchorEvent(eventKey string) bool {
	switch eventKey {
	case "career.screen.remaining_points.heading":
		return w.collector.anchor("career")
	case "technical.screen.general_points.heading":
		return w.collector.anchor("technical")
	}
	if (w.collector.screen == "career" && strings.HasPrefix(eventKey, "career.screen.")) ||
		(w.collector.screen == "technical" && strings.HasPrefix(eventKey, "technical.screen.")) {
		return false
	}
	if w.collector.screen == "technical" &&
		(eventKey == "career.screen.maximum_per_skill.heading" || eventKey == "career.screen.columns.heading") {
		return false
	}
	w.pending = nil
	w.collector.clear()
	return false
}

func (w *ActionBarWatcher) ObserveClear(bottom, right, top, left uint8) {
	if bottom < top || right < left {
		w.pending = nil
		w.collector.candidate = nil
		w.drops++
		return
	}
	w.pending = nil
	w.collector.candidate = nil
}

// ObserveGlyphEntry captures the seven 0763:026B words. It does not submit a
// character until the exact far return has passed its SS/SP guard.
func (w *ActionBarWatcher) ObserveGlyphEntry(caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if w.pending != nil {
		w.pending = nil
		w.drops++
		return
	}
	w.pending = &actionGlyphFrame{call: ActionBarGlyphCall{
		EntryStep: step, Caller: caller, Mode: uint8(args[0]), Glyph: uint8(args[1]), Repeat: uint8(args[2]),
		Background: uint8(args[3]), Foreground: uint8(args[4]), Row: uint8(args[5]), Column: uint8(args[6]),
	}, ss: ss, sp: sp}
}

func (w *ActionBarWatcher) ObserveInstruction(at Address, ss, sp uint16, step uint64) {
	f := w.pending
	if f == nil || at != f.call.Caller {
		return
	}
	w.pending = nil
	if ss != f.ss || sp != f.sp+actionGlyphStackDelta {
		w.drops++
		return
	}
	f.call.PostCallStep = step
	before := len(w.collector.events)
	w.collector.observe(f.call)
	if w.resolver != nil && len(w.collector.events) > before {
		request, ok := w.resolver.Resolve(w.collector.events[len(w.collector.events)-1])
		if !ok {
			w.requestMisses++
			return
		}
		w.requests = append(w.requests, request)
	}
}

func (w *ActionBarWatcher) Events() []ActionBarEvent {
	return append([]ActionBarEvent(nil), w.collector.events...)
}
func (w *ActionBarWatcher) Pending() bool { return w.pending != nil || w.collector.candidate != nil }
func (w *ActionBarWatcher) Drops() int    { return w.drops + w.collector.drops }
func (w *ActionBarWatcher) Misses() int   { return w.collector.misses }
func (w *ActionBarWatcher) Requests() []DisplayRequest {
	return append([]DisplayRequest(nil), w.requests...)
}
func (w *ActionBarWatcher) RequestMisses() int { return w.requestMisses }

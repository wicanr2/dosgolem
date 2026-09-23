package buckrogers

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// BodyIconIdentity is an exact, answer-free identity for one approved body
// screen text call. It intentionally contains no source bytes.
type BodyIconIdentity struct {
	EventKey               string
	Caller                 Address
	Length                 uint8
	SHA256                 [32]byte
	BG, FG, Row, Column    uint8
	TextKey, Translation   string
	Rect                   PixelRect
	DrawX, DrawY, Capacity int
}

type BodyIconCatalog struct{ byKey map[string]BodyIconIdentity }

var bodyIconKeys = []string{"body.icon.confirmation", "body.icon.save_prompt", "body.icon.old.label", "body.icon.old.action", "body.icon.new.label", "body.icon.new.action", "body.icon.selection.instruction"}
var bodyInitialKeys = []string{"body.icon.old.label", "body.icon.old.action", "body.icon.new.label", "body.icon.new.action", "body.icon.selection.instruction"}

// LoadBodyIconCatalog joins the formal events, translation, and safe-rect TSVs.
func LoadBodyIconCatalog(events, translations, rects []byte) (*BodyIconCatalog, error) {
	rows, err := readTSV("body-icon-events.tsv", events, []string{"event_key", "sequence", "text_key", "original_length", "original_sha256", "caller", "background", "foreground", "row", "column"})
	if err != nil {
		return nil, err
	}
	if len(rows) != len(bodyIconKeys) {
		return nil, fmt.Errorf("body icon event catalog requires exactly %d rows", len(bodyIconKeys))
	}
	texts, err := readTSV("body-icon.zh-TW.tsv", translations, []string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	textByKey := map[string]string{}
	for _, row := range texts {
		if _, ok := textByKey[row[0]]; ok {
			return nil, fmt.Errorf("body icon duplicate translation %q", row[0])
		}
		textByKey[row[0]] = row[1]
	}
	rectRows, err := readTSV("body-icon-text-safe-rects.tsv", rects, []string{"screen", "event_key", "x", "y", "width", "height", "draw_x", "draw_y", "capacity_cells", "line_count", "overflow_policy"})
	if err != nil {
		return nil, err
	}
	rectByKey := map[string]PixelRect{}
	drawByKey := map[string][3]int{}
	for _, row := range rectRows {
		vals := [7]int{}
		for i, col := range []int{2, 3, 4, 5, 6, 7, 8} {
			vals[i], err = strconv.Atoi(row[col])
			if err != nil || vals[i] < 0 {
				return nil, fmt.Errorf("body icon rectangle invalid for %q", row[1])
			}
		}
		if vals[4] != vals[0] || vals[5] != vals[1] || vals[3] != 8 || row[9] != "1" || row[10] != "single-line-reject" || vals[0]+vals[2] > 320 || vals[1]+vals[3] > 200 || vals[0]%8 != 0 || vals[1]%8 != 0 || vals[2]%8 != 0 {
			return nil, fmt.Errorf("body icon rectangle contract invalid for %q", row[1])
		}
		if _, ok := rectByKey[row[1]]; ok {
			return nil, fmt.Errorf("body icon duplicate rectangle %q", row[1])
		}
		rectByKey[row[1]] = PixelRect{vals[0], vals[1], vals[2], vals[3]}
		drawByKey[row[1]] = [3]int{vals[4], vals[5], vals[6]}
	}
	out := &BodyIconCatalog{byKey: make(map[string]BodyIconIdentity, len(rows))}
	for i, row := range rows {
		if row[0] != bodyIconKeys[i] || len(rows) != len(bodyIconKeys) {
			return nil, fmt.Errorf("body icon event order/coverage invalid at %q", row[0])
		}
		sequence, sequenceErr := strconv.Atoi(row[1])
		if sequenceErr != nil || sequence != i+1 {
			return nil, fmt.Errorf("body icon event sequence invalid at %q", row[0])
		}
		length, e := strconv.ParseUint(row[3], 10, 8)
		if e != nil {
			return nil, fmt.Errorf("body icon length invalid")
		}
		hash, e := hex.DecodeString(row[4])
		if e != nil || len(hash) != 32 {
			return nil, fmt.Errorf("body icon hash invalid")
		}
		var sha [32]byte
		copy(sha[:], hash)
		parts := strings.Split(row[5], ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("body icon caller invalid")
		}
		cs, e1 := strconv.ParseUint(parts[0], 16, 16)
		ip, e2 := strconv.ParseUint(parts[1], 16, 16)
		if e1 != nil || e2 != nil {
			return nil, fmt.Errorf("body icon caller invalid")
		}
		nums := [4]uint64{}
		for j, col := range []int{6, 7, 8, 9} {
			nums[j], err = strconv.ParseUint(row[col], 10, 8)
			if err != nil {
				return nil, fmt.Errorf("body icon style invalid")
			}
		}
		text := textByKey[row[2]]
		rect, hasRect := rectByKey[row[0]]
		draw, hasDraw := drawByKey[row[0]]
		if text == "" || !hasRect || !hasDraw || rect.Width != int(length)*8 || rect.X != int(nums[3])*8 || rect.Y != int(nums[2])*8 || draw[2] != int(length) {
			return nil, fmt.Errorf("body icon TSV join invalid for %q", row[0])
		}
		out.byKey[row[0]] = BodyIconIdentity{EventKey: row[0], Caller: Address{uint16(cs), uint16(ip)}, Length: uint8(length), SHA256: sha, BG: uint8(nums[0]), FG: uint8(nums[1]), Row: uint8(nums[2]), Column: uint8(nums[3]), TextKey: row[2], Translation: text, Rect: rect, DrawX: draw[0], DrawY: draw[1], Capacity: draw[2]}
	}
	if len(textByKey) != len(bodyIconKeys) || len(rectByKey) != len(bodyIconKeys) {
		return nil, fmt.Errorf("body icon catalog has orphan rows")
	}
	return out, nil
}

type BodyIconRoute string

const (
	BodyIconMove    BodyIconRoute = "move"
	BodyIconRefuse  BodyIconRoute = "refuse"
	BodyIconConfirm BodyIconRoute = "confirm"
)

type BodyIconEvent struct {
	Generation   uint64 `json:"generation"`
	Group        string `json:"group"`
	EventKey     string `json:"event_key"`
	EntryStep    uint64 `json:"entry_step"`
	PostCallStep uint64 `json:"post_call_step"`
}
type BodyIconTransition struct {
	Generation uint64          `json:"generation"`
	Group      string          `json:"group"`
	Events     []BodyIconEvent `json:"events"`
}

// BodyIconWatcher consumes only exact completed events from the selected fixed
// route. Any extra, missing, reordered, or identity-mismatched call fails closed.
type BodyIconWatcher struct {
	route           BodyIconRoute
	catalog         *BodyIconCatalog
	generation      uint64
	stage, next     int
	events          []BodyIconEvent
	transitions     []BodyIconTransition
	failed, started bool
}

func NewBodyIconWatcher(route BodyIconRoute, catalog *BodyIconCatalog) (*BodyIconWatcher, error) {
	if catalog == nil || (route != BodyIconMove && route != BodyIconRefuse && route != BodyIconConfirm) {
		return nil, fmt.Errorf("body icon route/catalog invalid")
	}
	return &BodyIconWatcher{route: route, catalog: catalog}, nil
}
func (w *BodyIconWatcher) Observe(e TextEvent) error {
	if w == nil || w.failed {
		return fmt.Errorf("body icon watcher failed closed")
	}
	key, matched := w.catalog.Match(e)
	if !matched && !w.started {
		for _, id := range w.catalog.byKey {
			if e.Caller == id.Caller && e.Caller != (Address{Segment: 0x37f1, Offset: 0x101e}) {
				w.failed = true
				return fmt.Errorf("body icon callsite had mismatching event identity")
			}
		}
		return nil
	}
	if key == "" || e.PostCallStep <= e.EntryStep {
		w.failed = true
		return fmt.Errorf("body icon event did not match exact identity")
	}
	w.started = true
	seq := w.sequence()
	if w.next >= len(seq) || key != seq[w.next] {
		w.failed = true
		return fmt.Errorf("body icon unexpected event %q at position %d", key, w.next)
	}
	generation := uint64(1)
	if w.next >= 5 {
		generation = 2
	}
	if (w.route == BodyIconRefuse || w.route == BodyIconConfirm) && w.next >= 6 {
		generation = 3
	}
	w.generation = generation
	group := bodyIconEventGroup(w.route, w.next)
	item := BodyIconEvent{Generation: w.generation, Group: group, EventKey: key, EntryStep: e.EntryStep, PostCallStep: e.PostCallStep}
	w.events = append(w.events, item)
	w.next++
	if w.groupComplete(seq, w.next) {
		start := len(w.events) - w.groupSize(seq, w.next)
		groupEvents := append([]BodyIconEvent(nil), w.events[start:]...)
		w.transitions = append(w.transitions, BodyIconTransition{Generation: w.generation, Group: group, Events: groupEvents})
	}
	return nil
}

func (w *BodyIconWatcher) transitionGroup(n int) string {
	if n == 5 || (w.route == BodyIconRefuse && n == 11) {
		return "body-selection"
	}
	if w.route == BodyIconMove && n == 6 {
		return "selection-redraw"
	}
	if n == 6 {
		return "confirmation"
	}
	if w.route == BodyIconConfirm && n == 7 {
		return "save_prompt"
	}
	return ""
}
func bodyIconEventGroup(route BodyIconRoute, index int) string {
	if index < 5 {
		return "body-selection"
	}
	if route == BodyIconMove {
		return "selection-redraw"
	}
	if index == 5 {
		return "confirmation"
	}
	if route == BodyIconRefuse {
		return "body-selection"
	}
	if route == BodyIconConfirm {
		return "save_prompt"
	}
	return ""
}
func (c *BodyIconCatalog) Match(e TextEvent) (string, bool) {
	if c == nil {
		return "", false
	}
	for key, id := range c.byKey {
		if e.Caller == id.Caller && e.OriginalLength == id.Length && e.OriginalSHA256 == id.SHA256 && e.Background == id.BG && e.Foreground == id.FG && e.Row == id.Row && e.Column == id.Column {
			return key, true
		}
	}
	return "", false
}
func (w *BodyIconWatcher) sequence() []string {
	switch w.route {
	case BodyIconMove:
		return append(append([]string{}, bodyInitialKeys...), "body.icon.selection.instruction")
	case BodyIconRefuse:
		return append(append(append([]string{}, bodyInitialKeys...), "body.icon.confirmation"), bodyInitialKeys...)
	case BodyIconConfirm:
		return append(append(append([]string{}, bodyInitialKeys...), "body.icon.confirmation"), "body.icon.save_prompt")
	}
	return nil
}
func (w *BodyIconWatcher) groupComplete(seq []string, n int) bool {
	if n == 5 {
		return true
	}
	if w.route == BodyIconMove && n == 6 {
		return true
	}
	if (w.route == BodyIconRefuse || w.route == BodyIconConfirm) && n == 6 {
		return true
	}
	if w.route == BodyIconRefuse && n == 11 {
		return true
	}
	if w.route == BodyIconConfirm && n == 7 {
		return true
	}
	return false
}
func (w *BodyIconWatcher) groupSize(seq []string, n int) int {
	if n == 5 {
		return 5
	}
	if w.route == BodyIconMove && n == 6 {
		return 1
	}
	if (w.route == BodyIconRefuse || w.route == BodyIconConfirm) && n == 6 {
		return 1
	}
	if w.route == BodyIconRefuse && n == 11 {
		return 5
	}
	if w.route == BodyIconConfirm && n == 7 {
		return 1
	}
	return 0
}
func bodyIconGroup(key string) string {
	if key == "body.icon.confirmation" {
		return "confirmation"
	}
	if key == "body.icon.save_prompt" {
		return "save_prompt"
	}
	if key == "body.icon.selection.instruction" {
		return "selection"
	}
	return "body_icon"
}
func (w *BodyIconWatcher) Transitions() []BodyIconTransition {
	out := make([]BodyIconTransition, len(w.transitions))
	copy(out, w.transitions)
	for i := range out {
		out[i].Events = append([]BodyIconEvent(nil), out[i].Events...)
	}
	return out
}
func (w *BodyIconWatcher) Complete() bool {
	return w != nil && !w.failed && w.next == len(w.sequence())
}
func (w *BodyIconWatcher) Failed() bool { return w == nil || w.failed }

type BodyIconInvalidation struct {
	Step        uint64   `json:"step"`
	Instruction Address  `json:"instruction"`
	Offset      uint16   `json:"offset"`
	WriteMode   uint8    `json:"write_mode"`
	Reason      string   `json:"reason"`
	Invalidated []string `json:"invalidated_keys"`
}
type RuntimeBodyIconOverlay struct {
	layer         *xlate.Layer
	catalog       *BodyIconCatalog
	font          *xlate.Font
	scale         int
	route         BodyIconRoute
	transition    int
	active        map[string]bool
	generation    uint64
	invalidations []BodyIconInvalidation
}

func NewRuntimeBodyIconOverlay(c *BodyIconCatalog, font *xlate.Font, scale int, route BodyIconRoute) (*RuntimeBodyIconOverlay, error) {
	if c == nil || font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || (route != BodyIconMove && route != BodyIconRefuse && route != BodyIconConfirm) {
		return nil, fmt.Errorf("body icon runtime inputs invalid")
	}
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	for _, id := range c.byKey {
		if len([]rune(id.Translation)) > id.Capacity {
			return nil, fmt.Errorf("body icon translation exceeds safe capacity")
		}
		for _, r := range id.Translation {
			g, ok := font.Glyphs[r]
			if !ok || len(g) != font.H*((font.W+7)/8) {
				return nil, fmt.Errorf("body icon missing glyph U+%04X", r)
			}
		}
	}
	return &RuntimeBodyIconOverlay{layer: &xlate.Layer{W: 320, H: 200}, catalog: c, font: font, scale: scale, route: route, active: map[string]bool{}}, nil
}
func (o *RuntimeBodyIconOverlay) Apply(t BodyIconTransition, palette [256][3]uint8) error {
	if o == nil {
		return fmt.Errorf("body icon transition invalid")
	}
	expectedGen, expectedGroup, expectedKeys, ok := expectedBodyIconTransition(o.route, o.transition)
	if !ok || t.Generation != expectedGen || t.Group != expectedGroup || len(t.Events) != len(expectedKeys) {
		return fmt.Errorf("body icon partial, mixed, or out-of-order transition")
	}
	for i, event := range t.Events {
		if event.Generation != expectedGen || event.Group != expectedGroup || event.EventKey != expectedKeys[i] || event.PostCallStep <= event.EntryStep || (i > 0 && t.Events[i-1].PostCallStep >= event.EntryStep) {
			return fmt.Errorf("body icon key/generation/ordering mismatch")
		}
	}
	if t.Group != "selection-redraw" {
		o.layer = &xlate.Layer{W: 320, H: 200}
		o.active = map[string]bool{}
	}
	for _, e := range t.Events {
		id, ok := o.catalog.byKey[e.EventKey]
		validGroup := t.Group == "body-selection" && (bodyIconGroup(e.EventKey) == "body_icon" || bodyIconGroup(e.EventKey) == "selection") || t.Group == "selection-redraw" && bodyIconGroup(e.EventKey) == "selection" || t.Group == bodyIconGroup(e.EventKey)
		if !ok || e.Generation != t.Generation || !validGroup {
			return fmt.Errorf("body icon transition key/generation mismatch")
		}
		if t.Group == "selection-redraw" && e.EventKey == "body.icon.selection.instruction" && len(o.active) > 0 { /* retain body labels across the proven movement redraw */
		}
		o.active[e.EventKey] = true
		stamp := &xlate.Stamp{Key: id.EventKey, X: id.Rect.X, Y: id.Rect.Y, Cells: id.Rect.Width / 8, CellW: 8, CellH: 8, Font: o.font,
			GlyphX: manualGlyphOffset(o.scale), GlyphY: manualGlyphOffset(o.scale), GlyphScale: 1,
			Text: []rune(id.Translation), State: xlate.Shown, BG: palette[id.BG], FG: palette[id.FG]}
		ink, err := menuInkRect(stamp, o.scale)
		if err != nil {
			return err
		}
		clear := PixelRect{id.Rect.X * o.scale, id.Rect.Y * o.scale, id.Rect.Width * o.scale, id.Rect.Height * o.scale}
		if ink.X < clear.X || ink.Y < clear.Y || ink.X+ink.Width > clear.X+clear.Width || ink.Y+ink.Height > clear.Y+clear.Height {
			return fmt.Errorf("body icon ink outside safe rectangle")
		}
		o.layer.Add(stamp)
	}
	o.generation = t.Generation
	o.transition++
	return nil
}

func expectedBodyIconTransition(route BodyIconRoute, index int) (uint64, string, []string, bool) {
	switch route {
	case BodyIconMove:
		if index == 0 {
			return 1, "body-selection", append([]string(nil), bodyInitialKeys...), true
		}
		if index == 1 {
			return 2, "selection-redraw", []string{"body.icon.selection.instruction"}, true
		}
	case BodyIconRefuse:
		if index == 0 {
			return 1, "body-selection", append([]string(nil), bodyInitialKeys...), true
		}
		if index == 1 {
			return 2, "confirmation", []string{"body.icon.confirmation"}, true
		}
		if index == 2 {
			return 3, "body-selection", append([]string(nil), bodyInitialKeys...), true
		}
	case BodyIconConfirm:
		if index == 0 {
			return 1, "body-selection", append([]string(nil), bodyInitialKeys...), true
		}
		if index == 1 {
			return 2, "confirmation", []string{"body.icon.confirmation"}, true
		}
		if index == 2 {
			return 3, "save_prompt", []string{"body.icon.save_prompt"}, true
		}
	}
	return 0, "", nil, false
}
func (o *RuntimeBodyIconOverlay) Prewrite(w machine.VideoWrite) {
	if o == nil || len(o.active) == 0 {
		return
	}
	keyList := make([]string, 0)
	if w.Offset > 0xffff {
		keyList = o.ActiveKeys()
	} else {
		for key := range o.active {
			id := o.catalog.byKey[key]
			x0, y0, x1, y1 := id.Rect.X, id.Rect.Y, id.Rect.X+id.Rect.Width, id.Rect.Y+id.Rect.Height
			off := int(w.Offset)
			if off%320 >= x0 && off%320 < x1 && off/320 >= y0 && off/320 < y1 {
				keyList = append(keyList, key)
			}
		}
	}
	sort.Strings(keyList)
	if len(keyList) == 0 {
		return
	} // Any writer, including unknown or same-value writes, invalidates atomically before the write.
	for _, key := range keyList {
		id := o.catalog.byKey[key]
		o.layer.Clear(id.Rect.X, id.Rect.Y, id.Rect.X+id.Rect.Width, id.Rect.Y+id.Rect.Height)
		delete(o.active, key)
	}
	reason := "a000-prewrite-intersection"
	if w.Offset > 0xffff {
		reason = "a000-offset-out-of-range"
	}
	o.invalidations = append(o.invalidations, BodyIconInvalidation{Step: w.Step, Instruction: Address{w.CS, w.IP}, Offset: uint16(w.Offset), WriteMode: w.WriteMode, Reason: reason, Invalidated: keyList})
}
func (o *RuntimeBodyIconOverlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	rgba := ScaleIndexedRGBA(indexed, palette, o.scale)
	missing := []rune{}
	drew := o.layer.Draw(rgba, o.scale, func(r rune) { missing = append(missing, r) })
	return rgba, missing, drew
}
func (o *RuntimeBodyIconOverlay) ActiveKeys() []string {
	out := make([]string, 0, len(o.active))
	for k := range o.active {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func (o *RuntimeBodyIconOverlay) Invalidations() []BodyIconInvalidation {
	return append([]BodyIconInvalidation(nil), o.invalidations...)
}
func (o *RuntimeBodyIconOverlay) InvalidationCount() int {
	if o == nil {
		return 0
	}
	return len(o.invalidations)
}
func (o *RuntimeBodyIconOverlay) Generation() uint64 {
	if o == nil {
		return 0
	}
	return o.generation
}
func (o *RuntimeBodyIconOverlay) SafeRects() []PixelRect {
	if o == nil {
		return nil
	}
	keys := o.ActiveKeys()
	out := make([]PixelRect, 0, len(keys))
	for _, key := range keys {
		id := o.catalog.byKey[key]
		r := id.Rect
		out = append(out, PixelRect{r.X * o.scale, r.Y * o.scale, r.Width * o.scale, r.Height * o.scale})
	}
	return out
}
func (o *RuntimeBodyIconOverlay) Scale() int {
	if o == nil {
		return 0
	}
	return o.scale
}

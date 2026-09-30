package buckrogers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

type ActionBarOverlayAction struct {
	EventKey, TextKey string
	TranslationRunes  int
	Background        uint8
	RuneForegrounds   []uint8
}

type runtimeActionGroup struct {
	x, y, width, height int
	background          uint8
	foregrounds         []uint8
}

// RuntimeActionBarOverlay owns presentation state only. A normal style is
// mandatory and never defaulted by the constructor.
type RuntimeActionBarOverlay struct {
	catalog *ActionBarRequestCatalog
	ref     *ActionBarRequestCatalog // zh-TW hotkey reference
	rects   *MenuOverlayRects
	font    *xlate.Font
	scale   int
	// styles is the spec 040 §3.1 normal colouring of each translated key,
	// derived from its "(X)"; a key without one is untranslated here.
	styles  map[string]ActionBarNormalStyle
	layer   *xlate.Layer
	screen  string
	groups  map[string]runtimeActionGroup
	actions []ActionBarOverlayAction
	// Missing counts requests this language does not translate.
	Missing int
}

// NewRuntimeActionBarOverlay is the zh-TW presenter: the catalog is its own
// hotkey reference.  A supplied style must equal the derived colouring of
// every label of its length (the explicit-style contract of spec 215).
func NewRuntimeActionBarOverlay(catalog *ActionBarRequestCatalog, rects *MenuOverlayRects,
	font *xlate.Font, scale int, style *ActionBarNormalStyle) (*RuntimeActionBarOverlay, error) {
	if catalog == nil || rects == nil || style == nil {
		return nil, fmt.Errorf("buckrogers: action runtime 缺少 catalog、矩形或明示 normal 配色")
	}
	o, err := NewRuntimeActionBarOverlayLang(catalog, catalog, rects, font, scale, true)
	if err != nil {
		return nil, err
	}
	for key, derived := range o.styles {
		if len(derived.RuneForegrounds) != len(style.RuneForegrounds) {
			continue
		}
		for i := range derived.RuneForegrounds {
			if derived.RuneForegrounds[i] != style.RuneForegrounds[i] {
				return nil, fmt.Errorf("buckrogers: action runtime %s 的明示配色與熱鍵推導不符", key)
			}
		}
	}
	return o, nil
}

// NewRuntimeActionBarOverlayLang builds a language lane presenter (spec 040
// §3.1): each label's normal colouring is derived against the zh-TW
// reference.  strict (zh-TW) fails on a label without a valid "(X)";
// otherwise that label is untranslated in this language.
func NewRuntimeActionBarOverlayLang(catalog, reference *ActionBarRequestCatalog, rects *MenuOverlayRects,
	font *xlate.Font, scale int, strict bool) (*RuntimeActionBarOverlay, error) {
	if catalog == nil || reference == nil || rects == nil {
		return nil, fmt.Errorf("buckrogers: action runtime 缺少 catalog、參考 catalog 或矩形")
	}
	if err := ValidateActionBarOverlayCoverage(catalog, rects); err != nil {
		return nil, err
	}
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: action runtime 字型或倍率無效")
	}
	styles := map[string]ActionBarNormalStyle{}
	for id, request := range catalog.byIdentity {
		if id.variant != "normal" || request.Translation == "" {
			continue
		}
		entry, ok := catalog.entryFor(ActionBarEvent{Screen: id.screen, EventKey: id.eventKey, Variant: id.variant})
		if !ok {
			return nil, fmt.Errorf("buckrogers: normal 配色找不到事件來源")
		}
		style, err := DeriveActionBarNormalStyle(request.Translation, reference.textFor(request.TextKey), entry.normalFirstFG, entry.normalRestFG)
		if err != nil {
			if strict {
				return nil, fmt.Errorf("buckrogers: action %s：%w", request.TextKey, err)
			}
			continue
		}
		if prior, ok := styles[request.TextKey]; ok && !equalUint8s(prior.RuneForegrounds, style.RuneForegrounds) {
			return nil, fmt.Errorf("buckrogers: action %s 在兩個畫面推導出不同配色", request.TextKey)
		}
		styles[request.TextKey] = style
	}
	return &RuntimeActionBarOverlay{catalog: catalog, ref: reference, rects: rects, font: font, scale: scale, styles: styles,
		layer: &xlate.Layer{W: 320, H: 200}, groups: map[string]runtimeActionGroup{}}, nil
}

func (o *RuntimeActionBarOverlay) reference() *ActionBarRequestCatalog { return o.ref }

func equalUint8s(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// validateActionLane prebuilds every translated label (spec 040 §3.3).
func (o *RuntimeActionBarOverlay) validateAll() error {
	for id, request := range o.catalog.byIdentity {
		if request.Translation == "" {
			continue
		}
		var style *ActionBarNormalStyle
		if id.variant == "normal" {
			st, ok := o.styles[request.TextKey]
			if !ok {
				continue // untranslated in this language
			}
			style = &st
		}
		event := ActionBarEvent{EntryStep: 1, PostCallStep: 2, Screen: id.screen, EventKey: id.eventKey, Variant: id.variant,
			OriginalLength: id.length, OriginalSHA256: id.hash, Row: id.row, Column: id.column, X0: id.x0, Y0: id.y0, X1: id.x1, Y1: id.y1}
		if _, err := BuildActionBarOverlay(o.catalog, o.rects, event, request, o.font, [256][3]uint8{}, o.scale, style); err != nil {
			return fmt.Errorf("buckrogers: action %s（%d×）：%w", id.eventKey, o.scale, err)
		}
	}
	return nil
}

func (o *RuntimeActionBarOverlay) clearAll() {
	o.layer.Clear(0, 0, 320, 200)
	o.groups = map[string]runtimeActionGroup{}
}

// ObserveAnchorEvent mirrors the exact Phase 75 screen-anchor contract.
func (o *RuntimeActionBarOverlay) ObserveAnchorEvent(eventKey string) bool {
	next := ""
	switch eventKey {
	case "career.screen.remaining_points.heading":
		next = "career"
	case "technical.screen.general_points.heading":
		next = "technical"
	}
	if next != "" {
		if o.screen != "" && o.screen != next {
			o.clearAll()
		}
		o.screen = next
		return true
	}
	if (o.screen == "career" && strings.HasPrefix(eventKey, "career.screen.")) ||
		(o.screen == "technical" && strings.HasPrefix(eventKey, "technical.screen.")) {
		return false
	}
	if o.screen == "technical" &&
		(eventKey == "career.screen.maximum_per_skill.heading" || eventKey == "career.screen.columns.heading") {
		return false
	}
	o.screen = ""
	o.clearAll()
	return false
}

func (o *RuntimeActionBarOverlay) Apply(event ActionBarEvent, request DisplayRequest, palette [256][3]uint8) error {
	if o.screen == "" || event.Screen != o.screen {
		return fmt.Errorf("buckrogers: action runtime event 沒有 exact screen anchor")
	}
	want, ok := o.catalog.Resolve(event)
	if !ok || want.EventKey != request.EventKey || want.TextKey != request.TextKey {
		return fmt.Errorf("buckrogers: action overlay request 與 exact event 不符")
	}
	r := o.rects.byEvent[event.EventKey]
	var style *ActionBarNormalStyle
	missing := want.Translation == ""
	if event.Variant == "normal" && !missing {
		st, ok := o.styles[want.TextKey]
		missing = !ok
		style = &st
	}
	if missing {
		// Spec 040 §3.2: this language lacks the label; clear what the
		// presenter drew there and let the original English show.
		o.Missing++
		o.layer.Clear(r.x, r.y, r.x+r.width, r.y+r.height)
		delete(o.groups, event.EventKey)
		o.reconcileGroups()
		return nil
	}
	built, err := BuildActionBarOverlay(o.catalog, o.rects, event, want, o.font, palette, o.scale, style)
	if err != nil {
		return err
	}
	o.layer.Clear(r.x, r.y, r.x+r.width, r.y+r.height)
	o.reconcileGroups()
	for _, stamp := range built.Layer.Stamps {
		stamp.State = xlate.Pending
		o.layer.Add(stamp)
	}
	o.groups[event.EventKey] = runtimeActionGroup{r.x, r.y, r.width, r.height, built.Background,
		append([]uint8(nil), built.RuneForegrounds...)}
	o.actions = append(o.actions, ActionBarOverlayAction{event.EventKey, request.TextKey, built.TranslationRunes,
		built.Background, append([]uint8(nil), built.RuneForegrounds...)})
	return nil
}

func actionStampGroup(key string) (string, int, bool) {
	group, indexText, ok := strings.Cut(key, "#")
	if !ok {
		return "", 0, false
	}
	index, err := strconv.Atoi(indexText)
	return group, index, err == nil
}

func (o *RuntimeActionBarOverlay) reconcileGroups() {
	counts := map[string]int{}
	for _, stamp := range o.layer.Stamps {
		if group, _, ok := actionStampGroup(stamp.Key); ok {
			counts[group]++
		}
	}
	for key, group := range o.groups {
		if counts[key] == len(group.foregrounds) {
			continue
		}
		o.layer.Clear(group.x, group.y, group.x+group.width, group.y+group.height)
		delete(o.groups, key)
	}
}

func (o *RuntimeActionBarOverlay) ClearTextCells(bottom, right, top, left uint8) error {
	if bottom < top || right < left || bottom >= 25 || right >= 40 {
		return fmt.Errorf("buckrogers: action clear 無效 bottom=%d right=%d top=%d left=%d", bottom, right, top, left)
	}
	o.layer.Clear(int(left)*8, int(top)*8, (int(right)+1)*8, (int(bottom)+1)*8)
	o.reconcileGroups()
	return nil
}

func (o *RuntimeActionBarOverlay) Frame(indexed []byte, palette [256][3]uint8) {
	o.layer.Frame(indexed, logicalRGB(indexed, palette))
	o.reconcileGroups()
	for _, stamp := range o.layer.Stamps {
		groupKey, index, ok := actionStampGroup(stamp.Key)
		group, exists := o.groups[groupKey]
		if !ok || !exists || index < 0 || index >= len(group.foregrounds) {
			continue
		}
		stamp.BG, stamp.FG = palette[group.background], palette[group.foregrounds[index]]
	}
}

func (o *RuntimeActionBarOverlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool) {
	rgba := ScaleIndexedRGBA(indexed, palette, o.scale)
	// xlate stamps only cover the translated 28 logical pixels. The original
	// label can be wider (Subtract occupies 64), so clear the entire proven
	// action rectangle before drawing the mixed-width Chinese label. Otherwise
	// its English suffix survives to the right of the translation.
	shown := make(map[string]map[int]bool, len(o.groups))
	for _, stamp := range o.layer.Stamps {
		key, index, ok := actionStampGroup(stamp.Key)
		if !ok || stamp.State != xlate.Shown {
			continue
		}
		if shown[key] == nil {
			shown[key] = map[int]bool{}
		}
		shown[key][index] = true
	}
	for key, group := range o.groups {
		indices := shown[key]
		complete := len(indices) == len(group.foregrounds)
		for index := range group.foregrounds {
			complete = complete && indices[index]
		}
		if !complete {
			// A mixed Pending/Shown group must never paint only part of a label.
			o.layer.Clear(group.x, group.y, group.x+group.width, group.y+group.height)
			delete(o.groups, key)
			continue
		}
		color := palette[group.background]
		for y := group.y * o.scale; y < (group.y+group.height)*o.scale; y++ {
			for x := group.x * o.scale; x < (group.x+group.width)*o.scale; x++ {
				pixel := (y*320*o.scale + x) * 4
				rgba[pixel], rgba[pixel+1], rgba[pixel+2], rgba[pixel+3] = color[0], color[1], color[2], 0xff
			}
		}
	}
	missing := []rune{}
	drew := o.layer.Draw(rgba, o.scale, func(r rune) { missing = append(missing, r) })
	return rgba, missing, drew
}

func (o *RuntimeActionBarOverlay) ActiveKeys() []string {
	keys := make([]string, 0, len(o.groups))
	for key := range o.groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (o *RuntimeActionBarOverlay) Actions() []ActionBarOverlayAction {
	out := make([]ActionBarOverlayAction, len(o.actions))
	for i, action := range o.actions {
		out[i] = action
		out[i].RuneForegrounds = append([]uint8(nil), action.RuneForegrounds...)
	}
	return out
}

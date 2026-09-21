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
	rects   *MenuOverlayRects
	font    *xlate.Font
	scale   int
	style   ActionBarNormalStyle
	layer   *xlate.Layer
	screen  string
	groups  map[string]runtimeActionGroup
	actions []ActionBarOverlayAction
}

func NewRuntimeActionBarOverlay(catalog *ActionBarRequestCatalog, rects *MenuOverlayRects,
	font *xlate.Font, scale int, style *ActionBarNormalStyle) (*RuntimeActionBarOverlay, error) {
	if catalog == nil || rects == nil || style == nil {
		return nil, fmt.Errorf("buckrogers: action runtime 缺少 catalog、矩形或明示 normal 配色")
	}
	if err := ValidateActionBarOverlayCoverage(catalog, rects); err != nil {
		return nil, err
	}
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("buckrogers: action runtime 字型或倍率無效")
	}
	confirmed := HotkeyPreservingActionBarNormalStyle().RuneForegrounds
	if len(style.RuneForegrounds) != len(confirmed) {
		return nil, fmt.Errorf("buckrogers: action runtime 快捷字母配色長度漂移")
	}
	for i := range confirmed {
		if style.RuneForegrounds[i] != confirmed[i] {
			return nil, fmt.Errorf("buckrogers: action runtime 快捷字母配色契約漂移")
		}
	}
	for id, request := range catalog.byIdentity {
		if id.variant != "normal" {
			continue
		}
		runes := []rune(request.Translation)
		if len(style.RuneForegrounds) != len(runes) {
			return nil, fmt.Errorf("buckrogers: normal 配色數量與譯文不符")
		}
		entry, ok := catalog.entryFor(ActionBarEvent{Screen: id.screen, EventKey: id.eventKey, Variant: id.variant})
		if !ok {
			return nil, fmt.Errorf("buckrogers: normal 配色找不到事件來源")
		}
		for _, color := range style.RuneForegrounds {
			if color != entry.normalFirstFG && color != entry.normalRestFG {
				return nil, fmt.Errorf("buckrogers: normal 配色含未證實色號 %d", color)
			}
		}
	}
	return &RuntimeActionBarOverlay{catalog: catalog, rects: rects, font: font, scale: scale,
		style: ActionBarNormalStyle{RuneForegrounds: append([]uint8(nil), style.RuneForegrounds...)},
		layer: &xlate.Layer{W: 320, H: 200}, groups: map[string]runtimeActionGroup{}}, nil
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
	var style *ActionBarNormalStyle
	if event.Variant == "normal" {
		style = &o.style
	}
	built, err := BuildActionBarOverlay(o.catalog, o.rects, event, request, o.font, palette, o.scale, style)
	if err != nil {
		return err
	}
	r := o.rects.byEvent[event.EventKey]
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

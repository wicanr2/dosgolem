package buckrogers

// This file is deliberately output-only: it observes dispatch/video callbacks
// and builds an RGBA layer; it never changes DOS memory, machine state or VRAM.

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

const (
	postJoinEventsSHA   = "92ebceaf691807118392d2e284660c14694e34281a64b8b14bc5e9dcfd160ec1"
	postJoinVariantsSHA = "8e81644e94213bd99e9f91c75e21e10e277f498a16257556516302da4e448eb5"
	postJoinTextSHA     = "0c2ed04b6d5942b01b41670b38ce6d11fe8af798c04cf3fb54354474f6202eb3"
)

var postJoinKeys = []string{
	"post_join_menu.option.purge_character", "post_join_menu.option.modify_character", "post_join_menu.option.join_a_game",
	"post_join_menu.option.view_character", "post_join_menu.option.remove_character_from_team", "post_join_menu.option.show_characters_game", "post_join_menu.option.begin_adventuring",
}

// PostJoinMenuCatalog accepts only the exact READY fixtures.  File hashes make
// accidental edits and schema-compatible substitutions fail before execution.
type postJoinVariant struct {
	key, variant string
	identity     menuIdentity
}
type PostJoinMenuCatalog struct {
	base     *MenuCatalog
	variants map[menuIdentity]postJoinVariant
}

func postJoinFixtureHash(name, want string, data []byte) error {
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != want {
		return fmt.Errorf("%s SHA-256 不符 READY fixture", name)
	}
	return nil
}

func LoadPostJoinMenuCatalog(events, variants, translations []byte) (*PostJoinMenuCatalog, error) {
	if err := postJoinFixtureHash("post-join-menu-events.tsv", postJoinEventsSHA, events); err != nil {
		return nil, err
	}
	if err := postJoinFixtureHash("post-join-menu-variants.tsv", postJoinVariantsSHA, variants); err != nil {
		return nil, err
	}
	if err := postJoinFixtureHash("post-join-menu.zh-TW.tsv", postJoinTextSHA, translations); err != nil {
		return nil, err
	}
	rows, err := readTSV("post-join-menu-variants.tsv", variants, []string{"event_key", "variant", "original_length", "original_sha256", "caller", "background", "foreground", "row", "column", "evidence"})
	if err != nil || len(rows) != 21 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("post-join variant rows 必須恰為 21")
	}
	variantMap := make(map[menuIdentity]postJoinVariant, len(rows))
	for i, r := range rows {
		if r[0] != postJoinKeys[i/3] || r[1] != []string{"initial_normal", "normal_redraw", "selected"}[i%3] || r[9] != "phase161-cycle-exit-a-b" {
			return nil, fmt.Errorf("post-join variant 順序或證據不符 at %d", i)
		}
		length, e := menuByte(r[2])
		if e != nil {
			return nil, fmt.Errorf("post-join variant length invalid")
		}
		hash, e := menuHash(r[3])
		if e != nil {
			return nil, fmt.Errorf("post-join variant hash invalid")
		}
		caller, e := menuAddress(r[4])
		if e != nil {
			return nil, fmt.Errorf("post-join variant caller invalid")
		}
		fields := [4]uint8{}
		for j := range fields {
			fields[j], e = menuByte(r[5+j])
			if e != nil {
				return nil, fmt.Errorf("post-join variant style invalid")
			}
		}
		id := menuIdentity{length, hash, caller, fields[0], fields[1], fields[2], fields[3]}
		if _, ok := variantMap[id]; ok {
			return nil, fmt.Errorf("post-join variant identity duplicate")
		}
		variantMap[id] = postJoinVariant{r[0], r[1], id}
	}
	base, err := loadExactCatalog("post-join-menu-events.tsv", "post-join-menu.zh-TW.tsv", events, translations)
	if err != nil {
		return nil, err
	}
	if len(base.byIdentity) != 7 {
		return nil, fmt.Errorf("post-join event rows 必須恰為 7")
	}
	return &PostJoinMenuCatalog{base: base, variants: variantMap}, nil
}

type PostJoinMenuGeneration struct {
	Generation uint64
	Selected   int
}

// PostJoinMenuWatcher arms identities at dispatcher entry, so Prewrite runs
// before the first A000 byte of the active layer.  A same-value write is still
// a VideoWrite and therefore invalidates exactly like any other write.
type PostJoinMenuWatcher struct {
	c               *PostJoinMenuCatalog
	pending         *TextEvent
	stage, selected int
	generation      uint64
	active, failed  bool
	generations     []PostJoinMenuGeneration
}

func NewPostJoinMenuWatcher(c *PostJoinMenuCatalog) (*PostJoinMenuWatcher, error) {
	if c == nil {
		return nil, fmt.Errorf("post-join catalog nil")
	}
	return &PostJoinMenuWatcher{c: c, selected: -1}, nil
}
func (w *PostJoinMenuWatcher) ObserveEntry(e TextEvent) error {
	if w == nil || w.failed || w.pending != nil || e.PostCallStep != 0 {
		return w.fail("entry state invalid")
	}
	id := menuIdentity{e.OriginalLength, e.OriginalSHA256, e.Caller, e.Background, e.Foreground, e.Row, e.Column}
	v, ok := w.c.variants[id]
	if !ok {
		return w.fail("unknown post-join identity")
	}
	expect := "initial_normal"
	row := w.stage
	if w.stage == 7 {
		expect = "selected"
		row = 0
	}
	if w.stage == 8 {
		expect = "normal_redraw"
		row = w.selected
	}
	if w.stage == 9 {
		expect = "selected"
		row = w.selected + 1
	}
	if row < 0 || row >= 7 || v.key != postJoinKeys[row] || v.variant != expect {
		return w.fail("variant/generation mismatch")
	}
	w.pending = &e
	return nil
}
func (w *PostJoinMenuWatcher) ObserveReturn(e TextEvent) error {
	if w == nil || w.failed || w.pending == nil || e.PostCallStep <= e.EntryStep || *w.pending != (TextEvent{EntryStep: e.EntryStep, Caller: e.Caller, OriginalLength: e.OriginalLength, OriginalSHA256: e.OriginalSHA256, Background: e.Background, Foreground: e.Foreground, Row: e.Row, Column: e.Column}) {
		return w.fail("return identity mismatch")
	}
	w.pending = nil
	if w.stage < 7 {
		w.stage++
		return nil
	}
	if w.stage == 7 {
		w.selected = 0
		w.stage = 8
		return w.rebuild()
	}
	if w.stage == 8 {
		w.active = false
		w.stage = 9
		return nil
	}
	if w.stage == 9 {
		w.selected++
		w.stage = 8
		return w.rebuild()
	}
	return w.fail("generation state invalid")
}
func (w *PostJoinMenuWatcher) Prewrite(v machine.VideoWrite) {
	if w == nil || w.failed || !postJoinIntersect(v.Offset) {
		return
	}
	// Any intersecting byte, even an unchanged byte, has already made the old
	// RGBA layer unsafe. Unknown writers fail closed rather than retain pixels.
	if v.CS != 0x0763 || v.IP != 0x184d || (w.pending == nil && !w.active) {
		w.fail("unknown or unarmed A000 writer")
		return
	}
	w.active = false
}
func postJoinIntersect(off uint32) bool {
	if off >= 320*200 {
		return true
	}
	x, y := int(off%320), int(off/320)
	rows := []int{13, 14, 15, 16, 18, 19, 20}
	widths := []int{15, 16, 11, 14, 26, 17, 17}
	for i, r := range rows {
		if y >= r*8 && y < r*8+8 && x >= 72 && x < 72+widths[i]*8 {
			return true
		}
	}
	return false
}
func (w *PostJoinMenuWatcher) ObserveDiscontinuity() {
	if w != nil {
		w.active = false
		w.pending = nil
		w.failed = true
	}
}
func (w *PostJoinMenuWatcher) rebuild() error {
	w.generation++
	w.active = true
	w.generations = append(w.generations, PostJoinMenuGeneration{w.generation, w.selected})
	return nil
}
func (w *PostJoinMenuWatcher) fail(s string) error {
	if w != nil {
		w.active = false
		w.pending = nil
		w.failed = true
	}
	return fmt.Errorf("post-join fail-closed: %s", s)
}
func (w *PostJoinMenuWatcher) Failed() bool { return w == nil || w.failed }
func (w *PostJoinMenuWatcher) Active() bool { return w != nil && w.active }
func (w *PostJoinMenuWatcher) Generations() []PostJoinMenuGeneration {
	return append([]PostJoinMenuGeneration(nil), w.generations...)
}

type RuntimePostJoinMenuOverlay struct {
	layer  *xlate.Layer
	c      *PostJoinMenuCatalog
	font   *xlate.Font
	scale  int
	active map[string]bool
}

func NewRuntimePostJoinMenuOverlay(c *PostJoinMenuCatalog, font *xlate.Font, scale int) (*RuntimePostJoinMenuOverlay, error) {
	if c == nil || font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) {
		return nil, fmt.Errorf("post-join runtime inputs invalid")
	}
	if scale == 3 {
		font = manualThreeXFont(font)
	}
	return &RuntimePostJoinMenuOverlay{layer: &xlate.Layer{W: 320, H: 200}, c: c, font: font, scale: scale, active: map[string]bool{}}, nil
}
func (o *RuntimePostJoinMenuOverlay) Apply(g PostJoinMenuGeneration, p [256][3]uint8) error {
	if o == nil || g.Selected < 0 || g.Selected >= 7 {
		if o != nil {
			o.clear()
		}
		return fmt.Errorf("post-join generation invalid")
	}
	entries := make([]MenuOverlayEntry, 0, 7)
	for i, k := range postJoinKeys {
		var id menuIdentity
		var e catalogEntry
		found := false
		for x, y := range o.c.base.byIdentity {
			if y.eventKey == k {
				id, e, found = x, y, true
				break
			}
		}
		if !found {
			o.clear()
			return fmt.Errorf("post-join key missing")
		}
		bg, fg := id.background, id.foreground
		if i == g.Selected {
			bg, fg = 15, 0
		}
		entries = append(entries, MenuOverlayEntry{k, e.textKey, e.translation, bg, fg, int(id.column) * 8, int(id.row) * 8, int(id.length) * 8, 8, int(id.column) * 8, int(id.row) * 8, int(id.length), 1, "single-line-reject"})
	}
	built, err := BuildMenuOverlay(entries, o.font, p, o.scale)
	if err != nil {
		o.clear()
		return err
	}
	o.layer = built.Layer
	o.active = map[string]bool{}
	for _, k := range postJoinKeys {
		o.active[k] = true
	}
	return nil
}
func (o *RuntimePostJoinMenuOverlay) clear() {
	if o != nil {
		o.layer = &xlate.Layer{W: 320, H: 200}
		o.active = map[string]bool{}
	}
}
func (o *RuntimePostJoinMenuOverlay) Prewrite(v machine.VideoWrite) {
	if o == nil || !postJoinIntersect(v.Offset) {
		return
	}
	for k := range o.active {
		delete(o.active, k)
	}
	for i, r := range []int{13, 14, 15, 16, 18, 19, 20} {
		w := []int{15, 16, 11, 14, 26, 17, 17}[i]
		o.layer.Clear(72, r*8, 72+w*8, r*8+8)
	}
}
func (o *RuntimePostJoinMenuOverlay) Draw(i []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	rgba := ScaleIndexedRGBA(i, p, o.scale)
	var miss []rune
	d := o.layer.Draw(rgba, o.scale, func(r rune) { miss = append(miss, r) })
	return rgba, miss, d
}
func (o *RuntimePostJoinMenuOverlay) ActiveKeys() []string {
	out := make([]string, 0, len(o.active))
	for k := range o.active {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

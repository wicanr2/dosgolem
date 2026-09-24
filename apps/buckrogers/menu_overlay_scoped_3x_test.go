package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

var scopedMenuFixtureKeys = []string{
	"menu.transition.old.create_new_character",
	"race.screen.prompt", "race.option.terran", "race.option.martian", "race.option.venusian",
	"race.option.mercurian", "race.option.tinker", "race.option.desert_runner", "race.heading.terran",
	"race.selection.normal.terran", "race.selection.selected.martian", "race.selection.normal.martian",
	"function.menu.option.create_new_character", "function.menu.option.add_character_to_team",
	"function.menu.option.load_saved_game", "function.menu.option.joystick_mouse_initialize",
	"function.menu.option.exit_to_dos", "function.menu.selected.create_new_character",
	"function.menu.instruction.choose_function", "function.menu.selected.add_character_to_team",
	"function.menu.normal.add_character_to_team",
}

func scopedMenuFixture(t *testing.T) (*MenuCatalog, *MenuOverlayRects, map[string]TextEvent) {
	t.Helper()
	var events, rects strings.Builder
	events.WriteString(strings.Join(menuEventHeader, "\t") + "\n")
	rects.WriteString(strings.Join(overlayRectHeader, "\t") + "\n")
	byKey := map[string]TextEvent{}
	for i, key := range scopedMenuFixtureKeys {
		hash := sha256.Sum256([]byte(key))
		bg, fg := uint8(0), uint8(10)
		if strings.Contains(key, ".selected.") {
			bg, fg = 15, 0
		}
		if key == "function.menu.instruction.choose_function" {
			fg = 13
		}
		fmt.Fprintf(&events, "%s\t%d\tmenu.synthetic\t2\t%x\t37F1:1856\t%d\t%d\t12\t9\n", key, i+1, hash, bg, fg)
		fmt.Fprintf(&rects, "%s\t72\t96\t16\t8\t72\t96\t2\t1\tsingle-line-reject\n", key)
		byKey[key] = TextEvent{EntryStep: 1, PostCallStep: 2, Caller: Address{0x37f1, 0x1856},
			OriginalLength: 2, OriginalSHA256: hash, Background: bg, Foreground: fg, Row: 12, Column: 9}
	}
	catalog, err := LoadMenuCatalog([]byte(events.String()), []byte("key\ttranslation\tsource\nmenu.synthetic\t地球\tsynthetic\n"))
	if err != nil {
		t.Fatal(err)
	}
	geometry, err := LoadMenuOverlayRects("synthetic.tsv", []byte(rects.String()))
	if err != nil {
		t.Fatal(err)
	}
	return catalog, geometry, byKey
}

func scopedSyntheticGOLEMFNT() []byte {
	runes := []rune{'X', '地', '球'}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	data := make([]byte, 16+len(runes)*37)
	copy(data[:8], "GOLEMFNT")
	binary.LittleEndian.PutUint16(data[8:10], 16)
	binary.LittleEndian.PutUint16(data[10:12], 16)
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(runes)))
	for i, r := range runes {
		offset := 16 + i*37
		binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(r))
		for j := 0; j < 32; j++ {
			data[offset+5+j] = 0xff
		}
	}
	return data
}

func scopedFontPair(t *testing.T) (*xlate.Font, *xlate.Font) {
	t.Helper()
	base, err := decodeScopedMenuFont(scopedSyntheticGOLEMFNT())
	if err != nil {
		t.Fatal(err)
	}
	base.Name = menuBaseFontName
	derived := manualThreeXFont(base)
	derived.Name = menuThreeXFontName
	return base, derived
}

func scopedEntry(key string, y int) MenuOverlayEntry {
	return MenuOverlayEntry{EventKey: key, TextKey: "menu.synthetic", Translation: "地球",
		Background: 15, Foreground: 0, X: 72, Y: y, Width: 16, Height: 8,
		DrawX: 72, DrawY: y, Capacity: 2, LineCount: 1, Overflow: "single-line-reject"}
}

func TestScopedThreeXBuilderBatchAndLegacyIsolation(t *testing.T) {
	base, derived := scopedFontPair(t)
	var palette [256][3]uint8
	palette[15], palette[0] = [3]uint8{240, 241, 242}, [3]uint8{3, 5, 7}
	a := scopedEntry("function.menu.selected.create_new_character", 96)
	b := scopedEntry("function.menu.selected.add_character_to_team", 104)
	got, err := BuildScopedThreeXMenuOverlay([]MenuOverlayEntry{a, b}, base, derived, palette, 3)
	if err != nil || got == nil || len(got.Events) != 2 {
		t.Fatalf("valid batch=%v err=%v", got, err)
	}
	for i, stamp := range got.Layer.Stamps {
		if stamp.Font != derived || stamp.GlyphX != 1 || stamp.GlyphY != 1 || stamp.GlyphScale != 1 ||
			stamp.BG != palette[15] || stamp.FG != palette[0] || !got.Events[i].Contained {
			t.Fatalf("22px style or geometry changed: %+v", stamp)
		}
	}
	for _, scale := range []int{2, 3, 4} {
		legacy, err := BuildMenuOverlay([]MenuOverlayEntry{a}, base, palette, scale)
		if err != nil || legacy.Layer.Stamps[0].Font != base {
			t.Fatalf("legacy %dx changed: %v", scale, err)
		}
	}
	badUnused := &xlate.Font{Name: base.Name, W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for r, glyph := range base.Glyphs {
		badUnused.Glyphs[r] = append([]byte(nil), glyph...)
	}
	badUnused.Glyphs['X'] = []byte{0xff}
	badDerived := manualThreeXFont(base)
	badDerived.Name = menuThreeXFontName
	badDerived.Glyphs['X'] = []byte{0xff}
	wrongPixels := manualThreeXFont(base)
	wrongPixels.Name = menuThreeXFontName
	wrongPixels.Glyphs['地'][0] ^= 0x80
	wrongName := manualThreeXFont(base)
	missingGlyph := manualThreeXFont(base)
	missingGlyph.Name = menuThreeXFontName
	delete(missingGlyph.Glyphs, 'X')
	race := scopedEntry("race.option.terran", 112)
	postJoin := scopedEntry("post_join.menu.synthetic", 112)
	spoof := scopedEntry("function.menu.selected.create_new_character.spoof", 112)
	badStyle := b
	badStyle.Overflow = "truncate"
	badGeometry := b
	badGeometry.Height = 7
	overlap := b
	overlap.Y, overlap.DrawY = 96, 96
	cases := []struct {
		name           string
		entries        []MenuOverlayEntry
		source, output *xlate.Font
		scale          int
	}{
		{"scale2", []MenuOverlayEntry{a}, base, derived, 2},
		{"scale4", []MenuOverlayEntry{a}, base, derived, 4},
		{"race mixed", []MenuOverlayEntry{a, race}, base, derived, 3},
		{"post-join mixed", []MenuOverlayEntry{a, postJoin}, base, derived, 3},
		{"spoof mixed", []MenuOverlayEntry{a, spoof}, base, derived, 3},
		{"unused bad source glyph", []MenuOverlayEntry{a}, badUnused, derived, 3},
		{"unused bad output glyph", []MenuOverlayEntry{a}, base, badDerived, 3},
		{"unnamed output", []MenuOverlayEntry{a}, base, wrongName, 3},
		{"missing output glyph", []MenuOverlayEntry{a}, base, missingGlyph, 3},
		{"wrong algorithm bytes", []MenuOverlayEntry{a}, base, wrongPixels, 3},
		{"late bad style", []MenuOverlayEntry{a, badStyle}, base, derived, 3},
		{"late bad geometry", []MenuOverlayEntry{a, badGeometry}, base, derived, 3},
		{"late overlap", []MenuOverlayEntry{a, overlap}, base, derived, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			built, err := BuildScopedThreeXMenuOverlay(tc.entries, tc.source, tc.output, palette, tc.scale)
			if built != nil || err == nil {
				t.Fatalf("expected nil,error; got=%v err=%v", built, err)
			}
		})
	}
}

func TestScopedMenuRuntimeCanonicalSequenceAndSnapshots(t *testing.T) {
	catalog, rects, events := scopedMenuFixture(t)
	source := scopedSyntheticGOLEMFNT()
	o, err := newScopedMenuRuntimeOverlay(rects, catalog, source, 3, sha256.Sum256(source))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewScopedMenuRuntimeOverlay(rects, catalog, source, 3); err == nil {
		t.Fatal("public constructor accepted noncanonical source hash")
	}
	source[0] ^= 0xff // owner must retain a private copy.
	keys := []string{"function.menu.selected.create_new_character", "race.option.terran", "function.menu.selected.add_character_to_team"}
	for i, key := range keys {
		event := events[key]
		request, ok := catalog.Resolve(event)
		if !ok {
			t.Fatal(key)
		}
		if i == 0 {
			request.Generation = 7
		} // not part of the three-field canonical display identity.
		forged := request
		for _, mutate := range []func(*DisplayRequest){
			func(r *DisplayRequest) { r.EventKey += ".spoof" },
			func(r *DisplayRequest) { r.TextKey += ".spoof" },
			func(r *DisplayRequest) { r.Translation += "偽" },
		} {
			mutate(&forged)
			before := len(o.layer.Stamps)
			if err := o.Apply(event, forged, [256][3]uint8{}); err == nil || len(o.layer.Stamps) != before {
				t.Fatalf("forged request accepted or mutated layer: %s err=%v", key, err)
			}
			forged = request
		}
		if err := o.Apply(event, request, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		want := o.scoped.derived
		if i == 1 {
			want = o.font
		}
		if len(o.layer.Stamps) != 1 || o.layer.Stamps[0].Font != want {
			t.Fatalf("menu→race→menu font route[%d] wrong", i)
		}
	}
	request, _ := catalog.Resolve(events[keys[0]])
	unknown := events[keys[0]]
	unknown.OriginalSHA256[0] ^= 0xff
	if err := o.Apply(unknown, request, [256][3]uint8{}); err == nil {
		t.Fatal("noncanonical event accepted")
	}
	o.layer.Stamps[0].State = xlate.Shown
	snapshot, err := o.SnapshotScopedLayer()
	if err != nil {
		t.Fatal(err)
	}
	indexed := make([]byte, 320*200)
	before, missing, drew := o.Draw(indexed, [256][3]uint8{})
	if !drew || len(missing) != 0 {
		t.Fatal("menu did not draw")
	}
	if err := o.RestoreScopedLayer(snapshot); err != nil {
		t.Fatal(err)
	}
	after, missing, drew := o.Draw(indexed, [256][3]uint8{})
	if !drew || len(missing) != 0 || !bytes.Equal(before, after) || o.layer.Stamps[0].Font != o.scoped.derived {
		t.Fatal("same-session restore changed font identity or RGBA")
	}
	fingerprint, err := o.ScopedMenuFontFingerprint()
	if err != nil || fingerprint != o.scoped.derivedHash {
		t.Fatal("session fingerprint unavailable")
	}
	realDerived := o.scoped.registry[menuThreeXFontName]
	copyDerived := manualThreeXFont(o.font)
	copyDerived.Name = menuThreeXFontName
	o.scoped.registry[menuThreeXFontName] = copyDerived
	if _, err := o.SnapshotScopedLayer(); err == nil {
		t.Fatal("same-name different-pointer registry accepted")
	}
	if err := o.RestoreScopedLayer(snapshot); err == nil {
		t.Fatal("restore accepted wrong registry pointer")
	}
	o.scoped.registry[menuThreeXFontName] = realDerived
	delete(o.scoped.registry, menuThreeXFontName)
	if err := o.RestoreScopedLayer(snapshot); err == nil {
		t.Fatal("restore accepted missing registry entry")
	}
	o.scoped.registry[menuThreeXFontName] = realDerived
	badSnapshot := bytes.Replace(snapshot, []byte(menuThreeXFontName), []byte("missing.menu.font.v1"), 1)
	if err := o.RestoreScopedLayer(badSnapshot); err == nil {
		t.Fatal("restore accepted unregistered snapshot font")
	}
	if o.layer.Stamps[0].Font != realDerived {
		t.Fatal("failed restore mutated active layer")
	}
	o.scoped.source[0] ^= 0xff
	if err := o.RestoreScopedLayer(snapshot); err == nil {
		t.Fatal("restore accepted source bytes drift")
	}
	o.scoped.source[0] ^= 0xff
	o.scoped.derived.Glyphs['地'][0] ^= 0x80
	if err := o.RestoreScopedLayer(snapshot); err == nil {
		t.Fatal("restore accepted fingerprint drift")
	}
}

func TestScopedMenuRuntimeFixedSourceIfAvailable(t *testing.T) {
	const path = "/project/workplace/current-font/buckrogers-eten-top-pad.golemfnt"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("private local GOLEMFNT is absent")
	}
	if err != nil {
		t.Fatal(err)
	}
	catalog, rects, _ := scopedMenuFixture(t)
	o, err := NewScopedMenuRuntimeOverlay(rects, catalog, data, 3)
	if err != nil {
		t.Fatal(err)
	}
	if o.scoped.derived.Name != menuThreeXFontName || o.scoped.registry[menuThreeXFontName] != o.scoped.derived {
		t.Fatal("fixed local font identity/registry mismatch")
	}
	fingerprint, err := o.ScopedMenuFontFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("local source SHA-256=%s derived Name=%s FontFingerprint=%x", menuFontSourceSHA, menuThreeXFontName, fingerprint)
}

func TestScopedMenuRuntimeFormalLocalCatalogIfAvailable(t *testing.T) {
	paths := []string{
		"/project/text/menu-events.tsv", "/project/text/menu.zh-TW.tsv",
		"/project/text/menu-text-safe-rects.tsv",
		"/project/workplace/current-font/buckrogers-eten-top-pad.golemfnt",
	}
	inputs := make([][]byte, len(paths))
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			t.Skip("local formal TSV or private GOLEMFNT absent")
		}
		if err != nil {
			t.Fatal(err)
		}
		inputs[i] = data
	}
	catalog, err := LoadMenuCatalog(inputs[0], inputs[1])
	if err != nil {
		t.Fatal(err)
	}
	rects, err := LoadMenuOverlayRects("menu-text-safe-rects.tsv", inputs[2])
	if err != nil {
		t.Fatal(err)
	}
	for _, scale := range []int{2, 3} {
		o, err := NewScopedMenuRuntimeOverlay(rects, catalog, inputs[3], scale)
		if err != nil {
			t.Fatal(err)
		}
		count, enlarged := 0, 0
		for identity, entry := range catalog.byIdentity {
			event := TextEvent{EntryStep: 1, PostCallStep: 2, Caller: identity.caller,
				OriginalLength: identity.length, OriginalSHA256: identity.hash,
				Background: identity.background, Foreground: identity.foreground,
				Row: identity.row, Column: identity.column}
			request, ok := catalog.Resolve(event)
			if !ok || request.EventKey != entry.eventKey {
				t.Fatalf("canonical resolve failed for %s", entry.eventKey)
			}
			if err := o.Apply(event, request, [256][3]uint8{}); err != nil {
				t.Fatalf("%dx %s: %v", scale, entry.eventKey, err)
			}
			var active *xlate.Stamp
			for _, stamp := range o.layer.Stamps {
				if stamp.Key == entry.eventKey {
					active = stamp
					break
				}
			}
			if active == nil {
				t.Fatalf("%dx %s missing stamp", scale, entry.eventKey)
			}
			want := o.font
			if scale == 3 && scopedThreeXMenuKeys[entry.eventKey] {
				want, enlarged = o.scoped.derived, enlarged+1
			}
			if active.Font != want {
				t.Fatalf("%dx %s routed to wrong font", scale, entry.eventKey)
			}
			count++
		}
		if count != 21 || enlarged != map[int]int{2: 0, 3: 10}[scale] {
			t.Fatalf("%dx formal coverage=%d enlarged=%d", scale, count, enlarged)
		}
	}
}

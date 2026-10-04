package phantasie

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 以獨立的事件矩形核對原生與兩倍合成，不從疊字內容推導允許區域。
func TestManualPixelsStayInQuestionRows(t *testing.T) {
	for _, scale := range []int{1, 2} {
		for event := 0; event < 4; event++ {
			r, recs := manualFixture()
			for ch, glyph := range r.Font.Glyphs {
				if ch != ' ' {
					for i := range glyph {
						glyph[i] = 0xff
					}
				}
			}
			o := ovNew(t, &Language{Name: "test", Cat: r.Cat, Font: r.Font, Wide: r.Wide, Enabled: true})
			rec := recs[event]
			ovFire(o, rec)
			for _, stamp := range o.Layer.Stamps {
				stamp.State = xlate.Shown
				stamp.FG = [3]uint8{255, 255, 255}
				stamp.BG = [3]uint8{0, 0, 0}
			}
			rgb := make([]byte, 320*200*3)
			before := ComposeImage(nil, nil, rgb, scale, nil)
			after := ComposeImage(o, nil, rgb, scale, func(ch rune) { t.Fatalf("missing %U", ch) })
			changed := 0
			for y := 0; y < 200*scale; y++ {
				for x := 0; x < 320*scale; x++ {
					at := after.PixOffset(x, y)
					if bytes.Equal(before.Pix[at:at+4], after.Pix[at:at+4]) {
						continue
					}
					changed++
					if x >= len(rec.Text)*8*scale || y < rec.Row*8*scale || y >= (rec.Row+1)*8*scale {
						t.Fatalf("scale %d event %d changed outside question at %d,%d", scale, event, x, y)
					}
				}
			}
			if changed == 0 {
				t.Fatalf("scale %d event %d did not draw", scale, event)
			}
		}
	}
}

// 合成資料，只驗證顯示契約，不包含原版答案。
func manualFixture() (*Resolver, []*EventRecord) {
	recs := []*EventRecord{
		{Caller: 0xB795, FmtPtr: 0xC1BC, Row: 10, Format: "What is the item number of a%c", Text: "What is the item number of a ", Args: [12]uint16{' '}},
		{Caller: 0xB7B4, FmtPtr: 0xC1DB, Row: 11, Format: "%s? (page 15,16)", Text: "TEST ITEM? (page 15,16)", Args: [12]uint16{0x1234}, ArgStrs: []ArgStr{{Ptr: 0x1234, Content: "TEST ITEM", Kind: KindStatic}}},
		{Caller: 0xB848, FmtPtr: 0xC1F3, Row: 10, Format: "What is the name", Text: "What is the name"},
		{Caller: 0xB861, FmtPtr: 0xC204, Row: 11, Format: "of spell %d? (back cover)", Text: "of spell 17? (back cover)", Args: [12]uint16{17}},
	}
	var protected []string
	for i, rec := range recs {
		rec.ID, rec.Overlay, rec.FmtKind = string(rune('a'+i)), "ov2", KindStatic
		rec.Cells = []byte(rec.Text)
		protected = append(protected, rec.Format)
	}
	cat := NewCatalog(nil, nil, protected)
	cat.manual = map[string]string{"prompt:item": "ITEM", "item:TEST ITEM": "TEST 99", "prompt:spell": "SPELL", "spell:17": "TEST MAGIC"}
	font := &xlate.Font{Name: "manual-test", W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for c := rune(32); c < 127; c++ {
		font.Glyphs[c] = make([]byte, 32)
	}
	return &Resolver{Cat: cat, Wide: func(rune) bool { return false }, Font: font}, recs
}

func TestManualHintsGuardedAndLiteral(t *testing.T) {
	r, recs := manualFixture()
	want := []string{"ITEM", "TEST 99", "SPELL", "TEST MAGIC"}
	for i, rec := range recs {
		if got := r.Resolve(rec); !got.OK || string(got.Zh) != want[i] {
			t.Fatalf("event %d: %+v", i, got)
		}
		for _, change := range []func(*EventRecord){
			func(x *EventRecord) { x.Caller++ }, func(x *EventRecord) { x.FmtPtr++ },
			func(x *EventRecord) { x.Col++ }, func(x *EventRecord) { x.Row++ },
			func(x *EventRecord) { x.Overlay = "ov1" }, func(x *EventRecord) { x.FmtKind = KindOther },
			func(x *EventRecord) { x.Text += "!" }, func(x *EventRecord) { x.Composed = &Piece{} },
		} {
			bad := *rec
			change(&bad)
			if got := r.Resolve(&bad); got.OK || got.Why != WhyProtected {
				t.Fatalf("unmatched event %d: %+v", i, got)
			}
		}
	}
	r.Cat.(*Catalog).manual["spell:17"] = "100% TEST"
	if got := r.Resolve(recs[3]); !got.OK || string(got.Zh) != "100% TEST" {
		t.Fatal("答案不可再次格式化", got)
	}
	bad := *recs[1]
	bad.Args[0]++
	if r.Resolve(&bad).OK {
		t.Fatal("不符的引數指標")
	}
	bad = *recs[3]
	bad.Args[0] = 55
	bad.Text = "of spell 55? (back cover)"
	if r.Resolve(&bad).OK {
		t.Fatal("超出手冊範圍")
	}
}

func TestManualHintsFallback(t *testing.T) {
	for _, mutate := range []func(*Resolver){
		func(r *Resolver) { r.Cat.(*Catalog).manual = nil },
		func(r *Resolver) { r.Cat.(*Catalog).manual["spell:17"] = "" },
		func(r *Resolver) { r.Cat.(*Catalog).manual["spell:17"] = strings.Repeat("A", 80) },
		func(r *Resolver) { r.Cat.(*Catalog).manual["spell:17"] = "A\nB" },
		func(r *Resolver) { delete(r.Font.Glyphs, 'T') },
		func(r *Resolver) { r.Font = nil },
		func(r *Resolver) { r.Wide = nil },
	} {
		r, recs := manualFixture()
		mutate(r)
		if got := r.Resolve(recs[3]); got.OK || got.Why != WhyProtected {
			t.Fatalf("必須保留原文：%+v", got)
		}
	}
}

func TestManualCatalogParsing(t *testing.T) {
	head := "key\ttranslation\tsource\n"
	for _, bad := range []string{
		"x\tvalue\tsource\n", "spell:0\tvalue\tsource\n", "spell:055\tvalue\tsource\n",
		"spell:1\t\tsource\n", "spell:1\tvalue\n", "spell:1\tvalue\tsource\nspell:1\tother\tsource\n",
		"spell:1\t\\cvalue\tsource\n", "spell:1\ta\\tb\tsource\n",
	} {
		if _, err := parseManualCatalog([]byte(head + bad)); err == nil {
			t.Fatal("錯誤資料應拒絕")
		}
	}
	if _, err := parseManualCatalog([]byte(head + "spell:1\tTEST\tsource\n")); err != nil {
		t.Fatal(err)
	}
}

func TestManualLanguageFailureIsOptional(t *testing.T) {
	// 沿用語言測試的真實 GOLEMFNT fixture。
	dir := t.TempDir()
	fontDir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ui.xx.tsv"), []byte("key\ttranslation\tsource\n"), 0600)
	os.WriteFile(filepath.Join(dir, "prose.xx.tsv"), []byte("key\ttranslation\tsource\n"), 0600)
	os.WriteFile(filepath.Join(dir, "protected.tsv"), []byte("key\tnote\n"), 0600)
	// 一個 ASCII 字模的 16x16 字型。
	b := append([]byte("GOLEMFNT\x10\x00\x10\x00\x01\x00\x00\x00 \x00\x00\x00\x01"), make([]byte, 32)...)
	os.WriteFile(filepath.Join(fontDir, "xx.golemfnt"), b, 0600)
	for _, present := range []bool{false, true} {
		if present {
			os.WriteFile(filepath.Join(dir, "manual.xx.tsv"), []byte("bad"), 0600)
		}
		l := LoadLanguage("xx", dir, fontDir)
		if !l.Enabled || (l.ManualErr != "") != present {
			t.Fatalf("optional=%t: %+v", present, l)
		}
	}
}

func TestManualOverlayLifecycle(t *testing.T) {
	r, recs := manualFixture()
	other, _ := manualFixture()
	other.Font.Name = "manual-other"
	other.Cat.(*Catalog).manual["spell:17"] = "OTHER MAGIC"
	o := ovNew(t,
		&Language{Name: "zh-TW", Cat: r.Cat, Font: r.Font, Wide: r.Wide, Enabled: true},
		&Language{Name: "test", Cat: other.Cat, Font: other.Font, Wide: other.Wide, Enabled: true})
	rec := ovFire(o, recs[3])
	text := func() string {
		var out strings.Builder
		for _, stamp := range o.groupStamps(rec.ID) {
			out.WriteString(string(stamp.Text))
		}
		return strings.TrimSpace(out.String())
	}
	if text() != "TEST MAGIC" || o.C.Get("protected") != 0 {
		t.Fatal("commit did not use hint", text())
	}
	o.OnSave1(987)
	if err := o.SetDisplay("test"); err != nil {
		t.Fatal(err)
	}
	if text() != "OTHER MAGIC" {
		t.Fatal("rebuild did not use hint", text())
	}
	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if o.Drawing() {
		t.Fatal("英文應關閉覆繪")
	}
	if err := o.SetDisplay("zh-TW"); err != nil {
		t.Fatal(err)
	}
	if text() != "TEST MAGIC" {
		t.Fatal("切回未恢復", text())
	}
	o.Layer.Clear(0, 0, 320, 200)
	o.OnLoad(987)
	if text() != "TEST MAGIC" {
		t.Fatal("shadow restore did not use hint", text())
	}
	ovFire(o, ovRec(0, 11, strings.Repeat(" ", len(rec.Text))))
	if len(o.groupStamps(rec.ID)) != 0 {
		t.Fatal("原版清除後殘留提示")
	}
}

package phantasie

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func scrollRec(row int, text string) *EventRecord {
	ptr := uint16(0x2000 + 12 + 40*(row-2))
	r := &EventRecord{BP: 0x2000, SP: 0x1F00, Caller: 0x99F0, Overlay: "ov1",
		Col: 0, Row: row, FmtPtr: 0xC21D, Format: "%s", FmtKind: KindStatic,
		Text: text, Cells: []byte(text), ArgStrs: []ArgStr{{Ptr: ptr, Kind: KindBuffer, Content: text}}}
	r.Args[0] = ptr
	return r
}

func scrollOverlay(t *testing.T, text, tr string) *Overlay {
	t.Helper()
	l := ovLang("zh-TW", nil)
	l.Cat = NewCatalog(nil, map[string]string{Digest(text): tr}, nil)
	o := ovNew(t, l)
	o.SetFont(rcFont())
	return o
}

func TestScrollRecognition(t *testing.T) {
	for _, row := range []int{2, 5, 21} {
		r := scrollRec(row, "Hl")
		if !isScrollRow(r) || displayCells(r) != 40 {
			t.Fatalf("row %d rejected", row)
		}
		before := append([]byte(nil), r.Cells...)
		mask := maskCells(r)
		if len(mask) != 40 || !bytes.Equal(mask[2:], bytes.Repeat([]byte{' '}, 38)) {
			t.Fatal("missing blank tail")
		}
		mask[0] = 'x'
		if !bytes.Equal(r.Cells, before) || r.Text != "Hl" || r.ArgStrs[0].Content != "Hl" {
			t.Fatal("original record was changed")
		}
	}
	cases := []struct {
		name string
		edit func(*EventRecord)
	}{
		{"overlay", func(r *EventRecord) { r.Overlay = "ov2" }},
		{"caller", func(r *EventRecord) { r.Caller++ }},
		{"format pointer", func(r *EventRecord) { r.FmtPtr++ }},
		{"format kind", func(r *EventRecord) { r.FmtKind = KindBuffer }},
		{"format", func(r *EventRecord) { r.Format = "%s " }},
		{"column", func(r *EventRecord) { r.Col = 1 }},
		{"row below", func(r *EventRecord) { r.Row = 1 }},
		{"row above", func(r *EventRecord) { r.Row = 22 }},
		{"composition", func(r *EventRecord) { r.Composed = &Piece{} }},
		{"extra argument", func(r *EventRecord) { r.ArgStrs = append(r.ArgStrs, r.ArgStrs[0]) }},
		{"argument kind", func(r *EventRecord) { r.ArgStrs[0].Kind = KindStatic }},
		{"argument piece", func(r *EventRecord) { r.ArgStrs[0].Piece = &Piece{} }},
		{"argument content", func(r *EventRecord) { r.ArgStrs[0].Content = "Xl" }},
		{"argument pointer", func(r *EventRecord) { r.ArgStrs[0].Ptr++ }},
		{"raw pointer", func(r *EventRecord) { r.Args[0]++ }},
		{"cells", func(r *EventRecord) { r.Cells[0] = 'X' }},
		{"empty", func(r *EventRecord) { r.Text = ""; r.Cells = nil; r.ArgStrs[0].Content = "" }},
		{"forty cells", func(r *EventRecord) {
			r.Text = strings.Repeat("H", 40)
			r.Cells = []byte(r.Text)
			r.ArgStrs[0].Content = r.Text
		}},
		{"control", func(r *EventRecord) { r.Text = "H\n"; r.Cells = []byte(r.Text); r.ArgStrs[0].Content = r.Text }},
		{"missing BP", func(r *EventRecord) { r.BP = 0 }},
		{"pointer overflow", func(r *EventRecord) { r.BP = 0xFFFE; r.ArgStrs[0].Ptr = 10; r.Args[0] = 10 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := scrollRec(5, "Hl")
			c.edit(r)
			if isScrollRow(r) || displayCells(r) != len(r.Text) {
				t.Fatal("unmeasured event expanded")
			}
		})
	}
}

func TestScrollLayoutAndPosition(t *testing.T) {
	r := scrollRec(5, "Hl")
	o := scrollOverlay(t, "Hl", rsC+"甲乙")
	ovFire(o, r)
	d := ovDump(o)
	if len(d) != 3 || d[1].X != 152 || d[1].Y != 40 || d[1].Text != "甲乙" ||
		d[2].X+d[2].Cells*d[2].CellW != 320 {
		t.Fatalf("wrong page center: %+v", d)
	}
	if why := CheckPosition(r, o.Layer.Stamps, posWrites(5, 0, 2)); why != "" {
		t.Fatal(why)
	}
	if x0, y0, x1, y1, ok := Footprint(posWrites(5, 0, 2)); !ok || x0 != 0 || y0 != 40 || x1 != 16 || y1 != 48 {
		t.Fatal("original footprint changed")
	}
	if why := CheckPosition(r, o.Layer.Stamps, posWrites(5, 0, 40)); why == "" {
		t.Fatal("padded original footprint was accepted")
	}
	if why := CheckPosition(r, o.Layer.Stamps[:1], posWrites(5, 0, 2)); why == "" {
		t.Fatal("incomplete visual coverage was accepted")
	}
	if r.Text != "Hl" || string(r.Cells) != "Hl" {
		t.Fatal("capture changed")
	}
	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if err := o.SetDisplay("zh-TW"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d, ovDump(o)) {
		t.Fatal("switch changed centered layout")
	}

	o = scrollOverlay(t, "Hl", "甲乙丙丁戊己庚辛壬癸")
	ovFire(o, scrollRec(5, "Hl"))
	if o.C.Get("truncated") != 0 || ovDump(o)[0].Text != "甲乙丙丁戊己庚辛壬癸" {
		t.Fatal("short source truncated full-page translation")
	}
	o = scrollOverlay(t, "Hl", "<blank>")
	ovFire(o, scrollRec(5, "Hl"))
	d = ovDump(o)
	if len(d) != 1 || d[0].Text != strings.Repeat(" ", 80) {
		t.Fatalf("blank row: %+v", d)
	}
}

func TestWholeLineBlank(t *testing.T) {
	for _, fmt := range []string{"%s", "%s ", " %s", "%4s", "%.1s", "%s%s", "X%s"} {
		r := scrollRec(5, "Hl")
		r.Format = fmt
		ui := map[string]string{}
		if fmt == "X%s" {
			ui[fmt] = "甲%s"
		}
		if fmt == "%s%s" {
			r.ArgStrs = append(r.ArgStrs, r.ArgStrs[0])
		}
		res := (&Resolver{Cat: NewCatalog(ui, map[string]string{Digest("Hl"): "<blank>"}, nil), Wide: ovWide}).Resolve(r)
		if fmt == "%s" {
			if !res.OK || len(res.Zh) != 0 {
				t.Fatalf("whole blank: %+v", res)
			}
		} else if res.OK || res.Why != WhyBadArg {
			t.Fatalf("%q accepted blank: %+v", fmt, res)
		}
	}
	for _, tr := range []string{rsC + "<blank>", "<blank>text"} {
		res := (&Resolver{Cat: NewCatalog(nil, map[string]string{Digest("Hl"): tr}, nil), Wide: ovWide}).Resolve(scrollRec(5, "Hl"))
		if res.OK || res.Why != WhyBadArg {
			t.Fatalf("invalid blank: %+v", res)
		}
	}
	r := scrollRec(5, "Hl")
	r.Composed = &Piece{Fmt: "%s", FmtKind: KindStatic, ArgStrs: r.ArgStrs, Appends: []ArgStr{{Content: "X", Kind: KindOther}}}
	res := (&Resolver{Cat: NewCatalog(nil, map[string]string{Digest("Hl"): "<blank>"}, nil), Wide: ovWide}).Resolve(r)
	if res.OK || res.Why != WhyBadArg {
		t.Fatal("appended template swallowed argument")
	}
	res = (&Resolver{Cat: NewCatalog(map[string]string{"%s": rsC + "%s"}, map[string]string{Digest("Hl"): "<blank>"}, nil), Wide: ovWide}).Resolve(scrollRec(5, "Hl"))
	if res.OK || res.Why != WhyBadArg {
		t.Fatal("centered template accepted blank")
	}
	// Existing nested literal semantics remain intact.
	r = &EventRecord{Format: "X%s", FmtKind: KindStatic, ArgStrs: []ArgStr{{Content: "Hl", Kind: KindBuffer, Piece: &Piece{Fmt: "Hl", FmtKind: KindStatic, Literal: "Hl"}}}}
	res = (&Resolver{Cat: NewCatalog(map[string]string{"X%s": "甲%s", "Hl": "<blank>"}, nil, nil), Wide: ovWide}).Resolve(r)
	if !res.OK || string(res.Zh) != "甲" {
		t.Fatalf("nested literal semantics changed: %+v", res)
	}
}

func TestScrollTailInvalidation(t *testing.T) {
	r := scrollRec(5, "Hl")
	o := scrollOverlay(t, "Hl", "甲乙")
	ovFire(o, r)
	idx := rcScreen(0)
	rcPaint(idx, 0, 40, "Hl", 3, 0)
	rcFrame(o, idx)
	if o.AuditStale(idx) != 0 {
		t.Fatal("fresh row stale")
	}
	o.OnSave1(123)
	// Only the virtual tail is inverted; original text remains unchanged.
	rcFill(idx, 160, 40, 168, 48, 3)
	o.OnInvert(5, 20, 1, false)
	rcFrame(o, idx)
	if o.AuditStale(idx) != 0 {
		t.Fatal("inverted tail stale")
	}
	if _, strict := o.AuditEvents(idx); strict != 0 {
		t.Fatal("inverted tail incorrectly shares source state")
	}
	// A later source event removes only its actual tail rectangle.
	ovFire(o, ovRec(20, 5, "Hello"))
	if ovOpaque(o, 160, 40, 200, 48) != 0 {
		t.Fatal("overwritten tail still covered")
	}
	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if err := o.SetDisplay("zh-TW"); err != nil {
		t.Fatal(err)
	}
	if ovOpaque(o, 160, 40, 200, 48) != 0 {
		t.Fatal("switch revived overwritten tail")
	}
	// Restoring a source page with foreign ink in the tail must gate it out.
	o.OnLoad(123)
	rcPaint(idx, 240, 40, "Hl", 3, 0)
	rcFrame(o, idx)
	if ovOpaque(o, 240, 40, 256, 48) != 0 {
		t.Fatal("restore retained foreign tail ink")
	}
	// A full page clear removes the expanded row.
	o.OnInt10(0x0600, 0, 0, 0x1827)
	if len(o.Layer.Stamps) != 0 {
		t.Fatal("clear left scroll stamps")
	}
}

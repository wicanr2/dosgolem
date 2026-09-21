package buckrogers

import (
	"bytes"
	"testing"
)

const rectFixture = "event_key\tx\ty\twidth\theight\tdraw_x\tdraw_y\tcapacity_cells\tline_count\toverflow_policy\n" +
	"race.option.terran\t8\t24\t64\t8\t8\t24\t8\t1\tsingle-line-reject\n"

func TestRuntimeMenuOverlayApplyFrameAndDraw(t *testing.T) {
	rects, err := LoadMenuOverlayRects("rects.tsv", []byte(rectFixture))
	if err != nil {
		t.Fatal(err)
	}
	o, err := NewRuntimeMenuOverlay(rects, overlayFont(), 2)
	if err != nil {
		t.Fatal(err)
	}
	e := fixtureMenuEvent(t)
	r := DisplayRequest{EventKey: "race.option.terran", TextKey: "race.terran", Translation: "地球"}
	var pal [256][3]uint8
	pal[10] = [3]uint8{1, 2, 3}
	indexed := bytes.Repeat([]byte{0}, 320*200)
	for y := 24; y < 32; y++ {
		for x := 8; x < 72; x++ {
			indexed[y*320+x] = 10
		}
	}
	if err := o.Apply(e, r, pal); err != nil {
		t.Fatal(err)
	}
	o.Frame(indexed, pal)
	rgba, missing, drew := o.Draw(indexed, pal)
	if !drew || len(missing) != 0 || len(rgba) != 640*400*4 || len(o.Actions()) != 1 {
		t.Fatalf("drew=%v missing=%v rgba=%d actions=%d", drew, missing, len(rgba), len(o.Actions()))
	}
}

func TestRuntimeMenuOverlayRejectsFlagsGeometryAndRects(t *testing.T) {
	rects, _ := LoadMenuOverlayRects("rects.tsv", []byte(rectFixture))
	for _, scale := range []int{0, 1, 4} {
		if _, err := NewRuntimeMenuOverlay(rects, overlayFont(), scale); err == nil {
			t.Fatalf("scale %d 應拒絕", scale)
		}
	}
	if _, err := LoadMenuOverlayRects("bad.tsv", []byte("bad\n")); err == nil {
		t.Fatal("錯誤 schema 應拒絕")
	}
	o, _ := NewRuntimeMenuOverlay(rects, overlayFont(), 2)
	e := fixtureMenuEvent(t)
	e.Column++
	if err := o.Apply(e, DisplayRequest{EventKey: "race.option.terran", TextKey: "race.terran", Translation: "地球"}, [256][3]uint8{}); err == nil {
		t.Fatal("runtime 幾何漂移應拒絕")
	}
}

func TestMergeMenuOverlayRectsRejectsDuplicate(t *testing.T) {
	r, _ := LoadMenuOverlayRects("rects.tsv", []byte(rectFixture))
	if _, err := MergeMenuOverlayRects(r, r); err == nil {
		t.Fatal("重複 event key 應拒絕")
	}
}

func TestRuntimeMenuOverlayClearTextCells(t *testing.T) {
	rects, _ := LoadMenuOverlayRects("rects.tsv", []byte(rectFixture))
	o, _ := NewRuntimeMenuOverlay(rects, overlayFont(), 2)
	e := fixtureMenuEvent(t)
	r := DisplayRequest{EventKey: "race.option.terran", TextKey: "race.terran", Translation: "地球"}
	if err := o.Apply(e, r, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	if err := o.ClearTextCells(3, 8, 3, 1); err != nil || len(o.ActiveKeys()) != 0 {
		t.Fatalf("已覆蓋的疊字應清除：err=%v keys=%v", err, o.ActiveKeys())
	}
	if err := o.ClearTextCells(2, 1, 3, 1); err == nil {
		t.Fatal("反向矩形應拒絕")
	}
}

func TestRuntimeMenuOverlaySupersedesSameOrigin(t *testing.T) {
	rects, _ := LoadMenuOverlayRects("rects.tsv", []byte(rectFixture))
	o, _ := NewRuntimeMenuOverlay(rects, overlayFont(), 2)
	e := fixtureMenuEvent(t)
	first := DisplayRequest{EventKey: "race.option.terran", TextKey: "first", Translation: "地球"}
	if err := o.Apply(e, first, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	second := DisplayRequest{EventKey: "race.option.terran", TextKey: "second", Translation: "地球"}
	if err := o.Apply(e, second, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	keys := o.ActiveKeys()
	if len(keys) != 1 || keys[0] != "race.option.terran" {
		t.Fatalf("同原點新輸出應取代舊輸出：%v", keys)
	}
}

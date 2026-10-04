package phantasie

import "testing"

func dimPaint(idx []uint8, col, row, width int, color uint8) {
	for _, r := range []int{2, 4} {
		for x := col * 8; x < (col+width)*8; x++ {
			idx[(row*8+r)*screenW+x] = color
		}
	}
}

func TestAuditDimRequiresKnownOperation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		row, width int
		dim        bool
		strict     int
	}{
		{"known", 3, 5, true, 0},
		{"other-row", 4, 5, true, 1},
		{"zero-width", 3, 0, true, 1},
		{"negative-width", 3, -1, true, 1},
		{"ordinary-invert", 3, 5, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, idx := auNew(t)
			dimPaint(idx, 2, 3, 5, 0)
			o.OnInvert(tc.row, 2, tc.width, tc.dim)
			rcFrame(o, idx)
			auWant(t, o, idx, 0, 0, tc.strict)
		})
	}
}

func TestAuditDimChecksRemainingPixels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		alter  func([]uint8)
		strict int
	}{
		{"clean", func([]uint8) {}, 0},
		{"erased-pixel-noise", func(idx []uint8) { idx[26*screenW+16] = 2 }, 1},
		{"other-line-noise", func(idx []uint8) { idx[25*screenW+17] = 2 }, 1},
		{"different-line-colors", func(idx []uint8) {
			for x := 16; x < 24; x++ {
				idx[28*screenW+x] = 3
			}
		}, 1},
		{"adjacent-unqualified-cell", func(idx []uint8) { dimPaint(idx, 4, 3, 1, 0) }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, idx := auNew(t)
			dimPaint(idx, 2, 3, 2, 0)
			o.OnInvert(3, 2, 2, true)
			tc.alter(idx)
			rcFrame(o, idx)
			// stale=0 確保雜色負例仍通過粗略字模判定，確實測到 strict。
			auWant(t, o, idx, 0, 0, tc.strict)
		})
	}
}

func TestAuditDimAfterInvertAndRestoration(t *testing.T) {
	o, idx := auNew(t)
	o.OnSave1(1) // 未變暗的舊頁面仍可由普通模型通過。
	dimPaint(idx, 2, 3, 5, 0)
	o.OnInvert(3, 2, 5, true)
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0)
	o.OnSave1(2)
	for y := 24; y < 32; y++ {
		for x := 16; x < 56; x++ {
			idx[y*screenW+x] ^= 3
		}
	}
	o.OnInvert(3, 2, 5, false)
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0) // 清除線現在同為3。
	if err := o.SetDisplay("en"); err != nil {
		t.Fatal(err)
	}
	if err := o.SetDisplay("zh-TW"); err != nil {
		t.Fatal(err)
	}
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0)
	dimPaint(idx, 2, 3, 5, 0)
	rcPaint(idx, 16, 24, "HHHHl", 3, 0)
	dimPaint(idx, 2, 3, 5, 0)
	o.OnLoad(2)
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0)
	rcPaint(idx, 16, 24, "HHHHl", 3, 0)
	o.OnLoad(1)
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0)
}

func TestAuditDimUsesMovedEventRow(t *testing.T) {
	o, idx := auNew(t)
	o.Layer.Scroll(0, 0, 320, 200, 8)
	rcPaint(idx, 16, 32, "HHHHl", 3, 0)
	dimPaint(idx, 2, 4, 5, 0)
	o.OnInvert(4, 2, 5, true)
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0)
}

func TestAuditDimRejectsInconsistentGroupRow(t *testing.T) {
	o := rcNew(t, "HHHHl", "甲A乙")
	rcDraw(t, o, 2, 3, "HHHHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHHHl", 3, 0)
	rcFrame(o, idx)
	if len(o.Layer.Stamps) < 2 {
		t.Fatal("合成事件應有兩段")
	}
	o.Layer.Stamps[1].Y += 8
	o.OnInvert(3, 2, 5, true)
	if len(o.log[0].dimCells) != 0 {
		t.Fatal("不一致列不應新增變暗資格")
	}
}

func TestAuditDimNewEventAndLogEviction(t *testing.T) {
	o, idx := auNew(t)
	dimPaint(idx, 2, 3, 5, 0)
	o.OnInvert(3, 2, 5, true)
	rcFrame(o, idx)
	if len(o.log[0].dimCells) != 5 {
		t.Fatal("已知變暗資格缺失")
	}
	old := o.log[0].ID
	rcDraw(t, o, 2, 3, "HHHHl")
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 1)
	if len(o.log[1].dimCells) != 0 {
		t.Fatal("同文同位置的新事件不能繼承資格")
	}
	for n := 0; n < MaxLog; n++ {
		o.appendLog(&EventRecord{ID: "new", Text: "HHHHl"})
	}
	if len(o.log) != MaxLog {
		t.Fatalf("Log長度=%d，要%d", len(o.log), MaxLog)
	}
	for _, e := range o.log {
		if e.ID == old || len(e.dimCells) > 0 {
			t.Fatal("淘汰後不能保留舊資格")
		}
	}
}

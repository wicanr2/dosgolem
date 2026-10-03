package phantasie

import (
	"testing"
)

// 獨立稽核（docs/spec/005 §5.1）的單元測試。合成字型與畫面，期望值為字面值，不使用任何原版素材。
// 稽核以字模遮罩與目前色號畫面為基準，不使用疊字層自己的顏色狀態。

// auNew 建一個已顯示的事件 g1：原文 "HHHHl"（col 2，row 3），譯文 "甲乙丙丁戊"，畫面是正常極性（墨 3、底 0）。
func auNew(t *testing.T) (*Overlay, []uint8) {
	t.Helper()
	o := rcNew(t, "HHHHl", "甲乙丙丁戊")
	rcDraw(t, o, 2, 3, "HHHHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHHHl", 3, 0)
	rcFrame(o, idx)
	return o, idx
}

func auWant(t *testing.T, o *Overlay, idx []uint8, stale, exposed, strict int) {
	t.Helper()
	if got := o.AuditStale(idx); got != stale {
		t.Errorf("AuditStale = %d，要 %d", got, stale)
	}
	if got := o.AuditExposed(idx); got != exposed {
		t.Errorf("AuditExposed = %d，要 %d", got, exposed)
	}
	if ex, st := o.AuditEvents(idx); ex != exposed || st != strict {
		t.Errorf("AuditEvents = (%d, %d)，要 (%d, %d)", ex, st, exposed, strict)
	}
}

func TestAuditCleanScreenReportsNothing(t *testing.T) {
	o, idx := auNew(t)
	auWant(t, o, idx, 0, 0, 0)
}

// 疊字被移除而畫面仍顯示原文：英文外露。
func TestAuditExposedWhenStampsAreGone(t *testing.T) {
	o, idx := auNew(t)
	o.Layer.Stamps = nil
	auWant(t, o, idx, 0, 1, 0)
}

// 只有一部分格沒被疊字蓋住也算外露（覆蓋檢查是整個事件矩形）。
func TestAuditExposedWhenOneCellUncovered(t *testing.T) {
	o, idx := auNew(t)
	setTransparent(o.Layer.Stamps[0], 2)
	auWant(t, o, idx, 0, 1, 0)
}

// 被後來事件覆寫的格不屬於原事件：A 的格 2、3 被未譯事件 B 以同字形覆寫（Clear 使 A 的對應格透明），
// 畫面仍像原文，但那兩格是 B 畫的，稽核不得把它算成 A 的外露。
func TestAuditExposedIgnoresCellsOverwrittenByLaterEvent(t *testing.T) {
	o := rcNew(t, "HHHH", "甲乙丙丁")
	rcDraw(t, o, 2, 3, "HHHH")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHHH", 3, 0)
	rcFrame(o, idx)
	b := rcRec("", 4, 3, "HH") // 未譯：沒有對應的 ui 鍵
	o.Begin(b, false)
	o.End()
	rcCount(t, o, "translated", 1)
	if s := o.Layer.Stamps[0]; len(s.Transparent) < 4 || !s.Transparent[2] || !s.Transparent[3] {
		t.Fatalf("A 的格 2、3 應被 B 的 Clear 轉透明，得到 %v", s.Transparent)
	}
	auWant(t, o, idx, 0, 0, 0)

	// 對照：B 的矩形沒有登記時，A 的透明格會被算成外露。
	o.rects = nil
	if got := o.AuditExposed(idx); got != 1 {
		t.Errorf("沒有後續事件矩形時 AuditExposed = %d，要 1", got)
	}
}

// 疊字蓋在已不是原文的畫面上：殘字。
func TestAuditStaleWhenCellRepainted(t *testing.T) {
	o, idx := auNew(t)
	rcPaint(idx, 24, 24, "l", 3, 0) // 第 1 格 'H' 換成 'l'，不呼叫 Frame
	if got := o.AuditStale(idx); got != 1 {
		t.Errorf("AuditStale = %d，要 1", got)
	}
}

// 整個事件被同一色號填滿：每個非透明疊字格都計為殘字。
func TestAuditStaleWhenGroupFilledWithOneColor(t *testing.T) {
	o, idx := auNew(t)
	rcFill(idx, 16, 24, 56, 32, 2)
	if got := o.AuditStale(idx); got != 5 {
		t.Errorf("AuditStale = %d，要 5", got)
	}
}

// 開啟中事件（A 已觸發、B 未觸發）的矩形略過：畫面被半寫不是缺陷。
func TestAuditSkipsOpenEvent(t *testing.T) {
	o, idx := auNew(t)
	rcFill(idx, 16, 24, 56, 32, 2)
	o.Begin(rcRec("", 2, 3, "HHHHl"), false) // 同一矩形的新事件，尚未 End
	if got := o.AuditStale(idx); got != 0 {
		t.Errorf("開啟中事件的矩形 AuditStale = %d，要 0", got)
	}
	if ex, st := o.AuditEvents(idx); ex != 0 || st != 0 {
		t.Errorf("開啟中事件的矩形 AuditEvents = (%d, %d)，要 (0, 0)", ex, st)
	}
}

// 事件只有一部分被反白：正常判定（反白格以對調後的底墨計），疊字被移除時仍判得出外露。
func TestAuditPartiallyInvertedEvent(t *testing.T) {
	o := rcNew(t, "HHHHl", "甲乙丙丁戊")
	rcDraw(t, o, 2, 3, "HHHHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHH", 0, 3) // 第 0 至 2 格反白
	rcPaint(idx, 40, 24, "Hl", 3, 0)
	rcFrame(o, idx)
	auWant(t, o, idx, 0, 0, 0)
	o.Layer.Stamps = nil
	// 反白格要被認成「仍顯示原文」，否則整個事件被略過而判不出外露。
	if got := o.AuditExposed(idx); got != 1 {
		t.Errorf("疊字移除後 AuditExposed = %d，要 1（反白格要判為仍顯示原文）", got)
	}
}

// 遮罩對應率：同一反白狀態內，遮罩為 0 的像素必須全為同一色號、非 0 的像素全為另一色號。
func TestAuditStrictMaskMapping(t *testing.T) {
	o, idx := auNew(t)
	// 第 0 格 'H' 的 4 個墨像素改成色號 2：格仍一致（墨保留 32/36），但墨像素有兩種色號。
	for i, p := range rcCellPixels('H', true) {
		if i < 4 {
			idx[(24+p[1])*screenW+16+p[0]] = 2
		}
	}
	if ex, st := o.AuditEvents(idx); ex != 0 || st != 1 {
		t.Errorf("AuditEvents = (%d, %d)，要 (0, 1)", ex, st)
	}
}

// 反白與正常兩種狀態分開統計：各自單一色號時 strict 為 0；反白那一側有雜色時只算那一側。
func TestAuditStrictCountsStatesSeparately(t *testing.T) {
	o := rcNew(t, "HHHHl", "甲乙丙丁戊")
	rcDraw(t, o, 2, 3, "HHHHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHH", 0, 3)
	rcPaint(idx, 40, 24, "Hl", 3, 0)
	rcFrame(o, idx)
	if ex, st := o.AuditEvents(idx); ex != 0 || st != 0 {
		t.Fatalf("乾淨的部分反白 AuditEvents = (%d, %d)，要 (0, 0)", ex, st)
	}
	// 反白那一側的第 0 格有 4 個墨像素（色號 0）改成色號 1。
	for i, p := range rcCellPixels('H', true) {
		if i < 4 {
			idx[(24+p[1])*screenW+16+p[0]] = 1
		}
	}
	if ex, st := o.AuditEvents(idx); ex != 0 || st != 1 {
		t.Errorf("反白側有雜色 AuditEvents = (%d, %d)，要 (0, 1)", ex, st)
	}
}

// 底與墨都是前景色的格（純色填滿）不是反白：swap 為 false，不得被判成反白而對調顏色。
func TestCellStatSolidForegroundCellIsNotInverted(t *testing.T) {
	o, idx := auNew(t)
	rcFill(idx, 24, 24, 32, 32, 3) // 第 1 格整格填前景色 3
	rec, stamps := o.records["g1"], o.groupStamps("g1")
	cells := o.scanCells(rec, stamps, idx, 0, 3)
	for k, want := range []bool{false, false, false, false, false} {
		if cells[k].swap != want {
			t.Errorf("第 %d 格 swap = %v，要 %v", k, cells[k].swap, want)
		}
	}
	// 對照：整格真的反白（底 3、墨 0）才是 swap。
	rcPaint(idx, 24, 24, "H", 0, 3)
	cells = o.scanCells(rec, stamps, idx, 0, 3)
	if !cells[1].swap {
		t.Errorf("反白的第 1 格 swap = false，要 true")
	}
	if cells[0].swap || cells[2].swap {
		t.Errorf("其他格不應是 swap：%v %v", cells[0].swap, cells[2].swap)
	}
}

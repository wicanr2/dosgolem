package strikecommander

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// strike_commander 規格 005 §4.1：簽章層、§3.1 第 5 條的清空、參考列、Overlay 的逐標籤比對。
// 沿用 observer_test.go 的合成 harness 與 killboard_test.go 的 board harness。

// refFromFrame 從 320×200 畫格切出 [Y+1, Y+1+h) × [0, 320)，越界列填 0（005 §4.1 的測試輔助）。
// h 在這裡獨立寫死（label 14、board 8），不呼叫 refRows，好讓「board 的 h 改成 14」的負對照失敗。
func refFromFrame(frame []byte, l Label) []byte {
	h := 14
	if l.Kind == KindBoard {
		h = 8
	}
	out := make([]byte, h*320)
	for y := l.Y + 1; y < l.Y+1+h; y++ {
		if y < 0 || y >= 200 {
			continue
		}
		copy(out[(y-l.Y-1)*320:(y-l.Y)*320], frame[y*320:(y+1)*320])
	}
	return out
}

func hasLog(o *LabelObserver, kind string) bool {
	log, _ := o.Log()
	for _, e := range log {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// --- 第 1 條：簽章相符時不清空 ---

func TestSigNoReset(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	gen, last := h.o.Generation(), h.commits[0].BlitReturn
	for i := uint64(1); i <= 3; i++ {
		h.f.steps = last + i*SigCheckSteps - 1
		h.idle()
		h.idle()
	}
	h.f.steps = last + 3*SigCheckSteps + 1
	h.idle()
	if keys(h.o.Visible()) != "Talk to Gwen" || h.o.Generation() != gen {
		t.Fatalf("簽章相符、經過 3 × SigCheckSteps 步：visible %q gen %d，要 Talk to Gwen、%d", keys(h.o.Visible()), h.o.Generation(), gen)
	}
	for _, k := range []string{"sig", "reset"} {
		if hasLog(h.o, k) {
			t.Fatalf("log 有 %q 紀錄", k)
		}
	}
	if _, ok := h.o.Base(); !ok {
		t.Fatal("簽章相符時 base 被停用")
	}
}

// --- 第 2 條：簽章層 ---

// sigMutation 是改壞簽章的一種方式。
type sigMutation struct {
	name string
	do   func(h *harness)
}

func sigMutations() []sigMutation {
	ms := []sigMutation{{"puts 入口", func(h *harness) { h.f.w8(tBase+segText, ipPutsEntry, 0x90) }}}
	for _, r := range baseRows {
		r := r
		ms = append(ms, sigMutation{r.name, func(h *harness) {
			for i, b := range r.bytes {
				if b >= 0 {
					h.f.w8(tBase+r.seg, r.off+uint16(i), b^0xFF)
					return
				}
			}
		}})
	}
	return ms
}

func TestSigLayer(t *testing.T) {
	t.Run("未滿 SigCheckSteps 不檢查", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		last := h.commits[0].BlitReturn
		h.f.w8(tBase+segText, ipPutsEntry, 0x90)
		h.f.steps = last + SigCheckSteps - 1
		h.idle()
		if len(h.o.Visible()) != 1 || hasLog(h.o, "sig") {
			t.Fatalf("未滿 %d 步就清空：visible %q", SigCheckSteps, keys(h.o.Visible()))
		}
		if _, ok := h.o.Base(); !ok {
			t.Fatal("未滿時 base 被停用")
		}
	})
	for _, m := range sigMutations() {
		t.Run("滿了 "+m.name, func(t *testing.T) {
			h := newHarness(t)
			h.label("Talk to Gwen", 114, 204)
			h.blit()
			gen, last := h.o.Generation(), h.commits[0].BlitReturn
			// 配對進行中（candidates 有一筆）且待提交清單有一筆。
			h.label("Fly Mission", 124, 195)
			h.blitEntry()
			h.label("Talk to Gwen", 114, 204)
			if !h.o.paired || len(h.o.candidates) != 1 || len(h.o.Pending()) != 1 {
				t.Fatalf("前置：paired %t candidates %d pending %d", h.o.paired, len(h.o.candidates), len(h.o.Pending()))
			}
			m.do(h)
			h.f.steps = last + SigCheckSteps
			h.idle()
			if len(h.o.Visible()) != 0 || h.o.Generation() != gen+1 {
				t.Fatalf("%s 不符：visible %q gen %d，要空、%d", m.name, keys(h.o.Visible()), h.o.Generation(), gen+1)
			}
			if h.o.baseKnown || h.o.baseOK {
				t.Fatalf("%s 不符：baseKnown %t baseOK %t，要 false", m.name, h.o.baseKnown, h.o.baseOK)
			}
			if len(h.o.Pending()) != 0 || h.o.paired || len(h.o.candidates) != 0 || h.o.cur != nil {
				t.Fatalf("%s 不符：pending %d paired %t candidates %d", m.name, len(h.o.Pending()), h.o.paired, len(h.o.candidates))
			}
			if !hasLog(h.o, "sig") {
				t.Fatalf("%s 不符：log 沒有 sig 紀錄", m.name)
			}
			h.blitReturn()
			if len(h.commits) != 1 {
				t.Fatal("配對旗標清除後的 blit 返回仍提交")
			}
		})
	}
	t.Run("恢復", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		last := h.commits[0].BlitReturn
		h.f.w8(tBase+segText, ipPutsEntry, 0x90)
		h.f.steps = last + SigCheckSteps
		h.idle()
		if len(h.o.Visible()) != 0 {
			t.Fatal("前置：沒有清空")
		}
		h.installProgram(tBase)
		h.label("Fly Mission", 124, 195)
		if b, ok := h.o.Base(); !ok || b != tBase {
			t.Fatalf("簽章恢復後 puts 入口沒有重新定位：%04X %t", b, ok)
		}
		h.blit()
		if len(h.commits) != 2 || keys(h.o.Visible()) != "Fly Mission" {
			t.Fatalf("恢復後提交 %d 次、visible %q", len(h.commits), keys(h.o.Visible()))
		}
	})
}

// --- 第 3 條：停用 base、定位到不同 base 時清空 ---

func TestSigBaseDrop(t *testing.T) {
	t.Run("puts 入口簽章不符而停用", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		gen := h.o.Generation()
		h.f.w8(tBase+segText, ipPutsEntry, 0x90)
		h.setString("Fly Mission")
		h.putsEntry()
		if len(h.o.Visible()) != 0 || h.o.Generation() != gen+1 || !hasLog(h.o, "sig") {
			t.Fatalf("停用 base：visible %q gen %d（要空、%d），sig 紀錄 %t", keys(h.o.Visible()), h.o.Generation(), gen+1, hasLog(h.o, "sig"))
		}
	})
	t.Run("定位到不同 base", func(t *testing.T) {
		const base2 = 0x0300
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		gen := h.o.Generation()
		h.installProgram(base2)
		h.f.w16(tDS, tPutsSP+4, tObj)
		h.f.w16(tDS, tPutsSP+6, tStr)
		h.f.bp = tP
		h.step(base2+segText, ipPutsEntry, tPutsSP)
		if b, ok := h.o.Base(); !ok || b != base2 {
			t.Fatalf("沒有定位到 %04X：%04X %t", base2, b, ok)
		}
		if len(h.o.Visible()) != 0 || h.o.Generation() != gen+1 || !hasLog(h.o, "sig") {
			t.Fatalf("不同 base：visible %q gen %d（要空、%d），sig 紀錄 %t", keys(h.o.Visible()), h.o.Generation(), gen+1, hasLog(h.o, "sig"))
		}
	})
	t.Run("集合原本為空時 generation 不加", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.f.w8(tBase+segText, ipPutsEntry, 0x90)
		h.putsEntry()
		if h.o.Generation() != 0 {
			t.Fatalf("可見集合為空時 gen %d，要 0", h.o.Generation())
		}
	})
}

// --- 第 4 條：參考列 ---

// bufRows 讀 harness 緩衝經列表 list 定址的列 [y0, y1)。
func (h *harness) bufRows(list uint16, y0, y1 int) []byte {
	out := make([]byte, (y1-y0)*320)
	for y := y0; y < y1; y++ {
		off := h.f.r16(tDS, list+uint16(2*y))
		for x := 0; x < 320; x++ {
			out[(y-y0)*320+x] = h.f.mem[lin(tBufSeg, off+uint16(x))]
		}
	}
	return out
}

func TestCommitRef(t *testing.T) {
	t.Run("label 讀返回時的緩衝", func(t *testing.T) {
		h := newHarness(t)
		for y := 170; y < 195; y++ {
			for x := 0; x < 320; x++ {
				h.bufPx(x, y, byte(x+y))
			}
		}
		h.label("Talk to Gwen", 114, 204)
		h.blitEntry()
		h.bufPx(150, 180, 0xEE) // blit 進行中改緩衝
		h.idle()
		h.blitReturn()
		ref := h.o.Visible()[0].Ref
		if len(ref) != 14*320 {
			t.Fatalf("label Ref 長度 %d，要 %d", len(ref), 14*320)
		}
		if want := h.bufRows(tList, 176, 190); string(ref) != string(want) {
			t.Fatal("label Ref 不等於返回時緩衝的 [176, 190) 列")
		}
		if ref[(180-176)*320+150] != 0xEE {
			t.Fatal("Ref 不是返回時讀的")
		}
		if c := h.commits[0].Labels[0].Ref; string(c) != string(ref) {
			t.Fatal("Commit.Labels 的 Ref 與可見集合不同")
		}
	})
	t.Run("board 長度 8 × 320", func(t *testing.T) {
		h := newBoardHarness(t)
		for x := 0; x < 320; x++ {
			h.bufPx(x, 50, byte(x))
		}
		h.boardPuts("GROUND", tBoardSeg, 0x1369, 177, 250, 45, 45, nil)
		h.blit()
		ref := h.o.Visible()[0].Ref
		if len(ref) != 8*320 {
			t.Fatalf("board Ref 長度 %d，要 %d", len(ref), 8*320)
		}
		if want := h.bufRows(tList, 46, 54); string(ref) != string(want) {
			t.Fatal("board Ref 不等於緩衝的 [46, 54) 列")
		}
	})
	t.Run("取自 blit 來源描述元的列表", func(t *testing.T) {
		const tList2 = 0xC000
		h := newHarness(t)
		for y := 0; y < 200; y++ {
			h.f.w16(tDS, tList2+uint16(2*y), uint16(0x8000+y*320))
		}
		for y := 176; y < 190; y++ {
			for x := 0; x < 320; x++ {
				h.bufPx(x, y, 0x11)                                // tList 定址的列
				h.f.w8(tBufSeg, uint16(0x8000+y*320+x), int(0x22)) // tList2 定址的列
			}
		}
		h.f.w16(tDS, tObj+objList, tList2) // 標籤自己的列表是 tList2，blit 來源仍是 tList
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		vis := h.o.Visible()
		if len(vis) != 1 || vis[0].List != tList2 {
			t.Fatalf("前置：visible %+v", vis)
		}
		if want := h.bufRows(tList, 176, 190); string(vis[0].Ref) != string(want) {
			t.Fatalf("Ref 要取自 blit 來源描述元的列表 %04X（第一個位元組 %02X）", tList, vis[0].Ref[0])
		}
	})
}

// --- 第 5 到 10 條：Overlay ---

// lfPal 是測試色盤：每個色號顏色不同。
func lfPal() [256][3]uint8 {
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(255 - i), uint8(i * 7)}
	}
	return pal
}

// lfFrame 是有紋理的畫格：色號依 (x, y) 變化，但避開 0 與 15（label 的遮蓋與字色）。
func lfFrame() []byte {
	idx := make([]byte, 64000)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			idx[y*320+x] = byte(20 + (x*3+y*5)%200)
		}
	}
	return idx
}

func lfLabel(idx []byte) Label {
	l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
	l.Ref = refFromFrame(idx, l)
	return l
}

func lfBoard(idx []byte) Label {
	l := Label{Key: "GROUND", X0: 177, X1: 250, Y: 45, Font: boardFont, Kind: KindBoard}
	l.Before, l.After = refFromFrame(idx, l), refFromFrame(idx, l)
	l.Ref = refFromFrame(idx, l)
	return l
}

func lfOverlay(t *testing.T, labels []Label) *Overlay {
	t.Helper()
	ov := textOverlay(t, testTextFonts(t), map[string]string{"Talk to Gwen": "測試譯文甲", "GROUND": "地面", "Fly Mission": "測試丙"}, 2)
	ov.Sync(labels, 1)
	if len(ov.Stamps()) != len(labels) {
		t.Fatalf("stamp %d 個，要 %d（每筆一個遮蓋）", len(ov.Stamps()), len(labels))
	}
	return ov
}

// coverOf 回 key 的遮蓋 stamp。
func coverOf(t *testing.T, ov *Overlay, key string) *xlate.Stamp {
	t.Helper()
	for _, s := range ov.Stamps() {
		if s.Key == key+".cover" {
			return s
		}
	}
	t.Fatalf("沒有 %s.cover", key)
	return nil
}

// boxDiffs 回比對框（1×）[x0, x1) × [y0, y1) 內合成圖與原版放大圖不同的輸出像素數。
func boxDiffs(out, idx []byte, pal [256][3]uint8, x0, y0, x1, y1 int) int {
	return diffsIn(out, ScaleIndexedRGBA(idx, pal, 2), 2, x0, y0, x1, y1)
}

func TestOverlayMatchPixel(t *testing.T) {
	pal := lfPal()
	for _, kind := range []string{"label", "board"} {
		t.Run(kind, func(t *testing.T) {
			base := lfFrame()
			l := lfLabel(base)
			h := 14
			if kind == "board" {
				l, h = lfBoard(base), 8
			}
			ov := lfOverlay(t, []Label{l})
			cv := coverOf(t, ov, l.Key)
			x0, x1, y0, y1 := cv.X, cv.X+cv.Cells*cv.CellW, l.Y+1, l.Y+1+h
			// 不改畫格：照畫。
			if out, _ := ov.Draw(base, pal); len(ov.Hidden) != 0 || boxDiffs(out, base, pal, x0, y0, x1, y1) == 0 {
				t.Fatalf("畫格與 Ref 相同時要照畫：Hidden %v", ov.Hidden)
			}
			in := []Point{{x0, y0}, {x1 - 1, y0}, {x0, y1 - 1}, {x1 - 1, y1 - 1}, {(x0 + x1) / 2, (y0 + y1) / 2}}
			for _, p := range in {
				idx := append([]byte(nil), base...)
				idx[p.Y*320+p.X] ^= 0x40
				out, _ := ov.Draw(idx, pal)
				if strings.Join(ov.Hidden, "|") != l.Key {
					t.Errorf("比對框內 (%d, %d) 改 1 點：Hidden %v，要 [%s]", p.X, p.Y, ov.Hidden, l.Key)
				}
				if n := boxDiffs(out, idx, pal, 0, 0, 320, 200); n != 0 {
					t.Errorf("比對框內 (%d, %d) 改 1 點：合成圖與原版放大圖差 %d 點，要 0", p.X, p.Y, n)
				}
			}
			out := []Point{{x0 - 1, (y0 + y1) / 2}, {x1, (y0 + y1) / 2}, {(x0 + x1) / 2, l.Y}, {(x0 + x1) / 2, l.Y + 1 + h}}
			for _, p := range out {
				idx := append([]byte(nil), base...)
				idx[p.Y*320+p.X] ^= 0x40
				o, _ := ov.Draw(idx, pal)
				if len(ov.Hidden) != 0 || boxDiffs(o, idx, pal, x0, y0, x1, y1) == 0 {
					t.Errorf("比對框外 (%d, %d) 改 1 點：Hidden %v，要照畫", p.X, p.Y, ov.Hidden)
				}
			}
		})
	}
}

func TestOverlayMatchPerLabel(t *testing.T) {
	pal := lfPal()
	base := lfFrame()
	a := lfLabel(base)
	b := Label{Key: "Fly Mission", X0: 124, X1: 195, Y: 140, Font: measuredFont, Kind: KindLabel}
	b.Ref = refFromFrame(base, b)
	ov := lfOverlay(t, []Label{a, b})
	idx := append([]byte(nil), base...)
	idx[180*320+150] ^= 0x40 // 只在 a 的框內
	out, _ := ov.Draw(idx, pal)
	if strings.Join(ov.Hidden, "|") != "Talk to Gwen" {
		t.Fatalf("Hidden %v，要只有 Talk to Gwen", ov.Hidden)
	}
	ca, cb := coverOf(t, ov, a.Key), coverOf(t, ov, b.Key)
	if n := boxDiffs(out, idx, pal, ca.X, 176, ca.X+ca.CellW, 190); n != 0 {
		t.Errorf("不符的 a 框內差 %d 點，要 0", n)
	}
	if n := boxDiffs(out, idx, pal, cb.X, 141, cb.X+cb.CellW, 155); n == 0 {
		t.Error("相符的 b 沒有照畫")
	}
}

func TestOverlayMatchInvalid(t *testing.T) {
	pal := lfPal()
	base := lfFrame()
	cases := map[string]func(l *Label){
		"Ref 為 nil": func(l *Label) { l.Ref = nil },
		"Ref 長度不對":  func(l *Label) { l.Ref = l.Ref[:13*320] },
		"Ref 長度多一列": func(l *Label) { l.Ref = append(l.Ref, make([]byte, 320)...) },
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			l := lfLabel(base)
			mod(&l)
			ov := lfOverlay(t, []Label{l})
			out, _ := ov.Draw(base, pal)
			if strings.Join(ov.Hidden, "|") != l.Key || boxDiffs(out, base, pal, 0, 0, 320, 200) != 0 {
				t.Fatalf("%s：Hidden %v，要不畫", name, ov.Hidden)
			}
		})
	}
	t.Run("indexed 長度不足", func(t *testing.T) {
		ov := lfOverlay(t, []Label{lfLabel(base)})
		if got := ov.Match(base[:63999]); strings.Join(got, "|") != "Talk to Gwen" {
			t.Fatalf("Match(63999 bytes) = %v，要不符", got)
		}
		if got := ov.Match(base); len(got) != 0 {
			t.Fatalf("對照：Match(64000 bytes) = %v，要相符", got)
		}
	})
}

// stampGeom 回 stamp 除 BG、FG 以外的欄位快照。
func stampGeom(ov *Overlay) string {
	var b strings.Builder
	for _, s := range ov.Stamps() {
		fmt.Fprintf(&b, "%s %d %d,%d %d×%d×%d %q %d,%d %d %p|", s.Key, s.State, s.X, s.Y, s.Cells, s.CellW, s.CellH, string(s.Text), s.GlyphX, s.GlyphY, s.GlyphScale, s.Font)
	}
	return b.String()
}

func TestOverlayMatchRecover(t *testing.T) {
	pal := lfPal()
	base := lfFrame()
	l := lfLabel(base)
	ov := lfOverlay(t, []Label{l})
	bad := append([]byte(nil), base...)
	bad[180*320+150] ^= 0x40
	ov.Draw(base, pal)
	snap := stampGeom(ov)
	if _, _ = ov.Draw(bad, pal); strings.Join(ov.Hidden, "|") != l.Key {
		t.Fatalf("不符的 Draw：Hidden %v", ov.Hidden)
	}
	out, _ := ov.Draw(base, pal)
	if len(ov.Hidden) != 0 || boxDiffs(out, base, pal, 114, 176, 204, 190) == 0 {
		t.Fatalf("下一次相符的 Draw 沒有恢復：Hidden %v", ov.Hidden)
	}
	if got := stampGeom(ov); got != snap {
		t.Fatalf("Draw 改了 stamp：\n%s\n%s", snap, got)
	}
}

func TestOverlayRefUpdate(t *testing.T) {
	a := lfFrame()
	b := append([]byte(nil), a...)
	b[180*320+150] ^= 0x40
	l := lfLabel(a)
	ov := lfOverlay(t, []Label{l})
	if len(ov.Match(a)) != 0 || len(ov.Match(b)) != 1 {
		t.Fatal("前置：第一次 Ref 取自 a")
	}
	first := ov.Stamps()[0]
	l2 := l
	l2.Ref = refFromFrame(b, l)
	ov.Sync([]Label{l2}, 1)
	if ov.Stamps()[0] != first {
		t.Fatal("generation 不變時重建了 stamp")
	}
	if len(ov.Match(b)) != 0 || len(ov.Match(a)) != 1 {
		t.Fatalf("generation 不變的 Sync 沒有改用新 Ref：Match(b) %v Match(a) %v", ov.Match(b), ov.Match(a))
	}
	l3 := l
	l3.Ref = nil
	ov.Sync([]Label{l3}, 1)
	if len(ov.Match(b)) != 0 {
		t.Fatal("新 Ref 為 nil 時要沿用舊值")
	}
}

func TestOverlayMatchDrawConsistent(t *testing.T) {
	pal := lfPal()
	base := lfFrame()
	a := lfLabel(base)
	b := Label{Key: "Fly Mission", X0: 124, X1: 195, Y: 140, Font: measuredFont, Kind: KindLabel}
	b.Ref = refFromFrame(base, b)
	ov := lfOverlay(t, []Label{a, b})
	frames := map[string][]byte{"相同": base}
	for _, p := range []Point{{150, 180}, {150, 150}, {0, 0}} {
		idx := append([]byte(nil), base...)
		idx[p.Y*320+p.X] ^= 0x40
		frames[fmt.Sprint(p)] = idx
	}
	both := append([]byte(nil), base...)
	both[180*320+150] ^= 0x40
	both[150*320+150] ^= 0x40
	frames["兩筆都不符"] = both
	for name, idx := range frames {
		m := ov.Match(idx)
		ov.Draw(idx, pal)
		if strings.Join(m, "|") != strings.Join(ov.Hidden, "|") {
			t.Errorf("%s：Match %v、Draw Hidden %v 不一致", name, m, ov.Hidden)
		}
	}
}

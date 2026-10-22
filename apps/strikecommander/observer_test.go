package strikecommander

import (
	"strings"
	"testing"
)

// 合成的 OPTTEST 執行環境（001 §3.1）：記憶體是可改寫的 map，暫存器由測試設定，
// 每次 step 呼叫一次 Observe，步數加 1。

const (
	tBase   = 0x0241
	tDS     = 0x4E66
	tObj    = 0xD61A
	tList   = 0xD07E
	tBufSeg = 0xC290
	tStr    = 0x2000
	tSrc    = 0xE000
	tDst    = 0xE7E8
	tP      = 0xFC20
	tQ      = 0xFC40
	tPutsSP = 0xFC00
	tPutcSP = 0xFBF0
	tBlitSP = 0xFB00
)

type fakeView struct {
	mem                    map[uint32]byte
	steps                  uint64
	cs, ip, ss, sp, bp, ds uint16
}

func (f *fakeView) Steps() uint64        { return f.steps }
func (f *fakeView) CS() uint16           { return f.cs }
func (f *fakeView) IP() uint16           { return f.ip }
func (f *fakeView) SS() uint16           { return f.ss }
func (f *fakeView) SP() uint16           { return f.sp }
func (f *fakeView) BP() uint16           { return f.bp }
func (f *fakeView) DS() uint16           { return f.ds }
func (f *fakeView) Read8(a uint32) uint8 { return f.mem[a] }

func (f *fakeView) w8(seg, off uint16, b ...int) {
	for i, c := range b {
		f.mem[lin(seg, off+uint16(i))] = byte(c)
	}
}

func (f *fakeView) w16(seg, off, v uint16) { f.w8(seg, off, int(v&0xFF), int(v>>8)) }

func (f *fakeView) r16(seg, off uint16) uint16 {
	return uint16(f.mem[lin(seg, off)]) | uint16(f.mem[lin(seg, off+1)])<<8
}

// harness 把一個 LabelObserver 與一個 fakeView 綁在一起。
type harness struct {
	t       *testing.T
	f       *fakeView
	o       *LabelObserver
	commits []Commit
}

func newHarness(t *testing.T) *harness {
	f := &fakeView{mem: map[uint32]byte{}, steps: 1000, ss: tDS, ds: tDS}
	h := &harness{t: t, f: f, o: NewLabelObserver()}
	h.o.OnCommit = func(c Commit) { h.commits = append(h.commits, c) }
	h.installProgram(tBase)
	// 輸出物件與緩衝描述元
	f.w16(tDS, tObj+objSeg, tBufSeg)
	f.w16(tDS, tObj+objList, tList)
	f.w16(tDS, tObj+objFont, measuredFont)
	for y := 0; y < 200; y++ {
		f.w16(tDS, tList+uint16(2*y), uint16(y*320+4))
	}
	// blit 描述元：全螢幕
	h.setDesc(tSrc, tBufSeg, fullScreen)
	h.setDesc(tDst, 0xFFFF, fullScreen)
	// 呼叫鏈：P 是格式化函式框架，Q 是輸出函式框架
	h.setChain(ipFmtRet, tBase+segText, ipLabelRet, tBase+segLabel)
	return h
}

func (h *harness) installProgram(base uint16) {
	h.f.w8(base+segText, ipPutsEntry, sigPutsEntry...)
	for _, r := range baseRows {
		b := append([]int(nil), r.bytes...)
		fill := base + segText
		wild := 0
		for i := range b {
			if b[i] < 0 {
				b[i] = int(fill >> (8 * wild) & 0xFF)
				wild++
			}
		}
		h.f.w8(base+r.seg, r.off, b...)
	}
}

func (h *harness) setDesc(desc, seg uint16, r rect) {
	h.f.w16(tDS, desc, seg)
	h.f.w16(tDS, desc+2, tList)
	h.f.w16(tDS, desc+4, uint16(r.x0))
	h.f.w16(tDS, desc+6, uint16(r.y0))
	h.f.w16(tDS, desc+8, uint16(r.x1))
	h.f.w16(tDS, desc+10, uint16(r.y1))
}

func (h *harness) setChain(p2, p4, q2, q4 uint16) {
	h.f.w16(tDS, tP, tQ)
	h.f.w16(tDS, tP+2, p2)
	h.f.w16(tDS, tP+4, p4)
	h.f.w16(tDS, tQ+2, q2)
	h.f.w16(tDS, tQ+4, q4)
}

func (h *harness) step(cs, ip, sp uint16) {
	h.f.cs, h.f.ip, h.f.sp = cs, ip, sp
	h.o.Observe(h.f)
	h.f.steps++
}

// idle 跑一步與觀察器無關的指令。
func (h *harness) idle() { h.step(0x1234, 0x5678, 0xF000) }

func (h *harness) setString(s string) {
	b := make([]int, len(s)+1)
	for i := range s {
		b[i] = int(s[i])
	}
	h.f.w8(tDS, tStr, b...)
}

// putsEntry 送出 puts 入口：[SP+4] = obj、[SP+6] = str，BP = P。
func (h *harness) putsEntry() {
	h.f.w16(tDS, tPutsSP+4, tObj)
	h.f.w16(tDS, tPutsSP+6, tStr)
	h.f.bp = tP
	h.step(tBase+segText, ipPutsEntry, tPutsSP)
}

func (h *harness) putc(obj uint16) {
	h.f.w16(tDS, tPutcSP+4, obj)
	h.step(tBase+segText, ipPutc, tPutcSP)
}

func (h *harness) putsReturn() { h.step(tBase+segText, ipPutsRet, tPutsSP) }

func (h *harness) setCursor(x, y uint16) {
	h.f.w16(tDS, tObj+objX, x)
	h.f.w16(tDS, tObj+objY, y)
}

// label 模擬一次完整的標籤輸出：進入時游標是上一行留下的 (CCh, 175)，
// 第一次 putc 時游標已置中到 x0，最後一次 putc 之後再前進到 x1。
func (h *harness) label(s string, x0, x1 int) {
	h.setString(s)
	h.setCursor(0xCC, 175)
	h.putsEntry()
	h.idle()
	h.setCursor(uint16(x0), 175)
	h.putc(tObj)
	h.setCursor(uint16(x0+5), 175)
	h.putc(tObj)
	h.setCursor(uint16(x1-6), 175) // 最後一次 putc 入口時的游標
	h.putc(tObj)
	h.setCursor(uint16(x1), 175) // putc 畫完後前進
	h.putsReturn()
}

func (h *harness) blitEntry() {
	h.f.w16(tDS, tBlitSP+4, tSrc)
	h.f.w16(tDS, tBlitSP+6, tDst)
	h.step(tBase+segBlit, ipBlitEntry, tBlitSP)
}

func (h *harness) blitReturn() { h.step(tBase+segBlit, ipBlitRet, tBlitSP) }

func (h *harness) blit() {
	h.blitEntry()
	h.idle()
	h.blitReturn()
}

func keys(ls []Label) string {
	var k []string
	for _, l := range ls {
		k = append(k, l.Key)
	}
	return strings.Join(k, "|")
}

// --- 第 1 項：定位 ---

func TestLocateBase(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	if b, ok := h.o.Base(); !ok || b != tBase {
		t.Fatalf("Base %04X %t，要 %04X true", b, ok, tBase)
	}
	if len(h.o.Pending()) != 1 {
		t.Fatalf("待提交 %d 筆，要 1", len(h.o.Pending()))
	}
}

func TestLocateRowMismatchDisablesBase(t *testing.T) {
	for _, r := range baseRows {
		t.Run(r.name, func(t *testing.T) {
			h := newHarness(t)
			// 改壞該列的第一個固定位元組（標籤呼叫點另外也改壞 base 決定的段值）
			for i, b := range r.bytes {
				if b >= 0 {
					h.f.w8(tBase+r.seg, r.off+uint16(i), b^0xFF)
					break
				}
			}
			h.label("Talk to Gwen", 114, 204)
			if _, ok := h.o.Base(); ok {
				t.Fatalf("%s 不符時 base 仍啟用", r.name)
			}
			if len(h.o.Pending()) != 0 {
				t.Fatalf("base 停用時仍記下標籤")
			}
		})
	}
	t.Run("標籤呼叫點段值", func(t *testing.T) {
		h := newHarness(t)
		h.f.w8(tBase+segLabel, ipLabelCall+15, 0x00, 0x00)
		h.label("Talk to Gwen", 114, 204)
		if _, ok := h.o.Base(); ok {
			t.Fatal("段值不符時 base 仍啟用")
		}
	})
}

func TestLocateSameCSDifferentProgram(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	if len(h.o.Pending()) != 1 {
		t.Fatalf("第一個程式：待提交 %d 筆，要 1", len(h.o.Pending()))
	}
	// 同一段換成另一個程式：puts 入口簽章不符。
	h.f.w8(tBase+segText, ipPutsEntry, 0x90, 0x90, 0x90)
	h.label("Fly Mission", 124, 195)
	if p := h.o.Pending(); len(p) > 1 || (len(p) == 1 && p[0].Key != "Talk to Gwen") {
		t.Fatalf("第二個程式被當成 puts：待提交 %q", keys(p))
	}
}

// --- 第 2 項：呼叫端 ---

func TestCallerCheck(t *testing.T) {
	good := [4]uint16{ipFmtRet, tBase + segText, ipLabelRet, tBase + segLabel}
	cases := []struct {
		name string
		idx  int
		val  uint16
	}{
		{"P+2", 0, 0x0620},
		{"P+4", 1, tBase + segText + 1},
		{"Q+2", 2, 0x082C},
		{"Q+4", 3, tBase + segLabel + 1},
		{"Q+2=0677", 2, 0x0677},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			v := good
			v[c.idx] = c.val
			h.setChain(v[0], v[1], v[2], v[3])
			h.label("Talk to Gwen", 114, 204)
			if len(h.o.Pending()) != 0 {
				t.Fatalf("%s 不符時仍接受", c.name)
			}
			h.blit()
			if len(h.commits) != 1 || h.commits[0].Accepted != 0 || h.commits[0].Ignored != 1 {
				t.Fatalf("提交 %+v，要接受 0、忽略 1", h.commits)
			}
		})
	}
	t.Run("全部相符", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		if len(h.commits) != 1 || h.commits[0].Accepted != 1 || h.commits[0].Ignored != 0 || keys(h.commits[0].Labels) != "Talk to Gwen" {
			t.Fatalf("提交 %+v，要接受 1、忽略 0", h.commits)
		}
	})
}

// --- 第 3 項：紀錄 ---

func TestRecordCursor(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 0x72, 204)
	p := h.o.Pending()
	if len(p) != 1 {
		t.Fatalf("待提交 %d 筆", len(p))
	}
	l := p[0]
	if l.X0 != 0x72 {
		t.Errorf("X0 %#x，要 0x72（不是 puts 入口的 0xCC）", l.X0)
	}
	if l.X1 != 204 {
		t.Errorf("X1 %d，要 puts 返回時的 204（不是最後一次 putc 入口的 198）", l.X1)
	}
	if l.Y != 175 || l.Font != measuredFont || l.Seg != tBufSeg || l.List != tList || l.Key != "Talk to Gwen" {
		t.Errorf("紀錄 %+v", l)
	}
}

func TestRecordPutcOtherObj(t *testing.T) {
	t.Run("先不符再相符", func(t *testing.T) {
		h := newHarness(t)
		h.setString("Talk to Gwen")
		h.setCursor(0xCC, 175)
		h.putsEntry()
		h.setCursor(50, 175)
		h.putc(0x1111) // 別的物件
		h.setCursor(114, 175)
		h.putc(tObj)
		h.setCursor(204, 175)
		h.putsReturn()
		p := h.o.Pending()
		if len(p) != 1 || p[0].X0 != 114 {
			t.Fatalf("待提交 %+v，要 X0 114", p)
		}
	})
	t.Run("只有不符", func(t *testing.T) {
		h := newHarness(t)
		h.setString("Talk to Gwen")
		h.setCursor(0xCC, 175)
		h.putsEntry()
		h.setCursor(114, 175)
		h.putc(0x1111)
		h.setCursor(204, 175)
		h.putsReturn()
		if p := h.o.Pending(); len(p) != 0 {
			t.Fatalf("沒有相符的 putc 仍記下 %+v", p)
		}
	})
}

func TestRecordBadStrings(t *testing.T) {
	t.Run("空字串", func(t *testing.T) {
		h := newHarness(t)
		h.setString("")
		h.putsEntry()
		h.putsReturn()
		if len(h.o.Pending()) != 0 {
			t.Fatal("空字串被記下")
		}
	})
	for _, s := range []string{"Talk\rto", "Talk\nto", "Talk\x80to"} {
		t.Run("含控制字元", func(t *testing.T) {
			h := newHarness(t)
			h.label(s, 114, 204)
			if len(h.o.Pending()) != 0 {
				t.Fatalf("%q 未被捨棄", s)
			}
		})
	}
	t.Run("64 bytes 內沒有 00h", func(t *testing.T) {
		h := newHarness(t)
		h.label(strings.Repeat("A", 70), 10, 300)
		if len(h.o.Pending()) != 0 {
			t.Fatal("沒有結尾的字串未被捨棄")
		}
		h2 := newHarness(t)
		h2.label(strings.Repeat("A", 63), 10, 300)
		if len(h2.o.Pending()) != 1 {
			t.Fatal("63 字加 00h 應接受")
		}
	})
}

func TestRecordOtherFont(t *testing.T) {
	h := newHarness(t)
	h.f.w16(tDS, tObj+objFont, 0x1234)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	vis := h.o.Visible()
	if len(vis) != 1 || vis[0].Font != 0x1234 {
		t.Fatalf("其他字型的標籤要照記：%+v", vis)
	}
	ov := textOverlay(t, testTextFonts(t), testCatalog(), 2)
	ov.Sync(vis, h.o.Generation())
	if len(ov.Stamps()) != 0 || len(ov.TextBoxes()) != 0 || len(ov.Skips) != 1 {
		t.Fatalf("字型 1234 不得產生 stamp：stamps %d skips %+v", len(ov.Stamps()), ov.Skips)
	}
}

func TestRecordPendingLimit(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 17; i++ {
		h.label(string(rune('A'+i)), 10, 20)
	}
	p := h.o.Pending()
	if len(p) != 16 || p[0].Key != "B" || p[15].Key != "Q" {
		t.Fatalf("待提交 %q，要 B 到 Q 共 16 筆", keys(p))
	}
}

// --- 第 4 項：提交 ---

func TestCommitOnReturnOnly(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	h.blitEntry()
	if len(h.o.Visible()) != 0 || h.o.Generation() != 0 || len(h.commits) != 0 {
		t.Fatalf("blit 入口就提交了：visible %q gen %d", keys(h.o.Visible()), h.o.Generation())
	}
	h.idle()
	h.blitReturn()
	if keys(h.o.Visible()) != "Talk to Gwen" || h.o.Generation() != 1 || len(h.commits) != 1 {
		t.Fatalf("返回後 visible %q gen %d", keys(h.o.Visible()), h.o.Generation())
	}
}

func TestCommitEmptySecondFrame(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	h.blit()
	if len(h.o.Visible()) != 0 || h.o.Generation() != 2 {
		t.Fatalf("第二次提交後 visible %q gen %d，要空、2", keys(h.o.Visible()), h.o.Generation())
	}
}

func TestCommitUnchangedKeepsGeneration(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.label("Talk to Gwen", 114, 204)
		h.blit()
	}
	if h.o.Generation() != 1 || len(h.commits) != 3 {
		t.Fatalf("集合不變 gen %d（要 1），提交 %d", h.o.Generation(), len(h.commits))
	}
	h.label("Talk to Gwen", 115, 204) // X0 不同
	h.blit()
	if h.o.Generation() != 2 {
		t.Fatalf("X0 改變後 gen %d，要 2", h.o.Generation())
	}
}

func TestCommitSegMismatch(t *testing.T) {
	h := newHarness(t)
	h.f.w16(tDS, tObj+objSeg, 0xAD22)
	h.label("Fly Mission", 124, 195)
	h.f.w16(tDS, tObj+objSeg, tBufSeg)
	h.blit()
	if len(h.o.Visible()) != 0 {
		t.Fatalf("緩衝段不同的紀錄被提交：%q", keys(h.o.Visible()))
	}
	h.blit()
	if len(h.o.Visible()) != 0 || len(h.commits) != 2 || len(h.commits[1].Labels) != 0 {
		t.Fatalf("緩衝段不同的紀錄出現在下一次提交：%q", keys(h.o.Visible()))
	}
}

func TestCommitPartialBlit(t *testing.T) {
	t.Run("不相交", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		h.label("Fly Mission", 124, 195)
		h.setDesc(tDst, 0xFFFF, rect{0, 0, 100, 50})
		h.setDesc(tSrc, tBufSeg, rect{0, 0, 100, 50})
		h.blit()
		if keys(h.o.Visible()) != "Talk to Gwen" || h.o.Generation() != 1 || len(h.commits) != 1 {
			t.Fatalf("非全螢幕 blit 改了可見集合：%q gen %d", keys(h.o.Visible()), h.o.Generation())
		}
		if keys(h.o.Pending()) != "Fly Mission" {
			t.Fatalf("非全螢幕 blit 後待提交 %q，要保留", keys(h.o.Pending()))
		}
		h.setDesc(tDst, 0xFFFF, fullScreen)
		h.setDesc(tSrc, tBufSeg, fullScreen)
		h.blit()
		if keys(h.o.Visible()) != "Fly Mission" {
			t.Fatalf("下一次全螢幕提交 %q", keys(h.o.Visible()))
		}
	})
	t.Run("相交", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		h.setDesc(tDst, 0xFFFF, rect{200, 180, 210, 182}) // 含端點，x 200 到 210 壓到 [114, 204)
		h.setDesc(tSrc, tBufSeg, rect{200, 180, 210, 182})
		h.blitEntry()
		if len(h.o.Visible()) != 0 || h.o.Generation() != 2 {
			t.Fatalf("壓到墨跡框的非全螢幕 blit 沒清空：%q gen %d", keys(h.o.Visible()), h.o.Generation())
		}
	})
	t.Run("相鄰不相交", func(t *testing.T) {
		h := newHarness(t)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		h.setDesc(tDst, 0xFFFF, rect{204, 176, 210, 189})
		h.setDesc(tSrc, tBufSeg, rect{204, 176, 210, 189})
		h.blit()
		if len(h.o.Visible()) != 1 {
			t.Fatal("x 204 起的矩形不壓 [114, 204)，不得清空")
		}
	})
}

func TestCommitUnpairedReturn(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	h.blitReturn() // 沒有入口
	if keys(h.o.Visible()) != "Talk to Gwen" || h.o.Generation() != 1 || len(h.commits) != 1 {
		t.Fatalf("沒有配對入口的返回被處理：%q gen %d", keys(h.o.Visible()), h.o.Generation())
	}
	// SP 不同的返回也不提交
	h.label("Fly Mission", 124, 195)
	h.blitEntry()
	h.step(tBase+segBlit, ipBlitRet, tBlitSP-2)
	if len(h.commits) != 1 {
		t.Fatal("SP 不同的返回被當成配對")
	}
	h.blitReturn()
	if keys(h.o.Visible()) != "Fly Mission" {
		t.Fatalf("配對返回後 %q", keys(h.o.Visible()))
	}
}

func TestCommitLabelAfterBlit(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	h.label("Fly Mission", 124, 195)
	if keys(h.o.Visible()) != "Talk to Gwen" {
		t.Fatalf("blit 之後的標籤進了這次提交：%q", keys(h.o.Visible()))
	}
	h.blit()
	if keys(h.o.Visible()) != "Fly Mission" {
		t.Fatalf("下一次提交 %q", keys(h.o.Visible()))
	}
}

// TestCommitReset 已由 005 §4.1 第 1 條 TestSigNoReset（labelframe_test.go）取代。

func TestCommitAcceptedIgnored(t *testing.T) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	// 不上畫面的 0677 呼叫，接著下一畫格的標籤呼叫
	h.setChain(ipFmtRet, tBase+segText, 0x0677, tBase+segLabel)
	h.label("Talk to Gwen", 114, 204)
	h.setChain(ipFmtRet, tBase+segText, ipLabelRet, tBase+segLabel)
	h.label("Talk to Gwen", 114, 204)
	h.blit()
	c := h.commits[1]
	if c.Accepted != 1 || c.Ignored != 1 || len(c.Labels) != 1 {
		t.Fatalf("第二次提交 %+v，要接受 1、忽略 1", c)
	}
}

// --- §2.8 墨跡框核對（收據工具用） ---

func TestInkCheck(t *testing.T) {
	run := func(x0, x1 int, paint func(h *harness)) InkResult {
		h := newHarness(t)
		h.o.InkCheck = true
		h.setString("Talk to Gwen")
		h.setCursor(0xCC, 175)
		h.putsEntry()
		h.setCursor(uint16(x0), 175)
		h.putc(tObj)
		h.setCursor(uint16(x1), 175)
		h.putsReturn()
		paint(h)
		h.blit()
		if len(h.commits) != 1 || len(h.commits[0].Ink) != 1 {
			t.Fatalf("沒有核對結果")
		}
		return h.commits[0].Ink[0]
	}
	px := func(h *harness, x, y int) { h.f.w8(tBufSeg, uint16(y*320+4+x), 15) }
	box := func(h *harness) {
		for x := 114; x < 204; x++ {
			px(h, x, 176)
		}
		for y := 176; y < 189; y++ {
			px(h, 114, y)
			px(h, 203, y)
		}
	}
	if r := run(114, 204, box); !r.OK() {
		t.Fatalf("框內差異應通過：%s", r)
	}
	if r := run(114, 204, func(h *harness) { box(h); px(h, 204, 180) }); r.OK() || r.Outside != 1 {
		t.Fatalf("框外差異應失敗：%s", r)
	}
	if r := run(114, 204, func(h *harness) { box(h); px(h, 150, 190) }); r.OK() {
		t.Fatalf("Y+15 列的差異應失敗：%s", r)
	}
	if r := run(110, 204, box); r.OK() || r.Left {
		t.Fatalf("框畫大（左緣沒有墨跡）應失敗：%s", r)
	}
}

package strikecommander

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// strike_commander 規格 004 §4.1 的單元測試：存讀檔對話框的繁中覆繪。
//
// 原版衍生物（SAVELOAD.SHP 與畫格 .idx）不進 Git：從環境變數 DOSGOLEM_SC_I015 指向的目錄
// （strike_commander 的 workplace/i015）讀 SAVELOAD.SHP、l8/pre.idx、l8/hover.idx、s2/pre.idx、t1/tsave.idx。
// 讀不到時 t.Skip 並印出原因；主代理驗收時必須提供，跳過不算通過。
//
// 字帶、墨跡框、墨跡色、條目尺寸與 Key 都在測試內獨立寫出（004 §2.2、§3.3、§3.4，RE 015 §2），
// 不迭代實作的表。

const (
	i015Env         = "DOSGOLEM_SC_I015"
	scGfxCatalogEnv = "DOSGOLEM_SC_GFX_CATALOG"
)

// --- 測試內獨立寫出的數值 ---

// tElem 是 004 §2.2 的一列（一般態）加上 RE 015 §2 的營房位置。band、ink 是 x0、y0、x1、y1（含端點，相對元素）。
type tElem struct {
	k, w, h   int
	band, ink [4]int
	x, y      int
	key       string
}

var tElems = []tElem{
	{0, 182, 131, [4]int{44, 6, 139, 17}, [4]int{47, 9, 133, 14}, 80, 30, "saveload/80,30/0"},
	{1, 107, 12, [4]int{8, 0, 103, 11}, [4]int{11, 3, 102, 8}, 114, 36, "saveload/114,36/1"},
	{2, 75, 12, [4]int{20, 1, 59, 9}, [4]int{24, 3, 51, 7}, 134, 127, "saveload/134,127/2"},
	{4, 75, 12, [4]int{20, 1, 59, 9}, [4]int{22, 3, 53, 7}, 134, 127, "saveload/134,127/4"},
	{6, 80, 12, [4]int{12, 1, 67, 9}, [4]int{15, 3, 63, 7}, 90, 142, "saveload/90,142/6"},
	{8, 80, 12, [4]int{2, 1, 77, 9}, [4]int{3, 3, 74, 7}, 174, 142, "saveload/174,142/8"},
}

// tElemFor 回 k（0 到 9）的元素列；按下態 {3,5,7,9} 用 k−1 的列、墨跡框下移 1 列。
func tElemFor(t *testing.T, k int) tElem {
	t.Helper()
	n, pressed := k, k == 3 || k == 5 || k == 7 || k == 9
	if pressed {
		n = k - 1
	}
	for _, e := range tElems {
		if e.k == n {
			if pressed {
				e.ink[1]++
				e.ink[3]++
			}
			return e
		}
	}
	t.Fatalf("k%d 不是有字的元素", k)
	return tElem{}
}

// tInk 是 0 組的墨跡色 F（004 §3.4）。
var tInk = map[uint8]bool{15: true, 20: true, 26: true, 34: true, 45: true, 49: true, 58: true, 66: true,
	74: true, 82: true, 93: true, 198: true, 199: true, 200: true}

// tSizes 是 RE 015 §2 的條目尺寸（k 0 到 20，三組相同）。
var tSizes = [21][2]int{
	{182, 131}, {107, 12}, {75, 12}, {75, 12}, {75, 12}, {75, 12}, {80, 12}, {80, 12}, {80, 12}, {80, 12},
	{20, 11}, {20, 11}, {20, 10}, {20, 10}, {10, 5}, {132, 44}, {95, 9}, {46, 12}, {46, 12}, {44, 12}, {44, 12},
}

const tSaveLoadSHA = "617f7270097913ea2c9841035fd201d4196d084e2408cc593eaf056d7f97ef6a"

// tPal 是測試色盤：每個色號的 RGB 都不同。
func tPal() [256][3]uint8 {
	var p [256][3]uint8
	for i := range p {
		p[i] = [3]uint8{uint8(i), uint8(255 - i), uint8(i * 37)}
	}
	return p
}

func testGfx() map[string]string {
	b, err := os.ReadFile("testdata/menu_gfx.test.tsv")
	if err != nil {
		panic(err)
	}
	c, err := ParseCatalog(b)
	if err != nil {
		panic(err)
	}
	return c
}

// --- 原版衍生物 ---

func i015Dir(t *testing.T) string {
	t.Helper()
	d := os.Getenv(i015Env)
	if d == "" {
		t.Skip(i015Env + " 未設定：004 §4.1 需要 workplace/i015 的 SAVELOAD.SHP 與畫格")
	}
	return d
}

func loadSHP(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(i015Dir(t), "SAVELOAD.SHP"))
	if err != nil {
		t.Skip("讀不到 SAVELOAD.SHP：" + err.Error())
	}
	if s := sha256.Sum256(b); hex.EncodeToString(s[:]) != tSaveLoadSHA || len(b) != 42014 {
		t.Fatalf("SAVELOAD.SHP 長度 %d、SHA-256 %x，要 42014、%s", len(b), s, tSaveLoadSHA)
	}
	return b
}

func loadIdx(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(i015Dir(t), rel))
	if err != nil {
		t.Skip("讀不到 " + rel + "：" + err.Error())
	}
	if len(b) != 64000 {
		t.Fatalf("%s 長度 %d，要 64000", rel, len(b))
	}
	return b
}

// tEntries 以測試自己的程式讀 PAK 位移表，回每個條目的長度（獨立於 pakEntries）。
func tEntries(file []byte) [][2]int {
	n := int(binary.LittleEndian.Uint32(file[4:])) / 4 // 第一個位移 / 4 = 1 + 條目數
	var out [][2]int
	for i := 0; i < n-1; i++ {
		s := int(binary.LittleEndian.Uint32(file[4+4*i:]) & 0xFFFFFF)
		e := len(file)
		if i+1 < n-1 {
			e = int(binary.LittleEndian.Uint32(file[4+4*(i+1):]) & 0xFFFFFF)
		}
		out = append(out, [2]int{s, e})
	}
	return out
}

func mustDecode(t *testing.T, file []byte, e int) Shape {
	t.Helper()
	s, err := decodeEntry(file, e)
	if err != nil {
		t.Fatalf("條目 %d：%v", e, err)
	}
	return s
}

// mkEvent 依條目 e 與位置組一個繪圖事件（解碼由第 2 項另外驗證）。
func mkEvent(t *testing.T, file []byte, e, x, y, order int) ArtEvent {
	t.Helper()
	g, k := e/21, e%21
	ent, b0 := mustDecode(t, file, e), mustDecode(t, file, k)
	return ArtEvent{Steps: uint64(order), G: g, K: k, X: x, Y: y, Order: order, W: ent.W, H: ent.H, Entry: ent.Pix, Base0: b0.Pix}
}

// paint 把條目 e 的解碼圖畫進 320×200 畫格（不透明實心矩形，RE 015 §2）。
func paint(t *testing.T, frame, file []byte, e, x, y int) {
	t.Helper()
	s := mustDecode(t, file, e)
	for yy := 0; yy < s.H; yy++ {
		for xx := 0; xx < s.W; xx++ {
			frame[(y+yy)*320+x+xx] = s.Pix[yy*s.W+xx]
		}
	}
}

// --- 合成 View：在 observer_test.go 的 harness 上加 shape 繪製常式與圖檔 ---

const (
	tSHPSeg  = 0xF5BC
	tShapeSP = 0xFA00
	tClip    = 0xFCE5
	tCurDesc = 0xDB2E
)

func (h *harness) installShape(file []byte) {
	h.f.w8(tBase+segShape, 0, sigShapeEntry...)
	for i, b := range file {
		h.f.mem[lin(tSHPSeg, 0)+uint32(i)] = b
	}
	h.setDesc(tClip, 0xFFFF, rect{93, 82, 103, 110})
	h.setDesc(tCurDesc, 0xA000, fullScreen)
}

// shape 送出一次 shape 繪製常式入口：[SP+4] 描述元、+6 x、+8 y、+0Ah 圖檔偏移、+0Ch 段、+0Eh 條目號。
func (h *harness) shape(desc uint16, x, y, e int) {
	h.f.w16(tDS, tShapeSP+4, desc)
	h.f.w16(tDS, tShapeSP+6, uint16(x))
	h.f.w16(tDS, tShapeSP+8, uint16(y))
	h.f.w16(tDS, tShapeSP+0x0A, 0)
	h.f.w16(tDS, tShapeSP+0x0C, tSHPSeg)
	h.f.w16(tDS, tShapeSP+0x0E, uint16(e))
	h.step(tBase+segShape, 0, tShapeSP)
}

func artHarness(t *testing.T, file []byte, learn bool) (*harness, *[]ArtEvent) {
	h := newHarness(t)
	if learn {
		h.label("Talk to Gwen", 114, 204)
	}
	h.installShape(file)
	evs := &[]ArtEvent{}
	h.o.OnArt = func(ev ArtEvent) { *evs = append(*evs, ev) }
	return h, evs
}

func artLogs(o *LabelObserver) []Event {
	var out []Event
	l, _ := o.Log()
	for _, e := range l {
		if e.Kind == "art" {
			out = append(out, e)
		}
	}
	return out
}

// --- 第 1 項：繪圖事件 ---

func TestDialogEventAccept(t *testing.T) {
	file := loadSHP(t)
	for _, e := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 21, 22, 25, 27, 29} {
		k := e % 21
		el := tElemFor(t, k)
		h, evs := artHarness(t, file, true)
		h.shape(tDst, el.x, el.y, e)
		if len(*evs) != 1 {
			t.Errorf("e %d：事件 %d 筆，要 1", e, len(*evs))
			continue
		}
		ev := (*evs)[0]
		if ev.G != e/21 || ev.K != k || ev.X != el.x || ev.Y != el.y || ev.W != el.w || ev.H != el.h {
			t.Errorf("e %d：事件 %+v", e, ArtEvent{G: ev.G, K: ev.K, X: ev.X, Y: ev.Y, W: ev.W, H: ev.H})
		}
		if got := artKey(ev); got != el.key {
			t.Errorf("e %d：Key %q，要 %q", e, got, el.key)
		}
		if string(ev.Entry) != string(mustDecode(t, file, e).Pix) || string(ev.Base0) != string(mustDecode(t, file, k).Pix) {
			t.Errorf("e %d：事件的解碼陣列與條目 %d、%d 不同", e, e, k)
		}
	}
}

func TestDialogEventOrder(t *testing.T) {
	file := loadSHP(t)
	h, evs := artHarness(t, file, true)
	for _, e := range []int{0, 4, 6, 8, 1} {
		el := tElemFor(t, e)
		h.shape(tDst, el.x, el.y, e)
	}
	if len(*evs) != 5 {
		t.Fatalf("事件 %d 筆，要 5", len(*evs))
	}
	for i, ev := range *evs {
		if ev.Order != i+1 {
			t.Errorf("第 %d 筆 Order %d，要 %d", i, ev.Order, i+1)
		}
	}
}

func TestDialogEventReject(t *testing.T) {
	file := loadSHP(t)
	el := tElemFor(t, 6)
	cases := []struct {
		name string
		mut  func(h *harness)
		desc uint16
		e    int
	}{
		{"簽章改一個 byte", func(h *harness) { h.f.w8(tBase+segShape, 0x0F, 0x07) }, tDst, 6},
		{"描述元段不是 FFFF", func(h *harness) {}, tCurDesc, 6},
		{"描述元是裁切矩形", func(h *harness) {}, tClip, 6},
		{"圖檔長度不是 42014", func(h *harness) { h.f.mem[lin(tSHPSeg, 0)] = 0x1F }, tDst, 6},
		{"圖檔中段改一個 byte", func(h *harness) { h.f.mem[lin(tSHPSeg, 0)+30000]++ }, tDst, 6},
		{"圖檔最後一個 byte 改動", func(h *harness) { h.f.mem[lin(tSHPSeg, 0)+42013]++ }, tDst, 6},
		{"e 10", func(h *harness) {}, tDst, 10},
		{"e 14", func(h *harness) {}, tDst, 14},
		{"e 15", func(h *harness) {}, tDst, 15},
		{"e 44（第 2 組）", func(h *harness) {}, tDst, 44},
		{"e 63（第 3 組）", func(h *harness) {}, tDst, 63},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, evs := artHarness(t, file, true)
			c.mut(h)
			h.shape(c.desc, el.x, el.y, c.e)
			if len(*evs) != 0 {
				t.Fatalf("應拒絕，卻產生事件 %+v", (*evs)[0].K)
			}
			logs := artLogs(h.o)
			if c.e == 44 {
				if len(logs) != 1 || !strings.Contains(logs[0].Detail, "第 2 組") {
					t.Fatalf("e 44 要記一筆 skip：%+v", logs)
				}
			} else if len(logs) != 0 {
				t.Fatalf("不應記錄：%+v", logs)
			}
		})
	}
	// 對照：沒有突變時同一組參數會被接受。
	h, evs := artHarness(t, file, true)
	h.shape(tDst, el.x, el.y, 6)
	if len(*evs) != 1 {
		t.Fatal("對照組沒有被接受")
	}
}

func TestDialogEventBaseFromCS(t *testing.T) {
	file := loadSHP(t)
	el := tElemFor(t, 6)
	t.Run("驗證通過", func(t *testing.T) {
		h, evs := artHarness(t, file, false)
		if _, ok := h.o.Base(); ok {
			t.Fatal("base 一開始應未知")
		}
		h.shape(tDst, el.x, el.y, 6)
		if b, ok := h.o.Base(); !ok || b != tBase || len(*evs) != 1 {
			t.Fatalf("base %04X %t、事件 %d 筆，要 %04X true 1", b, ok, len(*evs), tBase)
		}
	})
	t.Run("驗證失敗", func(t *testing.T) {
		h, evs := artHarness(t, file, false)
		h.f.w8(tBase+segText, ipPutsEntry+4, 0x00) // puts 入口簽章改一個 byte
		for i := 0; i < 3; i++ {
			h.shape(tDst, el.x, el.y, 6)
		}
		if _, ok := h.o.Base(); ok || len(*evs) != 0 {
			t.Fatalf("驗證失敗時不得採用：事件 %d 筆", len(*evs))
		}
		if logs := artLogs(h.o); len(logs) != 1 {
			t.Fatalf("同一個 CS 只記一次 log，記了 %d 筆", len(logs))
		}
	})
}

// --- 第 2 項：解碼 ---

func TestDecodeSaveLoad(t *testing.T) {
	file := loadSHP(t)
	ents := tEntries(file)
	if len(ents) != 63 {
		t.Fatalf("條目 %d 個，要 63", len(ents))
	}
	for e := 0; e < 63; e++ {
		s := mustDecode(t, file, e)
		want := tSizes[e%21]
		if s.W != want[0] || s.H != want[1] {
			t.Errorf("條目 %d 尺寸 %d×%d，要 %d×%d", e, s.W, s.H, want[0], want[1])
		}
		if n := ents[e][1] - ents[e][0]; s.Consumed != n {
			t.Errorf("條目 %d 消耗 %d bytes，條目長 %d", e, s.Consumed, n)
		}
	}
}

func TestDecodeMatchesFrames(t *testing.T) {
	file := loadSHP(t)
	type draw struct {
		frame string
		e, x  int
		y     int
	}
	draws := []draw{
		{"s2/pre.idx", 0, 80, 30}, {"s2/pre.idx", 2, 134, 127}, {"s2/pre.idx", 6, 90, 142}, {"s2/pre.idx", 8, 174, 142},
		{"l8/pre.idx", 1, 114, 36}, {"l8/pre.idx", 4, 134, 127}, {"l8/pre.idx", 6, 90, 142}, {"l8/pre.idx", 8, 174, 142},
		// 帳篷存檔（21 組，RE 015 t1）
		{"t1/tsave.idx", 21, 80, 30}, {"t1/tsave.idx", 23, 134, 127}, {"t1/tsave.idx", 27, 90, 142}, {"t1/tsave.idx", 29, 174, 142},
	}
	for _, d := range draws {
		frame := loadIdx(t, d.frame)
		s := mustDecode(t, file, d.e)
		el := tElemFor(t, d.e%21)
		diff := 0
		for y := el.band[1]; y <= el.band[3]; y++ {
			for x := el.band[0]; x <= el.band[2]; x++ {
				if frame[(d.y+y)*320+d.x+x] != s.Pix[y*s.W+x] {
					diff++
				}
			}
		}
		if diff != 0 {
			t.Errorf("%s 條目 %d：字帶內 %d 像素與畫格不同", d.frame, d.e, diff)
		}
	}
}

// --- 第 3 項：字帶涵蓋 ---

func TestBandCoverage(t *testing.T) {
	file := loadSHP(t)
	inBand := func(el tElem, x, y int) bool {
		return x >= el.band[0] && x <= el.band[2] && y >= el.band[1] && y <= el.band[3]
	}
	for _, g := range []int{0, 21} {
		// k2 與 k4 的差異像素全在字帶內。
		a, b := mustDecode(t, file, g+2), mustDecode(t, file, g+4)
		el := tElemFor(t, 2)
		for i := range a.Pix {
			if a.Pix[i] != b.Pix[i] && !inBand(el, i%a.W, i/a.W) {
				t.Errorf("組 %d：k2、k4 在 (%d,%d) 不同，不在字帶內", g, i%a.W, i/a.W)
			}
		}
		// k1 與 k0 同位置的差異像素全在 k1 字帶內。
		k0, k1 := mustDecode(t, file, g+0), mustDecode(t, file, g+1)
		e1 := tElemFor(t, 1)
		for y := 0; y < k1.H; y++ {
			for x := 0; x < k1.W; x++ {
				if k1.Pix[y*k1.W+x] != k0.Pix[(y+6)*k0.W+x+34] && !inBand(e1, x, y) {
					t.Errorf("組 %d：k1 與 k0 在 k1 的 (%d,%d) 不同，不在字帶內", g, x, y)
				}
			}
		}
		// k2、k4、k6、k8：區域 x 3 到 w−4、y 2 到 h−4 內，0 組屬於 F 但不是 15、26 的像素，外接框等於墨跡框。
		for _, k := range []int{2, 4, 6, 8} {
			el := tElemFor(t, k)
			s0 := mustDecode(t, file, k)
			sg := mustDecode(t, file, g+k)
			x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
			for y := 2; y <= s0.H-4; y++ {
				for x := 3; x <= s0.W-4; x++ {
					c := s0.Pix[y*s0.W+x]
					if !tInk[c] || c == 15 || c == 26 {
						continue
					}
					_ = sg.Pix[y*sg.W+x] // 21 組用同一批位置
					x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
				}
			}
			if [4]int{x0, y0, x1, y1} != el.ink {
				t.Errorf("組 %d k%d：外接框 (%d,%d)-(%d,%d)，墨跡框 %v", g, k, x0, y0, x1, y1, el.ink)
			}
		}
	}
	// 每個 k 的墨跡框（含按下態下移後）在字帶內；實作的表與測試內數值逐列相同。
	for k := 0; k <= 9; k++ {
		el := tElemFor(t, k)
		if el.ink[0] < el.band[0] || el.ink[1] < el.band[1] || el.ink[2] > el.band[2] || el.ink[3] > el.band[3] {
			t.Errorf("k%d 的墨跡框 %v 不在字帶 %v 內", k, el.ink, el.band)
		}
		pe, pink, ok := elementFor(k)
		if !ok || pe.w != el.w || pe.h != el.h ||
			[4]int{pe.band.x0, pe.band.y0, pe.band.x1, pe.band.y1} != el.band ||
			[4]int{pink.x0, pink.y0, pink.x1, pink.y1} != el.ink {
			t.Errorf("k%d：實作的表 %+v 墨跡框 %+v，測試 %+v", k, pe, pink, el)
		}
	}
	for k := 10; k < 21; k++ {
		if _, _, ok := elementFor(k); ok {
			t.Errorf("k%d 不應有元素列", k)
		}
	}
}

// --- 第 4 項：無字底 ---

// tBG 是 004 勘誤的底色集合 B（0 組）：標題 k0、k1 為 {9, 13}，按鈕 k2 到 k9 為 {8, 9, 13}。
func tBG(k int) map[uint8]bool {
	if k == 0 || k == 1 {
		return map[uint8]bool{9: true, 13: true}
	}
	return map[uint8]bool{8: true, 9: true, 13: true}
}

// tJudge 是 004 勘誤的墨跡判定框：按鈕沿用墨跡框（按下態已下移）；標題往下延 1 列含陰影，
// k0 右緣延到 134（第二次勘誤）。
func tJudge(t *testing.T, k int) [4]int {
	t.Helper()
	switch k {
	case 0:
		return [4]int{47, 9, 134, 15}
	case 1:
		return [4]int{11, 3, 102, 9}
	}
	return tElemFor(t, k).ink
}

func TestInpaint(t *testing.T) {
	file := loadSHP(t)
	for _, g := range []int{0, 1} {
		for k := 0; k <= 9; k++ {
			el := tElemFor(t, k)
			jb := tJudge(t, k)
			bg := tBG(k)
			ev := mkEvent(t, file, g*21+k, el.x, el.y, 1)
			w := el.w
			inBox := func(x, y int) bool { return x >= jb[0] && x <= jb[2] && y >= jb[1] && y <= jb[3] }
			// 獨立的墨跡判定：0 組條目在判定框內、色號不屬於 B 的位置（21 組同位置）。
			ink := func(x, y int) bool { return inBox(x, y) && !bg[ev.Base0[y*w+x]] }
			// 21 組「B 位置的對應色」：0 組色號屬於 B 的位置，在本條目的色號。
			bgG := map[uint8]bool{}
			for i, c := range ev.Base0 {
				if bg[c] {
					bgG[ev.Entry[i]] = true
				}
			}
			bw, bh := el.band[2]-el.band[0]+1, el.band[3]-el.band[1]+1
			got, fb := dialogBackgroundFallback(ev)
			if len(got) != bw*bh {
				t.Fatalf("組 %d k%d：無字底 %d 像素，要 %d", g, k, len(got), bw*bh)
			}
			nInk := 0
			var fbWant []string // 該列找不到屬於 B 的來源，退回原規則 4 的墨跡位置
			for y := 0; y < bh; y++ {
				for x := 0; x < bw; x++ {
					ex, ey := el.band[0]+x, el.band[1]+y
					orig := ev.Entry[ey*w+ex]
					out := got[y*bw+x]
					switch {
					case !inBox(ex, ey) && out != orig:
						t.Errorf("組 %d k%d：判定框外 (%d,%d) 被改", g, k, ex, ey)
					case inBox(ex, ey) && !ink(ex, ey) && out != orig:
						t.Errorf("組 %d k%d：判定框內非墨跡像素 (%d,%d) 被改", g, k, ex, ey)
					case ink(ex, ey):
						nInk++
						// 期望：同一列、字帶內、非墨跡、0 組色號屬於 B 的最近位置，取本條目同位置的色號，
						// 左右等距取左；找不到才退回最近的非墨跡像素。
						search := func(ok func(xx int) bool) (uint8, bool) {
							for d := 1; d < bw; d++ {
								for _, xx := range []int{ex - d, ex + d} {
									if xx >= el.band[0] && xx <= el.band[2] && ok(xx) {
										return ev.Entry[ey*w+xx], true
									}
								}
							}
							return 0, false
						}
						want, found := search(func(xx int) bool { return !ink(xx, ey) && bg[ev.Base0[ey*w+xx]] })
						if !found {
							fbWant = append(fbWant, fmt.Sprintf("(%d,%d)", ex, ey))
							want, found = search(func(xx int) bool { return !ink(xx, ey) })
						}
						if !found || out != want {
							t.Errorf("組 %d k%d：墨跡像素 (%d,%d) 回填 %d，要 %d", g, k, ex, ey, out, want)
						}
						// 第 4 項：回填後的色號屬於 B（0 組）或 B 位置的對應色（21 組）。
						if g == 0 && !bg[out] || g == 1 && !bgG[out] {
							t.Errorf("組 %d k%d：墨跡位置 (%d,%d) 回填 %d，不屬於 B", g, k, ex, ey, out)
						}
					}
				}
			}
			if len(fbWant) != 0 {
				t.Logf("組 %d k%d：退回原規則 4 的 %d 點：%s", g, k, len(fbWant), strings.Join(fbWant, " "))
			}
			if len(fb) != len(fbWant) {
				t.Errorf("組 %d k%d：退回清單 %d 點，要 %d", g, k, len(fb), len(fbWant))
			}
			if nInk == 0 {
				t.Errorf("組 %d k%d：判定框內沒有墨跡像素", g, k)
			}
			// 指定點：原色號與回填後必須屬於的集合。主代理給的 k1 (14,4) 是墨跡框相對座標，元素座標是 (25,4)
			// （元素座標 (14,4) 是 66）；(13,9) 是元素座標的陰影列（色號 8）。k0 (134,13) 是 E 字右側
			// 落在墨跡框外的陰影（色號 8），判定框右緣延到 134 後要回填成底色。
			type pt struct {
				x, y int
				orig uint8
				to   map[uint8]bool
			}
			var pts []pt
			if g == 0 && k == 0 {
				pts = []pt{{47, 11, 198, bg}, {51, 12, 198, bg}, {134, 13, 8, map[uint8]bool{9: true, 13: true}}}
			}
			if g == 0 && k == 1 {
				pts = []pt{{25, 4, 5, map[uint8]bool{9: true, 13: true}}, {13, 9, 8, map[uint8]bool{9: true, 13: true}}}
			}
			for _, p := range pts {
				o := ev.Entry[p.y*w+p.x]
				v := got[(p.y-el.band[1])*bw+p.x-el.band[0]]
				if (p.orig != 0 && o != p.orig) || !p.to[v] {
					t.Errorf("0 組 k%d 的 (%d,%d)：原色號 %d、回填 %d", k, p.x, p.y, o, v)
				}
			}
		}
	}
}

// --- 第 5 項：圖面 ---

func newArtOverlay(t *testing.T, scale int) *Overlay {
	t.Helper()
	ov, err := NewOverlay(testLabelFont(t), testCatalog(), testGfx(), scale)
	if err != nil {
		t.Fatal(err)
	}
	return ov
}

func artStampOf(t *testing.T, ov *Overlay, key string) (*tStamp, bool) {
	t.Helper()
	for _, s := range ov.artLayer.Art {
		if s.Key == key {
			return &tStamp{s.X, s.Y, s.Cells, s.CellW, s.CellH, s.Reference, s.Pix, s.PixScale, s.Order}, true
		}
	}
	return nil, false
}

// tStamp 是測試看的圖面欄位。
type tStamp struct {
	X, Y, Cells, CellW, CellH int
	Reference, Pix            []uint8
	PixScale, Order           int
}

func TestArtStamp(t *testing.T) {
	file := loadSHP(t)
	for _, sc := range []int{2, 3} {
		for _, e := range []int{0, 1, 2, 4, 6, 7, 8, 27} {
			el := tElemFor(t, e%21)
			ov := newArtOverlay(t, sc)
			ev := mkEvent(t, file, e, el.x, el.y, 1)
			ov.AddDialogArt(ev)
			// 事件當下的畫面（常式入口，還是舊畫面）與解碼值不同：全黑。
			ov.Draw(make([]byte, 64000), tPal())
			s, ok := artStampOf(t, ov, el.key)
			if !ok {
				t.Fatalf("%d 倍 e %d：沒有圖面 %q（misses %+v skips %+v）", sc, e, el.key, ov.Misses, ov.Skips)
			}
			bw, bh := el.band[2]-el.band[0]+1, el.band[3]-el.band[1]+1
			if s.X != el.x+el.band[0] || s.Y != el.y+el.band[1] || s.Cells != bw || s.CellW != 1 || s.CellH != bh || s.PixScale != sc {
				t.Errorf("%d 倍 e %d：圖面 %+v", sc, e, *s)
			}
			for y := 0; y < bh; y++ {
				for x := 0; x < bw; x++ {
					if s.Reference[y*bw+x] != ev.Entry[(el.band[1]+y)*el.w+el.band[0]+x] {
						t.Fatalf("%d 倍 e %d：Reference (%d,%d) 與解碼值不同", sc, e, x, y)
					}
				}
			}
			if r := ov.Art(); len(r) != 1 || r[0] != (ArtRect{el.key, s.X, s.Y, bw, bh}) {
				t.Errorf("Art() %+v", r)
			}
		}
	}
}

func TestArtKeyReplace(t *testing.T) {
	file := loadSHP(t)
	ov := newArtOverlay(t, 2)
	e6, e7 := tElemFor(t, 6), tElemFor(t, 7)
	ov.AddDialogArt(mkEvent(t, file, 6, e6.x, e6.y, 1))
	ov.AddDialogArt(mkEvent(t, file, 7, e7.x, e7.y, 2))
	if k := ov.ArtKeys(); len(k) != 1 || k[0] != "saveload/90,142/6" {
		t.Fatalf("一般態與按下態要同 Key 替換：%q", k)
	}
	ov.Draw(make([]byte, 64000), tPal())
	if s, _ := artStampOf(t, ov, "saveload/90,142/6"); s == nil || s.Order != 2 || len(ov.artLayer.Art) != 1 {
		t.Fatal("替換後 layer 應只剩按下態一列")
	}
	e0, e1 := tElemFor(t, 0), tElemFor(t, 1)
	ov.AddDialogArt(mkEvent(t, file, 0, e0.x, e0.y, 3))
	ov.AddDialogArt(mkEvent(t, file, 1, e1.x, e1.y, 4))
	got := strings.Join(ov.ArtKeys(), "|")
	if got != "saveload/90,142/6|saveload/80,30/0|saveload/114,36/1" {
		t.Fatalf("Key %q", got)
	}
}

func TestArtOrderOverlap(t *testing.T) {
	file := loadSHP(t)
	for _, sc := range []int{2, 3} {
		ov := newArtOverlay(t, sc)
		e0, e1 := tElemFor(t, 0), tElemFor(t, 1)
		ov.AddDialogArt(mkEvent(t, file, 0, e0.x, e0.y, 1))
		ov.AddDialogArt(mkEvent(t, file, 1, e1.x, e1.y, 2))
		frame := make([]byte, 64000)
		paint(t, frame, file, 0, 80, 30)
		paint(t, frame, file, 1, 114, 36)
		out, _ := ov.Draw(frame, tPal())
		s0, _ := artStampOf(t, ov, e0.key)
		s1, _ := artStampOf(t, ov, e1.key)
		if s0 == nil || s1 == nil || s1.Order <= s0.Order {
			t.Fatalf("%d 倍：k1 的 Order 要大於 k0", sc)
		}
		W, rowW := 320*sc, s1.Cells*sc
		for _, x := range []int{167, 168} {
			for y := s1.Y; y < s1.Y+s1.CellH; y++ {
				for dy := 0; dy < sc; dy++ {
					for dx := 0; dx < sc; dx++ {
						ox, oy := x*sc+dx, y*sc+dy
						px, py := (x-s1.X)*sc+dx, (y-s1.Y)*sc+dy
						o := out[4*(oy*W+ox) : 4*(oy*W+ox)+4]
						p := s1.Pix[4*(py*rowW+px) : 4*(py*rowW+px)+4]
						if string(o) != string(p) {
							t.Fatalf("%d 倍：絕對 (%d,%d) 的輸出像素不是 k1 圖面", sc, x, y)
						}
					}
				}
			}
		}
		// 不會漏出「儲存進度」：k0 的字帶與 k1 不同的區域都停畫，輸出與原版放大圖的差異只在 k1 字帶內。
		plain := ScaleIndexedRGBA(frame, tPal(), sc)
		for i := 0; i < len(out); i += 4 {
			if string(out[i:i+4]) == string(plain[i:i+4]) {
				continue
			}
			x, y := (i/4)%W/sc, (i/4)/W/sc
			if x < s1.X || x >= s1.X+s1.Cells || y < s1.Y || y >= s1.Y+s1.CellH {
				t.Fatalf("%d 倍：k1 字帶外 (%d,%d) 有差異（k0 圖面漏出）", sc, x, y)
			}
		}
	}
}

func TestArtClearOnFullBlit(t *testing.T) {
	file := loadSHP(t)
	h, _ := artHarness(t, file, true)
	ov := newArtOverlay(t, 2)
	h.o.OnArt = ov.AddDialogArt
	h.o.OnClearArt = func(uint64) { ov.ClearArt() }
	el := tElemFor(t, 6)
	h.shape(tDst, el.x, el.y, 6)
	ov.Draw(make([]byte, 64000), tPal())
	if len(ov.Art()) != 1 || len(ov.artLayer.Art) != 1 {
		t.Fatalf("圖面 %d 列", len(ov.Art()))
	}
	h.setDesc(tDst, 0xFFFF, rect{0, 0, 100, 100})
	h.blit()
	if len(ov.Art()) != 1 {
		t.Fatal("非全螢幕 blit 不得清圖面")
	}
	h.setDesc(tDst, 0xFFFF, fullScreen)
	h.blitEntry()
	if len(ov.Art()) != 0 || len(ov.artLayer.Art) != 0 {
		t.Fatalf("全螢幕 blit 入口後圖面應清空：%d、%d", len(ov.Art()), len(ov.artLayer.Art))
	}
}

// glyphBox 回 idx 與放大無字底不同的像素外接框（左、上）。
func glyphBox(t *testing.T, ov *Overlay, ev ArtEvent) (left, top int) {
	t.Helper()
	el := tElemFor(t, ev.K)
	bw := el.band[2] - el.band[0] + 1
	bg := dialogBackground(ev)
	var e *artEntry
	for _, a := range ov.arts {
		if a.ev.Order == ev.Order {
			e = a
		}
	}
	if e == nil {
		t.Fatal("沒有規劃")
	}
	sc := ov.scale
	ow := bw * sc
	left, top = 1<<30, 1<<30
	for i, c := range e.idx {
		x, y := i%ow, i/ow
		if c != bg[(y/sc)*bw+x/sc] {
			left, top = min(left, x), min(top, y)
		}
	}
	return
}

func TestArtGlyphPlacement(t *testing.T) {
	file := loadSHP(t)
	// CANCEL → 「地面」（2 字）。k6 字帶 x 12 到 67（56 寬）、墨跡框 x 15 到 63，相對字帶 3 到 51。
	// 2 倍：W = 32，tx = ((3+51+1)×2 − 32)/2 = 39，格內置中 0，高光左移 1 → 38；ty 一般 1、按下 2。
	// 3 倍：W = 36，tx = (165 − 36)/2 = 64，格內置中 (18−16)/2 = 1 → 65，高光 64；ty 一般 5、按下 8。
	want := map[int][3]int{2: {38, 1, 2}, 3: {64, 5, 8}}
	for sc, w := range want {
		ov := newArtOverlay(t, sc)
		e6, e7 := tElemFor(t, 6), tElemFor(t, 7)
		ev6 := mkEvent(t, file, 6, e6.x, e6.y, 1)
		ov.AddDialogArt(ev6)
		l6, t6 := glyphBox(t, ov, ev6)
		ov2 := newArtOverlay(t, sc)
		ev7 := mkEvent(t, file, 7, e7.x, e7.y, 2)
		ov2.AddDialogArt(ev7)
		l7, t7 := glyphBox(t, ov2, ev7)
		if l6 != w[0] || t6 != w[1] || l7 != w[0] || t7 != w[2] {
			t.Errorf("%d 倍：k6 字模左上 (%d,%d)、k7 (%d,%d)，要 (%d,%d)、(%d,%d)", sc, l6, t6, l7, t7, w[0], w[1], w[0], w[2])
		}
		if t7-t6 != map[int]int{2: 1, 3: 3}[sc] {
			t.Errorf("%d 倍：k6、k7 字模上緣相差 %d", sc, t7-t6)
		}
	}
}

func TestArtTextColors(t *testing.T) {
	file := loadSHP(t)
	// 004 §3.5 表（按下態同一般態）：{0 組 c, 0 組 h, 21 組 c, 21 組 h}。
	want := map[int][4]uint8{
		0: {49, 15, 55, 209}, 1: {49, 15, 55, 209},
		2: {82, 15, 31, 87}, 4: {82, 15, 31, 209}, 6: {82, 15, 31, 209}, 8: {82, 15, 31, 87},
	}
	for k := 0; k <= 9; k++ {
		el := tElemFor(t, k)
		w := want[normalKTest(k)]
		for gi, g := range []int{0, 21} {
			ev := mkEvent(t, file, g+k, el.x, el.y, 1)
			c, h := dialogColors(ev)
			if c != w[2*gi] || h != w[2*gi+1] {
				t.Errorf("組 %d k%d：c %d h %d，要 %d %d", g, k, c, h, w[2*gi], w[2*gi+1])
			}
		}
	}
	// 高光在筆畫左側：測試字模每列第 0 欄有點，k6 2 倍第一個字模在 (39, 1)，(38, 1) 是高光。
	for gi, g := range []int{0, 21} {
		ov := newArtOverlay(t, 2)
		el := tElemFor(t, 6)
		ov.AddDialogArt(mkEvent(t, file, g+6, el.x, el.y, 1))
		e := ov.arts[0]
		ow := 56 * 2
		c, h := e.idx[1*ow+39], e.idx[1*ow+38]
		if c != want[6][2*gi] || h != want[6][2*gi+1] {
			t.Errorf("組 %d k6：字模點 %d、左鄰高光 %d，要 %d、%d", g, c, h, want[6][2*gi], want[6][2*gi+1])
		}
		// 字模第 7 欄有點、第 6 欄沒有：第 6 欄是高光，第 8 欄（下一點）是字色。
		if e.idx[1*ow+39+6] != want[6][2*gi+1] || e.idx[1*ow+39+7] != want[6][2*gi] {
			t.Errorf("組 %d k6：第 6、7 欄 %d、%d", g, e.idx[1*ow+39+6], e.idx[1*ow+39+7])
		}
	}
}

func normalKTest(k int) int {
	if k == 3 || k == 5 || k == 7 || k == 9 {
		return k - 1
	}
	return k
}

// --- 第 6 項：逐欄 ---

// columnState 依輸出畫面判斷字帶每欄：shown（等於 Pix）、hidden（等於原版放大圖且不等於 Pix）。
func columnState(t *testing.T, out, plain []byte, s *tStamp, sc int) (shown, hidden, other int) {
	t.Helper()
	W, rowW := 320*sc, s.Cells*sc
	for cell := 0; cell < s.Cells; cell++ {
		eqPix, eqPlain := true, true
		for y := 0; y < s.CellH*sc; y++ {
			for dx := 0; dx < sc; dx++ {
				ox, oy := (s.X+cell)*sc+dx, s.Y*sc+y
				i := 4 * (oy*W + ox)
				j := 4 * (y*rowW + cell*sc + dx)
				if string(out[i:i+4]) != string(s.Pix[j:j+4]) {
					eqPix = false
				}
				if string(out[i:i+4]) != string(plain[i:i+4]) {
					eqPlain = false
				}
			}
		}
		switch {
		case eqPix:
			shown++
		case eqPlain:
			hidden++
		default:
			other++
		}
	}
	return
}

func TestArtPerColumn(t *testing.T) {
	file := loadSHP(t)
	el := tElemFor(t, 6)
	pal := tPal()
	frame := make([]byte, 64000)
	paint(t, frame, file, 0, 80, 30)
	paint(t, frame, file, 6, el.x, el.y)
	ov := newArtOverlay(t, 2)
	ov.AddDialogArt(mkEvent(t, file, 6, el.x, el.y, 1))
	out, _ := ov.Draw(frame, pal)
	s, _ := artStampOf(t, ov, el.key)
	if sh, hi, ot := columnState(t, out, ScaleIndexedRGBA(frame, pal, 2), s, 2); sh != 56 || hi != 0 || ot != 0 {
		t.Fatalf("畫格等於基準：顯示 %d、停畫 %d、其他 %d，要 56、0、0", sh, hi, ot)
	}
	// 一欄放一個游標像素：只有該欄停畫。
	cur := append([]byte(nil), frame...)
	cx, cy := s.X+20, s.Y+4
	cur[cy*320+cx] = 255
	out, _ = ov.Draw(cur, pal)
	plain := ScaleIndexedRGBA(cur, pal, 2)
	if sh, hi, ot := columnState(t, out, plain, s, 2); sh != 55 || hi != 1 || ot != 0 {
		t.Fatalf("一個游標像素：顯示 %d、停畫 %d、其他 %d，要 55、1、0", sh, hi, ot)
	}
	// 換成場景：全部停畫，輸出等於原版放大圖。
	scene := make([]byte, 64000)
	for i := range scene {
		scene[i] = uint8(i % 7)
	}
	out, _ = ov.Draw(scene, pal)
	if string(out) != string(ScaleIndexedRGBA(scene, pal, 2)) {
		t.Fatal("場景畫格時圖面要全部停畫")
	}
	// l8/hover.idx（游標在 CANCEL 上）：停畫欄數等於該畫格與基準不同的欄數。
	hover := loadIdx(t, "l8/hover.idx")
	diffCols := 0
	for cell := 0; cell < s.Cells; cell++ {
		for y := 0; y < s.CellH; y++ {
			if hover[(s.Y+y)*320+s.X+cell] != s.Reference[y*s.Cells+cell] {
				diffCols++
				break
			}
		}
	}
	out, _ = ov.Draw(hover, pal)
	sh, hi, ot := columnState(t, out, ScaleIndexedRGBA(hover, pal, 2), s, 2)
	t.Logf("hover：與基準不同 %d 欄；顯示 %d、停畫 %d、其他 %d", diffCols, sh, hi, ot)
	if hi != diffCols || sh+hi != s.Cells || ot != 0 || diffCols == 0 {
		t.Fatalf("hover：停畫 %d 欄、顯示 %d、其他 %d，與基準不同 %d 欄", hi, sh, ot, diffCols)
	}
}

// --- 第 7 項：Draw 順序 ---

func TestArtDrawOrder(t *testing.T) {
	file := loadSHP(t)
	pal := tPal()
	el := tElemFor(t, 6)
	ov := newArtOverlay(t, 2)
	ov.AddDialogArt(mkEvent(t, file, 6, el.x, el.y, 1))
	// 熱點標籤 stamp：遮蓋 x 90 到 169、y 141 到 154，與 CANCEL 字帶 (102,143)-(157,151) 重疊。
	// 合成情境（005 §3.2 第 4 條）：真實流程中對話框矩形與標籤比對框不重疊。Ref 取自同一畫格。
	frame := make([]byte, 64000)
	paint(t, frame, file, 6, el.x, el.y)
	gwen := Label{Key: "Talk to Gwen", X0: 90, X1: 170, Y: 140, Font: measuredFont, Kind: KindLabel}
	gwen.Ref = refFromFrame(frame, gwen)
	ov.Sync([]Label{gwen}, 1)
	if len(ov.Stamps()) != 2 {
		t.Fatalf("文字 stamp %d 個", len(ov.Stamps()))
	}
	snap := func() string {
		var b strings.Builder
		for _, s := range ov.Stamps() {
			fmt.Fprintf(&b, "%s %d %v %v|", s.Key, s.State, s.BG, s.FG)
		}
		return b.String()
	}
	out, _ := ov.Draw(frame, pal)
	before := snap()
	for i := 0; i < 3; i++ {
		out, _ = ov.Draw(frame, pal)
	}
	if after := snap(); after != before {
		t.Fatalf("多次 Draw 後文字 stamp 改變：\n%s\n%s", before, after)
	}
	// (104,145) 在遮蓋 stamp 與圖面字帶內、在文字 stamp（x 110 起）之外：輸出是遮蓋的底色 0 號。
	W := 640
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			i := 4 * ((145*2+dy)*W + 104*2 + dx)
			if [3]uint8{out[i], out[i+1], out[i+2]} != pal[0] {
				t.Fatalf("文字 stamp 要畫在圖面之上：(104,145) 是 %v", out[i:i+3])
			}
		}
	}
	// 對照：沒有文字 stamp 時同一點是圖面。
	ov2 := newArtOverlay(t, 2)
	ov2.AddDialogArt(mkEvent(t, file, 6, el.x, el.y, 1))
	out2, _ := ov2.Draw(frame, pal)
	i := 4 * ((145*2)*W + 104*2)
	if [3]uint8{out2[i], out2[i+1], out2[i+2]} == pal[0] {
		t.Fatal("對照組的圖面顏色恰為 0 號，測試無鑑別力")
	}
}

// --- 第 8 項：Miss、Skip ---

func TestArtMissSkip(t *testing.T) {
	file := loadSHP(t)
	el := tElemFor(t, 6)
	mk := func(gfx map[string]string) *Overlay {
		ov, err := NewOverlay(testLabelFont(t), testCatalog(), gfx, 2)
		if err != nil {
			t.Fatal(err)
		}
		ov.AddDialogArt(mkEvent(t, file, 6, el.x, el.y, 1))
		ov.Draw(make([]byte, 64000), tPal())
		return ov
	}
	gfx := testGfx()
	delete(gfx, "CANCEL")
	ov := mk(gfx)
	if len(ov.Art()) != 0 || len(ov.artLayer.Art) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Key != "CANCEL" || ov.Misses[0].Reason != MissTranslation {
		t.Fatalf("缺譯文：圖面 %d、misses %+v", len(ov.Art()), ov.Misses)
	}
	gfx = testGfx()
	gfx["CANCEL"] = "地乂"
	ov = mk(gfx)
	if len(ov.Art()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != MissGlyph || string(ov.Misses[0].Runes) != "乂" {
		t.Fatalf("缺字：圖面 %d、misses %+v", len(ov.Art()), ov.Misses)
	}
	gfx = testGfx()
	gfx["CANCEL"] = "測試譯文甲乙丙一" // 8 字：2 倍 128 輸出像素，字帶 112
	ov = mk(gfx)
	if len(ov.Art()) != 0 || len(ov.Skips) != 1 || ov.Skips[0].Key != "CANCEL" {
		t.Fatalf("超寬：圖面 %d、skips %+v", len(ov.Art()), ov.Skips)
	}
	// 對照：7 字（112 像素）放得下。
	gfx["CANCEL"] = "測試譯文甲乙丙"
	if ov = mk(gfx); len(ov.Art()) != 1 {
		t.Fatalf("7 字應放得下：skips %+v", ov.Skips)
	}
	// 1 倍不產生圖面。
	ov1, _ := NewOverlay(testLabelFont(t), testCatalog(), testGfx(), 1)
	ov1.AddDialogArt(mkEvent(t, file, 6, el.x, el.y, 1))
	if len(ov1.Art()) != 0 {
		t.Fatal("1 倍不產生圖面")
	}
}

// --- 第 9 項：lint 與 catalog ---

func TestLintArt(t *testing.T) {
	wide := testLabelFont(t).Wide
	b, err := os.ReadFile("testdata/menu_gfx.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if rep := LintArt(b, wide); !rep.OK() {
		t.Fatalf("testdata 應通過（含主選單 key CONTINUE GAME）：%v", rep.Errors)
	}
	cases := map[string]string{
		"缺 key":  strings.Replace(string(b), "QUIT TO DOS\t空中丙一\t測試（004 §4.1）\n", "", 1),
		"空白譯文":   strings.Replace(string(b), "CANCEL\t地面\t", "CANCEL\t\t", 1),
		"缺字":     strings.Replace(string(b), "CANCEL\t地面\t", "CANCEL\t地乂\t", 1),
		"半形字":    strings.Replace(string(b), "CANCEL\t地面\t", "CANCEL\t地a\t", 1),
		"key 重複": string(b) + "SAVE\t測試\t重複\n",
		"欄數不是 3": string(b) + "X\tY\n",
	}
	for name, s := range cases {
		if s == string(b) {
			t.Fatalf("%s：替換沒有生效", name)
		}
		if rep := LintArt([]byte(s), wide); rep.OK() {
			t.Errorf("%s 應報錯", name)
		}
	}
	// puts 輸出 CANCEL 時 001 不產生 stamp：圖形字 catalog 不併進 tr。
	ov, err := NewOverlay(testLabelFont(t), testCatalog(), testGfx(), 2)
	if err != nil {
		t.Fatal(err)
	}
	ov.Sync([]Label{{Key: "CANCEL", X0: 105, X1: 153, Y: 140, Font: measuredFont, Kind: KindLabel}}, 1)
	if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != MissTranslation {
		t.Fatalf("puts 輸出 CANCEL 不得產生 stamp：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
	}
}

// TestFormalGfxCatalog 以 LintArt 檢查正式圖形字 catalog（DOSGOLEM_SC_GFX_CATALOG）與字型（DOSGOLEM_SC_FONT）。
func TestFormalGfxCatalog(t *testing.T) {
	cp, fp := os.Getenv(scGfxCatalogEnv), os.Getenv(scFontEnv)
	if cp == "" || fp == "" {
		t.Skip(scGfxCatalogEnv + "、" + scFontEnv + " 未設定")
	}
	b, err := os.ReadFile(cp)
	if err != nil {
		t.Fatal(err)
	}
	f, err := LoadLabelFont(fp, "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	if rep := LintArt(b, f.Wide); !rep.OK() {
		t.Fatalf("正式圖形字 catalog：%v", rep.Errors)
	}
	c, err := ParseCatalog(b)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"SAVE GAME": "儲存進度", "LOAD GAME": "讀取進度", "SAVE": "儲存", "LOAD": "讀取", "CANCEL": "取消", "QUIT TO DOS": "離開遊戲"}
	for k, v := range want {
		if c[k] != v {
			t.Errorf("%q 譯文 %q，要 %q", k, c[k], v)
		}
	}
}

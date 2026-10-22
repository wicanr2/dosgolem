package strikecommander

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// strike_commander 規格 008 §4.1 的單元測試：主選單（MAINMENU.PAK）與 MENU 片頭選單（TOMENU.PAK）的繁中覆繪。
//
// 原版衍生物不進 Git：從 DOSGOLEM_SC_I018 指向的目錄（strike_commander 的 workplace/i018）讀
// dec/MAINMENU.bin、dec/TOMENU.bin（BIGTRE.TRE 內的兩個 PAK，RE 018 §1）與 a2、a3、a4、a5、m1 的畫格 .idx。
// 讀不到時 t.Skip；主代理驗收時必須提供，跳過不算通過。子條目由測試自己的 PAK 與 LZW 讀取程式取出
// （TOMENU 的條目是 LZW，執行期是解壓後的子條目，產品不需要 LZW），再比對 008 §2 的長度與 SHA-256。
//
// 元素表、M、Q、墨跡點數、SHA、字樣常數與 tx、ty 都在測試內獨立寫出（008 §3.2 到 §3.5、§4.1；
// tx、ty 取自 strike_commander workplace/i008/rv-a/out/pos.txt），不迭代實作的表。

const i018Env = "DOSGOLEM_SC_I018"

// --- 測試內獨立寫出的數值 ---

const (
	tMainMenuPAKSHA = "081e1d3f3028f90784040ccfbc881fa529b12eb5b28ab4a7755b56ed0aa2baba" // RE 018 §1.1
	tTomenuPAKSHA   = "e45f17d834a6eaf95259635ef8f98f6fa32ce19f93c8ccbf68c316f01fba6ee5" // RE 018 §1.2
	tMMButtonsSHA   = "3729dfc27900368bbe27e7fb1e7deaa2e16e5666476fbbf715b719139208d0af" // 008 §2
	tMMPlateSHA     = "abebe6c16aee0408fa27e93881f3a8ade463745c68ce3a1bdabcedef1210d380"
	tTMButtonsSHA   = "13dfa8ecdd986cf8b298aea6d64fccf62ed61a956f4fba476e689597b4f0e37a"
)

// 008 §3.2：MAINMENU 按鈕 (56, 34+17n)，字帶 x 2..208、y 2..12，置中框 x 如下、y 3..10。
var tMMCX = [5][2]int{{22, 190}, {46, 166}, {18, 195}, {9, 203}, {30, 178}}
var tMMKey = [5]string{"CONTINUE GAME", "LOAD GAME", "START NEW GAME", "TRAINING MISSION", "VIEW OBJECTS"}

func tMMPos(n int) (int, int) { return 56, 34 + 17*n }

const tPlateX, tPlateY = 45, 24

// tTM 是 008 §3.2 的 TOMENU 表（k 0 到 3）。
type tTMElem struct {
	x, y, w, h    int
	band, center  [4]int
	key, rectKey  string
	pressedOfNorm int
}

var tTM = [4]tTMElem{
	{89, 33, 166, 26, [4]int{2, 2, 163, 23}, [4]int{15, 4, 146, 21}, "PLAY STRIKE COMMANDER", "tomenu/89,33/0", 0},
	{90, 71, 165, 26, [4]int{2, 2, 162, 23}, [4]int{4, 4, 159, 21}, "PLAY TACTICAL OPERATIONS", "tomenu/90,71/1", 1},
	{89, 33, 166, 26, [4]int{2, 2, 163, 23}, [4]int{15, 4, 146, 21}, "PLAY STRIKE COMMANDER", "tomenu/89,33/0", 0},
	{90, 71, 166, 26, [4]int{2, 2, 162, 23}, [4]int{4, 4, 159, 21}, "PLAY TACTICAL OPERATIONS", "tomenu/90,71/1", 1},
}

// --- 原版衍生物 ---

func i018Dir(t *testing.T) string {
	t.Helper()
	d := os.Getenv(i018Env)
	if d == "" {
		t.Skip(i018Env + " 未設定：008 §4.1 需要 workplace/i018 的 PAK 與畫格")
	}
	return d
}

func tSHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// tPak 以測試自己的程式讀 PAK：回每個條目的型別（位移最高位元組）與原始位元組。
func tPak(t *testing.T, v []byte) (types []byte, raws [][]byte) {
	t.Helper()
	if int(binary.LittleEndian.Uint32(v)) != len(v) {
		t.Fatalf("PAK 總長 %d 不等於 %d", binary.LittleEndian.Uint32(v), len(v))
	}
	first := int(binary.LittleEndian.Uint32(v[4:]) & 0xFFFFFF)
	n := (first - 4) / 4
	for i := 0; i < n; i++ {
		o := binary.LittleEndian.Uint32(v[4+4*i:])
		a, b := int(o&0xFFFFFF), len(v)
		if i+1 < n {
			b = int(binary.LittleEndian.Uint32(v[4+4*(i+1):]) & 0xFFFFFF)
		}
		types, raws = append(types, byte(o>>24)), append(raws, v[a:b])
	}
	return
}

// tLZW：9 到 12 位元、LSB 先、256 清除、257 結束、新碼從 258 起；early 時碼數到上限前一個就加寬
// （strike_commander tools/m1/scpak.py 的描述，此處獨立實作）。
func tLZW(comp []byte, early bool) ([]byte, bool) {
	width, pos, nbits := 9, 0, len(comp)*8
	dic := make([][]byte, 258)
	for i := 0; i < 256; i++ {
		dic[i] = []byte{byte(i)}
	}
	var out, prev []byte
	for pos+width <= nbits {
		var val uint32
		for i := 0; i < 4; i++ {
			if bp := pos/8 + i; bp < len(comp) {
				val |= uint32(comp[bp]) << (8 * i)
			}
		}
		code := int(val>>(pos%8)) & (1<<width - 1)
		pos += width
		if code == 257 {
			break
		}
		if code == 256 {
			dic, width, prev = dic[:258], 9, nil
			continue
		}
		var cur []byte
		switch {
		case prev == nil:
			if code >= len(dic) {
				return nil, false
			}
			cur = dic[code]
		case code < len(dic):
			cur = dic[code]
			dic = append(dic, append(append([]byte(nil), prev...), cur[0]))
		case code == len(dic):
			cur = append(append([]byte(nil), prev...), prev[0])
			dic = append(dic, cur)
		default:
			return nil, false
		}
		out = append(out, cur...)
		prev = cur
		lim := 1 << width
		if early {
			lim--
		}
		if len(dic) >= lim && width < 12 {
			width++
		}
	}
	return out, true
}

func tUnpack(t *testing.T, typ byte, raw []byte) []byte {
	t.Helper()
	if typ != 0x20 {
		return raw
	}
	want := int(binary.LittleEndian.Uint32(raw))
	for _, early := range []bool{false, true} {
		if o, ok := tLZW(raw[4:], early); ok && len(o) == want {
			return o
		}
	}
	t.Fatal("LZW 解壓長度不符")
	return nil
}

// tMenuFiles 是 008 §2 的三個子條目，另加 TOMENU 底板（條目 0，收件時要拒絕）。
type tMenuFiles struct{ buttons, plate, tmButtons, tmPlate []byte }

func loadMenuFiles(t *testing.T) tMenuFiles {
	t.Helper()
	d := i018Dir(t)
	read := func(rel, sha string) []byte {
		b, err := os.ReadFile(filepath.Join(d, rel))
		if err != nil {
			t.Skip("讀不到 " + rel + "：" + err.Error())
		}
		if tSHA(b) != sha {
			t.Fatalf("%s SHA-256 %s，要 %s", rel, tSHA(b), sha)
		}
		return b
	}
	var f tMenuFiles
	mty, mraw := tPak(t, read("dec/MAINMENU.bin", tMainMenuPAKSHA))
	f.buttons, f.plate = tUnpack(t, mty[0], mraw[0]), tUnpack(t, mty[1], mraw[1])
	tty, traw := tPak(t, read("dec/TOMENU.bin", tTomenuPAKSHA))
	f.tmPlate, f.tmButtons = tUnpack(t, tty[0], traw[0]), tUnpack(t, tty[1], traw[1])
	for _, c := range []struct {
		b   []byte
		n   int
		sha string
	}{{f.buttons, 15606, tMMButtonsSHA}, {f.plate, 8593, tMMPlateSHA}, {f.tmButtons, 10314, tTMButtonsSHA}} {
		if len(c.b) != c.n || int(binary.LittleEndian.Uint32(c.b)) != c.n || tSHA(c.b) != c.sha {
			t.Fatalf("子條目長度 %d、SHA-256 %s，要 %d、%s", len(c.b), tSHA(c.b), c.n, c.sha)
		}
	}
	return f
}

func loadIdx18(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(i018Dir(t), rel+".idx"))
	if err != nil {
		t.Skip("讀不到 " + rel + "：" + err.Error())
	}
	if len(b) != 64000 {
		t.Fatalf("%s 長度 %d", rel, len(b))
	}
	return b
}

// mmEvent、plateEvent、tmEvent 依解碼值組事件（解碼由第 3 項對畫格驗證）。
func mmEvent(t *testing.T, f tMenuFiles, k, order int) ArtEvent {
	t.Helper()
	s := mustDecode(t, f.buttons, k)
	x, y := tMMPos(k % 5)
	return ArtEvent{Steps: uint64(order), File: ArtMainMenuButtons, K: k, X: x, Y: y, Order: order, W: s.W, H: s.H, Entry: s.Pix, Base0: s.Pix, Buttons: f.buttons}
}

func plateEvent(t *testing.T, f tMenuFiles, order int) ArtEvent {
	t.Helper()
	s := mustDecode(t, f.plate, 0)
	return ArtEvent{Steps: uint64(order), File: ArtMainMenuPlate, X: tPlateX, Y: tPlateY, Order: order, W: s.W, H: s.H, Entry: s.Pix, Base0: s.Pix}
}

func tmEvent(t *testing.T, f tMenuFiles, k, order int) ArtEvent {
	t.Helper()
	s := mustDecode(t, f.tmButtons, k)
	e := tTM[k]
	return ArtEvent{Steps: uint64(order), File: ArtTomenuButtons, K: k, X: e.x, Y: e.y, Order: order, W: s.W, H: s.H, Entry: s.Pix, Base0: s.Pix}
}

func paintPix(frame []byte, s Shape, x, y int) {
	for yy := 0; yy < s.H; yy++ {
		for xx := 0; xx < s.W; xx++ {
			frame[(y+yy)*320+x+xx] = s.Pix[yy*s.W+xx]
		}
	}
}

// --- 合成 View：檔案放在 tMenuSeg，MENU 入口在 tMenuCS:0002 ---

const (
	tMenuSeg = 0xB000
	tMenuCS  = 0x1DE3
)

func (h *harness) installFile(file []byte) {
	for i, b := range file {
		h.f.mem[lin(tMenuSeg, 0)+uint32(i)] = b
	}
}

func (h *harness) setShapeArgs(desc uint16, x, y, e int) {
	h.f.w16(tDS, tShapeSP+4, desc)
	h.f.w16(tDS, tShapeSP+6, uint16(x))
	h.f.w16(tDS, tShapeSP+8, uint16(y))
	h.f.w16(tDS, tShapeSP+0x0A, 0)
	h.f.w16(tDS, tShapeSP+0x0C, tMenuSeg)
	h.f.w16(tDS, tShapeSP+0x0E, uint16(e))
}

// optShape：OPTTEST 的 +3A97:0000 入口（004 §3.1）。menuShape：MENU 的 IP 0002 入口（008 §3.1）。
func (h *harness) optShape(desc uint16, x, y, e int) {
	h.setShapeArgs(desc, x, y, e)
	h.step(tBase+segShape, 0, tShapeSP)
}

func (h *harness) menuShape(ip uint16, desc uint16, x, y, e int) {
	h.setShapeArgs(desc, x, y, e)
	h.step(tMenuCS, ip, tShapeSP)
}

func menuHarness(t *testing.T, file []byte) (*harness, *[]ArtEvent) {
	h := newHarness(t)
	h.label("Talk to Gwen", 114, 204) // 學到 base（OPTTEST 入口用）
	h.f.w8(tBase+segShape, 0, sigShapeEntry...)
	h.f.w8(tMenuCS, ipMenuShape, sigShapeEntry...)
	h.setDesc(tCurDesc, 0xA000, fullScreen)
	h.installFile(file)
	evs := &[]ArtEvent{}
	h.o.OnArt = func(ev ArtEvent) { *evs = append(*evs, ev) }
	return h, evs
}

// --- 第 1 項：收件 ---

func TestMenuEventAccept(t *testing.T) {
	f := loadMenuFiles(t)
	for k := 0; k < 10; k++ {
		x, y := tMMPos(k % 5)
		h, evs := menuHarness(t, f.buttons)
		h.optShape(tDst, x, y, k)
		if len(*evs) != 1 {
			t.Fatalf("MAINMENU k%d：事件 %d 筆", k, len(*evs))
		}
		ev := (*evs)[0]
		if ev.File != ArtMainMenuButtons || ev.K != k || ev.X != x || ev.Y != y || ev.W != 211 || ev.H != 15 || ev.G != 0 {
			t.Errorf("MAINMENU k%d：事件 %v k%d (%d,%d) %d×%d", k, ev.File, ev.K, ev.X, ev.Y, ev.W, ev.H)
		}
		if string(ev.Entry) != string(mustDecode(t, f.buttons, k).Pix) || string(ev.Buttons) != string(f.buttons) {
			t.Errorf("MAINMENU k%d：解碼陣列或按鈕檔不符", k)
		}
		if want := fmt.Sprintf("mainmenu/%d,%d/%d", x, y, k%5); artKey(ev) != want {
			t.Errorf("MAINMENU k%d：Key %q，要 %q", k, artKey(ev), want)
		}
	}
	h, evs := menuHarness(t, f.plate)
	h.optShape(tDst, tPlateX, tPlateY, 0)
	if len(*evs) != 1 || (*evs)[0].File != ArtMainMenuPlate || (*evs)[0].W != 233 || (*evs)[0].H != 104 ||
		string((*evs)[0].Entry) != string(mustDecode(t, f.plate, 0).Pix) {
		t.Fatalf("底板事件 %+v", *evs)
	}
	for k := 0; k < 4; k++ {
		e := tTM[k]
		h, evs := menuHarness(t, f.tmButtons)
		h.setDesc(0xE4B0, 0xFFFF, fullScreen)
		h.menuShape(ipMenuShape, 0xE4B0, e.x, e.y, k)
		if len(*evs) != 1 {
			t.Fatalf("TOMENU k%d：事件 %d 筆", k, len(*evs))
		}
		ev := (*evs)[0]
		if ev.File != ArtTomenuButtons || ev.K != k || ev.W != e.w || ev.H != e.h || artKey(ev) != e.rectKey ||
			string(ev.Entry) != string(mustDecode(t, f.tmButtons, k).Pix) {
			t.Errorf("TOMENU k%d：事件 %v k%d %d×%d Key %q", k, ev.File, ev.K, ev.W, ev.H, artKey(ev))
		}
	}
	// MENU 入口不學 base：base 未知時也收。
	h2 := newHarness(t)
	h2.f.w8(tMenuCS, ipMenuShape, sigShapeEntry...)
	h2.installFile(f.tmButtons)
	var n int
	h2.o.OnArt = func(ArtEvent) { n++ }
	h2.setDesc(0xE4B0, 0xFFFF, fullScreen)
	h2.menuShape(ipMenuShape, 0xE4B0, 89, 33, 0)
	if _, ok := h2.o.Base(); n != 1 || ok {
		t.Fatalf("base 未知時 MENU 入口：事件 %d、base 已知 %t", n, ok)
	}
}

func TestMenuEventReject(t *testing.T) {
	f := loadMenuFiles(t)
	for name, file := range map[string][]byte{"MAINMENU 按鈕": f.buttons, "MAINMENU 底板": f.plate, "TOMENU 按鈕": f.tmButtons} {
		t.Run(name+" 長度相同、中段改一個 byte", func(t *testing.T) {
			bad := append([]byte(nil), file...)
			bad[len(bad)/2]++
			h, evs := menuHarness(t, bad)
			h.setDesc(0xE4B0, 0xFFFF, fullScreen)
			h.optShape(tDst, 56, 34, 0)
			h.menuShape(ipMenuShape, 0xE4B0, 89, 33, 0)
			if len(*evs) != 0 || len(artLogs(h.o)) != 0 {
				t.Fatalf("雜湊不符應拒絕：事件 %d、紀錄 %+v", len(*evs), artLogs(h.o))
			}
		})
	}
	t.Run("MENU 入口要求 IP 0002 且位元組相符", func(t *testing.T) {
		h, evs := menuHarness(t, f.tmButtons)
		h.setDesc(0xE4B0, 0xFFFF, fullScreen)
		h.menuShape(0x0000, 0xE4B0, 89, 33, 0) // IP 0 走 OPTTEST 的入口（CS 不是 base + 3A97h）
		h.menuShape(0x0003, 0xE4B0, 89, 33, 0)
		h.f.w8(tMenuCS, ipMenuShape+0x0F, 0x07) // 最後一個 byte
		h.menuShape(ipMenuShape, 0xE4B0, 89, 33, 0)
		if len(*evs) != 0 {
			t.Fatalf("應拒絕，卻產生事件 %d 筆", len(*evs))
		}
		h.f.w8(tMenuCS, ipMenuShape+0x0F, 0x06)
		h.menuShape(ipMenuShape, 0xE4B0, 89, 33, 0)
		if len(*evs) != 1 {
			t.Fatal("對照組沒有被接受")
		}
	})
	t.Run("同描述元的其他檔、條目超出", func(t *testing.T) {
		blob := make([]byte, 9000)
		binary.LittleEndian.PutUint32(blob, 9000)
		for name, c := range map[string]struct {
			file []byte
			e    int
		}{"TOMENU 底板": {f.tmPlate, 0}, "不明檔": {blob, 0}, "MAINMENU k10": {f.buttons, 10}, "底板條目 1": {f.plate, 1}, "TOMENU k4": {f.tmButtons, 4}} {
			h, evs := menuHarness(t, c.file)
			h.setDesc(0xE4B0, 0xFFFF, fullScreen)
			h.menuShape(ipMenuShape, 0xE4B0, 45, 24, c.e)
			h.optShape(tDst, 45, 24, c.e)
			if len(*evs) != 0 || len(artLogs(h.o)) != 0 {
				t.Errorf("%s：應不產生事件也不記錄：事件 %d、紀錄 %+v", name, len(*evs), artLogs(h.o))
			}
		}
	})
	t.Run("描述元不是全螢幕", func(t *testing.T) {
		h, evs := menuHarness(t, f.tmButtons)
		h.menuShape(ipMenuShape, tCurDesc, 89, 33, 0)
		h.setDesc(0xE4B0, 0xFFFF, rect{0, 0, 100, 100})
		h.menuShape(ipMenuShape, 0xE4B0, 89, 33, 0)
		if len(*evs) != 0 {
			t.Fatal("應拒絕")
		}
	})
}

// MAINMENU 的 k0、k3 不觸發 SAVELOAD 的按下態判定與 dropConfirmArt（008 §3.1）。
func TestMenuNoSaveLoadRules(t *testing.T) {
	f := loadMenuFiles(t)
	file := loadSHP(t)
	ov := newArtOverlay(t, 2)
	for i, k := range []int{15, 16, 17, 19} {
		el := tElemFor(t, k)
		ov.AddDialogArt(mkEvent(t, file, k, el.x, el.y, i+1))
	}
	before := strings.Join(ov.ArtKeys(), "|")
	ov.AddDialogArt(mmEvent(t, f, 0, 10))
	ov.AddDialogArt(mmEvent(t, f, 3, 11))
	got := ov.ArtKeys()
	if len(got) != 6 || strings.Join(got[:4], "|") != before || got[4] != "mainmenu/56,34/0" || got[5] != "mainmenu/56,85/3" {
		t.Fatalf("MAINMENU k0、k3 之後圖面 %q（之前 %q）", got, before)
	}
	// k3 是一般態：趟表是陰影 150 (+1,+1)、亮邊 187 (−1,−1)、字色，沒有按下態的上緣 (0,−1)。
	e := ov.arts[5]
	if p := e.text.passes; len(p) != 3 || p[0] != (glyphPass{150, 1, 1}) || p[1] != (glyphPass{187, -1, -1}) || p[2].dx != 0 || p[2].dy != 0 {
		t.Errorf("MAINMENU k3 的趟表 %+v", p)
	}
	if mmPressed(3) || !mmPressed(8) || !mmPressed(6) || mmPressed(4) {
		t.Error("MAINMENU 按下態要以 k ≥ 5 判")
	}
}

// --- 第 2 項：元素表 ---

func TestMenuElementTable(t *testing.T) {
	for k := 0; k < 10; k++ {
		n := k % 5
		x, y := tMMPos(n)
		sp, ok := artSpecFor(ArtEvent{File: ArtMainMenuButtons, K: k, X: x, Y: y})
		want := artSpec{tMMKey[n], 211, 15, box{2, 2, 208, 12}, box{tMMCX[n][0], 3, tMMCX[n][1], 10}, fmt.Sprintf("mainmenu/%d,%d/%d", x, y, n)}
		if !ok || sp != want {
			t.Errorf("MAINMENU k%d：%+v，要 %+v", k, sp, want)
		}
	}
	for n := 0; n < 5; n++ {
		sp, ok := artSpecFor(ArtEvent{File: ArtMainMenuPlate, Line: n, X: tPlateX, Y: tPlateY})
		want := artSpec{tMMKey[n], 233, 104, box{12, 11 + 17*n, 219, 22 + 17*n},
			box{tMMCX[n][0] + 11, 13 + 17*n, tMMCX[n][1] + 11, 20 + 17*n}, fmt.Sprintf("mainmenu-plate/%d", n)}
		if !ok || sp != want {
			t.Errorf("底板第 %d 行：%+v，要 %+v", n, sp, want)
		}
	}
	for k, e := range tTM {
		sp, ok := artSpecFor(ArtEvent{File: ArtTomenuButtons, K: k, X: e.x, Y: e.y})
		want := artSpec{e.key, e.w, e.h, box{e.band[0], e.band[1], e.band[2], e.band[3]}, box{e.center[0], e.center[1], e.center[2], e.center[3]}, e.rectKey}
		if !ok || sp != want {
			t.Errorf("TOMENU k%d：%+v，要 %+v", k, sp, want)
		}
	}
	for _, ev := range []ArtEvent{{File: ArtMainMenuButtons, K: 10}, {File: ArtMainMenuPlate, K: 1}, {File: ArtTomenuButtons, K: 4}} {
		if _, ok := artSpecFor(ev); ok {
			t.Errorf("%v k%d 不是元素", ev.File, ev.K)
		}
	}
}

// --- 第 3 項：解碼對畫格 ---

// tLayout 是一幅畫格上的元素：檔、條目、位置。
type tLayout struct {
	file []byte
	e    int
	x, y int
	name string
}

func mmLayout(f tMenuFiles, ks ...int) []tLayout {
	out := []tLayout{{f.plate, 0, tPlateX, tPlateY, "底板"}}
	for _, k := range ks {
		x, y := tMMPos(k % 5)
		out = append(out, tLayout{f.buttons, k, x, y, fmt.Sprintf("k%d", k)})
	}
	return out
}

// diffBoxes 回每個元素在最上層的像素中與畫格不同的點（依 layout 順序疊層）。
func diffBoxes(t *testing.T, frame []byte, lay []tLayout) map[string][]Point {
	t.Helper()
	top := make([]int, 64000)
	for i := range top {
		top[i] = -1
	}
	want := make([]byte, 64000)
	for i, l := range lay {
		s := mustDecode(t, l.file, l.e)
		for yy := 0; yy < s.H; yy++ {
			for xx := 0; xx < s.W; xx++ {
				p := (l.y+yy)*320 + l.x + xx
				top[p], want[p] = i, s.Pix[yy*s.W+xx]
			}
		}
	}
	out := map[string][]Point{}
	for i, l := range lay {
		out[l.name] = nil
		for p := range top {
			if top[p] == i && frame[p] != want[p] {
				out[l.name] = append(out[l.name], Point{p % 320, p / 320})
			}
		}
	}
	return out
}

func ptBox(ps []Point) (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = 1<<30, 1<<30, -1, -1
	for _, p := range ps {
		x0, y0, x1, y1 = min(x0, p.X), min(y0, p.Y), max(x1, p.X), max(y1, p.Y)
	}
	return
}

func TestMenuDecodeMatchesFrames(t *testing.T) {
	f := loadMenuFiles(t)
	tm := func(ks ...int) []tLayout {
		var out []tLayout
		for _, k := range ks {
			out = append(out, tLayout{f.tmButtons, k, tTM[k].x, tTM[k].y, fmt.Sprintf("T%d", k)})
		}
		return out
	}
	// 一般態畫格：每個元素在最上層的像素與畫格全同（008 §2、ana-report §8）。
	for frame, lay := range map[string][]tLayout{
		"a5/menu": mmLayout(f, 0, 1, 2, 3, 4), "a2/o250": mmLayout(f, 2, 3, 4), "a4/menu": mmLayout(f, 1, 2, 3, 4), "m1/menu": tm(0, 1),
	} {
		for name, ps := range diffBoxes(t, loadIdx18(t, frame), lay) {
			if len(ps) != 0 {
				t.Errorf("%s %s：與畫格不同 %d 點", frame, name, len(ps))
			}
		}
	}
	// 按下態畫格：按鈕元素的差異外接矩形不超過 11×11（游標）；底板的差異只在游標經過的 x 155..167（游標 x 160）。
	for frame, lay := range map[string][]tLayout{
		"a3/press_new": mmLayout(f, 3, 4, 7), "a2/press_view": mmLayout(f, 2, 3, 9), "a5/press_load": mmLayout(f, 0, 2, 3, 4, 6), "m1/press_sc": tm(2, 1),
	} {
		for name, ps := range diffBoxes(t, loadIdx18(t, frame), lay) {
			if len(ps) == 0 {
				continue
			}
			x0, y0, x1, y1 := ptBox(ps)
			t.Logf("%s %s：差異 %d 點，外接 (%d,%d)–(%d,%d)", frame, name, len(ps), x0, y0, x1, y1)
			if name == "底板" {
				if x0 < 155 || x1 > 167 {
					t.Errorf("%s 底板：差異不在游標經過的欄", frame)
				}
				continue
			}
			if x1-x0+1 > 11 || y1-y0+1 > 11 {
				t.Errorf("%s %s：差異外接 %d×%d 超過 11×11", frame, name, x1-x0+1, y1-y0+1)
			}
		}
	}
}

// --- 第 4 項：規則 P ---

// tBandSHA16 回 pix（w 寬）在 [x0..x1]×[y0..y1] 的列優先位元組的 SHA-256 前 16 碼。
func tBandSHA16(pix []uint8, w, x0, y0, x1, y1 int) string {
	var b []byte
	for y := y0; y <= y1; y++ {
		b = append(b, pix[y*w+x0:y*w+x1+1]...)
	}
	return tSHA(b)[:16]
}

var (
	tPM   = [5]int{1308, 1005, 1533, 1718, 1361}
	tPSHA = [5]string{"e9fd8984e0a3ce17", "0f801ce4461dd18a", "87aa41a187e52bd6", "edfa047825b0018e", "4d57a961c7ab8ef0"}
)

func TestRuleP(t *testing.T) {
	f := loadMenuFiles(t)
	for n := 0; n < 5; n++ {
		a, b := mustDecode(t, f.buttons, n).Pix, mustDecode(t, f.buttons, n+5).Pix
		m, _ := ruleP(a, b)
		cnt, x0, x1 := 0, 1<<30, -1
		for i, on := range m {
			if on {
				cnt++
				x0, x1 = min(x0, i%211), max(x1, i%211)
			}
		}
		if cnt != tPM[n] || x0 != tMMCX[n][0] || x1 != tMMCX[n][1] {
			t.Errorf("k%d：M %d 點、外接 x %d..%d，要 %d、%d..%d", n, cnt, x0, x1, tPM[n], tMMCX[n][0], tMMCX[n][1])
		}
		// 兩態（一般 k、按下 k+5 的事件）回填後的無字底相同，且 SHA 相符。
		bgN, _, err := artBackground(mmEvent(t, f, n, 1))
		if err != nil {
			t.Fatal(err)
		}
		bgP, _, err := artBackground(mmEvent(t, f, n+5, 1))
		if err != nil {
			t.Fatal(err)
		}
		if string(bgN) != string(bgP) {
			t.Errorf("k%d：兩態無字底不同", n)
		}
		if s := tBandSHA16(bgN, 207, 0, 0, 206, 10); s != tPSHA[n] {
			t.Errorf("k%d：無字底 SHA %s，要 %s", n, s, tPSHA[n])
		}
	}
}

// --- 第 5 項：規則 Q ---

var (
	tQN   = [5]int{1071, 810, 1245, 1394, 1118}
	tQSHA = [5]string{"448019bde89a4d7f", "710fdd2268ce5644", "22b56e374d64f86c", "ac03c2ea1b77628d", "3edfca704604a4e2"}
)

func TestRuleQ(t *testing.T) {
	f := loadMenuFiles(t)
	plate := mustDecode(t, f.plate, 0).Pix
	for n := 0; n < 5; n++ {
		a, b := mustDecode(t, f.buttons, n).Pix, mustDecode(t, f.buttons, n+5).Pix
		m, ref := ruleP(a, b)
		_, q := ruleQ(plate, n, m, ref)
		ev := plateEvent(t, f, 1)
		ev.Line, ev.Buttons = n, f.buttons
		bg, _, err := artBackground(ev)
		if err != nil {
			t.Fatal(err)
		}
		if s := tBandSHA16(bg, 208, 0, 0, 207, 11); q != tQN[n] || s != tQSHA[n] {
			t.Errorf("第 %d 行：Q %d 點、SHA %s，要 %d、%s", n, q, s, tQN[n], tQSHA[n])
		}
	}
}

// --- 第 6 項：TOMENU ---

func TestTomenuInk(t *testing.T) {
	f := loadMenuFiles(t)
	wantInk := [4]int{1338, 1369, 1338, 1369}
	wantSHA := [4]string{"00786165bf211c63", "2e5beb70e8dbdc0f", "00786165bf211c63", "2e5beb70e8dbdc0f"}
	for k := 0; k < 4; k++ {
		ev := tmEvent(t, f, k, 1)
		sp, _ := artSpecFor(ev)
		ink := 0
		for _, on := range tomenuMask(ev.Entry, sp) {
			if on {
				ink++
			}
		}
		bg, fb := tomenuBackground(ev.Entry, sp)
		bw := sp.band.x1 - sp.band.x0 + 1
		if s := tBandSHA16(bg, bw, 0, 0, bw-1, 21); ink != wantInk[k] || len(fb) != 0 || s != wantSHA[k] {
			t.Errorf("T%d：墨跡 %d、退回 %d、SHA %s，要 %d、0、%s", k, ink, len(fb), s, wantInk[k], wantSHA[k])
		}
	}
}

// --- 第 7 項：字樣 ---

func tMode(cnt map[uint8]int) uint8 {
	best, bc := -1, 0
	for c, v := range cnt {
		if v > bc || (v == bc && int(c) < best) {
			best, bc = int(c), v
		}
	}
	return uint8(best)
}

// TestMenuGlyphConstants 以 008 §3.5 的母體重算字樣常數，比對產品常數。
func TestMenuGlyphConstants(t *testing.T) {
	f := loadMenuFiles(t)
	plate := mustDecode(t, f.plate, 0).Pix
	inR := func(c uint8, lo, hi int) bool { return int(c) >= lo && int(c) <= hi }
	for n := 0; n < 5; n++ {
		a, b := mustDecode(t, f.buttons, n).Pix, mustDecode(t, f.buttons, n+5).Pix
		m, bg := ruleP(a, b)
		fill := func(x, y int) uint8 { return bg[(y-2)*207+x-2] }
		popN, popP := map[int]bool{}, map[int]bool{}
		for i, on := range m {
			x, y := i%211, i/211
			if on && a[i] != fill(x, y) {
				popN[i] = true
			}
			if on && b[i] != fill(x, y) {
				popP[i] = true
			}
		}
		cN, sh, hl, hlN, cP, e, hb := map[uint8]int{}, map[uint8]int{}, map[uint8]int{}, map[uint8]int{}, map[uint8]int{}, map[uint8]int{}, map[uint8]int{}
		for i := range popN {
			y, c := i/211, a[i]
			if inR(c, 139, 146) {
				cN[c]++
			}
			if y == 11 && inR(c, 147, 152) {
				sh[c]++
			}
			if inR(c, 182, 188) {
				// 亮邊：§3.5 的「上鄰或左鄰不在母體」的邊緣統計。§3.5 只寫「主要亮色」，這裡取
				// （上空數 + 左空數）/ 點數最大的亮色，點數不足 10 的不取（§3.5：k3、k4 的 188 共 2、7 點，不取）。
				hlN[c]++
				if !popN[i-211] {
					hl[c]++
				}
				if !popN[i-1] {
					hl[c]++
				}
			}
		}
		hlBest, hlScore := -1, -1.0
		for c := 182; c <= 188; c++ {
			if n := hlN[uint8(c)]; n >= 10 {
				if s := float64(hl[uint8(c)]) / float64(n); s > hlScore {
					hlBest, hlScore = c, s
				}
			}
		}
		for i := range popP {
			y, c := i/211, b[i]
			if inR(c, 141, 152) {
				cP[c]++
			}
			if y == 3 {
				e[c]++
			}
			if y == 10 && inR(c, 182, 188) {
				hb[c]++
			}
		}
		got := [6]uint8{tMode(cN), tMode(sh), uint8(hlBest), tMode(cP), tMode(e), tMode(hb)}
		want := [6]uint8{mmNormalC[n], mmShadow.col, mmHighlight.col, mmPressedC[n], mmPressedE[n], mmPressedHB[n]}
		if got != want {
			t.Errorf("k%d：母體重算 c、陰影、亮邊、按下 c、e、hb = %v，產品常數 %v", n, got, want)
		}
		// hp：該行 Q0 內、下鄰不在 Q0、色號 182..188 的眾數。
		dy := 10 + 17*n
		q0 := map[int]bool{}
		for py := 13 + 17*n; py <= 20+17*n; py++ {
			for px := 13; px <= 219; px++ {
				c, r := plate[py*233+px], bg[(py-dy-2)*207+px-11-2]
				if c != r && !(c >= 177 && c <= 181) {
					q0[py*233+px] = true
				}
			}
		}
		hp := map[uint8]int{}
		for i := range q0 {
			if !q0[i+233] && inR(plate[i], 182, 188) {
				hp[plate[i]]++
			}
		}
		if tMode(hp) != plateHP[n] || plateC != 176 {
			t.Errorf("第 %d 行：hp 母體重算 %d（%v），產品 %d", n, tMode(hp), hp, plateHP[n])
		}
	}
	// TOMENU：字色 18 是墨跡內 4..21 的眾數；亮邊 193 在墨跡內。
	for k := 0; k < 2; k++ {
		ev := tmEvent(t, f, k, 1)
		sp, _ := artSpecFor(ev)
		cnt := map[uint8]int{}
		has193 := false
		for i, on := range tomenuMask(ev.Entry, sp) {
			if on && inR(ev.Entry[i], 4, 21) {
				cnt[ev.Entry[i]]++
			}
			has193 = has193 || (on && ev.Entry[i] == 193)
		}
		if tMode(cnt) != tmC || !has193 || tmEdge.col != 193 {
			t.Errorf("T%d：字色眾數 %d、193 %t，產品 %d、%d", k, tMode(cnt), has193, tmC, tmEdge.col)
		}
	}
}

// TestMenuGlyphStyle：008 §3.5 每種態的各趟色號與位移（011 §3.6.3 以填色層逐趟畫），主選單與 TOMENU 按下態的
// tx、ey 與一般態相同。測試圖集的 CJK 字是實心方塊：填色欄 [tx + 1, tx + px − 1)、列 [ey + 1, ey + px − 1)。
// 點位 (dx, dy) 相對第一個字填色區的左上角 (tx + 1, ey + 1)；(px−2) 表示填色區寬（高）減 1 再加 1，即右（下）緣外一格。
func TestMenuGlyphStyle(t *testing.T) {
	f := loadMenuFiles(t)
	pal := tPal()
	for _, sc := range []int{2, 3} {
		// check 回 (tx, ey)；pts 每點 {dx, dy, 色號}，色號 −1 表示無字底。far 為 true 時 dx、dy 從填色區右下角外一格起算。
		type pt struct {
			far    bool
			dx, dy int
			col    int
		}
		check := func(name string, ev ArtEvent, pts []pt) (int, int) {
			t.Helper()
			ov := newArtOverlay(t, sc)
			ov.addArt(ev)
			if len(ov.arts) != 1 {
				t.Fatalf("%d 倍 %s：沒有圖面（%+v %+v）", sc, name, ov.Misses, ov.Skips)
			}
			e := ov.arts[0]
			at := e.text
			if c := at.display[0]; c < 0x3000 {
				t.Fatalf("%d 倍 %s：第一字 %q 不是 CJK，點位不適用", sc, name, c)
			}
			s := e.stamp(pal, sc)
			ow := e.rect.W * sc
			fill := at.px - 2
			for _, p := range pts {
				x, y := at.tx+1+p.dx, at.ey+1+p.dy
				if p.far {
					x, y = at.tx+1+fill+p.dx, at.ey+1+fill+p.dy
				}
				i := y*ow + x
				w := pal[e.idx[i]]
				if p.col >= 0 {
					w = pal[p.col]
				}
				if got := [3]uint8{s.Pix[4*i], s.Pix[4*i+1], s.Pix[4*i+2]}; got != w || s.Pix[4*i+3] != 255 {
					t.Errorf("%d 倍 %s：點 %+v (%d,%d) 是 %v/%d，要 %v", sc, name, p, x, y, got, s.Pix[4*i+3], w)
				}
			}
			return at.tx, at.ey
		}
		for n := 0; n < 5; n++ {
			// 一般態：陰影 150 (+1,+1)、亮邊 187 (−1,−1)、字色 c（k0 139，其餘 140）。
			nc := [5]int{139, 140, 140, 140, 140}
			tx0, ey0 := check(fmt.Sprintf("k%d", n), mmEvent(t, f, n, 1), []pt{
				{false, 0, 0, nc[n]}, {false, -1, -1, 187}, {true, 0, 0, 150}, {true, -1, -1, nc[n]}, {false, -2, -2, -1}, {true, 1, 1, -1}})
			// 按下態：上緣 e (0,−1)、下緣 hb (0,+1)、字色 c；tx、ey 與一般態相同。
			eC := [5]int{139, 140, 140, 140, 140}
			hbC := [5]int{183, 184, 185, 184, 186}
			pc := 148
			if n == 4 {
				pc = 149
			}
			tx1, ey1 := check(fmt.Sprintf("k%d", n+5), mmEvent(t, f, n+5, 1), []pt{
				{false, 0, 0, pc}, {false, 0, -1, eC[n]}, {true, -1, 0, hbC[n]}, {false, -1, 0, -1}, {true, 0, -1, -1}, {false, -1, -1, -1}})
			if tx1 != tx0 || ey1 != ey0 {
				t.Errorf("%d 倍 k%d：按下態 (%d,%d) 與一般態 (%d,%d) 不同", sc, n+5, tx1, ey1, tx0, ey0)
			}
			// 底板灰字：下緣亮邊 hp (0,+1)、字色 176。
			ev := plateEvent(t, f, 1)
			ev.Line, ev.Buttons = n, f.buttons
			hp := [5]int{183, 182, 183, 183, 182}
			check(fmt.Sprintf("底板第 %d 行", n), ev, []pt{{false, 0, 0, 176}, {true, -1, 0, hp[n]}, {false, 0, -1, -1}, {true, 0, -1, -1}})
		}
		for k := 0; k < 2; k++ {
			// TOMENU：亮邊 193 (+1,+1)、字色 18；按下態（k + 2）的 tx、ey 與一般態相同。
			pts := []pt{{false, 0, 0, 18}, {true, 0, 0, 193}, {true, -1, -1, 18}, {false, -1, -1, -1}, {true, 1, 1, -1}}
			tx0, ey0 := check(fmt.Sprintf("T%d", k), tmEvent(t, f, k, 1), pts)
			tx1, ey1 := check(fmt.Sprintf("T%d", k+2), tmEvent(t, f, k+2, 1), pts)
			if tx1 != tx0 || ey1 != ey0 {
				t.Errorf("%d 倍 T%d：按下態 (%d,%d) 與一般態 (%d,%d) 不同", sc, k+2, tx1, ey1, tx0, ey0)
			}
		}
	}
}

// --- 第 8 項：灰掉與防線 ---

// menuOverlay 依序加入事件（Order 1 起），回 overlay。
func menuOverlay(t *testing.T, sc int, evs ...ArtEvent) *Overlay {
	t.Helper()
	ov := newArtOverlay(t, sc)
	for i, ev := range evs {
		ev.Order, ev.Steps = i+1, uint64(i+1)
		ov.AddDialogArt(ev)
	}
	if len(ov.Misses) != 0 || len(ov.Skips) != 0 {
		t.Fatalf("misses %+v skips %+v", ov.Misses, ov.Skips)
	}
	return ov
}

func menuEvents(t *testing.T, f tMenuFiles, ks ...int) []ArtEvent {
	evs := []ArtEvent{plateEvent(t, f, 0)}
	for _, k := range ks {
		evs = append(evs, mmEvent(t, f, k, 0))
	}
	return evs
}

func TestMenuGrayAndDefense(t *testing.T) {
	f := loadMenuFiles(t)
	for n := 0; n < 5; n++ {
		if b := plateBand(n); b.y0 != 11+17*n {
			t.Errorf("底板第 %d 行字帶 y0 %d，要含第 1 列 %d", n, b.y0, 11+17*n)
		}
	}
	for _, sc := range []int{2, 3} {
		// a4/menu：CONTINUE 灰（只有底板），LOAD 到 VIEW OBJECTS 是按鈕。
		frame := loadIdx18(t, "a4/menu")
		ov := menuOverlay(t, sc, menuEvents(t, f, 1, 2, 3, 4)...)
		out, _ := ov.Draw(frame, tPal())
		plain := ScaleIndexedRGBA(frame, tPal(), sc)
		wantShown := [5]int{208, 0, 0, 0, 0}
		for n := 0; n < 5; n++ {
			s, ok := artStampOf(t, ov, fmt.Sprintf("mainmenu-plate/%d", n))
			if !ok {
				t.Fatalf("%d 倍：沒有底板第 %d 行", sc, n)
			}
			sh := refShownCols(frame, s)
			t.Logf("%d 倍 a4/menu 底板第 %d 行：顯示 %d 欄", sc, n, sh)
			if sh != wantShown[n] {
				t.Errorf("%d 倍：底板第 %d 行顯示 %d 欄，要 %d", sc, n, sh, wantShown[n])
			}
			if n == 0 && diffsIn(out, plain, sc, s.X, s.Y, s.X+s.Cells, s.Y+s.CellH) == 0 {
				t.Errorf("%d 倍：CONTINUE 一行沒有顯示灰色中文", sc)
			}
			if n > 0 {
				x, y := tMMPos(n)
				bs, ok := artStampOf(t, ov, fmt.Sprintf("mainmenu/%d,%d/%d", x, y, n))
				if !ok {
					t.Fatalf("%d 倍：沒有按鈕 %d 的圖面", sc, n)
				}
				if bsh := refShownCols(frame, bs); bsh != 207 {
					t.Errorf("%d 倍：按鈕 %d 顯示 %d 欄，要 207", sc, n, bsh)
				}
			}
		}
		// 按下態畫格：有按鈕那幾行的底板圖面仍有欄顯示（按下態按鈕列 1 與底板字帶第 1 列部分相同），
		// 這些欄都被 Order 較大的按鈕圖面蓋住：拿掉這幾行的底板圖面，輸出不變。
		for frame, ks := range map[string][]int{"a3/press_new": {2, 3, 4, 7}, "a2/press_view": {2, 3, 4, 9}, "a5/press_load": {0, 1, 2, 3, 4, 6}} {
			idx := loadIdx18(t, frame)
			full := menuOverlay(t, sc, menuEvents(t, f, ks...)...)
			outFull, _ := full.Draw(idx, tPal())
			part := menuOverlay(t, sc, menuEvents(t, f, ks...)...)
			var shown []string
			for n := 0; n < 5; n++ {
				key := fmt.Sprintf("mainmenu-plate/%d", n)
				has := false
				for _, k := range ks {
					has = has || k%5 == n
				}
				if !has {
					continue
				}
				if s, ok := artStampOf(t, full, key); ok {
					shown = append(shown, fmt.Sprintf("第 %d 行 %d", n, refShownCols(idx, s)))
				}
				part.dropArt(func(e *artEntry) bool { return e.rect.Key == key })
			}
			outPart, _ := part.Draw(idx, tPal())
			t.Logf("%d 倍 %s：有按鈕的行，底板圖面顯示欄數 %v", sc, frame, shown)
			if string(outFull) != string(outPart) {
				t.Errorf("%d 倍 %s：底板圖面露出（輸出與拿掉底板列時不同）", sc, frame)
			}
		}
	}
}

// refShownCols 回圖面在畫格上顯示的欄數：該欄每列的畫格色號都等於 Reference（204 的逐格比對，CellW 1）。
func refShownCols(frame []byte, s *tStamp) int {
	n := 0
	for c := 0; c < s.Cells; c++ {
		ok := true
		for y := 0; y < s.CellH && ok; y++ {
			ok = frame[(s.Y+y)*320+s.X+c] == s.Reference[y*s.Cells+c]
		}
		if ok {
			n++
		}
	}
	return n
}

// --- 第 9 項：延遲建列與 Order ---

func TestMenuPlateLifecycle(t *testing.T) {
	f := loadMenuFiles(t)
	file := loadSHP(t)
	ov := newArtOverlay(t, 2)
	el := tElemFor(t, 6)
	ov.AddDialogArt(mkEvent(t, file, 6, el.x, el.y, 1)) // SAVELOAD 的圖面不受底板事件影響
	ov.AddDialogArt(plateEvent(t, f, 2))
	if k := ov.ArtKeys(); len(k) != 1 || ov.platePending == nil {
		t.Fatalf("底板事件後到按鈕事件前不得有底板圖面：%q", k)
	}
	for i, k := range []int{2, 3, 4} {
		ov.AddDialogArt(mmEvent(t, f, k, 3+i))
	}
	want := "saveload/90,142/6|mainmenu-plate/0|mainmenu-plate/1|mainmenu-plate/2|mainmenu-plate/3|mainmenu-plate/4|mainmenu/56,68/2|mainmenu/56,85/3|mainmenu/56,102/4"
	if got := strings.Join(ov.ArtKeys(), "|"); got != want || ov.platePending != nil {
		t.Fatalf("Key %q，要 %q", got, want)
	}
	maxPlate, minBtn := 0, 1<<30
	for _, e := range ov.arts {
		switch {
		case strings.HasPrefix(e.rect.Key, "mainmenu-plate/"):
			if e.ev.Order != 2 {
				t.Errorf("%s Order %d，要底板事件的 2", e.rect.Key, e.ev.Order)
			}
			maxPlate = max(maxPlate, e.ev.Order)
		case strings.HasPrefix(e.rect.Key, "mainmenu/"):
			minBtn = min(minBtn, e.ev.Order)
		}
	}
	if maxPlate >= minBtn {
		t.Errorf("底板列 Order %d 不小於按鈕 %d", maxPlate, minBtn)
	}
	// layer 的 stamp Order 也照這個順序（按鈕在上）。
	ov.Draw(make([]byte, 64000), tPal())
	if s, _ := artStampOf(t, ov, "mainmenu-plate/2"); s == nil || s.Order != 2 {
		t.Fatal("底板列 stamp 的 Order 要等於底板事件")
	}
	// 按下、放開：同 Key 替換，底板列不變。
	ov.AddDialogArt(mmEvent(t, f, 9, 6))
	if got := strings.Join(ov.ArtKeys(), "|"); got != want {
		t.Fatalf("按下後 Key %q", got)
	}
	// 第二次底板事件（a6 重畫）移除上一輪的 mainmenu、mainmenu-plate 列。
	ov.AddDialogArt(plateEvent(t, f, 7))
	if got := strings.Join(ov.ArtKeys(), "|"); got != "saveload/90,142/6" {
		t.Fatalf("第二次底板事件後 Key %q", got)
	}
	ov.Draw(make([]byte, 64000), tPal())
	if len(ov.artLayer.Art) != 1 {
		t.Fatalf("layer 還有 %d 列", len(ov.artLayer.Art))
	}
	ov.AddDialogArt(mmEvent(t, f, 2, 8))
	for _, e := range ov.arts {
		if strings.HasPrefix(e.rect.Key, "mainmenu-plate/") && e.ev.Order != 7 {
			t.Errorf("重畫後 %s Order %d，要 7", e.rect.Key, e.ev.Order)
		}
	}
	// 五列的建立與觸發它的按鈕事件是否產生圖面無關。
	gfx := testGfx()
	delete(gfx, "START NEW GAME")
	ov2, err := NewOverlayText(testTextFonts(t), testCatalog(), gfx, 2)
	if err != nil {
		t.Fatal(err)
	}
	ov2.AddDialogArt(plateEvent(t, f, 1))
	ov2.AddDialogArt(mmEvent(t, f, 2, 2))
	if got := strings.Join(ov2.ArtKeys(), "|"); got != "mainmenu-plate/0|mainmenu-plate/1|mainmenu-plate/3|mainmenu-plate/4" || len(ov2.Misses) != 2 {
		t.Fatalf("按鈕事件 Miss 時 Key %q、misses %+v", got, ov2.Misses)
	}
	// 換倍率重建：底板列自帶行號與按鈕檔。
	ov.SetScale(3)
	if len(ov.arts) != 7 {
		t.Fatalf("換倍率後 %d 列：%q", len(ov.arts), ov.ArtKeys())
	}
}

// --- 第 10 項：lint ---

func TestMenuLint(t *testing.T) {
	fonts := testTextFonts(t)
	b, err := os.ReadFile("testdata/menu_gfx.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	keys := ArtCatalogKeys()
	want := []string{"SAVE GAME", "LOAD GAME", "SAVE", "LOAD", "CANCEL", "QUIT TO DOS", "ARE YOU SURE", "OVERWRITE", "YES", "NO",
		"CONTINUE GAME", "START NEW GAME", "TRAINING MISSION", "VIEW OBJECTS", "PLAY STRIKE COMMANDER", "PLAY TACTICAL OPERATIONS"}
	if a, w := append([]string(nil), keys...), append([]string(nil), want...); len(a) != 16 || func() bool { sort.Strings(a); sort.Strings(w); return strings.Join(a, "|") != strings.Join(w, "|") }() {
		t.Fatalf("ArtCatalogKeys %q", keys)
	}
	if rep := LintArtText(b, fonts); !rep.OK() {
		t.Fatalf("testdata 應通過：%v", rep.Errors)
	}
	for _, k := range want {
		var kept []string
		for _, line := range strings.SplitAfter(string(b), "\n") {
			if !strings.HasPrefix(line, k+"\t") {
				kept = append(kept, line)
			}
		}
		s := strings.Join(kept, "")
		if s == string(b) {
			t.Fatalf("缺 %s：替換沒有生效", k)
		}
		if rep := LintArtText([]byte(s), fonts); rep.OK() {
			t.Errorf("缺 %s 應報錯", k)
		}
	}
}

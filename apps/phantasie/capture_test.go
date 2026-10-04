package phantasie

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 唯讀鉤子（capture.go）的單元測試（docs/spec/001 §3、§10 第 1 項，002 §3 至 §5）。
// 不需要 oracle：記憶體用 capMem 假造，鉤子以 handleA、handleB、handleS、handleT、opEntry、opExit 直接驅動。
// 期望值都由規格推導成字面值，不拿被測函式的輸出當期望；雜湊用本檔自己的 crypto/sha256 與 hash/fnv 算，
// 不呼叫被測的 fnv64。輔助函式與型別一律加 cap 前綴。
//
// 假記憶體是 20 位元線性位址（段 × 16 + 偏移）的 1 MiB 陣列，所以 BA00:0000 與 B800:2000 是同一個位元組，
// 與原版的 CGA 兩個 bank 相同。位址與 DGROUP 偏移一律用字面值，不引用 capture.go 的常數，
// 常數被改動時簽章驗證或比對會對不上。
//
// DGROUP 內的配置（本檔的約定）：
//
//	0x1000、0x1010  靜態字串 "Hello"（FmtPtr 的兩個不同值）
//	0x1020          靜態字串 "%s"
//	0x638E          行緩衝區（buffer）
//	0x7522          怪物記錄（monster）
//	0x8071          城鎮名（town）
//	0x5000          玩家記錄（other）
//	0xFC00 以上     堆疊緩衝區（大於等於事件的 SP，buffer）

const (
	capImg    = 0x1000 // 映像段（測試值）
	capDG     = 0x1C9F // DGROUP 段 = 映像段 + 0C9Fh
	capBufSeg = 0x3000 // 頁面緩衝區段，寫在 DS:5BAA
	capResEnd = 0x53EA // 常駐碼結尾（002 §3）
	capFontAt = 0x3244 // FONT 在 DGROUP 內的位置（001 §8）
	capFontSz = 2032
	capText   = 0x3A36 // 格式化結果緩衝區
	capSP     = 0xFA00 // 事件 A 的 SP
	capBP     = 0xFA10 // 事件 A 的 BP
	capOpSP   = 0xE000 // S、T、畫面操作入口的 SP
)

// ---------- 假記憶體 ----------

type capMem struct {
	ram []byte
	dg  uint16
}

var _ memView = (*capMem)(nil)

func newCapMem(dg uint16) *capMem { return &capMem{ram: make([]byte, 0x110000), dg: dg} }

func capLin(seg, off uint16) int { return int(seg)<<4 + int(off) }

// read 回複本，長度恆為 n（超出範圍補 0），與 oracle.Bytes 一致。鉤子會改回傳的切片（row24 的新雜湊），所以不能回別名。
func (m *capMem) read(lin, n int) []byte {
	out := make([]byte, n)
	if lin >= 0 && lin < len(m.ram) {
		copy(out, m.ram[lin:])
	}
	return out
}

func (m *capMem) Bytes(off uint16, n int) []byte { return m.read(capLin(m.dg, off), n) }
func (m *capMem) Word(off uint16) uint16 {
	b := m.read(capLin(m.dg, off), 2)
	return uint16(b[0]) | uint16(b[1])<<8
}
func (m *capMem) Far(seg, off uint16, n int) []byte { return m.read(capLin(seg, off), n) }

func (m *capMem) setFar(seg, off uint16, b []byte) { copy(m.ram[capLin(seg, off):], b) }
func (m *capMem) setBytes(off uint16, b []byte)    { m.setFar(m.dg, off, b) }
func (m *capMem) setWord(off, v uint16)            { m.setBytes(off, []byte{byte(v), byte(v >> 8)}) }
func (m *capMem) setStr(off uint16, s string)      { m.setBytes(off, append([]byte(s), 0)) }

// ---------- 簽章表（規格字面值） ----------

// capSig 是 001 §3.1 表與 002 §3 的簽章：offset 是映像位移。
type capSig struct {
	name string
	off  int
	want []byte
}

var capSigs = []capSig{
	{"A", 0x26E9, []byte{0x8B, 0x46, 0x06}},
	{"B", 0x277D, []byte{0x8B, 0xC6, 0x5E, 0x5F}},
	{"L", 0x3D0F, []byte{0x8B, 0xDC, 0x8B, 0x47, 0x02}},
	{"S", 0x3E35, []byte{0x55, 0x8B, 0xEC}},
	{"T", 0x4DEF, []byte{0x55, 0x8B, 0xEC, 0xBA, 0xFF, 0x7F}},
	{"25A5", 0x25A5, []byte{0x55, 0x8B, 0xEC}},
}

// capOps 是 002 §3 的八個畫面操作掛點（入口、完成的映像位移）。
var capOps = []struct {
	name        string
	entry, exit int
}{
	{"int86", 0x4E60, 0x4EE0},
	{"invert", 0x0672, 0x073F},
	{"invert2", 0x0740, 0x0799},
	{"save1", 0x0D30, 0x0D4F},
	{"copy12", 0x0D50, 0x0D70},
	{"load1", 0x0CF0, 0x0D0F},
	{"load2", 0x0D10, 0x0D2F},
	{"row24", 0x372D, 0x376D},
}

var capEntrySig = []byte{0x55, 0x8B, 0xEC}

// ---------- 夾具 ----------

type capFx struct {
	t    *testing.T
	m    *capMem
	ov   *Overlay
	h    *Hooks
	mode uint8
	img  []byte // 映像內容（0x6000 bytes），與假記憶體同步，供期望雜湊
	font []byte // FONT 內容（2032 bytes）
}

func capLang() *Language {
	return &Language{
		Name:    "zh-TW",
		Cat:     NewCatalog(map[string]string{"Hello": "你好", "Bye": "再見"}, nil, nil),
		Font:    &xlate.Font{Name: "t"},
		Wide:    func(r rune) bool { return r > 0x2E7F },
		Enabled: true,
	}
}

// capPattern 回確定性的偽隨機位元組（LCG），不同 seed 得到互異的內容。
func capPattern(n int, seed uint32) []byte {
	out := make([]byte, n)
	x := seed*2654435761 + 12345
	for i := range out {
		x = x*1103515245 + 12345
		out[i] = byte(x >> 16)
	}
	return out
}

// capNew 建立夾具：映像填非零底圖並蓋上全部簽章，FONT 放在 DS:3244，視訊模式 04h，顯示語言 zh-TW。
func capNew(t *testing.T) *capFx {
	t.Helper()
	m := newCapMem(capDG)
	img := make([]byte, 0x6000)
	for i := range img {
		img[i] = byte(i%251 + 1)
	}
	for _, s := range capSigs {
		copy(img[s.off:], s.want)
	}
	for _, op := range capOps {
		copy(img[op.entry:], capEntrySig)
		img[op.exit] = 0xC3
	}
	m.setFar(capImg, 0, img)
	font := capPattern(capFontSz, 77)
	m.setBytes(capFontAt, font)
	// 頁面緩衝區描述（002 §2）：段、緩衝區 1 偏移、緩衝區 2 偏移。
	m.setWord(0x5BAA, capBufSeg)
	m.setWord(0x5BAC, 0)
	m.setWord(0x5BAE, 0x4000)
	m.setStr(0x1000, "Hello")
	m.setStr(0x1010, "Hello")
	m.setStr(0x1020, "%s")

	ov := NewOverlay()
	ov.AddLanguage(capLang())
	if err := ov.SetDisplay("zh-TW"); err != nil {
		t.Fatalf("SetDisplay：%v", err)
	}
	regions, err := DefaultRegions()
	if err != nil {
		t.Fatalf("DefaultRegions：%v", err)
	}
	f := &capFx{t: t, m: m, ov: ov, mode: 4, img: img, font: font}
	f.h = NewHooks(ov, regions, capImg, capDG, m, func() uint8 { return f.mode })
	return f
}

// poke 改映像的位元組（假記憶體與期望用的副本一起改）。
func (f *capFx) poke(off int, b ...byte) {
	copy(f.img[off:], b)
	f.m.setFar(capImg, uint16(off), b)
}

// arm 讓鉤子通過簽章驗證（以 gate 觸發，不經任何具體鉤子，計數器保持乾淨）。
func (f *capFx) arm() {
	f.t.Helper()
	ok, just := f.h.gate()
	if !ok || !just || !f.h.Armed() || f.h.Failed() {
		f.t.Fatalf("arm：ok=%v just=%v armed=%v failed=%v diag=%q", ok, just, f.h.Armed(), f.h.Failed(), f.h.Diag)
	}
}

// capCounters 要求非 0 計數器恰好等於 want。
func capCounters(t *testing.T, f *capFx, want map[string]uint64) {
	t.Helper()
	if want == nil {
		want = map[string]uint64{}
	}
	got := map[string]uint64{}
	for k, v := range f.ov.C.Snapshot() {
		if v != 0 {
			got[k] = v
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("計數器不符：\n  實際 %v\n  期望 %v", got, want)
	}
}

func capSha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func capFNV(b []byte) uint64 {
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}

// ---------- 事件 A、S、T 的呼叫輔助 ----------

// capCall 是一次 25A5 呼叫在 A 時的狀態。
type capCall struct {
	step       uint64
	sp, bp     uint16
	caller     uint16
	col, row   uint16
	fmtPtr     uint16
	args       []uint16
	text       string
	di         int // 負值表示取 min(len(text), 40)
	dsMismatch bool
}

func capBase() capCall {
	return capCall{step: 100, sp: capSP, bp: capBP, caller: 0x1234, col: 3, row: 5, fmtPtr: 0x1000, text: "Hello", di: -1}
}

// drawA 把呼叫狀態寫進假記憶體並觸發 A。
func (f *capFx) drawA(c capCall) {
	f.t.Helper()
	m := f.m
	m.setWord(c.bp+2, c.caller)
	m.setWord(c.bp+4, c.col)
	m.setWord(c.bp+6, c.row)
	m.setWord(c.bp+8, c.fmtPtr)
	for i := 0; i < 12; i++ {
		var v uint16
		if i < len(c.args) {
			v = c.args[i]
		}
		m.setWord(c.bp+10+uint16(2*i), v)
	}
	m.setStr(capText, c.text)
	di := c.di
	if di < 0 {
		di = len(c.text)
		if di > 40 {
			di = 40
		}
	}
	f.h.handleA(aRegs{step: c.step, sp: c.sp, bp: c.bp, di: uint16(di), dsMatches: !c.dsMismatch})
}

// open 回開啟中的事件記錄（沒有就失敗）。
func (f *capFx) open() *EventRecord {
	f.t.Helper()
	rec, ok := f.ov.Open()
	if !ok {
		f.t.Fatalf("沒有開啟中的事件")
	}
	return rec
}

// sprintf 模擬原版 sprintf(dest, fmt, args...)：先觸發 S（入口，此時 dest 還是舊內容），再把結果寫進 DS。
func (f *capFx) sprintf(step uint64, sp, caller, dest, fmtPtr uint16, result string, args ...uint16) {
	f.t.Helper()
	m := f.m
	m.setWord(sp, caller)
	m.setWord(sp+2, dest)
	m.setWord(sp+4, fmtPtr)
	for i := 0; i < 12; i++ {
		var v uint16
		if i < len(args) {
			v = args[i]
		}
		m.setWord(sp+6+uint16(2*i), v)
	}
	f.h.handleS(step, sp)
	m.setStr(dest, result)
}

// strcatHook 只觸發 T（入口）；strcat 的效果由呼叫端另外寫入。
func (f *capFx) strcatHook(sp, dest, src uint16) {
	f.m.setWord(sp+2, dest)
	f.m.setWord(sp+4, src)
	f.h.handleT(sp)
}

// strcat 觸發 T，再把追加後的內容寫進 DS（原版在 hook 之後才執行 strcat）。
func (f *capFx) strcat(sp, dest, src uint16, result string) {
	f.strcatHook(sp, dest, src)
	f.m.setStr(dest, result)
}

// ---------- 疊字層輔助 ----------

// capSeedGroup 放一個事件組：records 內的事件記錄加 Layer 內一筆 Shown 疊字（矩形等於事件矩形）。
func capSeedGroup(f *capFx, id string, col, row int, text string) {
	f.ov.records[id] = &EventRecord{ID: id, SP: capSP, BP: capBP, Caller: 0x1234, Col: col, Row: row,
		Text: text, FmtPtr: 0x1000, Format: text, FmtKind: KindStatic, Cells: []byte(text)}
	capStamp(f, id, col*8, row*8, len(text))
}

// capStamp 在 Layer 內加一筆 Shown 疊字（每格 8×8）。
func capStamp(f *capFx, key string, x, y, cells int) *xlate.Stamp {
	s := &xlate.Stamp{Key: key, X: x, Y: y, Cells: cells, CellW: 8, CellH: 8, State: xlate.Shown}
	f.ov.Layer.Stamps = append(f.ov.Layer.Stamps, s)
	return s
}

// capKeys 回 Layer 內疊字的 Key（依出現順序、去重）。
func capKeys(f *capFx) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range f.ov.Layer.Stamps {
		if !seen[s.Key] {
			seen[s.Key] = true
			out = append(out, s.Key)
		}
	}
	return out
}

func capStates(f *capFx) map[string]xlate.State {
	out := map[string]xlate.State{}
	for _, s := range f.ov.Layer.Stamps {
		out[s.Key+"@"+fmt.Sprint(s.X)] = s.State
	}
	return out
}

// op 觸發一個畫面操作的入口與完成。
func (f *capFx) op(name string) {
	f.h.opEntry(name, capOpSP)
	f.h.opExit(name)
}

// int86Entry 寫入 int86 的堆疊參數（[SP+2] 中斷號、[SP+4] 輸入 REGS 指標）與暫存器內容並觸發入口。
func (f *capFx) int86Entry(intNo, in, ax, bx, cx, dx uint16) {
	f.m.setWord(capOpSP+2, intNo)
	f.m.setWord(capOpSP+4, in)
	f.m.setWord(in, ax)
	f.m.setWord(in+2, bx)
	f.m.setWord(in+4, cx)
	f.m.setWord(in+6, dx)
	f.h.opEntry("int86", capOpSP)
}

func capIDs(s Shadow) []string {
	var out []string
	for _, e := range s {
		out = append(out, e.ID)
	}
	return out
}

// ---------- 簽章驗證 ----------

func TestCaptureVerifyArmsAndRecordsHashes(t *testing.T) {
	triggers := []struct {
		name string
		fire func(f *capFx)
	}{
		{"A", func(f *capFx) { f.h.handleA(aRegs{step: 1, sp: capSP, bp: capBP, dsMatches: false}) }},
		{"B", func(f *capFx) { f.h.handleB() }},
		{"S", func(f *capFx) { f.h.handleS(1, capOpSP) }},
		{"T", func(f *capFx) { f.h.handleT(capOpSP) }},
		{"opEntry", func(f *capFx) { f.h.opEntry("copy12", capOpSP) }},
		{"opExit", func(f *capFx) { f.h.opExit("copy12") }},
	}
	for _, tr := range triggers {
		t.Run(tr.name, func(t *testing.T) {
			f := capNew(t)
			if f.h.Armed() || f.h.Failed() || f.h.Diag != "" {
				t.Fatalf("尚未觸發時不得有狀態：armed=%v failed=%v diag=%q", f.h.Armed(), f.h.Failed(), f.h.Diag)
			}
			tr.fire(f)
			if !f.h.Armed() || f.h.Failed() || f.h.Diag != "ok" {
				t.Fatalf("armed=%v failed=%v diag=%q，期望 armed、ok", f.h.Armed(), f.h.Failed(), f.h.Diag)
			}
			// 映像雜湊：SHA-256 over 映像位移 0000 至 53EA（不含），002 §3。
			if want := capSha(f.img[:0x53EA]); f.h.ImageHash != want {
				t.Errorf("ImageHash = %s，期望 %s", f.h.ImageHash, want)
			}
			if want := capSha(f.font); f.h.FontHash != want {
				t.Errorf("FontHash = %s，期望 %s", f.h.FontHash, want)
			}
			if !reflect.DeepEqual(f.ov.font, f.font) {
				t.Errorf("Overlay 的 FONT 與 DS:3244 起 2032 bytes 不同")
			}
		})
	}
}

// 映像雜湊的範圍恰好是 [0, 53EA)：改 53E9 要變，改 53EA 不變。
func TestCaptureImageHashRange(t *testing.T) {
	base := capNew(t)
	base.arm()
	inside := capNew(t)
	inside.poke(0x53E9, base.img[0x53E9]^0xFF)
	inside.arm()
	outside := capNew(t)
	outside.poke(0x53EA, base.img[0x53EA]^0xFF)
	outside.poke(0x5FFF, 0x00)
	outside.arm()
	if inside.h.ImageHash == base.h.ImageHash {
		t.Errorf("改 53E9 後雜湊應不同")
	}
	if outside.h.ImageHash != base.h.ImageHash {
		t.Errorf("改 53EA 以後的位元組雜湊不應改變")
	}
	if want := capSha(base.img[:0x53EA]); base.h.ImageHash != want {
		t.Errorf("ImageHash = %s，期望 %s", base.h.ImageHash, want)
	}
}

// FONT 雜湊的範圍恰好是 DS:3244 起 2032 bytes。
func TestCaptureFontHashRange(t *testing.T) {
	f := capNew(t)
	f.m.setBytes(0x3244-1, []byte{0xEE})
	f.m.setBytes(0x3244+2032, []byte{0xEE})
	f.arm()
	if want := capSha(f.font); f.h.FontHash != want {
		t.Errorf("FontHash = %s，期望 %s（只含 3244 起 2032 bytes）", f.h.FontHash, want)
	}
}

// 規格 001 §8 只寫「在驗證點讀入」；程式在第一個 A 再讀一次（capture.go 的 fontChecked）。
// 本測試固定現況：第一個 A 重讀、之後不再重讀。
func TestCaptureFontRefreshedOnFirstA(t *testing.T) {
	f := capNew(t)
	f.arm()
	f2 := capPattern(capFontSz, 78)
	f.m.setBytes(0x3244, f2)
	f.drawA(capBase())
	if f.h.FontHash != capSha(f2) || !reflect.DeepEqual(f.ov.font, f2) {
		t.Errorf("第一個 A 應重讀 FONT：hash=%s", f.h.FontHash)
	}
	f3 := capPattern(capFontSz, 79)
	f.m.setBytes(0x3244, f3)
	f.h.handleB()
	f.drawA(capCall{step: 101, sp: capSP, bp: capBP, caller: 1, col: 1, row: 1, fmtPtr: 0x1000, text: "Hello", di: -1})
	if f.h.FontHash != capSha(f2) {
		t.Errorf("第二個 A 不應再重讀 FONT")
	}
}

func TestCaptureVerifyFailsOnAnySignature(t *testing.T) {
	type tc struct {
		name   string // 掛點名稱
		part   string // entry、exit 或 sig
		off    int
		want   []byte
		corrup int // 被破壞的位元組位移（相對 off）
	}
	var cases []tc
	for _, s := range capSigs {
		cases = append(cases, tc{s.name, "sig", s.off, s.want, 0}, tc{s.name, "sig", s.off, s.want, len(s.want) - 1})
	}
	for _, op := range capOps {
		cases = append(cases,
			tc{op.name, "entry", op.entry, capEntrySig, 0},
			tc{op.name, "entry", op.entry, capEntrySig, 2},
			tc{op.name, "exit", op.exit, []byte{0xC3}, 0})
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%s/%d", c.name, c.part, c.corrup), func(t *testing.T) {
			f := capNew(t)
			f.poke(c.off+c.corrup, f.img[c.off+c.corrup]^0xFF)
			f.h.handleB() // 第一次觸發，模式 04h，驗證
			if f.h.Armed() || !f.h.Failed() {
				t.Fatalf("armed=%v failed=%v，期望停用", f.h.Armed(), f.h.Failed())
			}
			d := f.h.Diag
			if d == "" || d == "ok" {
				t.Fatalf("Diag = %q，要有診斷", d)
			}
			// 掛點名稱：名稱之後接空白或全形括號，避免 invert 與 invert2 混在一起。
			if !regexp.MustCompile("掛點 " + regexp.QuoteMeta(c.name) + "[ （]").MatchString(d) {
				t.Errorf("Diag 沒有掛點名稱 %q：%s", c.name, d)
			}
			actual := fmt.Sprintf("% X", f.img[c.off:c.off+len(c.want)])
			if !strings.Contains(d, actual) {
				t.Errorf("Diag 沒有實際位元組 %q：%s", actual, d)
			}
			if c.part != "exit" {
				if exp := fmt.Sprintf("% X", c.want); !strings.Contains(d, exp) {
					t.Errorf("Diag 沒有期望位元組 %q：%s", exp, d)
				}
			}
			capCounters(t, f, nil)
		})
	}
}

// 失敗後永久停用：所有鉤子不處理、不計任何計數器（連 prearm 也不計），即使映像後來被修好或模式改變。
func TestCaptureFailedDisablesAllHooks(t *testing.T) {
	f := capNew(t)
	f.poke(0x3E35, 0x00) // S 簽章壞
	f.h.handleS(1, capOpSP)
	if !f.h.Failed() {
		t.Fatalf("應失敗")
	}
	diag := f.h.Diag
	f.poke(0x3E35, 0x55) // 修好
	f.m.setStr(0x1000, "Hello")
	exercise := func() {
		f.drawA(capBase())
		f.h.handleB()
		f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1000, "Hello")
		f.strcat(capOpSP, 0x638E, 0x1000, "HelloHello")
		for _, op := range capOps {
			f.h.opEntry(op.name, capOpSP)
		}
		for _, op := range capOps {
			f.h.opExit(op.name)
		}
		f.h.opExit("save1")
	}
	exercise()
	f.mode = 3
	exercise()
	f.mode = 4
	exercise()
	if f.h.Armed() || !f.h.Failed() || f.h.Diag != diag {
		t.Errorf("永久停用：armed=%v failed=%v diag=%q", f.h.Armed(), f.h.Failed(), f.h.Diag)
	}
	capCounters(t, f, nil)
	if f.h.Busy() {
		t.Errorf("停用後不得有在途操作")
	}
	if _, ok := f.ov.Open(); ok {
		t.Errorf("停用後不得開啟事件")
	}
	if _, miss := f.h.Compose.Match(0x638E, "Hello"); miss != MissDest {
		t.Errorf("停用後不得記錄 sprintf，miss = %v", miss)
	}
}

// 視訊模式不是 04h 時所有鉤子被忽略並計 prearm；此時不做驗證（壞映像不會被判失敗）。
func TestCapturePrearmIgnoresHooksOutsideMode04(t *testing.T) {
	for _, mode := range []uint8{0x00, 0x03, 0x05, 0x13, 0xFF} {
		t.Run(fmt.Sprintf("mode%02X", mode), func(t *testing.T) {
			f := capNew(t)
			f.mode = mode
			f.poke(0x26E9, 0x00) // A 簽章壞：若在非 04h 就驗證會誤判
			f.drawA(capCall{step: 1, sp: capSP, bp: capBP, caller: 1, col: 1, row: 1, fmtPtr: 0x1000, text: "Hello", di: -1, dsMismatch: true})
			f.h.handleB()
			f.sprintf(2, capOpSP, 0x4321, 0x638E, 0x1000, "Hello")
			f.strcatHook(capOpSP, 0x638E, 0x1000)
			f.h.opEntry("invert", capOpSP)
			f.h.opExit("invert")
			capCounters(t, f, map[string]uint64{"prearm": 6})
			if f.h.Armed() || f.h.Failed() || f.h.Diag != "" {
				t.Errorf("armed=%v failed=%v diag=%q，期望尚未驗證", f.h.Armed(), f.h.Failed(), f.h.Diag)
			}
			if f.h.Busy() {
				t.Errorf("prearm 的入口不得留下在途操作")
			}
			if _, ok := f.ov.Open(); ok {
				t.Errorf("prearm 的 A 不得開啟事件")
			}
			if _, miss := f.h.Compose.Match(0x638E, "Hello"); miss != MissDest {
				t.Errorf("prearm 的 S 不得記錄")
			}
			// 切到 04h：下一次觸發才驗證，這裡 A 簽章壞所以失敗。
			f.mode = 4
			f.h.handleB()
			if !f.h.Failed() || !strings.Contains(f.h.Diag, "掛點 A") {
				t.Errorf("模式 04h 的第一次觸發應做驗證並失敗：%q", f.h.Diag)
			}
		})
	}
}

// 修好映像後在 04h 驗證通過；前面的 prearm 計數保留。
func TestCapturePrearmThenArmedAtMode04(t *testing.T) {
	f := capNew(t)
	f.mode = 3
	f.h.handleB()
	f.h.handleT(capOpSP)
	f.mode = 4
	f.h.handleS(1, capOpSP)
	if !f.h.Armed() {
		t.Fatalf("04h 的第一次觸發應通過驗證：%q", f.h.Diag)
	}
	capCounters(t, f, map[string]uint64{"prearm": 2})
}

// 驗證觸發的那次 B 與操作完成：入口被忽略、完成才 armed，記 prearm_completion，不記 dup_close。
func TestCapturePrearmCompletion(t *testing.T) {
	t.Run("B", func(t *testing.T) {
		f := capNew(t)
		f.h.handleB()
		capCounters(t, f, map[string]uint64{"prearm_completion": 1})
		if !f.h.Armed() {
			t.Errorf("應 armed")
		}
		if _, ok := f.ov.Open(); ok {
			t.Errorf("不得開啟事件")
		}
		// armed 之後的 B 若沒有事件才是 dup_close。
		f.h.handleB()
		capCounters(t, f, map[string]uint64{"prearm_completion": 1, "dup_close": 1})
	})
	for _, op := range capOps {
		t.Run(op.name, func(t *testing.T) {
			f := capNew(t)
			f.mode = 3
			f.h.opEntry(op.name, capOpSP) // 入口時模式還是 03h
			f.mode = 4
			f.h.opExit(op.name) // 完成時已是 04h，驗證由此觸發
			capCounters(t, f, map[string]uint64{"prearm": 1, "prearm_completion": 1})
			if !f.h.Armed() {
				t.Errorf("應 armed")
			}
			if f.h.Busy() {
				t.Errorf("沒有入口就沒有在途操作")
			}
			// 之後的完成重複才是 dup_close。
			f.h.opExit(op.name)
			capCounters(t, f, map[string]uint64{"prearm": 1, "prearm_completion": 1, "dup_close": 1})
		})
	}
}

// 驗證通過的那一次 A、S、T 照常處理（001 §3.2：簽章通過後每個 A 都開啟事件）；只有 B 與完成因為沒有入口而記 prearm_completion。
func TestCaptureVerifyingTriggerIsProcessed(t *testing.T) {
	t.Run("A", func(t *testing.T) {
		f := capNew(t)
		f.drawA(capBase()) // 還沒 arm，這次 A 觸發驗證
		if !f.h.Armed() {
			t.Fatalf("應已 armed：%q", f.h.Diag)
		}
		if rec := f.open(); rec.ID != "g1" || rec.Text != "Hello" {
			t.Errorf("驗證觸發的 A 應開啟事件：%+v", rec)
		}
		f.h.handleB()
		capCounters(t, f, map[string]uint64{"events": 1, "translated": 1})
	})
	t.Run("S", func(t *testing.T) {
		f := capNew(t)
		f.m.setStr(0x1050, "You hit %s")
		f.m.setStr(0x7522, "Orc")
		f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1050, "You hit Orc", 0x7522)
		if pc, _ := f.h.Compose.Match(0x638E, "You hit Orc"); pc == nil {
			t.Errorf("驗證觸發的 S 應記錄 sprintf")
		}
	})
	t.Run("T", func(t *testing.T) {
		f := capNew(t)
		f.strcatHook(capOpSP, 0xFF8E, 0x1020)
		capCounters(t, f, map[string]uint64{"no_rec": 1})
	})
}

// ---------- handleA：EventRecord ----------

func TestCaptureHandleARecordFields(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.h.overlay = "OV1.TEST"
	f.m.setStr(0x1040, "%d %s %ld %s")
	f.m.setStr(0x1100, "Decoy")
	f.m.setStr(0x7522, "Orc")
	f.m.setStr(0x8071, "Tarok")
	c := capBase()
	c.step, c.col, c.row, c.fmtPtr = 777, 5, 7, 0x1040
	// %d、%s（怪物）、%ld（兩個字組，高字組是指向 Decoy 的誘餌）、%s（城鎮）。
	c.args = []uint16{3, 0x7522, 0x1170, 0x1100, 0x8071, 0x0505, 0x0506, 0x0507, 0x0508, 0x0509, 0x050A, 0x050B}
	c.text = "3 Orc 285217136 Tarok" // 0x11001170 = 285217136
	f.drawA(c)
	want := EventRecord{
		ID: "g1", Step: 777, SP: 0xFA00, BP: 0xFA10, Caller: 0x1234, Overlay: "OV1.TEST",
		Col: 5, Row: 7, Text: "3 Orc 285217136 Tarok", FmtPtr: 0x1040, Format: "%d %s %ld %s", FmtKind: KindStatic,
		Args:    [12]uint16{3, 0x7522, 0x1170, 0x1100, 0x8071, 0x0505, 0x0506, 0x0507, 0x0508, 0x0509, 0x050A, 0x050B},
		ArgStrs: []ArgStr{{Ptr: 0x7522, Content: "Orc", Kind: KindMonster}, {Ptr: 0x8071, Content: "Tarok", Kind: KindTown}},
		Cells:   []byte("3 Orc 285217136 Tarok"),
	}
	got := f.open()
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("EventRecord 不符：\n  實際 %+v\n  期望 %+v", *got, want)
	}
	capCounters(t, f, map[string]uint64{"events": 1})
}

// 格式字串指標的種類：靜態、行緩衝區、堆疊、其他；overlay 名稱影響 overlay 資料區的分類。
func TestCaptureFmtKind(t *testing.T) {
	cases := []struct {
		name    string
		fmtPtr  uint16
		overlay string
		want    Kind
	}{
		{"靜態", 0x1000, "", KindStatic},
		{"行緩衝區", 0x638E, "", KindBuffer},
		{"堆疊（大於等於 SP）", 0xFC00, "", KindBuffer},
		{"其他", 0x5000, "", KindOther},
		{"overlay 資料區，OV1 載入中", 0xB900, "OV1.TEST", KindStatic},
		{"overlay 資料區，沒有 overlay", 0xB900, "", KindOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := capNew(t)
			f.arm()
			f.h.overlay = c.overlay
			f.m.setStr(c.fmtPtr, "Hello")
			cc := capBase()
			cc.fmtPtr = c.fmtPtr
			f.drawA(cc)
			if got := f.open().FmtKind; got != c.want {
				t.Errorf("FmtKind = %v，期望 %v", got, c.want)
			}
		})
	}
}

// Text 取 DS:3A36 的 [:40]，DI 與長度相符才算正常。
func TestCaptureTextCappedAt40(t *testing.T) {
	long := "0123456789abcdefghij0123456789ABCDEFGHIJ-extra" // 46 字元
	f := capNew(t)
	f.arm()
	c := capBase()
	c.text, c.di = long, 40
	f.drawA(c)
	rec := f.open()
	if rec.Text != long[:40] || len(rec.Cells) != 40 || string(rec.Cells) != long[:40] {
		t.Errorf("Text = %q，Cells 長 %d，期望取前 40 個字元", rec.Text, len(rec.Cells))
	}
	capCounters(t, f, map[string]uint64{"events": 1})
}

func TestCaptureBadlen(t *testing.T) {
	cases := []struct {
		name string
		text string
		di   int
	}{
		{"DI 小於長度", "Hello", 3},
		{"DI 大於長度", "Hello", 9},
		{"長於 40 的文字 DI 填原長", "0123456789abcdefghij0123456789ABCDEFGHIJ-extra", 46},
		{"空字串但 DI 不為 0", "", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := capNew(t)
			f.arm()
			cc := capBase()
			cc.text, cc.di = c.text, c.di
			f.drawA(cc)
			// 事件照常開啟並配對，只是不記錄成可提交的內容。
			rec := f.open()
			if rec.ID != "g1" {
				t.Errorf("ID = %q", rec.ID)
			}
			f.h.handleB()
			capCounters(t, f, map[string]uint64{"events": 1, "badlen": 1})
			if len(f.ov.Layer.Stamps) != 0 {
				t.Errorf("badlen 的事件提交時不得動 Layer")
			}
		})
	}
	t.Run("相符不計", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.drawA(capBase())
		capCounters(t, f, map[string]uint64{"events": 1})
	})
}

// 格式字串讀滿 64 bytes 無 NUL：truncated_input，事件開啟但不解析（不讀引數、提交時不動 Layer）；63 加 NUL 不算。
func TestCaptureTruncatedFormat(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setBytes(0x1100, []byte(strings.Repeat("x", 64)))
	c := capBase()
	c.fmtPtr, c.args = 0x1100, []uint16{0x5000}
	f.drawA(c)
	rec := f.open()
	if rec.Format != strings.Repeat("x", 64) {
		t.Errorf("Format 長 %d，期望 64 個 x", len(rec.Format))
	}
	if rec.ArgStrs != nil {
		t.Errorf("讀滿的格式字串不解析，ArgStrs = %v", rec.ArgStrs)
	}
	f.h.handleB()
	capCounters(t, f, map[string]uint64{"events": 1, "truncated_input": 1})
	if len(f.ov.Layer.Stamps) != 0 {
		t.Errorf("提交時不得動 Layer")
	}

	g := capNew(t)
	g.arm()
	g.m.setStr(0x1100, strings.Repeat("x", 63))
	c2 := capBase()
	c2.fmtPtr = 0x1100
	g.drawA(c2)
	if rec := g.open(); rec.Format != strings.Repeat("x", 63) {
		t.Errorf("63 個字元加 NUL 的 Format 長 %d", len(rec.Format))
	}
	capCounters(t, g, map[string]uint64{"events": 1})
}

// %s 引數讀滿 80 bytes 無 NUL：truncated_input（內容 80 bytes）；79 加 NUL 不算。
func TestCaptureTruncatedArgument(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setBytes(0x1200, []byte(strings.Repeat("y", 80)))
	c := capBase()
	c.fmtPtr, c.args = 0x1020, []uint16{0x1200}
	f.drawA(c)
	rec := f.open()
	if len(rec.ArgStrs) != 1 || rec.ArgStrs[0].Content != strings.Repeat("y", 80) || rec.ArgStrs[0].Ptr != 0x1200 || rec.ArgStrs[0].Kind != KindStatic {
		t.Errorf("ArgStrs = %+v", rec.ArgStrs)
	}
	f.h.handleB()
	capCounters(t, f, map[string]uint64{"events": 1, "truncated_input": 1})
	if len(f.ov.Layer.Stamps) != 0 {
		t.Errorf("提交時不得動 Layer")
	}

	g := capNew(t)
	g.arm()
	g.m.setStr(0x1200, strings.Repeat("y", 79))
	c2 := capBase()
	c2.fmtPtr, c2.args = 0x1020, []uint16{0x1200}
	g.drawA(c2)
	if rec := g.open(); len(rec.ArgStrs) != 1 || len(rec.ArgStrs[0].Content) != 79 {
		t.Errorf("79 個字元加 NUL 的 ArgStrs = %+v", rec.ArgStrs)
	}
	capCounters(t, g, map[string]uint64{"events": 1})
}

// DS 與 SS 不同：不擷取（規格 001 §2 以 SS 等於 DS 為前提，§9 沒有列此計數器；本測試固定現況）。
func TestCaptureDSMismatch(t *testing.T) {
	f := capNew(t)
	f.arm()
	c := capBase()
	c.dsMismatch = true
	f.drawA(c)
	capCounters(t, f, map[string]uint64{"ds_mismatch": 1})
	if _, ok := f.ov.Open(); ok {
		t.Errorf("ds_mismatch 不得開啟事件")
	}
	f.h.handleB() // 沒有開啟中的事件
	capCounters(t, f, map[string]uint64{"ds_mismatch": 1, "dup_close": 1})
}

// 分類為 other 的 %s 引數計 arg_unclassified，鍵是（呼叫端 4 位十六進位、%s 序、指標高位元組 2 位十六進位）。
func TestCaptureArgUnclassified(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x1040, "%s %d %s %s")
	f.m.setStr(0x7522, "Orc")
	f.m.setStr(0x5000, "Player")
	c := capBase()
	c.fmtPtr = 0x1040
	// 第 0 個 %s：怪物；%d；第 1 個 %s：其他（字組 2）；第 2 個 %s：靜態（字組 3）。
	c.args = []uint16{0x7522, 5, 0x5000, 0x1000}
	c.text = "Orc 5 Player Hello"
	f.drawA(c)
	rec := f.open()
	if len(rec.ArgStrs) != 3 || rec.ArgStrs[1].Kind != KindOther || rec.ArgStrs[1].Content != "Player" ||
		rec.ArgStrs[0].Kind != KindMonster || rec.ArgStrs[2].Kind != KindStatic {
		t.Errorf("ArgStrs = %+v", rec.ArgStrs)
	}
	capCounters(t, f, map[string]uint64{"events": 1, "arg_unclassified": 1})
	if got := f.ov.C.KeySet("arg_unclassified"); !reflect.DeepEqual(got, []string{"1234/1/50"}) {
		t.Errorf("arg_unclassified 鍵 = %v，期望 [1234/1/50]", got)
	}

	// 兩個 other：指標高位元組不同，鍵各一筆。
	g := capNew(t)
	g.arm()
	g.m.setStr(0x5000, "Player")
	g.m.setStr(0x5100, "Bob")
	g.m.setStr(0x1040, "%d %s %s")
	c2 := capBase()
	c2.fmtPtr, c2.caller = 0x1040, 0x0ABC
	c2.args = []uint16{7, 0x5000, 0x5100}
	c2.text = "7 Player Bob"
	g.drawA(c2)
	capCounters(t, g, map[string]uint64{"events": 1, "arg_unclassified": 2})
	if got := g.ov.C.KeySet("arg_unclassified"); !reflect.DeepEqual(got, []string{"0ABC/0/50", "0ABC/1/51"}) {
		t.Errorf("arg_unclassified 鍵 = %v", got)
	}
}

// 格式字串被判為不解析（truncated_input）時不讀引數，不計 arg_unclassified。
func TestCaptureArgUnclassifiedSkippedWhenNotParsed(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x5000, "Player")
	c := capBase()
	c.fmtPtr, c.args = 0x1020, []uint16{0x5000}
	c.di = 2 // badlen
	f.drawA(c)
	capCounters(t, f, map[string]uint64{"events": 1, "badlen": 1})
}

// ---------- 事件配對 ----------

func TestCaptureABCommitsAtB(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.drawA(capBase())
	if len(f.ov.Layer.Stamps) != 0 {
		t.Errorf("A 之後、B 之前 Layer 不變")
	}
	if rec := f.open(); rec.ID != "g1" {
		t.Errorf("ID = %q，期望 g1", rec.ID)
	}
	f.h.handleB()
	if len(f.ov.Layer.Stamps) == 0 {
		t.Errorf("B 之後應提交疊字")
	}
	if _, ok := f.ov.Open(); ok {
		t.Errorf("B 之後不得有開啟中的事件")
	}
	capCounters(t, f, map[string]uint64{"events": 1, "translated": 1})
	// 沒有開啟中的事件再來一個 B：dup_close，不算 unpaired。
	f.h.handleB()
	capCounters(t, f, map[string]uint64{"events": 1, "translated": 1, "dup_close": 1})
}

// 同 SP、BP、Caller、Col、Row、FmtPtr 與 Text 的 A 是重入（IRQ0 雙觸發），Step 不同也一樣。
func TestCaptureDupOpen(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.drawA(capBase())
	c := capBase()
	c.step = 101
	f.drawA(c)
	capCounters(t, f, map[string]uint64{"events": 1, "dup_open": 1})
	if rec := f.open(); rec.ID != "g1" || rec.Step != 100 {
		t.Errorf("重入不得換掉事件：ID=%s Step=%d", rec.ID, rec.Step)
	}
	f.h.handleB()
	capCounters(t, f, map[string]uint64{"events": 1, "dup_open": 1, "translated": 1})
}

// 任一識別欄位不同：上一個事件缺 B，記 unpaired，先提交上一個事件，再開新事件。
func TestCaptureUnpairedWhenIdentityDiffers(t *testing.T) {
	variants := []struct {
		name string
		mod  func(c *capCall)
	}{
		{"SP", func(c *capCall) { c.sp += 2 }},
		{"BP", func(c *capCall) { c.bp = 0xFA40 }},
		{"Caller", func(c *capCall) { c.caller = 0x1235 }},
		{"Col", func(c *capCall) { c.col = 4 }},
		{"Row", func(c *capCall) { c.row = 6 }},
		{"FmtPtr", func(c *capCall) { c.fmtPtr = 0x1010 }},
		{"Text", func(c *capCall) { c.text = "Jello" }},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			f := capNew(t)
			f.arm()
			f.drawA(capBase())
			c := capBase()
			v.mod(&c)
			f.drawA(c)
			capCounters(t, f, map[string]uint64{"events": 2, "unpaired": 1, "translated": 1})
			if rec := f.open(); rec.ID != "g2" {
				t.Errorf("新事件 ID = %q，期望 g2", rec.ID)
			}
			if len(f.ov.Layer.Stamps) == 0 {
				t.Errorf("上一個事件應以已擷取的資料提交")
			}
		})
	}
}

// 規格 001 §3.2：重入「忽略」。重入的 A 不得再計 badlen、truncated_input、arg_unclassified
// （IRQ0 雙觸發會讓同一個事件的診斷計數翻倍）。capture.go 在 Begin 判定重入之前就已計數，本測試直接失敗。
func TestCaptureDupOpenIgnoredNoDoubleCount(t *testing.T) {
	t.Run("badlen", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		c := capBase()
		c.di = 3
		f.drawA(c)
		f.drawA(c)
		capCounters(t, f, map[string]uint64{"events": 1, "dup_open": 1, "badlen": 1})
	})
	t.Run("truncated_input", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.m.setBytes(0x1100, []byte(strings.Repeat("x", 64)))
		c := capBase()
		c.fmtPtr = 0x1100
		f.drawA(c)
		f.drawA(c)
		capCounters(t, f, map[string]uint64{"events": 1, "dup_open": 1, "truncated_input": 1})
	})
	t.Run("arg_unclassified", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.m.setStr(0x5000, "Player")
		c := capBase()
		c.fmtPtr, c.args, c.text = 0x1020, []uint16{0x5000}, "Player"
		f.drawA(c)
		f.drawA(c)
		capCounters(t, f, map[string]uint64{"events": 1, "dup_open": 1, "arg_unclassified": 1})
	})
}

// ---------- 組句 ----------

func TestCaptureHandleSRecordsSprintf(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x1050, "You hit %s")
	f.m.setStr(0x7522, "Orc")
	f.sprintf(50, capOpSP, 0x4321, 0x638E, 0x1050, "You hit Orc",
		0x7522, 0x0601, 0x0602, 0x0603, 0x0604, 0x0605, 0x0606, 0x0607, 0x0608, 0x0609, 0x060A, 0x060B)
	pc, miss := f.h.Compose.Match(0x638E, "You hit Orc")
	if pc == nil || miss != MissNone {
		t.Fatalf("Match = %v, %v", pc, miss)
	}
	if pc.Fmt != "You hit %s" || pc.FmtKind != KindStatic || pc.Literal != "You hit Orc" {
		t.Errorf("Piece = %+v", pc)
	}
	wantArgs := [12]uint16{0x7522, 0x0601, 0x0602, 0x0603, 0x0604, 0x0605, 0x0606, 0x0607, 0x0608, 0x0609, 0x060A, 0x060B}
	if pc.Args != wantArgs {
		t.Errorf("Args = %v，期望 %v（格式字串之後 12 個字組）", pc.Args, wantArgs)
	}
	if len(pc.ArgStrs) != 1 || pc.ArgStrs[0] != (ArgStr{Ptr: 0x7522, Content: "Orc", Kind: KindMonster}) {
		t.Errorf("ArgStrs = %+v", pc.ArgStrs)
	}
	if r := f.h.Compose.recs[0x638E]; r == nil || r.step != 50 {
		t.Errorf("記錄的 Step 應為 50：%+v", r)
	}
	capCounters(t, f, nil)
}

// S 的 %s 引數分類為 other 也計 arg_unclassified，呼叫端取 [SP]。
func TestCaptureHandleSArgUnclassified(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x1050, "Hi %s")
	f.m.setStr(0x5000, "Player")
	f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1050, "Hi Player", 0x5000)
	capCounters(t, f, map[string]uint64{"arg_unclassified": 1})
	if got := f.ov.C.KeySet("arg_unclassified"); !reflect.DeepEqual(got, []string{"4321/0/50"}) {
		t.Errorf("鍵 = %v", got)
	}
}

// sprintf(638E, ...) 之後以 638E 當格式字串畫出：Composed 非空。
func TestCaptureComposedFromSprintf(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x1050, "You hit %s")
	f.m.setStr(0x7522, "Orc")
	f.sprintf(50, capOpSP, 0x4321, 0x638E, 0x1050, "You hit Orc", 0x7522)
	c := capBase()
	c.fmtPtr, c.text = 0x638E, "You hit Orc"
	f.drawA(c)
	rec := f.open()
	if rec.FmtKind != KindBuffer || rec.Format != "You hit Orc" {
		t.Errorf("FmtKind = %v，Format = %q", rec.FmtKind, rec.Format)
	}
	p := rec.Composed
	if p == nil {
		t.Fatalf("Composed 應非空")
	}
	if p.Fmt != "You hit %s" || p.FmtKind != KindStatic || p.Literal != "You hit Orc" || len(p.Appends) != 0 {
		t.Errorf("Composed = %+v", p)
	}
	if len(p.ArgStrs) != 1 || p.ArgStrs[0] != (ArgStr{Ptr: 0x7522, Content: "Orc", Kind: KindMonster}) {
		t.Errorf("Composed.ArgStrs = %+v", p.ArgStrs)
	}
	capCounters(t, f, map[string]uint64{"events": 1, "composed": 1})

	// 重入不重複計組句。
	f.drawA(c)
	capCounters(t, f, map[string]uint64{"events": 1, "composed": 1, "dup_open": 1})
}

func TestCaptureComposedMisses(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(f *capFx)
		cur    string // A 時 DS:638E 的內容
		counts map[string]uint64
		keys   []string
	}{
		{"沒有記錄", func(f *capFx) {}, "You hit Orc", map[string]uint64{"composed_miss_dest": 1}, nil},
		{"內容不符", func(f *capFx) {
			f.sprintf(50, capOpSP, 0x4321, 0x638E, 0x1050, "You hit Orc", 0x7522)
		}, "You hit Elf", map[string]uint64{"composed_miss_content": 1}, nil},
		{"前綴相符（未追蹤的追加）", func(f *capFx) {
			f.sprintf(50, capOpSP, 0x4321, 0x638E, 0x1050, "You hit Orc", 0x7522)
		}, "You hit Orc!", map[string]uint64{"composed_miss_prefix": 1}, []string{"1234"}},
		{"目前字串含百分號", func(f *capFx) {
			f.sprintf(50, capOpSP, 0x4321, 0x638E, 0x1020, "100%", 0x1060)
		}, "100%", map[string]uint64{"composed_miss_percent": 1}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := capNew(t)
			f.arm()
			f.m.setStr(0x1050, "You hit %s")
			f.m.setStr(0x1060, "100%")
			f.m.setStr(0x7522, "Orc")
			c.setup(f)
			f.m.setStr(0x638E, c.cur)
			cc := capBase()
			cc.fmtPtr, cc.text = 0x638E, c.cur
			f.drawA(cc)
			if f.open().Composed != nil {
				t.Errorf("驗證失敗不得關聯")
			}
			want := map[string]uint64{"events": 1}
			for k, v := range c.counts {
				want[k] = v
			}
			capCounters(t, f, want)
			if got := f.ov.C.KeySet("composed_miss_prefix"); !reflect.DeepEqual(got, c.keys) {
				t.Errorf("composed_miss_prefix 鍵 = %v，期望 %v", got, c.keys)
			}
		})
	}
}

// 巢狀關聯在 S 時建立；A 時內層指標內容不符就清掉，外層仍關聯。
func TestCaptureComposedNested(t *testing.T) {
	setup := func(t *testing.T) *capFx {
		f := capNew(t)
		f.arm()
		f.m.setStr(0x1050, "You hit %s")
		f.m.setStr(0x1080, "big %s")
		f.m.setStr(0x7522, "Orc")
		f.sprintf(10, 0xE000, 0x4321, 0xFC00, 0x1080, "big Orc", 0x7522)
		f.sprintf(20, 0xE100, 0x4322, 0x638E, 0x1050, "You hit big Orc", 0xFC00)
		return f
	}
	call := func(f *capFx) *EventRecord {
		c := capBase()
		c.fmtPtr, c.text = 0x638E, "You hit big Orc"
		f.drawA(c)
		return f.open()
	}
	t.Run("內層仍相符", func(t *testing.T) {
		f := setup(t)
		p := call(f).Composed
		if p == nil || p.Fmt != "You hit %s" || p.Literal != "You hit big Orc" || len(p.ArgStrs) != 1 {
			t.Fatalf("Composed = %+v", p)
		}
		a := p.ArgStrs[0]
		if a.Ptr != 0xFC00 || a.Content != "big Orc" || a.Kind != KindBuffer || a.Piece == nil {
			t.Fatalf("巢狀引數 = %+v", a)
		}
		if a.Piece.Fmt != "big %s" || a.Piece.Literal != "big Orc" || len(a.Piece.ArgStrs) != 1 ||
			a.Piece.ArgStrs[0] != (ArgStr{Ptr: 0x7522, Content: "Orc", Kind: KindMonster}) {
			t.Errorf("巢狀 Piece = %+v", a.Piece)
		}
		capCounters(t, f, map[string]uint64{"events": 1, "composed": 1})
	})
	t.Run("內層緩衝區被重用", func(t *testing.T) {
		f := setup(t)
		f.m.setStr(0xFC00, "xxxxxxx")
		p := call(f).Composed
		if p == nil || len(p.ArgStrs) != 1 || p.ArgStrs[0].Piece != nil {
			t.Fatalf("內層內容不符時 Piece 要清掉：%+v", p)
		}
		capCounters(t, f, map[string]uint64{"events": 1, "composed": 1})
	})
}

// 戰鬥指令列：sprintf(638E, ...)，strcat 追加兩次，再以靜態 "%s"、引數 638E 畫出。
func TestCaptureBattleLineAppends(t *testing.T) {
	setup := func(t *testing.T) *capFx {
		f := capNew(t)
		f.arm()
		f.m.setStr(0x1090, "Order: ")
		f.m.setStr(0x1070, "Fight ")
		f.m.setStr(0xFC40, "Spell ")
		f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1090, "Order: ")
		return f
	}
	draw := func(f *capFx) *EventRecord {
		c := capBase()
		c.fmtPtr, c.args, c.text = 0x1020, []uint16{0x638E}, "Order: Fight Spell "
		f.drawA(c)
		return f.open()
	}
	t.Run("有 strcat 掛點", func(t *testing.T) {
		f := setup(t)
		f.strcat(capOpSP, 0x638E, 0x1070, "Order: Fight ")
		f.strcat(capOpSP, 0x638E, 0xFC40, "Order: Fight Spell ")
		capCounters(t, f, nil)
		rec := draw(f)
		if rec.Composed != nil {
			t.Errorf("格式字串是靜態 %%s，不應有 Composed")
		}
		if len(rec.ArgStrs) != 1 {
			t.Fatalf("ArgStrs = %+v", rec.ArgStrs)
		}
		a := rec.ArgStrs[0]
		if a.Ptr != 0x638E || a.Content != "Order: Fight Spell " || a.Kind != KindBuffer || a.Piece == nil {
			t.Fatalf("引數 = %+v", a)
		}
		p := a.Piece
		if p.Fmt != "Order: " || p.FmtKind != KindStatic || p.Literal != "Order: Fight Spell " {
			t.Errorf("Piece = %+v", p)
		}
		wantApp := []ArgStr{{Ptr: 0x1070, Content: "Fight ", Kind: KindStatic}, {Ptr: 0xFC40, Content: "Spell ", Kind: KindBuffer}}
		if !reflect.DeepEqual(p.Appends, wantApp) {
			t.Errorf("Appends = %+v，期望 %+v", p.Appends, wantApp)
		}
		capCounters(t, f, map[string]uint64{"events": 1})
	})
	t.Run("沒有 strcat 掛點：前綴相符", func(t *testing.T) {
		f := setup(t)
		f.m.setStr(0x638E, "Order: Fight Spell ")
		rec := draw(f)
		if rec.ArgStrs[0].Piece != nil {
			t.Errorf("內容不符不得關聯")
		}
		capCounters(t, f, map[string]uint64{"events": 1, "composed_miss_prefix": 1})
		if got := f.ov.C.KeySet("composed_miss_prefix"); !reflect.DeepEqual(got, []string{"1234"}) {
			t.Errorf("鍵 = %v", got)
		}
	})
}

// IRQ0 雙觸發：同一個 T 入口觸發兩次，此時 strcat 都還沒執行，第二次的組合字串已不等於 DS 內容，Appends 只有一筆。
func TestCaptureStrcatDoubleFire(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x1070, "Fight ")
	f.m.setStr(0x1090, "Order: ")
	f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1090, "Order: ")
	f.strcatHook(capOpSP, 0x638E, 0x1070)
	f.strcatHook(capOpSP, 0x638E, 0x1070)
	f.m.setStr(0x638E, "Order: Fight ")
	capCounters(t, f, nil) // 第二次是內容不符，不是 no_rec
	c := capBase()
	c.fmtPtr, c.args, c.text = 0x1020, []uint16{0x638E}, "Order: Fight "
	f.drawA(c)
	a := f.open().ArgStrs[0]
	if a.Piece == nil || len(a.Piece.Appends) != 1 || a.Piece.Appends[0].Content != "Fight " || a.Piece.Literal != "Order: Fight " {
		t.Errorf("Piece = %+v", a.Piece)
	}
	capCounters(t, f, map[string]uint64{"events": 1})
}

// T 的目的沒有 sprintf 記錄：no_rec（載入器把 .ovr 追加到堆疊緩衝區等），不影響組句。
func TestCaptureStrcatNoRecord(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setStr(0x1070, ".OVR")
	f.m.setStr(0xFF8E, "OV1")
	f.strcatHook(capOpSP, 0xFF8E, 0x1070)
	capCounters(t, f, map[string]uint64{"no_rec": 1})
	f.strcatHook(capOpSP, 0x638E, 0x1070) // 638E 也沒有記錄
	capCounters(t, f, map[string]uint64{"no_rec": 2})
	// 有記錄但內容不符：不是 no_rec。
	f.m.setStr(0x1090, "Order: ")
	f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1090, "Order: ")
	f.m.setStr(0x638E, "other")
	f.strcatHook(capOpSP, 0x638E, 0x1070)
	capCounters(t, f, map[string]uint64{"no_rec": 2})
}

// %s 引數與 S 的格式字串也依 overlay 與 SP 分類。
func TestCaptureArgKindUsesOverlayAndSP(t *testing.T) {
	t.Run("A 的 %s 引數指向 overlay 資料區", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.h.overlay = "OV1.TEST"
		f.m.setStr(0xB900, "Ghoul")
		c := capBase()
		c.fmtPtr, c.args, c.text = 0x1020, []uint16{0xB900}, "Ghoul"
		f.drawA(c)
		if a := f.open().ArgStrs; len(a) != 1 || a[0].Kind != KindStatic || a[0].Content != "Ghoul" {
			t.Errorf("OV1 載入中，overlay 資料區是 static：%+v", a)
		}
		capCounters(t, f, map[string]uint64{"events": 1})
	})
	t.Run("沒有 overlay 時是 other", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.m.setStr(0xB900, "Ghoul")
		c := capBase()
		c.fmtPtr, c.args, c.text = 0x1020, []uint16{0xB900}, "Ghoul"
		f.drawA(c)
		if a := f.open().ArgStrs; len(a) != 1 || a[0].Kind != KindOther {
			t.Errorf("ArgStrs = %+v", a)
		}
		capCounters(t, f, map[string]uint64{"events": 1, "arg_unclassified": 1})
		if got := f.ov.C.KeySet("arg_unclassified"); !reflect.DeepEqual(got, []string{"1234/0/B9"}) {
			t.Errorf("鍵 = %v", got)
		}
	})
	t.Run("S 的格式字串在堆疊上", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.m.setStr(0xFC80, "x %s")
		f.m.setStr(0x7522, "Orc")
		f.sprintf(1, capOpSP, 0x4321, 0x638E, 0xFC80, "x Orc", 0x7522)
		pc, _ := f.h.Compose.Match(0x638E, "x Orc")
		if pc == nil || pc.FmtKind != KindBuffer {
			t.Errorf("堆疊上的格式字串是 buffer：%+v", pc)
		}
	})
}

// 故障注入 NoStrcat（001 §10 第 5 項的負對照）：T 掛點不處理，戰鬥指令列的組句失敗，composed_miss_prefix 大於 0。
func TestCaptureNoStrcatFault(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.h.NoStrcat = true
	f.m.setStr(0x1090, "Order: ")
	f.m.setStr(0x1070, "Fight ")
	f.sprintf(1, capOpSP, 0x4321, 0x638E, 0x1090, "Order: ")
	f.strcat(capOpSP, 0x638E, 0x1070, "Order: Fight ")
	f.strcatHook(capOpSP, 0xFF8E, 0x1070) // 沒有記錄的目的：NoStrcat 時也不計 no_rec
	capCounters(t, f, nil)
	c := capBase()
	c.fmtPtr, c.args, c.text = 0x1020, []uint16{0x638E}, "Order: Fight "
	f.drawA(c)
	if a := f.open().ArgStrs[0]; a.Piece != nil {
		t.Errorf("沒有追加記錄，內容不符不得關聯：%+v", a.Piece)
	}
	capCounters(t, f, map[string]uint64{"events": 1, "composed_miss_prefix": 1})
}

// Int10Probe（證據探針）：INT 10h AH=06h、07h 完成時收到入口參數與入口、完成時的 B800:0000 起 4000h bytes；其他不呼叫。
func TestCaptureInt10Probe(t *testing.T) {
	type call struct {
		ax, bx, cx, dx uint16
		pre, post      []byte
	}
	f := capNew(t)
	f.arm()
	var got []call
	f.h.Int10Probe = func(ax, bx, cx, dx uint16, pre, post []byte) {
		got = append(got, call{ax, bx, cx, dx, pre, post})
	}
	pre, post := capPattern(0x4000, 91), capPattern(0x4000, 92)
	for _, ax := range []uint16{0x0601, 0x0701} {
		got = nil
		f.m.setFar(0xB800, 0, pre)
		f.int86Entry(0x10, 0x9000, ax, 0x0700, 0x0102, 0x0203)
		f.m.setFar(0xB800, 0, post)
		f.h.opExit("int86")
		if len(got) != 1 {
			t.Fatalf("AX=%04X：探針呼叫 %d 次，期望 1", ax, len(got))
		}
		g := got[0]
		if g.ax != ax || g.bx != 0x0700 || g.cx != 0x0102 || g.dx != 0x0203 {
			t.Errorf("AX=%04X：探針參數 %04X %04X %04X %04X", ax, g.ax, g.bx, g.cx, g.dx)
		}
		if !reflect.DeepEqual(g.pre, pre) || !reflect.DeepEqual(g.post, post) {
			t.Errorf("AX=%04X：pre、post 應分別是入口與完成時的畫面", ax)
		}
	}
	got = nil
	for _, c := range []struct{ intNo, ax uint16 }{{0x10, 0x0004}, {0x10, 0x0B00}, {0x16, 0x0601}, {0x21, 0x0701}} {
		f.int86Entry(c.intNo, 0x9000, c.ax, 0, 0, 0)
		f.h.opExit("int86")
	}
	if len(got) != 0 {
		t.Errorf("INT 10h AH=00、0Bh 與其他中斷號不得呼叫探針：%d 次", len(got))
	}
}

// ---------- handleB 與 Busy ----------

func TestCaptureBusyBetweenEntryAndExit(t *testing.T) {
	f := capNew(t)
	f.arm()
	if f.h.Busy() {
		t.Fatalf("初始不得忙碌")
	}
	f.h.opEntry("copy12", capOpSP)
	if !f.h.Busy() {
		t.Errorf("入口之後完成之前應忙碌")
	}
	f.h.opEntry("save1", capOpSP)
	f.h.opExit("copy12")
	if !f.h.Busy() {
		t.Errorf("save1 仍在途，應忙碌")
	}
	f.h.opExit("save1")
	if f.h.Busy() {
		t.Errorf("全部完成後不得忙碌")
	}
	capCounters(t, f, map[string]uint64{"op_copy12": 1, "op_save1": 1})
}

// 驗證觸發的入口本身要處理（armed 之後才有在途操作）；prearm 的入口不留在途操作。
func TestCaptureBusyAtVerifyingEntry(t *testing.T) {
	f := capNew(t)
	f.h.opEntry("copy12", capOpSP)
	if !f.h.Armed() || !f.h.Busy() {
		t.Errorf("驗證觸發的入口要記錄在途操作：armed=%v busy=%v", f.h.Armed(), f.h.Busy())
	}
	g := capNew(t)
	g.mode = 3
	g.h.opEntry("copy12", capOpSP)
	if g.h.Busy() {
		t.Errorf("prearm 的入口不得留下在途操作")
	}
}

// 每個操作的完成沒有入口：dup_close，不套用。
func TestCaptureOpExitWithoutEntry(t *testing.T) {
	for _, op := range capOps {
		t.Run(op.name, func(t *testing.T) {
			f := capNew(t)
			f.arm()
			f.h.opExit(op.name)
			capCounters(t, f, map[string]uint64{"dup_close": 1})
		})
	}
}

// 入口重複觸發（同一次呼叫）：完成只套用一次；完成重複記 dup_close。
func TestCaptureOpIdempotentAndPaired(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.h.opEntry("save1", capOpSP)
	f.h.opEntry("save1", capOpSP)
	f.h.opExit("save1")
	capCounters(t, f, map[string]uint64{"op_save1": 1})
	if f.h.Busy() {
		t.Errorf("完成後不得忙碌")
	}
	f.h.opExit("save1")
	capCounters(t, f, map[string]uint64{"op_save1": 1, "dup_close": 1})
}

// ---------- int86 ----------

func TestCaptureInt86OnlyInt10Applies(t *testing.T) {
	cases := []struct {
		name  string
		intNo uint16
		ax    uint16
	}{
		{"INT 16h AH=00（讀鍵）", 0x16, 0x0000},
		{"INT 16h AH=0B", 0x16, 0x0B00},
		{"INT 21h AH=0B", 0x21, 0x0B00},
		{"INT 21h AH=07", 0x21, 0x0700},
		{"INT 17h AH=00", 0x17, 0x0000},
		{"INT 11h AH=06", 0x11, 0x0600},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := capNew(t)
			f.arm()
			capStamp(f, "g1", 0, 0, 40) // 整列的 Shown 疊字：被當成模式改變會被清除，被當成 0Bh 會變 Pending
			f.int86Entry(c.intNo, 0x9000, c.ax, 0, 0x0000, 0x1827)
			if !f.h.Busy() {
				t.Errorf("入口之後應忙碌")
			}
			f.h.opExit("int86")
			if f.h.Busy() {
				t.Errorf("完成後不得忙碌")
			}
			capCounters(t, f, nil) // 沒有 dup_close，也沒有任何 op_int10_*
			if st := capStates(f); !reflect.DeepEqual(st, map[string]xlate.State{"g1@0": xlate.Shown}) {
				t.Errorf("Layer 不得變動：%v", st)
			}
		})
	}
}

func TestCaptureInt86Int10Actions(t *testing.T) {
	t.Run("AH=00 模式改變清空 Layer", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		capStamp(f, "g1", 0, 0, 5)
		f.int86Entry(0x10, 0x9000, 0x0004, 0, 0, 0)
		f.h.opExit("int86")
		capCounters(t, f, map[string]uint64{"op_int10_00": 1})
		if len(f.ov.Layer.Stamps) != 0 {
			t.Errorf("Layer 應清空")
		}
	})
	t.Run("AH=06 AL=0 視窗清除，CX 左上、DX 右下", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		capStamp(f, "g1", 16, 8, 2) // 列 1，欄 2、3：被清除
		capStamp(f, "g2", 0, 8, 2)  // 列 1，欄 0、1：保持
		capStamp(f, "g3", 16, 16, 2)
		// CH=1 CL=2 DH=1 DL=3：像素矩形 [16, 8, 32, 16)。
		f.int86Entry(0x10, 0x9000, 0x0600, 0x0700, 0x0102, 0x0103)
		f.h.opExit("int86")
		capCounters(t, f, map[string]uint64{"op_int10_06": 1})
		if got := capKeys(f); !reflect.DeepEqual(got, []string{"g2", "g3"}) {
			t.Errorf("Layer 內剩 %v，期望 [g2 g3]", got)
		}
	})
	t.Run("AH=0B 全部 Pending", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		capStamp(f, "g1", 0, 0, 5)
		capStamp(f, "g2", 40, 40, 5)
		f.int86Entry(0x10, 0x9000, 0x0B00, 0x0001, 0, 0)
		f.h.opExit("int86")
		capCounters(t, f, map[string]uint64{"op_int10_0B": 1})
		want := map[string]xlate.State{"g1@0": xlate.Pending, "g2@40": xlate.Pending}
		if got := capStates(f); !reflect.DeepEqual(got, want) {
			t.Errorf("狀態 = %v", got)
		}
	})
	t.Run("參數在入口取，完成時套用", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		capStamp(f, "g1", 0, 0, 5)
		f.int86Entry(0x10, 0x9000, 0x0004, 0, 0, 0)
		// 入口之後記憶體被改寫（例如輸入 REGS 被重用）。
		f.m.setWord(capOpSP+2, 0x16)
		f.m.setWord(0x9000, 0x0B00)
		f.h.opExit("int86")
		capCounters(t, f, map[string]uint64{"op_int10_00": 1})
		if len(f.ov.Layer.Stamps) != 0 {
			t.Errorf("應以入口的 AX=0004 套用（模式改變）")
		}
	})
}

// ---------- invert、invert2 ----------

func TestCaptureInvertParameterOrderAndGroup(t *testing.T) {
	setup := func(t *testing.T) *capFx {
		f := capNew(t)
		f.arm()
		capStamp(f, "g1", 40, 24, 2)  // 列 3，x 40 至 56：與反白矩形相交
		capStamp(f, "g1", 120, 24, 2) // 同一事件組的另一筆：整組跟著 Pending
		capStamp(f, "g2", 0, 0, 2)
		capStamp(f, "g3", 8, 24, 1)  // 列 3，x 8 至 16：參數順序錯時會被選到
		capStamp(f, "g4", 56, 24, 2) // 列 3，x 56 至 72：寬度多算一格時會被選到
		return f
	}
	want := map[string]xlate.State{
		"g1@40": xlate.Pending, "g1@120": xlate.Pending, "g2@0": xlate.Shown, "g3@8": xlate.Shown, "g4@56": xlate.Shown,
	}
	for _, name := range []string{"invert", "invert2"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			// [SP+2] row、[SP+4] col、[SP+6] width：矩形 [48, 24, 56, 32)。
			f.m.setWord(capOpSP+2, 3)
			f.m.setWord(capOpSP+4, 6)
			f.m.setWord(capOpSP+6, 1)
			f.h.opEntry(name, capOpSP)
			if got := capStates(f); got["g1@40"] != xlate.Shown {
				t.Errorf("入口時不得套用")
			}
			f.h.opExit(name)
			if got := capStates(f); !reflect.DeepEqual(got, want) {
				t.Errorf("狀態 = %v，期望 %v", got, want)
			}
			if name == "invert" {
				capCounters(t, f, map[string]uint64{"op_invert": 1})
			} else {
				capCounters(t, f, map[string]uint64{"op_invert2": 1, "dim": 1})
			}
		})
	}
}

// 寬度以字元格計：寬度 3 的矩形涵蓋 [48, 24, 72, 32)，兩種寬度的結果不同。
func TestCaptureInvertWidth(t *testing.T) {
	f := capNew(t)
	f.arm()
	capStamp(f, "g4", 56, 24, 2)
	capStamp(f, "g5", 72, 24, 2) // 剛好在矩形之外
	f.m.setWord(capOpSP+2, 3)
	f.m.setWord(capOpSP+4, 6)
	f.m.setWord(capOpSP+6, 3)
	f.op("invert")
	want := map[string]xlate.State{"g4@56": xlate.Pending, "g5@72": xlate.Shown}
	if got := capStates(f); !reflect.DeepEqual(got, want) {
		t.Errorf("狀態 = %v，期望 %v", got, want)
	}
}

// ---------- 頁面影子的雜湊流程 ----------

func TestCaptureSave1RegistersEmptyShadowByScreenHash(t *testing.T) {
	f := capNew(t)
	f.arm()
	p := capPattern(0x4000, 1)
	f.m.setFar(0xB800, 0, p)
	f.m.setFar(0xB800, 0x4000, []byte{0xAB, 0xCD}) // 螢幕雜湊範圍只到 4000h bytes
	f.op("save1")
	capCounters(t, f, map[string]uint64{"op_save1": 1})
	if _, ok := f.ov.sh.empties[capFNV(p)]; !ok || len(f.ov.sh.empties) != 1 || len(f.ov.sh.known) != 0 {
		t.Errorf("Layer 為空時應登記空影子：empties=%v known=%d", f.ov.sh.empties, len(f.ov.sh.known))
	}
}

func TestCaptureSave1RegistersKnownShadow(t *testing.T) {
	f := capNew(t)
	f.arm()
	capSeedGroup(f, "g1", 1, 5, "Hello")
	capSeedGroup(f, "g2", 0, 24, "Bye")
	p := capPattern(0x4000, 2)
	f.m.setFar(0xB800, 0, p)
	f.op("save1")
	k := f.ov.sh.known[capFNV(p)]
	if k == nil || !reflect.DeepEqual(capIDs(k.s), []string{"g1", "g2"}) || len(f.ov.sh.known) != 1 {
		t.Fatalf("known = %+v", f.ov.sh.known)
	}
}

// save1、load1、load2 的完整流程：load 入口取對應緩衝區的雜湊，完成時由影子還原。
func TestCaptureLoadRestoresByEntryHash(t *testing.T) {
	f := capNew(t)
	f.arm()
	p1, p2 := capPattern(0x4000, 11), capPattern(0x4000, 12)
	// 畫面 1：g1。畫面 2：g2。
	capSeedGroup(f, "g1", 1, 5, "Hello")
	f.m.setFar(0xB800, 0, p1)
	f.op("save1")
	f.ov.Layer.Stamps = nil
	capSeedGroup(f, "g2", 2, 6, "Bye")
	f.m.setFar(0xB800, 0, p2)
	f.op("save1")
	if len(f.ov.sh.known) != 2 {
		t.Fatalf("known 應有兩筆：%d", len(f.ov.sh.known))
	}
	// 緩衝區 1 放畫面 1，緩衝區 2 放畫面 2（002 §2：DS:5BAC = 0，DS:5BAE = 4000h）。
	f.m.setFar(capBufSeg, 0, p1)
	f.m.setFar(capBufSeg, 0x4000, p2)

	f.ov.Layer.Stamps = nil
	f.h.opEntry("load2", capOpSP)
	f.m.setFar(capBufSeg, 0x4000, capPattern(0x4000, 99)) // 入口之後緩衝區被改寫：仍以入口時的雜湊還原
	f.h.opExit("load2")
	if got := capKeys(f); !reflect.DeepEqual(got, []string{"g2"}) {
		t.Errorf("load2 後 Layer 的事件 = %v，期望 [g2]", got)
	}
	f.m.setFar(capBufSeg, 0x4000, p2)

	f.ov.Layer.Stamps = nil
	f.h.opEntry("load1", capOpSP)
	f.m.setFar(capBufSeg, 0, capPattern(0x4000, 98))
	f.h.opExit("load1")
	if got := capKeys(f); !reflect.DeepEqual(got, []string{"g1"}) {
		t.Errorf("load1 後 Layer 的事件 = %v，期望 [g1]", got)
	}
	capCounters(t, f, map[string]uint64{"op_save1": 2, "op_load1": 1, "op_load2": 1, "shadow_restore": 2})
}

// 緩衝區位置由 DS:5BAA、5BAC、5BAE 決定（不是固定值）。
func TestCaptureLoadUsesBufferDescriptors(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setWord(0x5BAA, 0x3100)
	f.m.setWord(0x5BAC, 0x0200)
	f.m.setWord(0x5BAE, 0x4200)
	p1, p2 := capPattern(0x4000, 21), capPattern(0x4000, 22)
	f.m.setFar(0x3100, 0x0200, p1)
	f.m.setFar(0x3100, 0x4200, p2)
	f.op("load1")
	f.op("load2")
	// 未知內容：整層清除並登記空影子。
	for _, p := range [][]byte{p1, p2} {
		if _, ok := f.ov.sh.empties[capFNV(p)]; !ok {
			t.Errorf("內容 %x… 應登記空影子", p[:4])
		}
	}
	capCounters(t, f, map[string]uint64{"op_load1": 1, "op_load2": 1})
}

func TestCaptureLoadUnknownClearsLayer(t *testing.T) {
	f := capNew(t)
	f.arm()
	capStamp(f, "g1", 8, 40, 5)
	u := capPattern(0x4000, 31)
	f.m.setFar(capBufSeg, 0, u)
	f.op("load1")
	if len(f.ov.Layer.Stamps) != 0 {
		t.Errorf("未知內容應清除整層")
	}
	if _, ok := f.ov.sh.empties[capFNV(u)]; !ok {
		t.Errorf("應登記空影子")
	}
	capCounters(t, f, map[string]uint64{"op_load1": 1})
}

// row24：入口取緩衝區 1 的雜湊與「第 24 列換成螢幕內容」後的雜湊，完成時合併影子。
func TestCaptureRow24MergesShadow(t *testing.T) {
	f := capNew(t)
	f.arm()
	f.m.setWord(0x5BAC, 0x0400) // 緩衝區 1 不在段起點：+1E00h、+3E00h 相對緩衝區起點
	screen0 := capPattern(0x4000, 41)
	capSeedGroup(f, "g1", 1, 5, "Hello")
	capSeedGroup(f, "g2", 0, 24, "Bye")
	f.m.setFar(0xB800, 0, screen0)
	f.op("save1") // known[hash(screen0)] = [g1, g2]
	// 緩衝區 1 的內容等於存入時的畫面。
	f.m.setFar(capBufSeg, 0x0400, screen0)
	// 第 24 列的內容改變：舊的 g2 被清掉，新事件 g3 畫在第 24 列。
	screen1 := capPattern(0x4000, 42)
	f.m.setFar(0xB800, 0, screen1)
	f.ov.Layer.Clear(0, 192, 320, 200)
	capSeedGroup(f, "g3", 3, 24, "Hello")

	// 期望的新雜湊：緩衝區 1 的 +1E00h 與 +3E00h 起各 140h bytes 換成 B800:1E00 與 BA00:1E00（線性位址 B800:3E00）。
	exp := append([]byte(nil), screen0...)
	copy(exp[0x1E00:0x1E00+0x140], screen1[0x1E00:0x1E00+0x140])
	copy(exp[0x3E00:0x3E00+0x140], screen1[0x3E00:0x3E00+0x140])

	f.h.opEntry("row24", capOpSP)
	f.h.opExit("row24")
	k := f.ov.sh.known[capFNV(exp)]
	if k == nil {
		t.Fatalf("known 沒有新雜湊 %016x；現有 %d 筆", capFNV(exp), len(f.ov.sh.known))
	}
	if got := capIDs(k.s); !reflect.DeepEqual(got, []string{"g1", "g3"}) {
		t.Errorf("新影子 = %v，期望 [g1 g3]（舊影子去掉第 24 列的 g2，加上 Layer 內第 24 列的 g3）", got)
	}
	if len(f.ov.sh.known) != 2 || len(f.ov.sh.empties) != 0 {
		t.Errorf("known=%d empties=%d，期望 2、0", len(f.ov.sh.known), len(f.ov.sh.empties))
	}
	capCounters(t, f, map[string]uint64{"op_save1": 1, "op_row24": 1})
}

func TestCaptureRow24EmptyAndUnknown(t *testing.T) {
	t.Run("合併後為空：登記空影子", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		screen0 := capPattern(0x4000, 51)
		capSeedGroup(f, "g2", 0, 24, "Bye")
		f.m.setFar(0xB800, 0, screen0)
		f.op("save1")
		f.m.setFar(capBufSeg, 0, screen0)
		screen1 := capPattern(0x4000, 52)
		f.m.setFar(0xB800, 0, screen1)
		f.ov.Layer.Clear(0, 192, 320, 200)
		exp := append([]byte(nil), screen0...)
		copy(exp[0x1E00:0x1E00+0x140], screen1[0x1E00:0x1E00+0x140])
		copy(exp[0x3E00:0x3E00+0x140], screen1[0x3E00:0x3E00+0x140])
		f.op("row24")
		if _, ok := f.ov.sh.empties[capFNV(exp)]; !ok {
			t.Errorf("empties 應有新雜湊；empties=%v", f.ov.sh.empties)
		}
	})
	t.Run("舊雜湊未知：不登記", func(t *testing.T) {
		f := capNew(t)
		f.arm()
		f.m.setFar(capBufSeg, 0, capPattern(0x4000, 61))
		f.m.setFar(0xB800, 0, capPattern(0x4000, 62))
		f.op("row24")
		if len(f.ov.sh.known) != 0 || len(f.ov.sh.empties) != 0 {
			t.Errorf("未知的舊雜湊不得登記：known=%d empties=%d", len(f.ov.sh.known), len(f.ov.sh.empties))
		}
		capCounters(t, f, map[string]uint64{"op_row24": 1})
	})
}

// row24 的入口雜湊取在入口，且不改假記憶體（鉤子只讀）。
func TestCaptureRow24DoesNotWriteMemory(t *testing.T) {
	f := capNew(t)
	f.arm()
	buf1 := capPattern(0x4000, 71)
	f.m.setFar(capBufSeg, 0, buf1)
	f.m.setFar(0xB800, 0, capPattern(0x4000, 72))
	before := append([]byte(nil), f.m.ram...)
	f.op("row24")
	if !reflect.DeepEqual(before, f.m.ram) {
		t.Errorf("row24 鉤子改動了記憶體")
	}
}

// copy12 只計數：不動影子與 Layer。
func TestCaptureCopy12OnlyCounts(t *testing.T) {
	f := capNew(t)
	f.arm()
	capSeedGroup(f, "g1", 1, 5, "Hello")
	f.m.setFar(capBufSeg, 0, capPattern(0x4000, 81))
	f.op("copy12")
	capCounters(t, f, map[string]uint64{"op_copy12": 1})
	if len(f.ov.sh.known) != 0 || len(f.ov.sh.empties) != 0 || !reflect.DeepEqual(capKeys(f), []string{"g1"}) {
		t.Errorf("copy12 不得處理影子或 Layer")
	}
}

// ---------- stringArgWords ----------

func TestCaptureStringArgWords(t *testing.T) {
	cases := []struct {
		format string
		want   []int
	}{
		{"", nil},
		{"Hello", nil},
		{"%s", []int{0}},
		{"%d %s", []int{1}},
		{"%ld %s", []int{2}},
		{"%lu%s%ld%s", []int{2, 5}},
		{"%s%s%s", []int{0, 1, 2}},
		{"%c%u %s", []int{2}},
		{"100%%%s", []int{0}},
		{"%-10s|%5d|%s", []int{0, 2}},
		{"%5.2s%d%s", []int{0, 2}},
		{"%d%c%s%ld%ld%s", []int{2, 7}},
		{"%d %d %d %s", []int{3}},
		{"%ld%ld%ld", nil},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			segs, err := ParseFormat(c.format)
			if err != nil {
				t.Fatalf("ParseFormat：%v", err)
			}
			got := stringArgWords(segs)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("stringArgWords(%q) = %v，期望 %v", c.format, got, c.want)
			}
		})
	}
}

// ---------- 掛點表與簽章表的一致性（靜態） ----------

// 規格 002 §3 的八個畫面操作與 001 §3.1 的六個簽章，名稱與位置逐一對照 capture.go 的表。
func TestCaptureTablesMatchSpec(t *testing.T) {
	if len(opHooks) != len(capOps) {
		t.Fatalf("opHooks 有 %d 個，規格 %d 個", len(opHooks), len(capOps))
	}
	got := map[string][2]int{}
	for _, op := range opHooks {
		got[op.name] = [2]int{int(op.entry), int(op.exit)}
	}
	for _, op := range capOps {
		if g, ok := got[op.name]; !ok || g != [2]int{op.entry, op.exit} {
			t.Errorf("掛點 %s = %04X/%04X，期望 %04X/%04X", op.name, g[0], g[1], op.entry, op.exit)
		}
	}
	if len(signatures) != len(capSigs) {
		t.Fatalf("signatures 有 %d 個，規格 %d 個", len(signatures), len(capSigs))
	}
	var names []string
	for _, s := range signatures {
		names = append(names, s.name)
		found := false
		for _, w := range capSigs {
			if w.name == s.name {
				found = true
				if int(s.off) != w.off || !reflect.DeepEqual(s.want, w.want) {
					t.Errorf("簽章 %s = %04X % X，期望 %04X % X", s.name, s.off, s.want, w.off, w.want)
				}
			}
		}
		if !found {
			t.Errorf("規格沒有簽章 %s", s.name)
		}
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"25A5", "A", "B", "L", "S", "T"}) {
		t.Errorf("簽章名稱 = %v", names)
	}
}

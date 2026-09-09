package machine

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 載入器（MZ／COM／PSP／MCB）的獨立契約測試。
//
// 期望值照 MZ 檔頭與 DOS 的載入約定自己算，不引用分支的既有測試。
// 這一層錯了**不會報錯**：程式照樣跑，只是從第一道指令起就在錯的位址上，
// 而症狀會出現在幾百萬道指令之後的某個 far call。

// buildTestMZ 組一支最小的 MZ：程式碼加一個重定位項。
//
// 檔頭欄位（word 為單位）：
//
//	00 簽章 'MZ' / 02 最後一頁的位元組數 / 04 頁數（512 bytes 一頁）
//	06 重定位項數 / 08 檔頭大小（段） / 0A 最少額外段 / 0C 最多額外段
//	0E 初始 SS（相對載入段）/ 10 初始 SP / 12 檢查碼
//	14 初始 IP / 16 初始 CS（相對載入段）/ 18 重定位表位移
func buildTestMZ(code []byte, relocAt uint16, ip, cs, ss, sp uint16) []byte {
	const hdrParas = 2 // 32 bytes：檔頭 ＋ 一個重定位項擺得下
	hdr := make([]byte, hdrParas*16)
	img := append([]byte(nil), code...)
	total := len(hdr) + len(img)

	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(hdr[off:], v) }
	copy(hdr, "MZ")
	put(0x02, uint16(total%512))
	put(0x04, uint16((total+511)/512))
	put(0x06, 1) // 一個重定位項
	put(0x08, hdrParas)
	put(0x0A, 0x10) // 最少額外 10h 段
	put(0x0C, 0xFFFF)
	put(0x0E, ss)
	put(0x10, sp)
	put(0x14, ip)
	put(0x16, cs)
	put(0x18, 0x1C) // 重定位表在檔頭的 1Ch
	binary.LittleEndian.PutUint16(hdr[0x1C:], relocAt)
	binary.LittleEndian.PutUint16(hdr[0x1E:], 0) // 重定位項的段一律 0
	return append(hdr, img...)
}

// 重定位要把「檔案裡寫的段」加上實際載入段。
//
// 不做的話程式跳到的是**檔案裡那個相對段**——通常落在 IVT 或 BDA 上，
// 而那裡有的是合法位元組，CPU 會照跑，只是跑的是別的東西。
func TestMZRelocationAddsLoadSegment(t *testing.T) {
	m := New()
	code := make([]byte, 0x20)
	binary.LittleEndian.PutUint16(code[0x04:], 0x1234) // 待重定位的段值
	img := buildTestMZ(code, 0x04, 0x0010, 0x0000, 0x0000, 0x0100)
	if err := m.LoadEXE(img); err != nil {
		t.Fatal(err)
	}
	got := m.Read16(uint32(LoadSeg)*16 + 0x04)
	if want := uint16(0x1234 + LoadSeg); got != want {
		t.Fatalf("重定位後是 %04X，預期 %04X（0x1234 ＋ 載入段 %04X）", got, want, LoadSeg)
	}
}

// 進入點：CS／SS 是**相對載入段**的，IP／SP 照抄；DS 與 ES 指向 PSP。
//
// DS 指錯的症狀最安靜：C runtime 會拿它當資料段基底，讀出來的是別人的
// 位元組，而每一個看起來都合法。
func TestMZEntryStateFollowsHeader(t *testing.T) {
	m := New()
	img := buildTestMZ(make([]byte, 0x20), 0x04, 0x0012, 0x0001, 0x0002, 0x0FFE)
	if err := m.LoadEXE(img); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	if c.Seg[cpu.CS] != LoadSeg+1 || c.IP != 0x0012 {
		t.Errorf("進入點 %04X:%04X，預期 %04X:0012", c.Seg[cpu.CS], c.IP, LoadSeg+1)
	}
	if c.Seg[cpu.SS] != LoadSeg+2 || c.R[cpu.SP] != 0x0FFE {
		t.Errorf("堆疊 %04X:%04X，預期 %04X:0FFE", c.Seg[cpu.SS], c.R[cpu.SP], LoadSeg+2)
	}
	if c.Seg[cpu.DS] != PSPSeg || c.Seg[cpu.ES] != PSPSeg {
		t.Errorf("DS／ES ＝ %04X／%04X，預期都是 PSP 段 %04X",
			c.Seg[cpu.DS], c.Seg[cpu.ES], PSPSeg)
	}
	// DOS 交棒時中斷是開的。關著的話任何等 BIOS 時鐘的迴圈都轉不出來。
	if !c.Flag(cpu.IF) {
		t.Error("交棒時 IF 是 0——等計時器的迴圈會變成死迴圈")
	}
}

// PSP 的三個欄位有具體的呼叫端：
//
//   - 00h：`int 20h`（`.COM` 的 `retn` 慣例落點）
//   - 02h：記憶體上限段（C runtime 拿它算堆積大小）
//   - 2Ch：環境段（`getenv` 走它）
func TestPSPHasTheFieldsRuntimesRead(t *testing.T) {
	m := New()
	if err := m.LoadEXE(buildTestMZ(make([]byte, 0x10), 0x00, 0, 0, 0, 0x100)); err != nil {
		t.Fatal(err)
	}
	psp := uint32(PSPSeg) * 16
	if a, b := m.Read8(psp), m.Read8(psp+1); a != 0xCD || b != 0x20 {
		t.Errorf("PSP 開頭是 %02X %02X，預期 CD 20（int 20h）", a, b)
	}
	if got := m.Read16(psp + 2); got != MemTop {
		t.Errorf("記憶體上限是 %04X，預期 %04X", got, MemTop)
	}
	if got := m.Read16(psp + 0x2C); got == 0 {
		t.Error("環境段是 0——getenv 會讀到 0000:xxxx")
	}
}

// `.COM` 的載入約定：整個檔案就是映像，載在 PSP 段的 0100h，
// 四個段暫存器全部指向 PSP，堆疊在段頂而且壓了一個 0。
//
// 那個 0 是 `.COM` 時代的收工慣例：程式做 `retn` 會落到 PSP:0000 的 `int 20h`。
func TestCOMLoadPlacesImageAndStack(t *testing.T) {
	m := New()
	code := []byte{0x90, 0x90, 0xC3}
	if err := m.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	if c.Seg[cpu.CS] != PSPSeg || c.IP != 0x100 {
		t.Errorf("進入點 %04X:%04X，預期 %04X:0100", c.Seg[cpu.CS], c.IP, PSPSeg)
	}
	for _, s := range []int{cpu.DS, cpu.ES, cpu.SS} {
		if c.Seg[s] != PSPSeg {
			t.Errorf("段暫存器 %d ＝ %04X，預期 %04X", s, c.Seg[s], PSPSeg)
		}
	}
	if c.R[cpu.SP] != 0xFFFE {
		t.Errorf("SP ＝ %04X，預期 FFFE", c.R[cpu.SP])
	}
	if got := m.Read16(cpu.Addr(PSPSeg, 0xFFFE)); got != 0 {
		t.Errorf("堆疊頂是 %04X，預期 0000", got)
	}
	for i, b := range code {
		if got := m.Read8(cpu.Addr(PSPSeg, uint16(0x100+i))); got != b {
			t.Fatalf("映像第 %d 個位元組是 %02X，預期 %02X", i, got, b)
		}
	}
	// **可配置區不能落在程式自己的段裡**：它的堆疊在段頂。
	if m.FreeSeg < PSPSeg+0x1000 {
		t.Errorf("FreeSeg ＝ %04X，落在 .COM 自己的段裡（堆疊在 %04X:FFFE）",
			m.FreeSeg, PSPSeg)
	}
}

// 空檔與塞不進一個段的映像都要**失敗**，而且訊息要分得開。
// 靜靜地載一半的話，程式從一個沒有指令的位址開始跑。
func TestCOMLoadRejectsImpossibleImages(t *testing.T) {
	if err := New().LoadCOM(nil); err == nil {
		t.Error("空的 .COM 竟然載得起來")
	}
	if err := New().LoadCOM(make([]byte, 0xFF00)); err == nil {
		t.Error("塞不進一個段的 .COM 竟然載得起來")
	}
}

// MZ 的檔頭壞掉要回錯，不要當成 `.COM` 硬載。
func TestMZRejectsTruncatedHeader(t *testing.T) {
	if err := New().LoadEXE([]byte{'M', 'Z'}); err == nil {
		t.Error("只有簽章的 MZ 竟然載得起來")
	}
}

// 中斷向量表：每一個向量都要指到一段**能執行而且會回來**的碼。
//
// 指到 0000:0000 的話，任何一個沒實作的中斷都會把 CPU 帶去執行 IVT 本身
// ——那裡的位元組是位址，被讀成指令之後 CPU 會跑進一片亂碼，
// 而且要幾千道指令之後才會出事。
func TestEveryVectorPointsAtExecutableCode(t *testing.T) {
	m := New()
	for v := 0; v < 256; v++ {
		seg := m.Read16(uint32(v)*4 + 2)
		off := m.Read16(uint32(v) * 4)
		if seg == 0 && off == 0 {
			t.Fatalf("向量 %02X 指到 0000:0000", v)
		}
		if b := m.Read8(cpu.Addr(seg, off)); b == 0x00 {
			t.Errorf("向量 %02X 指到 %04X:%04X，第一個位元組是 00", v, seg, off)
		}
	}
}

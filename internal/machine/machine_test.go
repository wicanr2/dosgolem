package machine

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// TestMouseVectorIsNotIRET 釘住整個專案最容易安靜出錯的一格。
//
// 滑鼠偵測直接讀 `0000:00CC` 拿到段:位移，再讀**那個位址的第一個位元組**；
// 是 `CFh`（IRET）或向量是 `0000:0000` 就判定「沒有驅動」
// （`rich2/docs/re/182` §2）。
//
// 所有向量都指到同一個 IRET 是最自然的寫法，而它會讓遊戲從此不發
// `int 33h`——**沒有錯誤訊息，只是永遠不用滑鼠**。
func TestMouseVectorIsNotIRET(t *testing.T) {
	m := New()
	off := m.Read16(0x33 * 4)
	seg := m.Read16(0x33*4 + 2)
	if seg == 0 && off == 0 {
		t.Fatal("int 33h 的向量是 0000:0000——遊戲會判定沒有滑鼠")
	}
	first := m.Read8(cpu.Addr(seg, off))
	if first == 0xCF {
		t.Fatalf("int 33h 指到的第一個位元組是 %02X（IRET）——遊戲會判定沒有滑鼠", first)
	}

	// 其餘向量要是合法位址，但**允許**是 IRET。
	for v := 0; v < 256; v++ {
		if v == 0x33 {
			continue
		}
		o, s := m.Read16(uint32(v)*4), m.Read16(uint32(v)*4+2)
		if s == 0 && o == 0 {
			t.Fatalf("向量 %02Xh 是 0000:0000——取到它的程式會跳進垃圾", v)
		}
	}
}

// TestEquipmentFlagsAreSet 釘住 BDA 那一格。
//
// BASIC 的 `SCREEN 13` 讀 `0040:0010` 判斷顯示卡，讀到 0 就回
// Illegal function call——而那個錯誤出現的位置離這裡很遠
// （`rich2/docs/re/005`「缺的是 BIOS 資料區」）。
func TestEquipmentFlagsAreSet(t *testing.T) {
	m := New()
	eq := m.Read16(0x0040*16 + 0x10)
	if eq == 0 {
		t.Fatal("裝置旗標是 0——BASIC 的 SCREEN 會判定顯示卡不對")
	}
	// bit 4–5 ＝ 00 表示「EGA 或更新」。不是 00 的話 BASIC 會以為是 CGA／MDA。
	if eq&0x30 != 0 {
		t.Errorf("裝置旗標 %04X 的 bit4–5 不是 00，BASIC 會判成 EGA 以前的卡", eq)
	}
	// bit 1 ＝ 有數學共處理器。**要是 0**：本程式的浮點走自己內建的模擬器。
	if eq&0x02 != 0 {
		t.Errorf("裝置旗標 %04X 說有 x87；這個 binary 走內建的浮點模擬器", eq)
	}
}

// buildMZ 造一個最小的 MZ 執行檔，含一筆重定位。
func buildMZ(t *testing.T, code []byte, relocAt uint16) []byte {
	t.Helper()
	const hdrPar = 2 // 32 bytes 檔頭（重定位表放在 1Ch 之後）
	body := make([]byte, 64)
	copy(body, code)
	// 在 relocAt 放一個「相對載入段」的段值 0，載入後應該變成 LoadSeg。
	binary.LittleEndian.PutUint16(body[relocAt:], 0)

	total := hdrPar*16 + len(body)
	out := make([]byte, total)
	h := out[:32]
	h[0], h[1] = 'M', 'Z'
	binary.LittleEndian.PutUint16(h[2:], uint16(total%512))
	binary.LittleEndian.PutUint16(h[4:], uint16((total+511)/512))
	binary.LittleEndian.PutUint16(h[6:], 1)      // 一筆重定位
	binary.LittleEndian.PutUint16(h[8:], hdrPar) // 檔頭段數
	binary.LittleEndian.PutUint16(h[14:], 0x20)  // SS
	binary.LittleEndian.PutUint16(h[16:], 0x100) // SP
	binary.LittleEndian.PutUint16(h[20:], 0x0)   // IP
	binary.LittleEndian.PutUint16(h[22:], 0x0)   // CS
	binary.LittleEndian.PutUint16(h[24:], 0x1C)  // 重定位表位移
	binary.LittleEndian.PutUint16(h[0x1C:], relocAt)
	binary.LittleEndian.PutUint16(h[0x1E:], 0) // 段 0
	copy(out[hdrPar*16:], body)
	return out
}

// TestLoadEXEAppliesRelocations 釘住「重定位一定要套」。
//
// 檔案裡的遠指標段值是**相對載入段**的。沒加上實際載入段的話，
// 第一個 far call 就飛到錯的地方——而那看起來像「程式自己壞了」。
func TestLoadEXEAppliesRelocations(t *testing.T) {
	m := New()
	if err := m.LoadEXE(buildMZ(t, []byte{0x90, 0xF4}, 0x10)); err != nil {
		t.Fatal(err)
	}
	got := m.Read16(LoadSeg*16 + 0x10)
	if got != LoadSeg {
		t.Errorf("重定位後的段值是 %04X，預期 %04X", got, LoadSeg)
	}
	if m.CPU.Seg[cpu.CS] != LoadSeg {
		t.Errorf("CS ＝ %04X，預期 %04X", m.CPU.Seg[cpu.CS], LoadSeg)
	}
	if m.CPU.Seg[cpu.DS] != PSPSeg || m.CPU.Seg[cpu.ES] != PSPSeg {
		t.Errorf("DS／ES 要指向 PSP，得到 %04X／%04X",
			m.CPU.Seg[cpu.DS], m.CPU.Seg[cpu.ES])
	}
	// 載入之後可以真的執行：NOP 然後 HLT。
	for i := 0; i < 4 && !m.CPU.Halted; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if !m.CPU.Halted {
		t.Error("跑不到 HLT——載入點或 CS:IP 不對")
	}
}

// TestLoadEXERejectsTruncatedImage 釘住「映像被截斷要報錯，不要安靜載入」。
//
// `RUN.EXE` 是雙層打包，只剝外層的話會少掉尾端 14%——而那**不會有任何
// 錯誤訊息**，裡面全是看得懂的程式碼與字串（`rich2/docs/re/105`）。
// 載入器至少要在「重定位一筆都套不上」時說話。
func TestLoadEXERejectsTruncatedImage(t *testing.T) {
	m := New()
	if err := m.LoadEXE([]byte{'M', 'Z'}); err == nil {
		t.Error("只有兩個位元組也載得進去？")
	}
	if err := m.LoadEXE([]byte("not an exe at all")); err == nil {
		t.Error("不是 MZ 也載得進去？")
	}
}

// TestPSPEnvironmentSegment 釘住 `PSP+2Ch`。
//
// Microsoft C runtime 啟動時讀它（`__setenvp`），指到 0 會讓後續的
// heap 初始化判定失敗——又是一個「0 是合法值」的坑
// （`docs/spec/004` §1.3）。
func TestPSPEnvironmentSegment(t *testing.T) {
	m := New()
	if err := m.LoadEXE(buildMZ(t, []byte{0xF4}, 0x10)); err != nil {
		t.Fatal(err)
	}
	if seg := m.Read16(PSPSeg*16 + 0x2C); seg == 0 {
		t.Fatal("PSP+2Ch 是 0——C runtime 的 heap 初始化會判定失敗")
	}
	if top := m.Read16(PSPSeg*16 + 2); top != MemTop {
		t.Errorf("PSP+2 的記憶體上限是 %04X，預期 %04X", top, MemTop)
	}
}

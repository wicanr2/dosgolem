package machine

import (
	"bytes"
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

func TestLoadOverlayUsesSeparateRelocationFactorAndPreservesCPU(t *testing.T) {
	m := New()
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x2222, 0x3333
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x4444, 0x5555
	data := buildMZ(t, []byte{0x90, 0xF4}, 0x10)
	if err := m.LoadOverlay(data, 0x3000, 0x1234); err != nil {
		t.Fatal(err)
	}
	if got := m.Read16(0x3000*16 + 0x10); got != 0x1234 {
		t.Fatalf("覆疊重定位後是 %04X，預期 1234", got)
	}
	if m.CPU.Seg[cpu.CS] != 0x2222 || m.CPU.IP != 0x3333 ||
		m.CPU.Seg[cpu.SS] != 0x4444 || m.CPU.R[cpu.SP] != 0x5555 {
		t.Fatal("覆疊載入改動了呼叫端 CPU 狀態")
	}
}

func TestLoadOverlayFailureDoesNotPartiallyOverwriteDestination(t *testing.T) {
	m := New()
	const dst = uint32(0x3000 * 16)
	m.WriteBytes(dst, []byte{0xAA, 0xBB, 0xCC, 0xDD})
	data := buildMZ(t, []byte{0x90, 0xF4}, 0x10)
	data[0x1C], data[0x1D] = 0xFF, 0x7F // relocation offset outside image
	if err := m.LoadOverlay(data, 0x3000, 0x1234); err == nil {
		t.Fatal("越界重定位竟然載入成功")
	}
	got := []byte{m.Read8(dst), m.Read8(dst + 1), m.Read8(dst + 2), m.Read8(dst + 3)}
	if !bytes.Equal(got, []byte{0xAA, 0xBB, 0xCC, 0xDD}) {
		t.Fatalf("失敗後目的區被部分改寫：% X", got)
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

// TestMachineIsA80186 釘住機型（`docs/spec/002` §1.1）。
//
// `RUN_full.EXE` 的主程式區有 3,345 個 80186 的 `PUSH imm`（`68` 1,779 次、
// `6A` 1,566 次）。8086 把 `60`–`6F` 當成條件跳躍的別名，於是 `68 FF 1F`
// 被解成 `JS` 而不是 `PUSH imm16`——**指令長度差一個 byte，後面整串錯位，
// 而且一個錯誤訊息都沒有**：第一次實跑就這樣飛進 A0000 後面的空白區
// （全是 `00 00` ＝ `add [bx+si],al`）跑滿兩百萬道指令。
func TestMachineIsA80186(t *testing.T) {
	m := New()
	if m.CPU.Model < cpu.Model80186 {
		t.Fatal("機器上的 CPU 是 8086——PUSH imm16 會被當成條件跳躍，整串錯位")
	}

	// 真的執行一次：`68 34 12` 要推 1234h，而且 IP 前進 3。
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0700, 0
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x0800, 0x100
	m.WriteBytes(cpu.Addr(0x0700, 0), []byte{0x68, 0x34, 0x12, 0x6A, 0xFF})
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if m.CPU.IP != 3 {
		t.Fatalf("PUSH imm16 之後 IP ＝ %d，預期 3", m.CPU.IP)
	}
	if v := m.Read16(cpu.Addr(0x0800, m.CPU.R[cpu.SP])); v != 0x1234 {
		t.Errorf("推上去的是 %04X，預期 1234", v)
	}

	// `6A FF` 的立即數要**符號延伸**：推的是 FFFF，不是 00FF。
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if v := m.Read16(cpu.Addr(0x0800, m.CPU.R[cpu.SP])); v != 0xFFFF {
		t.Errorf("PUSH imm8 推的是 %04X，預期 FFFF（要符號延伸）", v)
	}
}

func TestKeyboardIRQ1DeliversScanCodesAndHonorsIF(t *testing.T) {
	m := New()
	m.IRQ0Every = 0
	// ⚠ 落點要挑在低位記憶體的固定配置**外面**：0000:0500 是 EMSSeg 的
	// EMM 驅動 header（`CD F6 CF`），拿它當暫存區的話讀回來永遠是 CDh，
	// 而測試會把那當成「送出了掃描碼」。
	// in al,60h; mov [5000h],al; iret
	m.WriteBytes(cpu.Addr(0x0900, 0), []byte{0xE4, 0x60, 0xA2, 0x00, 0x50, 0xCF})
	m.Write16(0x09*4, 0)
	m.Write16(0x09*4+2, 0x0900)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0800, 0
	m.CPU.Seg[cpu.DS] = 0
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x0700, 0x100
	m.WriteBytes(cpu.Addr(0x0800, 0), []byte{0x90, 0x90, 0x90, 0x90})
	m.QueueScanCodes(0x01)

	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if got := m.Read8(0x5000); got != 0 {
		t.Fatalf("IF關閉時送出了掃描碼%02X", got)
	}
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	for i := 0; i < 4; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if got := m.Read8(0x5000); got != 0x01 {
		t.Fatalf("IRQ1處理程式讀到%02X，預期01", got)
	}
	if got := m.PortsIn[0x60]; got != 1 {
		t.Fatalf("port 60h讀取%d次，預期1", got)
	}
}

func TestKeyboardQueueSurvivesSnapshotRestore(t *testing.T) {
	m := New()
	m.QueueScanCodes(0x01, 0x81)
	s := m.Snapshot()
	m.keyQueue = nil
	m.kbdData = 0xFF
	m.Restore(s)
	if len(m.keyQueue) != 2 || m.keyQueue[0].Code() != 0x01 || m.keyQueue[1].Code() != 0x81 {
		t.Fatalf("還原後掃描碼佇列=%+v", m.keyQueue)
	}
	if m.kbdData != 0 {
		t.Fatalf("還原後port 60h資料=%02X，預期00", m.kbdData)
	}
}

// TestRetraceBitToggles 釘住「3DAh 的值一定要會變」。
//
// VGA 的回掃等待是兩段：先等這一次結束、再等下一次開始。**回任何定值
// 都一定有一段轉不出來**，而症狀是「程式沒走到那一步」，
// 完全不指向模擬器（`rich2/tools/dosemu.py` 的 on_in：`SS.EXE` 跑滿
// 四億條指令沒開 SS.YJA 就是死在這）。
func TestRetraceBitToggles(t *testing.T) {
	m := New()
	seenSet, seenClear := false, false
	for i := 0; i < 200 && !(seenSet && seenClear); i++ {
		if m.In8(0x3DA)&0x08 != 0 {
			seenSet = true
		} else {
			seenClear = true
		}
	}
	if !seenSet || !seenClear {
		t.Fatal("3DAh 的回掃位元不會變——等待迴圈有一段轉不出來")
	}

	// bit0（顯示中）也要會變：程式等的可能是這一個。
	m2 := New()
	set, clear := false, false
	for i := 0; i < 200; i++ {
		if m2.In8(0x3DA)&0x01 != 0 {
			set = true
		} else {
			clear = true
		}
	}
	if !set || !clear {
		t.Error("3DAh 的 bit0 不會變")
	}
}

// TestPITCountsDown 釘住「PIT 是遞減計數器」。
//
// 回定值的話用 PIT 做延遲的迴圈不會結束。
func TestPITCountsDown(t *testing.T) {
	m := New()
	seen := map[uint8]bool{}
	for i := 0; i < 8; i++ {
		seen[m.In8(0x40)] = true
	}
	if len(seen) < 4 {
		t.Errorf("讀 8 次 PIT 只看到 %d 種值——延遲迴圈會卡住", len(seen))
	}
}

// TestPortLogKeepsSequence 釘住埠寫入的完整序列。
//
// VGA 的 plane 選擇（3C4/3C5）是「先寫索引再寫值」的兩步，只留最後
// 一次的值看不出選過哪些 plane——序列本身才是證據。
func TestPortLogKeepsSequence(t *testing.T) {
	m := New()
	for _, w := range []struct {
		p uint16
		v uint8
	}{{0x3C4, 0x02}, {0x3C5, 0x01}, {0x3C4, 0x02}, {0x3C5, 0x08}} {
		m.Out8(w.p, w.v)
	}
	if n := len(m.PortLog); n != 4 {
		t.Fatalf("PortLog 有 %d 筆，要 4 筆", n)
	}
	if m.PortLog[1].Val != 0x01 || m.PortLog[3].Val != 0x08 {
		t.Errorf("序列被覆蓋了：%+v", m.PortLog)
	}
	if m.Ports[0x3C5] != 0x08 {
		t.Errorf("Ports 該留最後一次的值，拿到 %02X", m.Ports[0x3C5])
	}
}

// TestTimerStubRestoresDSAfterInt1C 釘住 BIOS 計時器 stub 的還原順序。
//
// IBM PC BIOS 的 TIMER_INT 是 `push ds/ax/dx` → `DS=40h` → 推進計數 →
// **`int 1Ch`** → `pop dx/ax/ds` → `iret`。還原在 `int 1Ch` **之後**，
// 所以 1Ch 的處理常式把 DS 弄髒也不會影響被中斷的程式。
//
// 這一格錯了不會有任何錯誤訊息：DS 漏回去之後，被中斷的程式接著用錯的
// 段讀資料，看起來還是在正常執行。源平合戰的 int 1Ch 是
// `cli / pusha / mov ds,自己的段 … popa / sti / jmp far 舊向量`——
// `pusha` 不含 DS，所以它一定會留下自己的 DS，靠 BIOS 救。
// 舊版 stub 在 `int 1Ch` 之前就 pop 完，遊戲的位元碼直譯器因此換了段抓
// 位元碼，四百道之後查表出界，十九萬道之後才死在低位記憶體。
func TestTimerStubRestoresDSAfterInt1C(t *testing.T) {
	m := New()

	// 掛一個「弄髒 DS 就走」的 int 1Ch，形狀照遊戲那支：
	//	mov ax,1234h / mov ds,ax / iret
	const dirty = 0x1234
	// ⚠ **不要把測試用的碼放進 StubSeg**：那一段是向量 stub 與 BIOS
	// 計時器常式的家，寫進去會把它們蓋掉，而症狀是「BIOS 沒做該做的事」。
	const scratch = 0x2000
	handler := uint16(0x0300)
	m.WriteBytes(cpu.Addr(scratch, handler), []byte{
		0xB8, byte(dirty & 0xFF), byte(dirty >> 8), 0x8E, 0xD8, 0xCF,
	})
	m.Write16(0x1C*4, handler)
	m.Write16(0x1C*4+2, scratch)

	// 被中斷的程式：DS 是別的段，跑一道 nop 就好。
	const userDS = 0x5678
	code := uint16(0x0400)
	m.WriteBytes(cpu.Addr(scratch, code), []byte{0x90, 0x90, 0xF4}) // nop nop hlt
	m.CPU.Seg[cpu.CS] = scratch
	m.CPU.IP = code
	m.CPU.Seg[cpu.DS] = userDS
	m.CPU.Seg[cpu.SS] = scratch
	m.CPU.R[cpu.SP] = 0x0200
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)

	// 直接送一次計時器中斷，然後把 stub ＋ 1Ch ＋ 回來的路跑完。
	m.CPU.Interrupt(0x08)
	for i := 0; i < 200 && !m.CPU.Halted; i++ {
		if err := m.Step(); err != nil {
			t.Fatalf("第 %d 道出錯：%v", i, err)
		}
	}

	if got := m.CPU.Seg[cpu.DS]; got != userDS {
		t.Errorf("中斷回來後 DS ＝ %04X，要 %04X——int 1Ch 弄髒的 DS 漏回被中斷的程式了",
			got, userDS)
	}
	// BIOS 的 tick 計數還是要有推進。
	if tick := m.Read16(0x40*16 + 0x6C); tick == 0 {
		t.Error("0040:006C 沒有推進——stub 沒做 BIOS 該做的事")
	}
}

// TestEGABitMaskKeepsUntouchedBits 釘住 Bit Mask 之外的位元要從 latch 補回去。
//
// **少了 latch 的症狀不是壞掉，是一張有規律雜訊的圖**：遮罩外的位元被歸零，
// 畫面上是一條一條的直線，看起來像時序問題不像少了一個暫存器。
func TestEGABitMaskKeepsUntouchedBits(t *testing.T) {
	m := New()
	m.SetVideoMode(0x10) // EGA 640×350 平面模式
	const at = 0xA0000
	// 先把四個平面填成已知值（Map Mask 全開、bit mask 全開）。
	m.Write8(at, 0xFF)

	// 只開 bit0，寫 0x00：其餘七個位元應該保持 1。
	m.Out8(0x3CE, 0x08) // Bit Mask
	m.Out8(0x3CF, 0x01)
	_ = m.Read8(at) // ★ 讀一次把 latch 鎖起來
	m.Write8(at, 0x00)

	m.Out8(0x3CE, 0x08)
	m.Out8(0x3CF, 0xFF)
	if got := m.Read8(at); got != 0xFE {
		t.Fatalf("寫入之後讀回 %02X，應該是 FE——Bit Mask 之外的位元沒有從 latch 補回去", got)
	}
}

// TestEGASetResetPicksColour 釘住 Set/Reset：打開的平面用 Set/Reset 的顏色，
// 不是 CPU 寫進去的值。
func TestEGASetResetPicksColour(t *testing.T) {
	m := New()
	m.SetVideoMode(0x10)
	const at = 0xA0000
	m.Out8(0x3CE, 0x01) // Enable Set/Reset：四個平面全開
	m.Out8(0x3CF, 0x0F)
	m.Out8(0x3CE, 0x00) // Set/Reset：顏色 0101b ＝ 平面 0 與 2 填 1
	m.Out8(0x3CF, 0x05)
	_ = m.Read8(at)
	m.Write8(at, 0x00) // CPU 的值應該被忽略

	m.Out8(0x3CE, 0x01) // 關掉 Set/Reset 再讀，免得影響
	m.Out8(0x3CF, 0x00)
	for plane, want := range []uint8{0xFF, 0x00, 0xFF, 0x00} {
		m.Out8(0x3CE, 0x04) // Read Map Select
		m.Out8(0x3CF, uint8(plane))
		if got := m.Read8(at); got != want {
			t.Errorf("平面 %d 是 %02X，應該是 %02X——Set/Reset 沒生效", plane, got, want)
		}
	}
}

// TestEGAWriteMode1CopiesLatches 釘住寫入模式 1：latch 原封不動寫回去。
// 那是搬圖形（讀一格、寫一格）的做法，CPU 寫進去的值完全不參與。
func TestEGAWriteMode1CopiesLatches(t *testing.T) {
	m := New()
	const src, dst = 0xA0000, 0xA0100
	m.Out8(0x3C4, 0x02) // Map Mask：只寫平面 1
	m.Out8(0x3C5, 0x02)
	m.Write8(src, 0xAB)
	m.Out8(0x3C5, 0x0F)

	m.Out8(0x3CE, 0x05) // Mode：寫入模式 1
	m.Out8(0x3CF, 0x01)
	_ = m.Read8(src)
	m.Write8(dst, 0x00) // 值被忽略

	m.Out8(0x3CE, 0x05)
	m.Out8(0x3CF, 0x00)
	m.Out8(0x3CE, 0x04)
	m.Out8(0x3CF, 0x01) // 讀平面 1
	if got := m.Read8(dst); got != 0xAB {
		t.Fatalf("搬過去讀回 %02X，應該是 AB", got)
	}
}

// TestSnapshotKeepsTheKeyboardAlive 釘住還原之後鍵盤還送得出中斷。
//
// `Restore` 把 `Steps` 倒回過去。鍵盤的下一次中斷排在 `nextKey`，
// 漏掉它的話那個值會停在未來——`keyTick` 的「時間還沒到」從此永遠成立，
// **後面每一個鍵都靜靜地留在佇列裡**。
//
// 症狀不是錯誤：從同一個快照展開多個變體時，第一個收得到鍵，
// 後面每一個都「按了沒反應」，看起來像那幾種送法不對。
func TestSnapshotKeepsTheKeyboardAlive(t *testing.T) {
	m := New()
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	// int 09h 還指著 BIOS stub 的時候 keyTick 不送（改記 keyStalls），
	// 所以要先裝一支「程式自己的」處理常式，否則量不到要量的東西。
	m.Write16(0x09*4+2, 0x2000)
	m.Steps = 1_000
	snap := m.Snapshot()

	// 往前跑並送掉一個鍵：nextKey 於是落在 5000 之後。
	m.Steps = 5_000
	m.PushKey(0x0A)
	m.keyTick()
	if m.IRQ1Delivered() == 0 {
		t.Fatal("第一個鍵就沒送出去，這個測試量不到要量的東西")
	}
	if m.nextKey <= snap.steps {
		t.Fatalf("nextKey 是 %d，沒有落在快照（%d）之後", m.nextKey, snap.steps)
	}

	m.Restore(snap)
	before := m.IRQ1Delivered()
	m.PushKey(0x0A)
	m.keyTick()
	if m.IRQ1Delivered() == before {
		t.Fatal("還原之後鍵送不出去——nextKey 還停在未來")
	}
}

// TestSnapshotCarriesTheKeyQueue 釘住還沒送出去的鍵跟著快照走。
//
// 不跟著走的話，上一個變體剩下的鍵會流進下一個變體，
// 而「對照組什麼都不送」會憑空收到一個鍵。
func TestSnapshotCarriesTheKeyQueue(t *testing.T) {
	m := New()
	m.PushKey(0x1C)
	snap := m.Snapshot()
	if got := len(snap.keyQueue); got != 2 {
		t.Fatalf("快照裡有 %d 個鍵盤事件，應該是 2（按下 ＋ 放開）", got)
	}
	m.keyQueue = nil
	m.kbdData = 0x99
	m.Restore(snap)
	if got := m.KeyQueueLen(); got != 2 {
		t.Fatalf("還原之後佇列有 %d 個事件，應該是 2", got)
	}
	if m.kbdData != snap.kbdData {
		t.Errorf("埠 0x60 讀得到的值沒還原：%#02x ≠ %#02x", m.kbdData, snap.kbdData)
	}

	// 反過來也要成立：快照之後才排進去的鍵，還原時要消失。
	m.PushKey(0x39)
	m.Restore(snap)
	if got := m.KeyQueueLen(); got != 2 {
		t.Fatalf("還原沒清掉快照之後排進去的鍵：佇列有 %d 個事件", got)
	}
}

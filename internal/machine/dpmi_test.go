package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// DPMI 主機的契約測試。期望值照 DPMI 1.0 規格自己列——**DOSBox-X 抄不到**，
// 它沒有這一層（見 `docs/knowledge-base/030-protected-mode-and-dos4gw.md`）。

// newDPMITest 造一台只有一點記憶體的 LE 機器與一個主機。
func newDPMITest(t *testing.T) (*LEMachine, *DPMIHost) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x1000), Ports: map[uint16]uint8{}}
	m.CPU = cpu386.New(m)
	return m, NewDPMIHost(m)
}

func dpmiCall(h *DPMIHost, c *cpu386.CPU, ax uint16) bool {
	c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | uint32(ax)
	return h.Handle(c)
}

// 配出來的描述子是**空的**：base 0、limit 0、不可寫。
//
// 給 4 GB 的話，程式忘記設 base 就直接用會讀到別人的資料，而那不會報錯。
func TestDPMIAllocatedDescriptorsStartEmpty(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU

	c.R[cpu386.ECX] = 3
	if !dpmiCall(h, c, 0x0000) {
		t.Fatal("AX=0000h 沒實作")
	}
	if c.EFlags&cpu386.CF != 0 {
		t.Fatalf("配置失敗：AX=%04X", uint16(c.R[cpu386.EAX]))
	}
	first := uint16(c.R[cpu386.EAX])
	for i := uint16(0); i < 3; i++ {
		sel := first + i*8
		d, ok := c.Descriptors[sel]
		if !ok {
			t.Fatalf("selector %04X 沒有描述子", sel)
		}
		if d.Base != 0 || d.Limit != 0 || d.Writable {
			t.Errorf("selector %04X 的描述子不是空的：%+v", sel, d)
		}
	}
	// 間隔要與 AX=0003h 回的一致，否則程式算出來的第二個 selector 是錯的。
	if !dpmiCall(h, c, 0x0003) {
		t.Fatal("AX=0003h 沒實作")
	}
	if got := uint16(c.R[cpu386.EAX]); got != 8 {
		t.Errorf("selector 間隔回 %d，預期 8", got)
	}
}

// 設 base／limit 之後，那個 selector 真的能定址到該段記憶體。
//
// 只把值記在描述子表裡而 CPU 不用它的話，程式設完 base 仍然讀到 0——
// 而它會以為那段記憶體是空的。
func TestDPMISetBaseAndLimitAffectAddressing(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	m.Mem[0x800] = 0x5A

	c.R[cpu386.ECX] = 1
	dpmiCall(h, c, 0x0000)
	sel := uint16(c.R[cpu386.EAX])

	c.R[cpu386.EBX] = uint32(sel)
	c.R[cpu386.ECX], c.R[cpu386.EDX] = 0, 0x800 // base ＝ 0x00000800
	dpmiCall(h, c, 0x0007)
	c.R[cpu386.EBX] = uint32(sel)
	c.R[cpu386.ECX], c.R[cpu386.EDX] = 0, 0xFF // limit ＝ 0xFF
	dpmiCall(h, c, 0x0008)

	if got := c.Descriptors[sel]; got.Base != 0x800 || got.Limit != 0xFF {
		t.Fatalf("描述子是 %+v，預期 base=800 limit=FF", got)
	}
	// 取回來要一致。
	c.R[cpu386.EBX] = uint32(sel)
	dpmiCall(h, c, 0x0006)
	if base := uint32(uint16(c.R[cpu386.ECX]))<<16 | uint32(uint16(c.R[cpu386.EDX])); base != 0x800 {
		t.Errorf("AX=0006h 回 base %08X，預期 800", base)
	}
}

// 不存在的 selector 要回**無效 selector**（8022h），不是安靜地成功。
func TestDPMIRejectsUnknownSelector(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	for _, fn := range []uint16{0x0006, 0x0007, 0x0008, 0x0009, 0x000A} {
		c.R[cpu386.EBX] = 0xF000
		c.EFlags &^= cpu386.CF
		if !dpmiCall(h, c, fn) {
			t.Fatalf("AX=%04X 沒實作", fn)
		}
		if c.EFlags&cpu386.CF == 0 {
			t.Errorf("AX=%04X 對不存在的 selector 竟然成功", fn)
			continue
		}
		if got := uint16(c.R[cpu386.EAX]); got != 0x8022 {
			t.Errorf("AX=%04X 回錯誤碼 %04X，預期 8022", fn, got)
		}
	}
}

// 線性記憶體配置要真的給得出可以讀寫的位址，而且**不重疊**。
func TestDPMIAllocatedBlocksAreUsableAndDisjoint(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU

	alloc := func(size uint32) (base, handle uint32) {
		c.R[cpu386.EBX] = size >> 16
		c.R[cpu386.ECX] = size & 0xffff
		if !dpmiCall(h, c, 0x0501) {
			t.Fatal("AX=0501h 沒實作")
		}
		if c.EFlags&cpu386.CF != 0 {
			t.Fatalf("配 %d bytes 失敗：AX=%04X", size, uint16(c.R[cpu386.EAX]))
		}
		base = uint32(uint16(c.R[cpu386.EBX]))<<16 | uint32(uint16(c.R[cpu386.ECX]))
		handle = uint32(uint16(c.R[cpu386.ESI]))<<16 | uint32(uint16(c.R[cpu386.EDI]))
		return base, handle
	}

	a, ha := alloc(0x100)
	b, _ := alloc(0x100)
	if a == b {
		t.Fatal("兩次配置回同一個位址")
	}
	if b < a+0x100 {
		t.Fatalf("第二塊 %08X 與第一塊 %08X（0x100 bytes）重疊", b, a)
	}
	// 配出來的位址要真的能寫。
	if err := m.Write8(a, 0x42); err != nil {
		t.Fatalf("寫配出來的位址失敗：%v", err)
	}
	if got, _ := m.Read8(a); got != 0x42 {
		t.Errorf("讀回 %02X，預期 42", got)
	}

	// 釋放之後 handle 就不認得了。
	c.R[cpu386.ESI], c.R[cpu386.EDI] = ha>>16, ha&0xffff
	dpmiCall(h, c, 0x0502)
	if c.EFlags&cpu386.CF != 0 {
		t.Fatal("釋放失敗")
	}
	c.R[cpu386.ESI], c.R[cpu386.EDI] = ha>>16, ha&0xffff
	dpmiCall(h, c, 0x0502)
	if c.EFlags&cpu386.CF == 0 {
		t.Error("釋放同一個 handle 兩次竟然成功")
	}

	// 位址單調遞增：釋放過的不重用，這樣「誰還留著舊指標」才查得出來。
	cAddr, _ := alloc(0x10)
	if cAddr <= b {
		t.Errorf("釋放之後配到 %08X，回頭用了舊位址", cAddr)
	}
}

// 大得離譜的配置要失敗，不能把主機的記憶體吃光。
func TestDPMIRefusesAbsurdAllocation(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	c.R[cpu386.EBX] = 0xFFFF // 4 GB−
	c.R[cpu386.ECX] = 0xFFFF
	dpmiCall(h, c, 0x0501)
	if c.EFlags&cpu386.CF == 0 {
		t.Fatal("配 4 GB 竟然成功")
	}
	if got := uint16(c.R[cpu386.EAX]); got != 0x8013 {
		t.Errorf("錯誤碼 %04X，預期 8013（實體記憶體不足）", got)
	}
}

// 實模式向量要能取回載入器擺進去的值。
//
// 回 0:0 的話，程式 chain 到 IVT 本身——那裡的位元組是位址，
// 被讀成指令之後會跑進一片亂碼。
func TestDPMIRealModeVectorRoundTrip(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	h.SetRealModeVector(0x08, 0x1234, 0x5678)

	c.R[cpu386.EBX] = 0x08
	dpmiCall(h, c, 0x0200)
	if seg, off := uint16(c.R[cpu386.ECX]), uint16(c.R[cpu386.EDX]); seg != 0x1234 || off != 0x5678 {
		t.Fatalf("AX=0200h 回 %04X:%04X，預期 1234:5678", seg, off)
	}

	// 程式把自己的接上去之後，再問要拿到新的那一個。
	c.R[cpu386.EBX] = 0x08
	c.R[cpu386.ECX], c.R[cpu386.EDX] = 0x9000, 0x0010
	dpmiCall(h, c, 0x0201)
	seg, off := h.RealModeVector(0x08)
	if seg != 0x9000 || off != 0x0010 {
		t.Errorf("設完之後是 %04X:%04X，預期 9000:0010", seg, off)
	}
}

// `AX=0900h`／`0901h` 回的是**先前**的中斷狀態，而且真的改 IF。
//
// 回錯的話，成對使用（關掉→做事→還原）的程式會在還原時把 IF 設反，
// 而症狀是「後來某個地方收不到計時器中斷」。
func TestDPMIInterruptFlagCallsReturnPreviousState(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU

	c.EFlags |= cpu386.IF
	dpmiCall(h, c, 0x0900) // 取並關
	if uint8(c.R[cpu386.EAX]) != 1 {
		t.Errorf("先前 IF 是 1，AL 回 %d", uint8(c.R[cpu386.EAX]))
	}
	if c.EFlags&cpu386.IF != 0 {
		t.Error("AX=0900h 沒有把 IF 關掉")
	}
	dpmiCall(h, c, 0x0901) // 取並開
	if uint8(c.R[cpu386.EAX]) != 0 {
		t.Errorf("先前 IF 是 0，AL 回 %d", uint8(c.R[cpu386.EAX]))
	}
	if c.EFlags&cpu386.IF == 0 {
		t.Error("AX=0901h 沒有把 IF 打開")
	}
}

// 鎖定要留下紀錄：程式鎖了才做 DMA 或把位址交給中斷處理常式。
func TestDPMILockIsRecorded(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	c.R[cpu386.EBX], c.R[cpu386.ECX] = 0x0001, 0x2000 // 位址 0x00012000
	c.R[cpu386.ESI], c.R[cpu386.EDI] = 0x0000, 0x0400 // 長度 0x400
	dpmiCall(h, c, 0x0600)
	if len(h.Locks) != 1 || h.Locks[0].Base != 0x00012000 || h.Locks[0].Size != 0x400 {
		t.Fatalf("鎖定紀錄是 %+v，預期 base=12000 size=400", h.Locks)
	}
}

// 沒實作的功能要回 false 並記一筆，**不能安靜地回成功**。
func TestDPMIUnknownFunctionIsRecorded(t *testing.T) {
	m, h := newDPMITest(t)
	if dpmiCall(h, m.CPU, 0x0300) { // 模擬實模式中斷：目前沒做
		t.Fatal("AX=0300h 沒實作卻回了 true")
	}
	if h.Unimplemented[0x0300] != 1 {
		t.Errorf("沒實作的功能沒有記一筆：%+v", h.Unimplemented)
	}
}

// 版本要回得出來：程式先問版本再決定用哪些功能。
func TestDPMIVersionIsAnswered(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	dpmiCall(h, c, 0x0400)
	if c.EFlags&cpu386.CF != 0 {
		t.Fatal("AX=0400h 失敗")
	}
	if got := uint16(c.R[cpu386.EAX]); got == 0 {
		t.Error("版本回 0——程式會判定沒有 DPMI")
	}
	if c.R[cpu386.EBX]&1 == 0 {
		t.Error("旗標沒有標出 32 位元主機")
	}
}

// `AX=0100h` 配出來的東西要**同時**給得出實模式段與 selector，
// 而且兩者指到同一段記憶體。
//
// 段號與 selector 指到不同地方的話，程式寫進 selector、把段號交給實模式
// 那一邊，而那一邊讀到的是別的東西——它不會報錯，只是拿到垃圾。
func TestDPMIDOSMemoryGivesMatchingSegmentAndSelector(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU

	c.R[cpu386.EBX] = 4 // 4 段 ＝ 64 bytes
	if !dpmiCall(h, c, 0x0100) {
		t.Fatal("AX=0100h 沒實作")
	}
	if c.EFlags&cpu386.CF != 0 {
		t.Fatalf("配置失敗：AX=%04X", uint16(c.R[cpu386.EAX]))
	}
	segment := uint16(c.R[cpu386.EAX])
	selector := uint16(c.R[cpu386.EDX])
	if segment == 0 || selector == 0 {
		t.Fatalf("段號 %04X／selector %04X 有一個是 0", segment, selector)
	}

	// selector 寫得進去，而且實模式段位址讀得到同一份資料。
	if !c.WriteSegmentBytes(selector, 0, []byte("HI")) {
		t.Fatal("selector 寫不進去——描述子沒設好")
	}
	base := uint32(segment) * 16
	if got, _ := m.Read8(base); got != 'H' {
		t.Errorf("段位址 %05X 讀到 %02X，預期 'H'——段號與 selector 指到不同地方", base, got)
	}

	// **一定要在 1 MB 以下**：段號只有 16 位元，之上的位址表達不出來。
	if base >= 0x100000 {
		t.Errorf("DOS 記憶體配在 %08X，超過 1 MB", base)
	}
	if uint32(len(m.Mem)) < base+64 {
		t.Errorf("記憶體只有 %d bytes，配出來的區塊沒有 backing", len(m.Mem))
	}
}

// DOS 記憶體與線性記憶體**不能互相踩到**。
//
// 共用一個游標的話，程式先配一大塊線性記憶體再要 DOS 記憶體，
// 拿到的位址會超過 1 MB——段號截斷之後指到映像頭上，寫下去就把自己的碼改了。
func TestDPMIDOSMemoryAndLinearMemoryDoNotOverlap(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU

	c.R[cpu386.EBX], c.R[cpu386.ECX] = 0, 0x8000 // 32 KB 線性記憶體
	dpmiCall(h, c, 0x0501)
	if c.EFlags&cpu386.CF != 0 {
		t.Fatal("配線性記憶體失敗")
	}
	linear := uint32(uint16(c.R[cpu386.EBX]))<<16 | uint32(uint16(c.R[cpu386.ECX]))
	if linear < 0x100000 {
		t.Errorf("線性記憶體配在 %08X，落在 1 MB 以下的實模式空間裡", linear)
	}

	c.R[cpu386.EBX] = 0x10
	dpmiCall(h, c, 0x0100)
	if c.EFlags&cpu386.CF != 0 {
		t.Fatal("配 DOS 記憶體失敗")
	}
	dosBase := uint32(uint16(c.R[cpu386.EAX])) * 16
	if dosBase >= linear && dosBase < linear+0x8000 {
		t.Errorf("DOS 記憶體 %08X 落在線性區塊 %08X 裡面", dosBase, linear)
	}
}

// 配不出來的時候要回報**還剩多少**（BX），不能只說失敗。
//
// 不回報的話程式無從決定要降級到多小，多半就直接放棄。
func TestDPMIDOSMemoryFailureReportsLargestAvailable(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	c.R[cpu386.EBX] = 0xFFFF // 要 1 MB
	c.R[cpu386.ECX] = 0
	dpmiCall(h, c, 0x0100)
	if c.EFlags&cpu386.CF == 0 {
		t.Fatal("配 1 MB DOS 記憶體竟然成功")
	}
	if got := uint16(c.R[cpu386.EAX]); got != 0x8013 {
		t.Errorf("錯誤碼 %04X，預期 8013", got)
	}
	if uint16(c.R[cpu386.EBX]) == 0xFFFF {
		t.Error("BX 沒有被改成「還剩多少段」")
	}
	// 剩下的數量要真的配得出來。
	left := uint16(c.R[cpu386.EBX])
	if left == 0 {
		t.Fatal("回報還剩 0 段——但這台機器什麼都還沒配")
	}
	c.R[cpu386.EBX] = uint32(left)
	dpmiCall(h, c, 0x0100)
	if c.EFlags&cpu386.CF != 0 {
		t.Errorf("照回報的 %d 段配還是失敗——那個數字是假的", left)
	}
}

// 釋放之後 selector 就不認得了，而且位址不重用。
func TestDPMIDOSMemoryFreeInvalidatesSelector(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	c.R[cpu386.EBX] = 2
	dpmiCall(h, c, 0x0100)
	first := uint16(c.R[cpu386.EDX])
	firstSeg := uint16(c.R[cpu386.EAX])

	c.R[cpu386.EDX] = uint32(first)
	dpmiCall(h, c, 0x0101)
	if c.EFlags&cpu386.CF != 0 {
		t.Fatal("釋放失敗")
	}
	if _, ok := c.Descriptors[first]; ok {
		t.Error("釋放之後描述子還在——舊 selector 仍然寫得進去")
	}
	c.R[cpu386.EDX] = uint32(first)
	dpmiCall(h, c, 0x0101)
	if c.EFlags&cpu386.CF == 0 {
		t.Error("釋放同一個 selector 兩次竟然成功")
	}

	c.R[cpu386.EBX] = 2
	dpmiCall(h, c, 0x0100)
	if uint16(c.R[cpu386.EAX]) == firstSeg {
		t.Error("釋放之後配到同一個段——誰還留著舊指標就查不出來了")
	}
}

// `AX=0102h`：縮小一定成立、最後一塊可以原地長大、
// **中間那一塊長大要失敗**（搬家會讓程式手上的段號失效）。
func TestDPMIDOSMemoryResizeRules(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	alloc := func(paras uint16) uint16 {
		c.R[cpu386.EBX] = uint32(paras)
		dpmiCall(h, c, 0x0100)
		if c.EFlags&cpu386.CF != 0 {
			t.Fatalf("配 %d 段失敗", paras)
		}
		return uint16(c.R[cpu386.EDX])
	}
	middle := alloc(4)
	last := alloc(4)

	// 縮小。
	c.R[cpu386.EDX], c.R[cpu386.EBX] = uint32(middle), 2
	dpmiCall(h, c, 0x0102)
	if c.EFlags&cpu386.CF != 0 {
		t.Errorf("縮小失敗：AX=%04X", uint16(c.R[cpu386.EAX]))
	}
	if d := c.Descriptors[middle]; d.Limit != 2*16-1 {
		t.Errorf("縮小之後 limit 是 %X，預期 %X", d.Limit, 2*16-1)
	}

	// 最後一塊原地長大。
	c.R[cpu386.EDX], c.R[cpu386.EBX] = uint32(last), 8
	dpmiCall(h, c, 0x0102)
	if c.EFlags&cpu386.CF != 0 {
		t.Errorf("最後一塊長大失敗：AX=%04X", uint16(c.R[cpu386.EAX]))
	}
	if d := c.Descriptors[last]; d.Limit != 8*16-1 {
		t.Errorf("長大之後 limit 是 %X，預期 %X", d.Limit, 8*16-1)
	}

	// 中間那一塊長大要失敗，而且要說還剩多少。
	c.R[cpu386.EDX], c.R[cpu386.EBX] = uint32(middle), 16
	dpmiCall(h, c, 0x0102)
	if c.EFlags&cpu386.CF == 0 {
		t.Fatal("中間那一塊長大竟然成功——它的段號會失效")
	}
	if got := uint16(c.R[cpu386.EAX]); got != 0x8013 {
		t.Errorf("錯誤碼 %04X，預期 8013", got)
	}
	if uint16(c.R[cpu386.EBX]) == 16 {
		t.Error("BX 沒有被改成「還剩多少段」")
	}
}

// 不存在的 selector 要被拒絕。
func TestDPMIDOSMemoryRejectsUnknownSelector(t *testing.T) {
	m, h := newDPMITest(t)
	c := m.CPU
	for _, fn := range []uint16{0x0101, 0x0102} {
		c.R[cpu386.EDX], c.R[cpu386.EBX] = 0xF000, 1
		c.EFlags &^= cpu386.CF
		dpmiCall(h, c, fn)
		if c.EFlags&cpu386.CF == 0 {
			t.Errorf("AX=%04X 對不存在的 selector 竟然成功", fn)
			continue
		}
		if got := uint16(c.R[cpu386.EAX]); got != 0x8022 {
			t.Errorf("AX=%04X 回錯誤碼 %04X，預期 8022", fn, got)
		}
	}
}

package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// DOS 記憶體服務（`AH=48h`／`49h`／`4Ah`）與 MCB 鏈的獨立契約測試。
//
// 期望值照 DOS 的規格自己列，不引用分支的既有測試：
//
//   - 配置回的是**資料段**，它前一格是那塊的 MCB。
//   - 配不出來要回 CF=1、AX=8，而且 **BX 要回最大的一塊**——
//     呼叫端的慣用法是「先要 FFFFh 探出上限，再用 BX 要一次」。
//   - 釋放要真的還回去，而且相鄰的自由區塊要合併：不合併的話
//     「配了又放」幾千次之後空間被切成碎片，一樣要不到大塊，
//     而症狀與「記憶體真的不夠」一模一樣。
//   - MCB 鏈要與配置器的狀態一致：程式會自己走鏈算「我構得到多少段」。

// allocPara 配 n 段，回資料段（失敗回 0）。
func allocPara(m *machine.Machine, d *DOS, n uint16) uint16 {
	m.CPU.R[cpu.BX] = n
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		return 0
	}
	return m.CPU.R[cpu.AX]
}

func freePara(m *machine.Machine, d *DOS, seg uint16) bool {
	m.CPU.Seg[cpu.ES] = seg
	call(m, d, 0x21, 0x4900)
	return m.CPU.Flags&cpu.CF == 0
}

// 配出來的區塊不能互相重疊，也不能落在已經有人住的地方。
//
// **配置器把有人住的段配出去時，症狀是程式碼被自己寫壞**——
// 那看起來像模擬器把記憶體寫爛了，而不是像配置器的問題。
func TestAllocationsDoNotOverlap(t *testing.T) {
	m, d := newTest(t)
	const n = 0x40
	var segs []uint16
	for i := 0; i < 8; i++ {
		s := allocPara(m, d, n)
		if s == 0 {
			t.Fatalf("第 %d 次配置就失敗了", i)
		}
		segs = append(segs, s)
	}
	for i := range segs {
		for j := i + 1; j < len(segs); j++ {
			lo, hi := segs[i], segs[j]
			if lo > hi {
				lo, hi = hi, lo
			}
			if hi < lo+n {
				t.Fatalf("第 %d 塊（%04X）與第 %d 塊（%04X）重疊，每塊 %Xh 段",
					i, segs[i], j, segs[j], n)
			}
		}
	}
}

// 配不出來的三件事：CF=1、AX=8、**BX ＝ 最大的一塊**。
//
// BX 不填的話呼叫端的「再要一次」會拿它自己傳進去的值再問一次，
// 於是無限重試——外面看起來就是程式卡住不動。
func TestAllocFailureReportsLargestBlock(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 0xFFFF
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("要 FFFFh 段竟然成功")
	}
	if m.CPU.R[cpu.AX] != 8 {
		t.Errorf("錯誤碼是 %d，預期 8（記憶體不足）", m.CPU.R[cpu.AX])
	}
	avail := m.CPU.R[cpu.BX]
	if avail == 0 || avail == 0xFFFF {
		t.Fatalf("BX 回 %04X，要回實際可用的段數", avail)
	}
	// 拿 DOS 自己回的數字再要一次一定要成功。
	if seg := allocPara(m, d, avail); seg == 0 {
		t.Fatalf("拿 DOS 回的 %04X 再要一次還是失敗", avail)
	}
}

// 釋放要真的還回去，相鄰的自由區塊要合併。
//
// 判準是「放掉三塊相鄰的之後，要得回一塊比單塊大的」——
// 不合併的實作在這裡會失敗，而它在「配了就用到結束」的程式上完全正常。
func TestFreeCoalescesAdjacentBlocks(t *testing.T) {
	m, d := newTest(t)
	const n = 0x100
	a := allocPara(m, d, n)
	b := allocPara(m, d, n)
	c := allocPara(m, d, n)
	if a == 0 || b == 0 || c == 0 {
		t.Fatal("配置失敗")
	}
	for _, s := range []uint16{a, b, c} {
		if !freePara(m, d, s) {
			t.Fatalf("釋放 %04X 失敗", s)
		}
	}
	// 三塊各 n 段、中間兩格 MCB：合併之後至少放得下 3n。
	if got := allocPara(m, d, 3*n); got == 0 {
		t.Fatal("三塊相鄰的自由區塊沒有合併——碎片化之後一樣要不到大塊")
	}
}

// 釋放不認識的區塊要**照實回錯**（AX=9，MCB 位址無效）。
//
// 悄悄成功會把「釋放了不屬於自己的東西」藏起來，而那是真 DOS 會抓的錯。
func TestFreeUnknownBlockFailsLoudly(t *testing.T) {
	m, d := newTest(t)
	if freePara(m, d, 0x9000) {
		t.Fatal("釋放沒配過的段竟然成功")
	}
	if m.CPU.R[cpu.AX] != 9 {
		t.Errorf("錯誤碼是 %d，預期 9", m.CPU.R[cpu.AX])
	}
}

// `AH=4Ah` 縮小之後，讓出來的空間要能再配出去。
//
// 只報成功而不還空間的話，「先配光再縮回自己要的大小」這個慣用法會
// 在下一次配置時失敗——而程式已經把記憶體算進自己的計畫裡了。
func TestShrinkReturnsTheSpace(t *testing.T) {
	m, d := newTest(t)
	big := allocPara(m, d, 0x400)
	if big == 0 {
		t.Fatal("配不到 400h 段")
	}
	before := allocPara(m, d, 0x100)
	if before == 0 {
		t.Fatal("配不到 100h 段")
	}
	if !freePara(m, d, before) {
		t.Fatal("釋放失敗")
	}

	m.CPU.Seg[cpu.ES] = big
	m.CPU.R[cpu.BX] = 0x10
	call(m, d, 0x21, 0x4A00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("縮小失敗：AX=%04X", m.CPU.R[cpu.AX])
	}
	// 縮小讓出 3F0h 段，現在要 300h 應該還在那一塊裡拿得到。
	if got := allocPara(m, d, 0x300); got == 0 {
		t.Fatal("縮小之後的空間沒有回到可配置區")
	}
}

// MCB 鏈要走得通，而且與配置器一致：每一格的 `M`／`Z` 簽章、
// 大小欄加起來要接得上下一格，最後一格是 `Z`。
//
// **程式會自己走鏈**（從自己的 PSP 往上加，算「我總共構得到多少段」），
// 鏈斷掉或大小對不上的話它算出來的數字與事實無關——而它不會因此報錯，
// 只是拿錯的數字去做決定，然後在很遠的地方失敗。
func TestMCBChainIsWalkable(t *testing.T) {
	m, d := newTest(t)
	if seg := allocPara(m, d, 0x80); seg == 0 {
		t.Fatal("配置失敗")
	}
	seg := uint16(machine.PSPSeg - 1)
	for i := 0; ; i++ {
		if i > 64 {
			t.Fatal("鏈走不完——大小欄接不上下一格")
		}
		sig := m.Read8(uint32(seg) * 16)
		size := m.Read16(uint32(seg)*16 + 3)
		switch sig {
		case 'Z':
			if int(seg)+1+int(size) > machine.MemTop+1 {
				t.Errorf("鏈尾 %04X ＋ %04X 段超出 MemTop", seg, size)
			}
			return
		case 'M':
			// 繼續往下一格。
		default:
			t.Fatalf("第 %d 格（%04X）的簽章是 %02X，只能是 4Dh('M') 或 5Ah('Z')",
				i, seg, sig)
		}
		next := seg + 1 + size
		if next <= seg {
			t.Fatalf("第 %d 格（%04X）的大小是 %04X，鏈不會前進", i, seg, size)
		}
		seg = next
	}
}

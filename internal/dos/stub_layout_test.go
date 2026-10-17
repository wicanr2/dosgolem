package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestFontStubsLeaveVectorStubsIntact 釘住 `docs/spec/194-stubseg-font-collision-and-program-path` §1：
// 字型 stub 蓋掉向量 9 的 `CD 09 CF` 的話，程式 chain 回舊 INT 9 時跑到 `retf`，
// 每按一個鍵堆疊少 2 byte，幾百萬道指令之後才在某個 `retf` 跳進垃圾。
//
// 只在鍵盤攔截（`docs/spec/447`）關閉時成立：開啟時向量 9 的 stub 刻意改成 `E9 91 04`（447 §2.2 的唯一例外），
// 由 kbd_intercept_test.go 驗。
func TestFontStubsLeaveVectorStubsIntact(t *testing.T) {
	m, _ := newTest(t)
	base := uint32(machine.StubSeg) * 16
	for _, v := range []uint8{0x08, 0x09} {
		at := base + uint32(machine.StubOff(v))
		if got := [3]uint8{m.Mem[at], m.Mem[at+1], m.Mem[at+2]}; got != [3]uint8{0xCD, v, 0xCF} {
			t.Fatalf("向量 %02Xh 的預設 stub 是 % X，應為 CD %02X CF", v, got, v)
		}
	}
	for _, off := range []uint16{fontFullOff, fontHalfOff} {
		if off < 0x400 {
			t.Fatalf("字型 stub 位移 %03Xh 落在向量 stub 區 0x000–0x3FF", off)
		}
	}
}

// per-vector stub 陣列（StubSeg 的 0x000–0x3FF）不准被任何服務蓋到。
//
// **那 256 格不是備用空間，是「舊向量」。** 程式掛自己的處理常式之前會先
// 存下向量現在的值，做完事再 chain 回去；chain 回去的目標就是這裡。
// 被蓋掉的話程式自己那一段跑得完全正常，回來才踩到別人的位元組——
// 而 `IRET` 換成 `RETF` 只差兩個位元組，症狀是幾萬道指令之後莫名其妙
// 停在某個等待迴圈裡（`docs/re/eob1-dosgolem-handoff-255.md` 第四關）。
//
// 服務型資料表要放 StubSeg 的話，位移向 `machine` 那份佈局圖要，
// 不要自己挑數字。
//
// 只在鍵盤攔截（`docs/spec/447`）關閉時成立：開啟時向量 9 的 stub 改成 `jmp near` 到 BIOS int 09h 的客體碼，
// 被蓋的仍是向量 9 自己的預設行為（447 §2.2 的唯一例外）。
func TestPerVectorStubsAreNeverOverwritten(t *testing.T) {
	m, d := newTest(t)

	check := func(when string) {
		t.Helper()
		for v := 0; v < 256; v++ {
			at := uint32(machine.StubSeg)*16 + uint32(machine.StubOff(uint8(v)))
			got := [3]byte{m.Read8(at), m.Read8(at + 1), m.Read8(at + 2)}
			want := [3]byte{0xCD, byte(v), 0xCF}
			if got != want {
				t.Fatalf("%s：向量 %02Xh 的 stub 是 % X，預期 % X（位移 %03X 被別的東西佔走）",
					when, v, got[:], want[:], machine.StubOff(uint8(v)))
			}
		}
	}
	check("初始化後")

	// 會往 StubSeg 寫資料表的服務全叫一遍。
	call(m, d, 0x15, 0xC000) // 系統設定表
	check("int 15h AH=C0h 之後")
	call(m, d, 0x21, 0x3800) // 國別資訊
	check("int 21h AH=38h 之後")
	call(m, d, 0x21, 0x6300) // DBCS 表
	check("int 21h AH=63h 之後")
	call(m, d, 0x15, 0x5000) // DOS/V 字型 entry
	check("int 15h AX=5000h 之後")
}

// 字型 entry 是 far call 的目標，結尾必須是 `RETF`；
// 而向量 stub 是中斷的目標，結尾必須是 `IRET`。兩者不能共用一格。
func TestFontStubsEndWithFarReturn(t *testing.T) {
	m, _ := newTest(t)
	for _, tc := range []struct {
		name string
		off  uint16
	}{{"全形", machine.FontFullOff}, {"半形", machine.FontHalfOff}} {
		at := uint32(machine.StubSeg)*16 + uint32(tc.off)
		if got := m.Read8(at); got != 0xCD {
			t.Fatalf("%s字型 entry 的第一個位元組是 %02X，預期 CD", tc.name, got)
		}
		if got := m.Read8(at + 2); got != 0xCB {
			t.Fatalf("%s字型 entry 以 %02X 結尾，預期 CB（RETF）", tc.name, got)
		}
		if tc.off < 0x400 {
			t.Fatalf("%s字型 entry 落在位移 %03X，壓到 per-vector stub 陣列", tc.name, tc.off)
		}
	}
}

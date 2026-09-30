package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestFontStubsLeaveVectorStubsIntact 釘住 `docs/spec/194-stubseg-font-collision-and-program-path` §1：
// 字型 stub 蓋掉向量 9 的 `CD 09 CF` 的話，程式 chain 回舊 INT 9 時跑到 `retf`，
// 每按一個鍵堆疊少 2 byte，幾百萬道指令之後才在某個 `retf` 跳進垃圾。
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

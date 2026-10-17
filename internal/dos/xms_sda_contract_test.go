package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// XMS move 的驗證（`docs/spec/436-xms-move-validation`）與 DOS 可交換資料區
// （`docs/spec/437-swappable-data-area`）的契約測試。

// xmsMoveDesc 在 2000:0000 寫一個 AH=0Bh 描述子後呼叫，回 AX 與 BL。
func xmsMoveDesc(m *machine.Machine, d *DOS, n uint32, srcH uint16, srcOff uint32, dstH uint16, dstOff uint32) (uint16, uint8) {
	desc := cpu.Addr(0x2000, 0)
	m.Write16(desc, uint16(n))
	m.Write16(desc+2, uint16(n>>16))
	m.Write16(desc+4, srcH)
	m.Write16(desc+6, uint16(srcOff))
	m.Write16(desc+8, uint16(srcOff>>16))
	m.Write16(desc+0x0A, dstH)
	m.Write16(desc+0x0C, uint16(dstOff))
	m.Write16(desc+0x0E, uint16(dstOff>>16))
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0x2000, 0
	ax := xmsCallAH(m, d, 0x0B00)
	return ax, uint8(m.CPU.R[cpu.BX])
}

func TestXMSMoveRejectsInvalidRequests(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.DX] = 1 // 1 KB
	xmsCallAH(m, d, 0x0900)
	h := m.CPU.R[cpu.DX]
	m.Write8(0x7000, 0xC3) // 常規記憶體的哨兵：失敗的搬移不准動它

	conv := uint32(0x0700 << 16) // 0700:0000 ＝ 線性 7000h
	cases := []struct {
		name   string
		n      uint32
		srcH   uint16
		srcOff uint32
		dstH   uint16
		dstOff uint32
		bl     uint8
	}{
		{"長度奇數", 3, h, 0, 0, conv, 0xA7},
		{"來源 handle 不存在", 2, 0xFFFF, 0, 0, conv, 0xA3},
		{"來源位移超出", 2, h, 1024, 0, conv, 0xA4},
		{"來源長度超出", 4, h, 1022, 0, conv, 0xA7},
		{"目的 handle 不存在", 2, 0, conv, h + 9, 0, 0xA5},
		{"目的位移超出", 2, 0, conv, h, 2048, 0xA6},
		{"常規端超過 10FFF0h", 0x20, 0, 0xFFFF<<16 | 0xFFF0, h, 0, 0xA7},
	}
	for _, tc := range cases {
		ax, bl := xmsMoveDesc(m, d, tc.n, tc.srcH, tc.srcOff, tc.dstH, tc.dstOff)
		if ax != 0 || bl != tc.bl {
			t.Errorf("%s：AX=%d BL=%02X，預期 AX=0 BL=%02X", tc.name, ax, bl, tc.bl)
		}
		if got := m.Read8(0x7000); got != 0xC3 {
			t.Fatalf("%s：失敗的搬移改了常規記憶體（%02X）", tc.name, got)
		}
	}
	// 合法的搬移照常成功。
	if ax, _ := xmsMoveDesc(m, d, 2, h, 0, 0, conv); ax != 1 {
		t.Error("合法搬移失敗")
	}
}

func TestSwappableDataAreaMatchesInDOS(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x5D06)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("AX=5D06h 設了 CF")
	}
	sdaSeg, sdaOff := m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI]
	if cx, dx := m.CPU.R[cpu.CX], m.CPU.R[cpu.DX]; cx < 0x1A || dx != 0x1A {
		t.Errorf("CX=%04X DX=%04X，預期 DX=1Ah、CX 至少 1Ah", cx, dx)
	}
	sda := uint32(sdaSeg)*16 + uint32(sdaOff)
	if w := m.Read16(sda); w != 0 {
		t.Errorf("SDA+0 的 word（critical error 與 InDOS）= %04X，預期 0", w)
	}
	if got := m.Read8(sda + 2); got != 0xFF {
		t.Errorf("SDA+2（critical error 磁碟機）= %02X，預期 FFh", got)
	}
	if psp := m.Read16(sda + 0x10); psp != d.blockOwner() {
		t.Errorf("SDA+10h 目前 PSP = %04X，預期 %04X", psp, d.blockOwner())
	}
	// AH=34h 的 InDOS 必須就是 SDA+1。
	call(m, d, 0x21, 0x3400)
	if got := uint32(m.CPU.Seg[cpu.ES])*16 + uint32(m.CPU.R[cpu.BX]); got != sda+1 {
		t.Errorf("AH=34h 指到 %05X，預期 SDA+1 = %05X", got, sda+1)
	}
	// SDA 不准蓋到 StubSeg 的 DBCS 表與環境區塊的 MCB。
	end := uint32(machine.SDAOff) + machine.SDASize
	if machine.SDAOff < machine.CountryOff+34 || end > 0x4F0 {
		t.Errorf("SDA 範圍 %03X–%03X 撞到鄰居", machine.SDAOff, end)
	}
}

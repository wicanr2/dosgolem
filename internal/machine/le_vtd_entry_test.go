package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func TestMOO2AbsentVTDEntryPreservesState(t *testing.T) {
	// 只模擬未安裝裝置的查詢；所有高半部與架構旗標均須保持。
	masks := []uint32{cpu386.CF, cpu386.PF, cpu386.AF, cpu386.ZF, cpu386.SF, cpu386.OF}
	for _, high := range []uint32{0, 1, 0xa5a5, 0xffff} {
		for pattern := 0; pattern < 64; pattern++ {
			mem := []byte{0xcd, 0x2f, 0xaa, 0x55}
			c := cpu386.New(startupBus(mem))
			c.R = [8]uint32{0xf1684 | high<<16, 0x36df20, 1, 5 | high<<16, 0x3ebad0, 0, 0x3ebb20, high << 16}
			c.Seg = [6]uint16{cpu386.SegCS: 0x180, cpu386.SegDS: 0x188, cpu386.SegES: 0, cpu386.SegGS: 0x20, cpu386.SegSS: 0x188}
			c.EIP, c.EFlags = 0x36da47, 0x602
			for bit, mask := range masks {
				if pattern&(1<<bit) != 0 {
					c.EFlags |= mask
				}
			}
			r, seg, ip, flags := c.R, c.Seg, c.EIP, c.EFlags
			before := append([]byte(nil), mem...)
			s := NewMOO2StartupDOS(nil)
			for repeat := 0; repeat < 2; repeat++ {
				if !s.Handle(c, 0x2f) || c.R != r || c.Seg != seg || c.EIP != ip || c.EFlags != flags || !bytes.Equal(mem, before) || s.Calls() != 0 {
					t.Fatalf("未安裝 VTD 查詢改變完整狀態：high=%X flags=%X", high, flags)
				}
			}
		}
	}
}

func TestMOO2AbsentVTDEntryRejectsUnknownInputs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ax, bx, di uint32
		es         uint16
		fd2        bool
	}{
		{name: "其他設定", ax: 0x1684, bx: 5, fd2: true},
		{name: "非零 ES", ax: 0x1684, bx: 5, es: 0x188},
		{name: "非零 DI", ax: 0x1684, bx: 5, di: 1},
		{name: "DI 上界", ax: 0x1684, bx: 5, di: 0xffff},
		{name: "其他裝置", ax: 0x1684, bx: 4},
		{name: "其他裝置上界", ax: 0x1684, bx: 0xffff},
		{name: "名稱查詢", ax: 0x1684, bx: 0},
		{name: "其他功能", ax: 0x1683, bx: 5},
		{name: "未知功能", ax: 0xffff, bx: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := []byte{0xaa, 0x55}
			c := cpu386.New(startupBus(mem))
			c.R = [8]uint32{tc.ax, 0x1234, 0x5678, tc.bx, 0x9abc, 0xdef0, 0x1111, tc.di}
			c.Seg = [6]uint16{cpu386.SegCS: 0x180, cpu386.SegDS: 0x188, cpu386.SegES: tc.es, cpu386.SegSS: 0x188}
			c.EIP, c.EFlags = 0x36da47, 0x246
			r, seg, ip, flags := c.R, c.Seg, c.EIP, c.EFlags
			before := append([]byte(nil), mem...)
			s := NewMOO2StartupDOS(nil).FD2StartupDOS
			if tc.fd2 {
				s = NewFD2StartupDOS(nil)
			}
			if s.Handle(c, 0x2f) || c.R != r || c.Seg != seg || c.EIP != ip || c.EFlags != flags || !bytes.Equal(mem, before) || s.Calls() != 0 {
				t.Fatal("未知裝置／形狀未保持拒絕邊界")
			}
		})
	}
}

func TestMOO2AbsentVTDEntryOriginalConsumer(t *testing.T) {
	// 原版只重定位保存目的；不以此最小樣本取代自然啟動路徑。
	mem := bytes.Repeat([]byte{0xa5}, 1024)
	code := []byte{0xcd, 0x2f, 0x66, 0x89, 0x3d, 0x00, 0x02, 0x00, 0x00, 0x66, 0xc7, 0x05, 0x02, 0x02, 0x00, 0x00, 0x00, 0x00}
	copy(mem, code)
	c := cpu386.New(startupBus(mem))
	c.SetDescriptor(0x188, cpu386.Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	c.R = [8]uint32{0xf1684, 0x36df20, 1, 5, 0x3ebad0, 0, 0x3ebb20, 0x380000}
	c.Seg = [6]uint16{cpu386.SegCS: 0x180, cpu386.SegDS: 0x188, cpu386.SegES: 0, cpu386.SegGS: 0x20, cpu386.SegSS: 0x188}
	c.EFlags = 0x246
	s := NewMOO2StartupDOS(nil)
	c.IntHook = s.Handle
	r, seg, flags := c.R, c.Seg, c.EFlags
	for step, ip := range []uint32{2, 9, 18} {
		if err := c.Step(); err != nil {
			t.Fatalf("第 %d 個原版指令失敗：%v", step, err)
		}
		if c.EIP != ip || c.R != r || c.Seg != seg || c.EFlags != flags || s.Calls() != 0 {
			t.Fatalf("第 %d 個原版指令改變預期外狀態", step)
		}
	}
	expected := bytes.Repeat([]byte{0xa5}, 1024)
	copy(expected, code)
	copy(expected[512:516], []byte{0, 0, 0, 0})
	if !bytes.Equal(mem, expected) {
		t.Fatal("空入口未保存為兩個 word，或破壞相鄰記憶體")
	}
}

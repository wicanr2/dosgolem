package machine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func newMOO2VBETest(t *testing.T, paras uint32) (*LEMachine, *MOO2StartupDOS, uint16, uint32) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x120000), DOSArenaBase: 0x10000, Ports: map[uint16]uint8{}}
	m.CPU = cpu386.New(m)
	m.CPU.SetDescriptor(0x10, cpu386.Descriptor{Limit: 0xffffffff, Writable: true})
	m.CPU.Seg[cpu386.SegES] = 0x10
	s := NewMOO2StartupDOS(nil)
	s.AttachMachine(m)
	m.CPU.R[cpu386.EBX] = paras
	dpmiCall(s.DPMI, m.CPU, 0x0100)
	if m.CPU.EFlags&cpu386.CF != 0 {
		t.Fatalf("測試 DOS 區塊配置失敗：AX=%X", m.CPU.R[cpu386.EAX])
	}
	return m, s, uint16(m.CPU.R[cpu386.EAX]), 0x110000
}

func callMOO2VBE(m *LEMachine, s *MOO2StartupDOS, segment uint16, ax uint16, cx uint16, packetBase uint32) bool {
	packet := m.Mem[packetBase : packetBase+50]
	clear(packet)
	binary.LittleEndian.PutUint16(packet[28:], ax)
	binary.LittleEndian.PutUint16(packet[24:], cx)
	binary.LittleEndian.PutUint16(packet[34:], segment)
	m.CPU.R[cpu386.EBX] = 0x10
	m.CPU.R[cpu386.ECX] = 0
	m.CPU.R[cpu386.EDI] = packetBase
	return dpmiCall(s.DPMI, m.CPU, 0x0300)
}

func TestMOO2VBEControllerInfoReturnsReadableDOSBlock(t *testing.T) {
	m, s, segment, packetBase := newMOO2VBETest(t, 513)
	if !callMOO2VBE(m, s, segment, 0x4f00, 0, packetBase) || m.CPU.EFlags&cpu386.CF != 0 {
		t.Fatalf("受限 VBE 4F00h 失敗：trace=%+v", s.DPMI.RealModeLast)
	}
	packet := m.Mem[packetBase : packetBase+50]
	if got := binary.LittleEndian.Uint16(packet[28:]); got != 0x004f {
		t.Fatalf("實模式 AX=%04X，預期 004Fh", got)
	}
	if got := binary.LittleEndian.Uint16(packet[32:]); got != 2 {
		t.Fatalf("實模式 flags=%04X，預期 0002h", got)
	}
	if uint16(m.CPU.R[cpu386.EAX]) != 0x0300 || s.DPMI.RealModeLast == nil || !s.DPMI.RealModeLast.Returned {
		t.Fatalf("外層 DPMI 狀態錯誤：EAX=%X trace=%+v", m.CPU.R[cpu386.EAX], s.DPMI.RealModeLast)
	}
	base := uint32(segment) * 16
	header := m.Mem[base : base+20]
	if !bytes.Equal(header[:4], []byte("VESA")) ||
		binary.LittleEndian.Uint16(header[4:]) != 0x0200 ||
		binary.LittleEndian.Uint32(header[6:]) != 0xc000016c ||
		binary.LittleEndian.Uint32(header[10:]) != 1 ||
		binary.LittleEndian.Uint32(header[14:]) != 0xc0000100 ||
		binary.LittleEndian.Uint16(header[18:]) != 32 {
		t.Fatalf("VBE 控制器資訊頭錯誤：% X", header)
	}
	if !bytes.Equal(m.Mem[base+20:base+256], make([]byte, 236)) {
		t.Fatal("首筆 VBE 呼叫覆蓋未觀測的延伸欄位")
	}
	if binary.LittleEndian.Uint16(m.Mem[moo2VBEModeList+53*2:]) != 0xffff ||
		!bytes.HasPrefix(m.Mem[moo2VBEOEMName:], []byte("S3 Incorporated. Trio64\x00")) {
		t.Fatal("VBE 遠指標沒有可讀的模式清單或 OEM 字串")
	}
	selector := s.DPMI.DOSMemory()[0].Selector
	if value, ok := m.CPU.ReadSegment8(selector, 0); !ok || value != 'V' {
		t.Fatal("保護模式 selector 讀不到實模式 VBE 輸出")
	}
}

func TestMOO2VBEMode0101MatchesOriginalBuffer(t *testing.T) {
	m, s, segment, packetBase := newMOO2VBETest(t, 513)
	if !callMOO2VBE(m, s, segment, 0x4f00, 0, packetBase) {
		t.Fatal("控制器資訊前置呼叫失敗")
	}
	base := uint32(segment) * 16
	clear(m.Mem[base : base+256])
	if !callMOO2VBE(m, s, segment, 0x4f01, 0x0101, packetBase) || m.CPU.EFlags&cpu386.CF != 0 {
		t.Fatalf("模式 0101h 查詢失敗：%+v", s.DPMI.RealModeLast)
	}
	want, err := hex.DecodeString("d0eafd13c290b998a4d0d3be332d9be06eabde2a19764f88ab68d483e64e35c3")
	if err != nil {
		t.Fatal(err)
	}
	got := sha256.Sum256(m.Mem[base : base+256])
	if !bytes.Equal(got[:], want) {
		t.Fatalf("原版模式資訊緩衝雜湊不符：%x", got)
	}
	packet := m.Mem[packetBase : packetBase+50]
	if binary.LittleEndian.Uint16(packet[28:]) != 0x004f ||
		binary.LittleEndian.Uint16(packet[32:]) != 2 ||
		binary.LittleEndian.Uint16(packet[24:]) != 0x0101 ||
		s.DPMI.RealModeLast.Entry != "MOO2 VBE 4F01 平台服務" {
		t.Fatalf("模式資訊回傳封包錯誤：% X；trace=%+v", packet, s.DPMI.RealModeLast)
	}
	for i := 44; i < 256; i++ {
		m.Mem[base+uint32(i)] = 0xa5
	}
	if !callMOO2VBE(m, s, segment, 0x4f01, 0x0101, packetBase) {
		t.Fatal("尾端保留樣本查詢失敗")
	}
	for i := 44; i < 256; i++ {
		if m.Mem[base+uint32(i)] != 0xa5 {
			t.Fatalf("未觀測尾端位元組 %d 被覆蓋", i)
		}
	}
}

func TestMOO2VBERejectsUnknownOrUnallocatedBuffers(t *testing.T) {
	for name, tc := range map[string]struct {
		paras        uint32
		ax           uint16
		cx           uint16
		segmentDelta uint16
	}{
		"small-block":       {8, 0x4f00, 0, 0},
		"small-mode-block":  {8, 0x4f01, 0x0101, 0},
		"unallocated-range": {513, 0x4f00, 0, 0x200},
		"unknown-function":  {513, 0x4f02, 0, 0},
		"unknown-mode":      {513, 0x4f01, 0x0102, 0},
	} {
		t.Run(name, func(t *testing.T) {
			m, s, segment, packetBase := newMOO2VBETest(t, tc.paras)
			if callMOO2VBE(m, s, segment+tc.segmentDelta, tc.ax, tc.cx, packetBase) {
				t.Fatal("未知或超界 VBE 呼叫竟然成功")
			}
			if s.DPMI.Unimplemented[0x0300] != 1 || m.Mem[uint32(segment)*16] != 0 || m.Mem[moo2VBEModeList] != 0 {
				t.Fatalf("失敗路徑寫入資料或未記錄缺口：trace=%+v", s.DPMI.RealModeLast)
			}
		})
	}
}

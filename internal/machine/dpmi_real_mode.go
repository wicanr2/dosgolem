package machine

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

type DPMIRealModeTrace struct {
	Interrupt    uint8
	Entry        string
	Stop         string
	Steps        int
	Returned     bool
	Error        string
	InputPacket  string
	OutputPacket string
}

type RealModePortIO interface {
	In8(uint16) (uint8, bool)
	Out8(uint16, uint8) bool
}

type dpmiRealBus struct {
	io  RealModePortIO
	m   *LEMachine
	err error
}

func (b *dpmiRealBus) Read8(a uint32) uint8 {
	if uint64(a) >= uint64(len(b.m.Mem)) {
		if b.err == nil {
			b.err = fmt.Errorf("實模式讀取越界 %05X", a)
		}
		return 0
	}
	return b.m.Mem[a]
}
func (b *dpmiRealBus) Write8(a uint32, v uint8) {
	if uint64(a) >= uint64(len(b.m.Mem)) {
		if b.err == nil {
			b.err = fmt.Errorf("實模式寫入越界 %05X", a)
		}
		return
	}
	if b.err == nil {
		b.m.Mem[a] = v
	}
}
func (b *dpmiRealBus) In8(port uint16) uint8 {
	if b.io != nil {
		if v, ok := b.io.In8(port); ok {
			return v
		}
	}
	if b.err == nil {
		b.err = fmt.Errorf("實模式 IN 埠 %04X 尚未支援", port)
	}
	return 0
}
func (b *dpmiRealBus) Out8(port uint16, v uint8) {
	if b.io != nil && b.io.Out8(port, v) {
		return
	}
	if b.err == nil {
		b.err = fmt.Errorf("實模式 OUT 埠 %04X 值 %02X 尚未支援", port, v)
	}
}

// 僅支援規格186批次19列出的子集；不以固定回傳取代handler。
func (h *DPMIHost) simulateRealModeInterrupt(c *cpu386.CPU) error {
	tr := &DPMIRealModeTrace{Interrupt: uint8(c.R[cpu386.EBX])}
	h.RealModeLast = tr
	if len(h.RealModeHistory) < 32 {
		h.RealModeHistory = append(h.RealModeHistory, tr)
	}
	if uint16(c.R[cpu386.ECX]) != 0 || c.R[cpu386.EBX]&0xff00 != 0 {
		return fmt.Errorf("DPMI0300目前僅支援CX0／BH0")
	}
	var packet [50]byte
	for i := range packet {
		v, ok := c.ReadSegment8(c.Seg[cpu386.SegES], c.R[cpu386.EDI]+uint32(i))
		if !ok {
			return fmt.Errorf("DPMI0300封包不可讀")
		}
		packet[i] = v
	}
	tr.InputPacket = fmt.Sprintf("% X", packet[:])
	// 先驗证完整輸出descriptor，避免執行後才發現目的區唯讀。
	d, ok := c.Descriptors[c.Seg[cpu386.SegES]]
	end := uint64(c.R[cpu386.EDI]) + 50
	if !ok || !d.Writable || end > uint64(d.Limit)+1 || uint64(d.Base)+end > uint64(len(h.m.Mem)) {
		return fmt.Errorf("DPMI0300封包不可寫")
	}
	word := func(i int) uint16 { return binary.LittleEndian.Uint16(packet[i:]) }
	if word(38) != 0 || word(40) != 0 {
		return fmt.Errorf("DPMI0300 FS／GS尚未支援")
	}
	seg, off := h.RealModeVector(tr.Interrupt)
	tr.Entry = fmt.Sprintf("%04X:%04X", seg, off)
	if seg == 0 && off == 0 {
		return fmt.Errorf("DPMI0300中斷%02X未註冊", tr.Interrupt)
	}
	ss, sp := word(48), word(46)
	if ss == 0 && sp == 0 {
		if h.realStack == 0 {
			// 低位配置可增長；高位近堆不應把DOS堆疊游標推入VGA或1MiB以上。
			if uint32(len(h.m.Mem)) < dosMemTop && uint32(len(h.m.Mem)) > h.dosBrk {
				h.dosBrk = (uint32(len(h.m.Mem)) + 15) &^ 15
			}
			base, ok := h.allocDOS(256)
			if !ok {
				return fmt.Errorf("DPMI0300預設stack配置失敗")
			}
			h.realStack = uint16(base >> 4)
		}
		ss, sp = h.realStack, 4096
	}
	if sp < 6 || uint64(ss)*16+uint64(sp) > uint64(len(h.m.Mem)) {
		return fmt.Errorf("DPMI0300 stack無效")
	}
	bus := &dpmiRealBus{m: h.m, io: h.RealModeIO}
	r := cpu.New(bus)
	r.Model = cpu.Model80386
	offsets := [...]int{28, 24, 20, 16, -1, 8, 4, 0}
	for i, o := range offsets {
		if o >= 0 {
			r.R[i] = word(o)
		}
	}
	r.EAXHi = word(30)
	r.Seg[cpu.DS], r.Seg[cpu.ES], r.Seg[cpu.SS] = word(36), word(34), ss
	r.R[cpu.SP] = sp
	r.SetFlags(word(32))
	// 不寫入guest stub：IRET回到host的哨兵位址即代表呼叫返回。
	const retSeg, retOff = uint16(0xffff), uint16(0xfff0)
	push := func(v uint16) {
		r.R[cpu.SP] -= 2
		a := cpu.Addr(r.Seg[cpu.SS], r.R[cpu.SP])
		bus.Write8(a, byte(v))
		bus.Write8(cpu.Addr(r.Seg[cpu.SS], r.R[cpu.SP]+1), byte(v>>8))
	}
	push(word(32))
	push(retSeg)
	push(retOff)
	r.Flags &^= cpu.IF | cpu.TF
	r.Seg[cpu.CS], r.IP = seg, off
	r.IntHook = func(rc *cpu.CPU, n uint8) bool {
		if h.RealModeInterrupt != nil && h.RealModeInterrupt(rc, n) {
			return true
		}
		ns, no := h.RealModeVector(n)
		if ns == 0 && no == 0 {
			bus.err = fmt.Errorf("實模式nested INT %02X未註冊 AX=%04X BX=%04X CX=%04X DX=%04X DS=%04X ES=%04X", n, rc.R[cpu.AX], rc.R[cpu.BX], rc.R[cpu.CX], rc.R[cpu.DX], rc.Seg[cpu.DS], rc.Seg[cpu.ES])
			return true
		}
		push(rc.Flags)
		push(rc.Seg[cpu.CS])
		push(rc.IP)
		rc.Flags &^= cpu.IF | cpu.TF
		rc.Seg[cpu.CS], rc.IP = ns, no
		return true
	}
	lastIRET := false
	for tr.Steps = 0; tr.Steps < 200000; tr.Steps++ {
		tr.Stop = fmt.Sprintf("%04X:%04X", r.Seg[cpu.CS], r.IP)
		if r.Seg[cpu.CS] == retSeg && r.IP == retOff {
			if !lastIRET {
				return fmt.Errorf("實模式handler未以IRET返回")
			}
			if r.Seg[cpu.SS] != ss || r.R[cpu.SP] != sp {
				return fmt.Errorf("實模式IRET返回stack不一致")
			}
			for i, o := range offsets {
				if o >= 0 {
					binary.LittleEndian.PutUint16(packet[o:], r.R[i])
				}
			}
			binary.LittleEndian.PutUint16(packet[30:], r.EAXHi)
			binary.LittleEndian.PutUint16(packet[32:], r.Flags)
			binary.LittleEndian.PutUint16(packet[34:], r.Seg[cpu.ES])
			binary.LittleEndian.PutUint16(packet[36:], r.Seg[cpu.DS])
			// CS:IP／SS:SP輸入欄位依DPMI只作call context，保留給呼叫者。
			if !c.WriteSegmentBytes(c.Seg[cpu386.SegES], c.R[cpu386.EDI], packet[:]) {
				return fmt.Errorf("DPMI0300輸出失敗")
			}
			tr.OutputPacket = fmt.Sprintf("% X", packet[:])
			tr.Returned = true
			return nil
		}
		if clock, ok := h.RealModeIO.(interface {
			AdvanceRealMode(*cpu.CPU, *LEMachine) error
		}); ok {
			if err := clock.AdvanceRealMode(r, h.m); err != nil {
				return err
			}
		}
		lastIRET = bus.Read8(cpu.Addr(r.Seg[cpu.CS], r.IP)) == 0xcf
		if err := r.Step(); err != nil {
			return err
		}
		if bus.err != nil {
			return bus.err
		}
		if r.Halted {
			return fmt.Errorf("實模式handler HLT尚未支援")
		}
	}
	return fmt.Errorf("實模式handler超過200000指令")
}

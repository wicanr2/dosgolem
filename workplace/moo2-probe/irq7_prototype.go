package main

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 規格305的可丟棄診斷。僅在正式收據結束後執行，不恢復主迴圈。
type irq7PrototypeBus struct {
	m       *machine.LEMachine
	p       *machine.LEOPLPorts
	err     error
	ioCount int
}

func (b *irq7PrototypeBus) Read8(a uint32) uint8 {
	v, err := b.m.Read8(a)
	if err != nil && b.err == nil {
		b.err = err
	}
	return v
}
func (b *irq7PrototypeBus) Write8(a uint32, v uint8) {
	if err := b.m.Write8(a, v); err != nil && b.err == nil {
		b.err = err
	}
}
func (b *irq7PrototypeBus) logIO(direction string, port uint16, value uint8, ok bool) {
	if b.ioCount < 64 {
		fmt.Printf("irq7_prototype_io index=%d direction=%s port=%04X value=%02X supported=%t\n", b.ioCount, direction, port, value, ok)
	}
	b.ioCount++
}
func (b *irq7PrototypeBus) In8(port uint16) uint8 {
	v, ok := b.p.In8(port)
	b.logIO("in", port, v, ok)
	if !ok && b.err == nil {
		b.err = fmt.Errorf("實模式IN埠%04X未支援", port)
	}
	return v
}
func (b *irq7PrototypeBus) Out8(port uint16, v uint8) {
	ok := b.p.Out8(port, v)
	b.logIO("out", port, v, ok)
	if !ok && b.err == nil {
		b.err = fmt.Errorf("實模式OUT埠%04X值%02X未支援", port, v)
	}
}

func runIRQ7Prototype(m *machine.LEMachine, h *machine.DPMIHost, p *machine.LEOPLPorts) {
	before := *m.CPU
	s := p.State()
	if !s.PICPending || !s.DSPIRQPending || s.PICInService || before.EFlags&cpu.IF == 0 {
		fmt.Println("irq7_prototype_rejected reason=不是304待派送IRQ7狀態")
		return
	}
	segments := [2]uint16{}
	for i, selector := range []uint16{before.Seg[cpu386.SegDS], before.Seg[cpu386.SegES]} {
		d, ok := before.Descriptors[selector]
		if !ok || d.Base&15 != 0 || d.Base > 0xffff0 {
			fmt.Printf("irq7_prototype_rejected reason=descriptor不能表示為實模式段 selector=%X\n", selector)
			return
		}
		segments[i] = uint16(d.Base >> 4)
	}
	scratch := before
	scratch.R[cpu386.EAX], scratch.R[cpu386.EBX] = 0x0100, 256
	if !h.Handle(&scratch) || scratch.EFlags&cpu.CF != 0 {
		fmt.Println("irq7_prototype_rejected reason=私有堆疊配置失敗")
		return
	}
	ss := uint16(scratch.R[cpu386.EAX])
	bus := &irq7PrototypeBus{m: m, p: p}
	r := cpu.New(bus)
	r.Model = cpu.Model80386
	for i := range r.R {
		r.R[i] = uint16(before.R[i])
	}
	r.EAXHi = uint16(before.R[cpu386.EAX] >> 16)
	r.Seg[cpu.DS], r.Seg[cpu.ES], r.Seg[cpu.SS] = segments[0], segments[1], ss
	r.R[cpu.SP] = 4096
	r.Seg[cpu.CS], r.IP = 0xffff, 0xfff0
	r.SetFlags(uint16(before.EFlags))
	r.IntHook = func(rc *cpu.CPU, n uint8) bool {
		if n == 0x0f {
			return false
		}
		if h.RealModeInterrupt != nil && h.RealModeInterrupt(rc, n) {
			return true
		}
		bus.err = fmt.Errorf("實模式巢狀INT%02X未支援", n)
		return true
	}
	fmt.Printf("irq7_prototype_begin draft=true extra_dispatch_micros=1 protected_r=%X protected_seg=%X protected_flags=%X synthetic_stack=%04X:1000 real_r=%X real_seg=%X real_flags=%X before_device=%+v\n", before.R, before.Seg, before.EFlags, ss, r.R, r.Seg, r.Flags, s)
	err := p.AdvanceRealMode(r, m)
	fmt.Printf("irq7_prototype_entry address_space=dosgolem_real_mode_linear csip=%04X:%04X linear=%X r=%X seg=%X flags=%X frame=%X error=%v\n", r.Seg[cpu.CS], r.IP, cpu.Addr(r.Seg[cpu.CS], r.IP), r.R, r.Seg, r.Flags, m.Mem[uint32(ss)*16+4090:uint32(ss)*16+4096], err)
	var tail [16]string
	steps, returned, lastIRET := 0, false, false
	for ; err == nil && steps < 20000; steps++ {
		if r.Seg[cpu.CS] == 0xffff && r.IP == 0xfff0 {
			if lastIRET && r.Seg[cpu.SS] == ss && r.R[cpu.SP] == 4096 {
				returned = true
			} else {
				err = fmt.Errorf("返回哨兵未經IRET或堆疊不一致")
			}
			break
		}
		if err = p.AdvanceRealMode(r, m); err != nil {
			break
		}
		a := cpu.Addr(r.Seg[cpu.CS], r.IP)
		bytes := [8]byte{}
		for j := range bytes {
			bytes[j] = bus.Read8(a + uint32(j))
		}
		tail[steps%len(tail)] = fmt.Sprintf("step=%d address_space=dosgolem_real_mode_linear csip=%04X:%04X linear=%X bytes=%X", steps, r.Seg[cpu.CS], r.IP, a, bytes)
		lastIRET = bytes[0] == 0xcf
		if bus.err != nil {
			err = bus.err
			break
		}
		if err = r.Step(); err != nil {
			break
		}
		if bus.err != nil {
			err = bus.err
			break
		}
		if r.Halted {
			err = fmt.Errorf("實模式handler HLT未支援")
			break
		}
	}
	if err == nil && !returned {
		err = fmt.Errorf("實模式handler超過20000指令")
	}
	fmt.Printf("irq7_prototype_stop returned=%t steps=%d csip=%04X:%04X r=%X seg=%X flags=%X io_count=%d protected_core_unchanged=%t error=%v after_device=%+v\n", returned, steps, r.Seg[cpu.CS], r.IP, r.R, r.Seg, r.Flags, bus.ioCount, before.R == m.CPU.R && before.Seg == m.CPU.Seg && before.EIP == m.CPU.EIP && before.EFlags == m.CPU.EFlags, err, p.State())
	for _, row := range tail {
		if row != "" {
			fmt.Printf("irq7_prototype_tail %s\n", row)
		}
	}
}

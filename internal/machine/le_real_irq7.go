package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

const leRealIRQ7StepLimit = 20000

// LERealIRQ7Trace 保存規格305的邊界，僅保留最近一次，不解ISR內部。
type LERealIRQ7Trace struct {
	CallerR                 [8]uint32
	CallerSeg               [6]uint16
	CallerEIP, CallerFlags  uint32
	Entry, Stop             string
	Steps                   int
	Returned                bool
	Error                   string
	InputR, OutputR         [8]uint16
	InputSeg, OutputSeg     [4]uint16
	InputFlags, OutputFlags uint16
	IO                      []LEOPLPortEvent
}

type leRealIRQ7 struct {
	h              *DPMIHost
	stackSegment   uint16
	active, failed bool
}

type leIRQ7Bus struct {
	dpmiRealBus
	trace *LERealIRQ7Trace
}

func (b *leIRQ7Bus) In8(port uint16) uint8 {
	v := b.dpmiRealBus.In8(port)
	if len(b.trace.IO) < 64 {
		b.trace.IO = append(b.trace.IO, LEOPLPortEvent{Port: port, Value: v})
	}
	return v
}
func (b *leIRQ7Bus) Out8(port uint16, v uint8) {
	b.dpmiRealBus.Out8(port, v)
	if len(b.trace.IO) < 64 {
		b.trace.IO = append(b.trace.IO, LEOPLPortEvent{Port: port, Value: v, Write: true})
	}
}

// 私有CPU與低位堆疊實作公開passdown契約；寄存器映射是明示平台近似。
func (d *leRealIRQ7) dispatch(c *cpu386.CPU, m *LEMachine, p *LEOPLPorts) (result error) {
	if d.failed || d.active || d.h == nil || d.h.m != m || m.CPU != c {
		return fmt.Errorf("IRQ7 轉送上下文無效")
	}
	vector, err := m.Read32(0x0f * 4)
	if err != nil || vector == 0 || uint64(uint16(vector>>16))*16+uint64(uint16(vector)) >= uint64(len(m.Mem)) {
		return fmt.Errorf("IRQ7 尚未安裝有效IVT")
	}
	var segments [2]uint16
	for i, selector := range []uint16{c.Seg[cpu386.SegDS], c.Seg[cpu386.SegES]} {
		desc, ok := c.Descriptors[selector]
		if !ok || desc.Base&15 != 0 || desc.Base > 0xffff0 {
			return fmt.Errorf("IRQ7 descriptor不能表示為實模式段 %04X", selector)
		}
		segments[i] = uint16(desc.Base >> 4)
	}
	if d.stackSegment == 0 {
		if uint32(len(m.Mem)) < dosMemTop && uint32(len(m.Mem)) > d.h.dosBrk {
			d.h.dosBrk = (uint32(len(m.Mem)) + 15) &^ 15
		}
		base, ok := d.h.allocDOS(256)
		if !ok || base == 0 {
			return fmt.Errorf("IRQ7 私有低位堆疊配置失敗")
		}
		d.stackSegment = uint16(base >> 4)
	}
	stackBase := uint32(d.stackSegment) * 16
	if uint64(stackBase)+4096 > uint64(len(m.Mem)) || stackBase+4096 > dosMemTop {
		return fmt.Errorf("IRQ7 私有低位堆疊不可寫")
	}
	tr := &LERealIRQ7Trace{CallerR: c.R, CallerSeg: c.Seg, CallerEIP: c.EIP, CallerFlags: c.EFlags, Entry: fmt.Sprintf("%04X:%04X", uint16(vector>>16), uint16(vector))}
	p.IRQ7Last = tr
	bus := &leIRQ7Bus{dpmiRealBus: dpmiRealBus{m: m, io: p}, trace: tr}
	r := cpu.New(bus)
	r.Model = cpu.Model80386
	for i := range r.R {
		r.R[i] = uint16(c.R[i])
	}
	r.EAXHi = uint16(c.R[cpu386.EAX] >> 16)
	r.Seg[cpu.DS], r.Seg[cpu.ES], r.Seg[cpu.SS] = segments[0], segments[1], d.stackSegment
	r.R[cpu.SP] = 4096
	r.Seg[cpu.CS], r.IP = 0xffff, 0xfff0
	r.SetFlags(uint16(c.EFlags))
	tr.InputR, tr.InputSeg, tr.InputFlags = r.R, r.Seg, r.Flags
	r.IntHook = func(rc *cpu.CPU, n uint8) bool {
		if n == 0x0f {
			return false
		}
		if d.h.RealModeInterrupt != nil && d.h.RealModeInterrupt(rc, n) {
			return true
		}
		if bus.err == nil {
			bus.err = fmt.Errorf("IRQ7 實模式巢狀INT%02X未支援", n)
		}
		return true
	}
	saved := leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
	d.active = true
	defer func() {
		tr.Stop = fmt.Sprintf("%04X:%04X", r.Seg[cpu.CS], r.IP)
		tr.OutputR, tr.OutputSeg, tr.OutputFlags = r.R, r.Seg, r.Flags
		c.R, c.Seg, c.EIP, c.EFlags = saved.r, saved.seg, saved.eip, saved.flags
		c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth = saved.fpuControl, saved.fpuStatus, saved.fpuStack, saved.fpuDepth
		d.active = false
		if result != nil {
			d.failed = true
			tr.Error = result.Error()
		}
	}()
	if err := p.deliverIRQ7Real(r, m); err != nil {
		return err
	}
	if bus.err != nil {
		return bus.err
	}
	if r.Seg[cpu.CS] != uint16(vector>>16) || r.IP != uint16(vector) || r.R[cpu.SP] != 4090 {
		return fmt.Errorf("IRQ7 實際派送入口或框架不一致")
	}
	p.IRQ7Passdowns++
	lastIRET := false
	for tr.Steps = 0; tr.Steps <= leRealIRQ7StepLimit; tr.Steps++ {
		if r.Seg[cpu.CS] == 0xffff && r.IP == 0xfff0 {
			if !lastIRET || r.Seg[cpu.SS] != d.stackSegment || r.R[cpu.SP] != 4096 || p.picInService {
				return fmt.Errorf("IRQ7 未以有效IRET／堆疊／EOI返回")
			}
			tr.Returned = true
			p.IRQ7Returns++
			return nil
		}
		if tr.Steps == leRealIRQ7StepLimit {
			break
		}
		if err := p.AdvanceRealMode(r, m); err != nil {
			return err
		}
		lastIRET = bus.Read8(cpu.Addr(r.Seg[cpu.CS], r.IP)) == 0xcf
		if bus.err != nil {
			return bus.err
		}
		if err := r.Step(); err != nil {
			return fmt.Errorf("IRQ7 原版實模式處理器停止 %04X:%04X r=%X seg=%X flags=%X：%w", r.Seg[cpu.CS], r.IP, r.R, r.Seg, r.Flags, err)
		}
		if bus.err != nil {
			return bus.err
		}
		if r.Halted {
			return fmt.Errorf("IRQ7 實模式handler HLT未支援")
		}
	}
	return fmt.Errorf("IRQ7 實模式handler超過%d指令，停止%04X:%04X", leRealIRQ7StepLimit, r.Seg[cpu.CS], r.IP)
}

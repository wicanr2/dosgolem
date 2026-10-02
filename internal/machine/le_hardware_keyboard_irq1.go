package machine

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

const leIRQ1Return = uint32(0xffffffc0)
const leIRQ1Limit = 20000

// LEHardwareIRQ1Trace 只保存最近的原始入口／出口，不命名遊戲RAM欄位。
type LEHardwareIRQ1Trace struct {
	CallerR                [8]uint32
	CallerSeg              [6]uint16
	CallerEIP, CallerFlags uint32
	Scan                   byte
	Entry, Stop            uint32
	Steps                  int
	Returned               bool
	Error                  string
	IO                     []LEOPLPortEvent
}
type leHardwareKeyboardIRQ1 struct {
	m                                                  *LEMachine
	s                                                  *FD2StartupDOS
	p                                                  *LEOPLPorts
	queue                                              []byte
	data, latch                                        byte
	full, pending, inService, active, failed, returned bool
	stackSelector                                      uint16
	stackDescriptor                                    cpu386.Descriptor
	frame                                              [12]byte
	started, completed                                 uint64
	last                                               *LEHardwareIRQ1Trace
}

func installLEHardwareKeyboardIRQ1(m *LEMachine, s *FD2StartupDOS, p *LEOPLPorts) {
	d := &leHardwareKeyboardIRQ1{m: m, s: s, p: p}
	p.keyboardIRQ1 = d
	previous := m.CPU.StepHook
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if previous != nil {
			if handled, err := previous(c); handled || err != nil {
				return handled, err
			}
		}
		return d.step(c)
	}
}

// QueueHardwareScan 在controller排隊，不寫BDA或遊戲欄位；307目前只驗Esc。
func (s *MOO2StartupDOS) QueueHardwareScan(scan byte) error {
	p, ok := s.DPMI.RealModeIO.(*LEOPLPorts)
	if !ok || p.keyboardIRQ1 == nil {
		return fmt.Errorf("硬體鍵盤尚未附掛")
	}
	d := p.keyboardIRQ1
	if d.failed || scan != 1 && scan != 0x81 || len(d.queue)+btoi(d.full) >= 128 {
		return fmt.Errorf("硬體鍵盤輸入無效／已滿或已失敗")
	}
	d.queue = append(d.queue, scan)
	d.refill()
	return nil
}
func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}
func (d *leHardwareKeyboardIRQ1) refill() {
	if !d.full && !d.pending && !d.inService && !d.active && len(d.queue) > 0 {
		d.data = d.queue[0]
		d.queue = d.queue[1:]
		d.full, d.pending = true, true
	}
}
func (s *MOO2StartupDOS) HardwareIRQ1State() (queued int, full, pending, inService, active, failed bool, started, completed uint64, last *LEHardwareIRQ1Trace) {
	if p, ok := s.DPMI.RealModeIO.(*LEOPLPorts); ok && p.keyboardIRQ1 != nil {
		d := p.keyboardIRQ1
		return len(d.queue), d.full, d.pending, d.inService, d.active, d.failed, d.started, d.completed, d.last
	}
	return
}
func (d *leHardwareKeyboardIRQ1) dispatch(enabled bool) (result error) {
	if d.failed {
		return fmt.Errorf("IRQ1 橋接已失敗，不可繼續計時")
	}
	d.refill()
	clock := d.p.BIOSClock
	if !enabled || !d.pending || d.inService || d.active || d.p.picMasks[0]&2 != 0 ||
		clock.InService || clock.protectedIRQ0.active || d.p.realIRQ7 != nil && d.p.realIRQ7.active {
		return nil
	}
	vector := d.s.dosVectors[9]
	if vector == 0 || d.s.protectedIRQ0.isDefault(vector, 9) {
		return nil
	}
	c := d.m.CPU
	// 選定AH25模型，不混入另一套DPMI或客製real模式09h。
	if d.s.DPMI.realVec[9] != 0 || d.s.DPMI.protVec[9] != 0 || binary.LittleEndian.Uint32(d.m.Mem[36:40]) != 0 ||
		!d.s.protectedIRQ0.validTarget(c, vector) || uint32(vector) == leIRQ1Return {
		d.failed = true
		return fmt.Errorf("IRQ1 向量超出已驗平台契約")
	}
	kernel, ok := d.s.protectedIRQ0.defaultVector(9)
	if !ok {
		d.failed = true
		return fmt.Errorf("IRQ1 預設入口無效")
	}
	if d.stackSelector == 0 {
		base, ok := d.s.DPMI.alloc(4096)
		if !ok {
			d.failed = true
			return fmt.Errorf("IRQ1 私有堆疊配置失敗")
		}
		d.stackDescriptor = cpu386.Descriptor{Base: base, Limit: 4095, Writable: true}
		d.stackSelector = d.s.DPMI.AllocSelector(d.stackDescriptor)
	}
	if c.Descriptors[d.stackSelector] != d.stackDescriptor {
		d.failed = true
		return fmt.Errorf("IRQ1 私有descriptor已改變")
	}
	saved := leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
	binary.LittleEndian.PutUint32(d.frame[:4], leIRQ1Return)
	binary.LittleEndian.PutUint32(d.frame[4:8], uint32(kernel>>32))
	binary.LittleEndian.PutUint32(d.frame[8:], c.EFlags&^(cpu386.IF|0x100))
	if !c.WriteSegmentBytes(d.stackSelector, 4084, d.frame[:]) {
		d.failed = true
		return fmt.Errorf("IRQ1 私有框架不可寫")
	}
	d.last = &LEHardwareIRQ1Trace{CallerR: c.R, CallerSeg: c.Seg, CallerEIP: c.EIP, CallerFlags: c.EFlags, Scan: d.data, Entry: uint32(vector)}
	c.Seg[cpu386.SegCS], c.Seg[cpu386.SegSS], c.EIP, c.R[cpu386.ESP] = uint16(vector>>32), d.stackSelector, uint32(vector), 4084
	c.EFlags &^= cpu386.IF | 0x100
	d.active, d.returned, d.pending, d.inService = true, false, false, true
	d.started++
	defer func() {
		if result != nil {
			d.failed = true
			d.last.Error = result.Error()
		}
		c.R, c.Seg, c.EIP, c.EFlags = saved.r, saved.seg, saved.eip, saved.flags
		c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth = saved.fpuControl, saved.fpuStatus, saved.fpuStack, saved.fpuDepth
		d.active = false
	}()
	for i := 0; i < leIRQ1Limit; i++ {
		d.last.Stop = c.EIP
		d.last.Steps = i + 1
		if err := c.Step(); err != nil {
			return fmt.Errorf("IRQ1 原始處理器停止 %04X:%08X：%w", c.Seg[cpu386.SegCS], c.EIP, err)
		}
		if d.returned {
			d.completed++
			d.last.Returned = true
			return nil
		}
	}
	return fmt.Errorf("IRQ1 原始處理器超過%d步", leIRQ1Limit)
}
func (d *leHardwareKeyboardIRQ1) step(c *cpu386.CPU) (bool, error) {
	if !d.active || d.s.protectedIRQ0.active {
		return false, nil
	}
	if c.Seg[cpu386.SegSS] != d.stackSelector || c.Descriptors[d.stackSelector] != d.stackDescriptor {
		return true, fmt.Errorf("IRQ1 私有堆疊改變")
	}
	vector := uint64(c.Seg[cpu386.SegCS])<<32 | uint64(c.EIP)
	if d.s.protectedIRQ0.isDefault(vector, 9) {
		return true, d.defaultBIOS(c)
	}
	op, ok := c.ReadSegment8(c.Seg[cpu386.SegCS], c.EIP)
	if !ok || op != 0xcf {
		return false, nil
	}
	if c.R[cpu386.ESP] != 4084 || d.full || d.inService {
		return true, fmt.Errorf("IRQ1 最外層IRETD缺框架／讀取或EOI")
	}
	for i, v := range d.frame {
		actual, ok := c.ReadSegment8(d.stackSelector, 4084+uint32(i))
		if !ok || actual != v {
			return true, fmt.Errorf("IRQ1 外層框架已改變")
		}
	}
	d.returned = true
	return true, nil
}

// BIOS為公開介面近似；nested框架與後續遊戲寫回仍由原始CPU指令形成。
func (d *leHardwareKeyboardIRQ1) defaultBIOS(c *cpu386.CPU) error {
	sp := c.R[cpu386.ESP]
	if sp > 4072 || !d.full || !d.inService || d.m.Mem[0x417] != 0 || d.m.Mem[0x418] != 0 {
		return fmt.Errorf("IRQ1 BIOS巢狀狀態未支援")
	}
	var frame [12]byte
	for i := range frame {
		v, ok := c.ReadSegment8(d.stackSelector, sp+uint32(i))
		if !ok {
			return fmt.Errorf("IRQ1 BIOS巢狀框架不可讀")
		}
		frame[i] = v
	}
	ret, cs, flags := binary.LittleEndian.Uint32(frame[:4]), binary.LittleEndian.Uint32(frame[4:8]), binary.LittleEndian.Uint32(frame[8:])
	desc, known := c.Descriptors[uint16(cs)]
	if cs > 65535 || !known || desc.Base != 0 || ret < 6 || ret > desc.Limit || flags&(1<<17) != 0 {
		return fmt.Errorf("IRQ1 BIOS巢狀返回無效")
	}
	var call [6]byte
	for i := range call {
		v, ok := c.ReadSegment8(uint16(cs), ret-6+uint32(i))
		if !ok {
			return fmt.Errorf("IRQ1 BIOS遠CALL不可讀")
		}
		call[i] = v
	}
	if call[0] != 0xff || call[1] != 0x1d {
		return fmt.Errorf("IRQ1 BIOS只接受實際遠CALL")
	}
	var pointer [6]byte
	address := binary.LittleEndian.Uint32(call[2:])
	if address > ^uint32(0)-5 {
		return fmt.Errorf("IRQ1 BIOS指標溢位")
	}
	for i := range pointer {
		v, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], address+uint32(i))
		if !ok {
			return fmt.Errorf("IRQ1 BIOS指標不可讀")
		}
		pointer[i] = v
	}
	if binary.LittleEndian.Uint32(pointer[:4]) != c.EIP || binary.LittleEndian.Uint16(pointer[4:]) != c.Seg[cpu386.SegCS] {
		return fmt.Errorf("IRQ1 BIOS指標與入口不符")
	}
	if _, ok := d.s.protectedIRQ0.defaultVector(9); !ok {
		return fmt.Errorf("IRQ1 BIOS入口已污染")
	}
	k := d.m.Keyboard
	if k == nil {
		k = &LEBIOSKeyboard{machine: d.m}
	}
	head, tail, err := k.positions()
	if err != nil || d.data == 1 && nextBIOSKey(tail) == head {
		return fmt.Errorf("IRQ1 BIOS緩衝區無效或已滿")
	}
	code, ok := d.p.In8(0x60)
	if !ok {
		return fmt.Errorf("IRQ1 BIOS沒有controller輸入")
	}
	if d.m.Keyboard == nil {
		d.m.Keyboard = k
	}
	if code == 1 {
		if err := k.Enqueue(0x011b); err != nil {
			return err
		}
	}
	if !d.p.Out8(0x20, 0x20) {
		return fmt.Errorf("IRQ1 BIOS EOI未處理")
	}
	c.R[cpu386.ESP], c.Seg[cpu386.SegCS], c.EIP, c.EFlags = sp+12, uint16(cs), ret, flags
	return nil
}

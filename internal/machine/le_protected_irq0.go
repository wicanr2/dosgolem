package machine

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// 規格 277：受限 DOS/4GW 轉送橋接，不模擬核心位址、硬體週期或完整 IRQ 鏈。
const leIRQ0StackSize = 4096
const leIRQ0StepLimit = 100000
const leIRQ0Return = uint32(0xffffffe0)

type leProtectedIRQ0 struct {
	m                  *LEMachine
	s                  *FD2StartupDOS
	p                  *LEOPLPorts
	active             bool
	returned           bool
	failed             bool
	stackSelector      uint16
	stackDescriptor    cpu386.Descriptor
	defaultSelector    uint16
	defaultDescriptor  cpu386.Descriptor
	defaultBase        uint32
	frame              [12]byte
	started, completed uint64
}

func installLEProtectedIRQ0(m *LEMachine, s *FD2StartupDOS, p *LEOPLPorts) *leProtectedIRQ0 {
	d := &leProtectedIRQ0{m: m, s: s, p: p}
	previous := m.CPU.StepHook
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if previous != nil {
			if handled, err := previous(c); handled || err != nil {
				return handled, err
			}
		}
		return d.step(c)
	}
	p.BIOSClock.protectedIRQ0 = d
	return d
}

func (d *leProtectedIRQ0) validTarget(c *cpu386.CPU, vector uint64) bool {
	selector, offset := uint16(vector>>32), uint32(vector)
	desc, ok := c.Descriptors[selector]
	if !ok || selector == 0 || desc.Base != 0 || offset == leIRQ0Return || offset == leMouseCallbackReturn {
		return false
	}
	_, ok = c.ReadSegment8(selector, offset)
	return ok
}

func (d *leProtectedIRQ0) defaultVector(number uint8) (uint64, bool) {
	c := d.m.CPU
	if d.defaultSelector == 0 {
		base, ok := d.s.DPMI.alloc(256)
		if !ok {
			return 0, false
		}
		for i := uint32(0); i < 256; i++ {
			d.m.Mem[base+i] = 0xcf
		}
		d.defaultBase = base
		d.defaultDescriptor = cpu386.Descriptor{Base: 0, Limit: base + 255}
		d.defaultSelector = d.s.DPMI.AllocSelector(d.defaultDescriptor)
	}
	if c.Descriptors[d.defaultSelector] != d.defaultDescriptor {
		return 0, false
	}
	vector := uint64(d.defaultSelector)<<32 | uint64(d.defaultBase+uint32(number))
	value, ok := c.ReadSegment8(d.defaultSelector, uint32(vector))
	return vector, ok && value == 0xcf
}

func (d *leProtectedIRQ0) isDefault(vector uint64, number uint8) bool {
	return d.defaultSelector != 0 && vector == uint64(d.defaultSelector)<<32|uint64(d.defaultBase+uint32(number))
}

func (d *leProtectedIRQ0) dispatch() (bool, error) {
	if d.failed {
		return false, errors.New("IRQ0 橋接已失敗，不可繼續執行")
	}
	if d.active {
		return false, nil
	}
	vector := d.s.dosVectors[8]
	if vector == 0 {
		return false, nil
	}
	if d.isDefault(vector, 8) {
		if _, ok := d.defaultVector(8); !ok {
			return false, errors.New("IRQ0 預設入口已污染或不可讀")
		}
		return false, nil
	}
	c := d.m.CPU
	if !d.validTarget(c, vector) {
		return false, errors.New("IRQ0 保護模式目標已無效")
	}
	defaultVector, ok := d.defaultVector(8)
	if !ok {
		return false, errors.New("IRQ0 核心返回入口已無效")
	}
	if d.stackSelector == 0 {
		base, ok := d.s.DPMI.alloc(leIRQ0StackSize)
		if !ok {
			return false, errors.New("IRQ0 私有堆疊配置失敗")
		}
		d.stackDescriptor = cpu386.Descriptor{Base: base, Limit: leIRQ0StackSize - 1, Writable: true}
		d.stackSelector = d.s.DPMI.AllocSelector(d.stackDescriptor)
	}
	if c.Descriptors[d.stackSelector] != d.stackDescriptor {
		return false, errors.New("IRQ0 私有堆疊描述子已改變")
	}
	// 核心框架的旗標已清 IF／TF；外層上下文另存，不假稱框架是被中斷玩家位置。
	saved := leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
	binary.LittleEndian.PutUint32(d.frame[:4], leIRQ0Return)
	binary.LittleEndian.PutUint32(d.frame[4:8], uint32(defaultVector>>32))
	binary.LittleEndian.PutUint32(d.frame[8:], c.EFlags&^(cpu386.IF|0x100))
	if !c.WriteSegmentBytes(d.stackSelector, leIRQ0StackSize-12, d.frame[:]) {
		return false, errors.New("IRQ0 私有返回框架不可寫")
	}
	c.Seg[cpu386.SegCS], c.Seg[cpu386.SegSS] = uint16(vector>>32), d.stackSelector
	c.EIP, c.R[cpu386.ESP] = uint32(vector), leIRQ0StackSize-12
	c.EFlags &^= cpu386.IF | 0x100
	d.active, d.returned = true, false
	d.started++
	d.p.BIOSClock.Pending = false
	d.p.BIOSClock.InService = true
	d.p.BIOSClock.Deliveries++
	defer func() {
		c.R, c.Seg, c.EIP, c.EFlags = saved.r, saved.seg, saved.eip, saved.flags
		c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth = saved.fpuControl, saved.fpuStatus, saved.fpuStack, saved.fpuDepth
		d.active = false
	}()
	for i := 0; i < leIRQ0StepLimit; i++ {
		if err := c.Step(); err != nil {
			d.failed = true
			return true, fmt.Errorf("IRQ0 原版處理器停止 %04X:%08X r=%X seg=%X flags=%X：%w", c.Seg[cpu386.SegCS], c.EIP, c.R, c.Seg, c.EFlags, err)
		}
		if d.returned {
			d.completed++
			return true, nil
		}
	}
	d.failed = true
	return true, fmt.Errorf("IRQ0 原版處理器超過 %d 步上限，停止 %04X:%08X", leIRQ0StepLimit, c.Seg[cpu386.SegCS], c.EIP)
}

func (d *leProtectedIRQ0) step(c *cpu386.CPU) (bool, error) {
	if !d.active {
		return false, nil
	}
	endChain := c.Seg[cpu386.SegCS] == d.defaultSelector && c.EIP >= d.defaultBase && c.EIP-d.defaultBase < 256
	if endChain {
		vector, ok := d.defaultVector(8)
		if !ok || vector != uint64(c.Seg[cpu386.SegCS])<<32|uint64(c.EIP) {
			return true, errors.New("IRQ0 預設核心鏈入口無效或未建模")
		}
	}
	op, err := c.Bus.Read8(c.EIP)
	if err != nil || op != 0xcf {
		if endChain {
			return true, errors.New("IRQ0 預設核心鏈入口已污染或不可讀")
		}
		return false, nil
	}
	if c.Seg[cpu386.SegSS] != d.stackSelector || c.R[cpu386.ESP] != leIRQ0StackSize-12 ||
		c.Descriptors[d.stackSelector] != d.stackDescriptor ||
		!d.validTarget(c, uint64(c.Seg[cpu386.SegCS])<<32|uint64(c.EIP)) {
		return true, errors.New("IRQ0 只接受私有框架的最外層 IRETD")
	}
	for i, expected := range d.frame {
		value, ok := c.ReadSegment8(d.stackSelector, c.R[cpu386.ESP]+uint32(i))
		if !ok || value != expected {
			return true, errors.New("IRQ0 返回框架已污染或不可讀")
		}
	}
	if endChain {
		// 規格 281：完整框架確認後，執行受限 BIOS08h；核心布局不搬入 CPU。
		if err := d.p.BIOSClock.validateDefaultBIOS(d.m); err != nil {
			return true, err
		}
		incrementLEBIOSClockData(d.m)
		d.p.Out8(0x20, 0x20)
	}
	d.returned = true
	return true, nil
}

// IRQ0State 只供有界觀測；不注入向量、時間或遊戲等待值。
func (s *MOO2StartupDOS) IRQ0State() (active, failed bool, started, completed uint64) {
	if d := s.protectedIRQ0; d != nil {
		return d.active, d.failed, d.started, d.completed
	}
	return
}

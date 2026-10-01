package machine

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// 規格 255：只建模明示的保護模式回呼橋接，不冒充硬體 IRQ 時序。
const leMouseCallbackStackSize = 4096
const leMouseCallbackQueueLimit = 4096
const leMouseCallbackReturn = uint32(0xfffffff0)

type leMouseCallbackTarget struct {
	selector uint16
	offset   uint32
}

type leMouseCallbackEvent struct {
	target               leMouseCallbackTarget
	flags, buttons, x, y uint16
	deltaX, deltaY       int16
}

type leMouseCallbackSnapshot struct {
	r                     [8]uint32
	seg                   [6]uint16
	eip, flags            uint32
	fpuControl, fpuStatus uint16
	fpuStack              [8]float64
	fpuDepth              uint8
}

type leMouseCallbackDispatcher struct {
	m                  *LEMachine
	dpmi               *DPMIHost
	mask               uint16
	target             leMouseCallbackTarget
	queue              []leMouseCallbackEvent
	active             bool
	stackSelector      uint16
	stackDescriptor    cpu386.Descriptor
	frameSelector      uint16
	saved              leMouseCallbackSnapshot
	started, completed uint64
}

func installLEMouseCallback(m *LEMachine, dpmi *DPMIHost) *leMouseCallbackDispatcher {
	d := &leMouseCallbackDispatcher{m: m, dpmi: dpmi}
	previous := m.CPU.StepHook
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if previous != nil {
			if handled, err := previous(c); handled || err != nil {
				return handled, err
			}
		}
		return d.step(c)
	}
	return d
}

func (d *leMouseCallbackDispatcher) reset() {
	if d != nil {
		d.mask, d.target, d.queue = 0, leMouseCallbackTarget{}, nil
	}
}

func (d *leMouseCallbackDispatcher) validTarget(c *cpu386.CPU, target leMouseCallbackTarget) bool {
	desc, ok := c.Descriptors[target.selector]
	if !ok || target.selector == 0 || desc.Base != 0 || target.offset == leMouseCallbackReturn {
		return false
	}
	_, ok = c.ReadSegment8(target.selector, target.offset)
	return ok
}

func (d *leMouseCallbackDispatcher) register(c *cpu386.CPU) bool {
	if d == nil || d.m.CPU != c {
		return false
	}
	mask := uint16(c.R[cpu386.ECX])
	if mask & ^uint16(0x7f) != 0 {
		return false
	}
	if mask == 0 {
		d.reset()
		return true
	}
	target := leMouseCallbackTarget{c.Seg[cpu386.SegES], c.R[cpu386.EDX]}
	if !d.validTarget(c, target) {
		return false
	}
	d.mask, d.target = mask, target
	return true
}

// InjectMouseEvent 送明示的絕對位置、三按鍵與 mickey 位移；不推算主機速度。
// SetMouseState 則只設定查詢初態，兩個入口的用途不同。
func (s *MOO2StartupDOS) InjectMouseEvent(x, y, buttons uint16, deltaX, deltaY int16) error {
	if buttons & ^uint16(7) != 0 {
		return errors.New("滑鼠事件只支援三個按鍵")
	}
	x, y = s.mouseRangeX.constrain(x), s.mouseRangeY.constrain(y)
	var flags uint16
	if x != s.mouseX || y != s.mouseY || deltaX != 0 || deltaY != 0 {
		flags = 1
	}
	for bit := uint(0); bit < 3; bit++ {
		button := uint16(1 << bit)
		if (buttons^s.mouseButtons)&button != 0 {
			flag := uint16(2 << (2 * bit))
			if buttons&button == 0 {
				flag <<= 1
			}
			flags |= flag
		}
	}
	d := s.mouseCallback
	if d != nil && flags&d.mask != 0 {
		if len(d.queue) >= leMouseCallbackQueueLimit {
			return errors.New("滑鼠回呼事件佇列已滿")
		}
		d.queue = append(d.queue, leMouseCallbackEvent{d.target, flags, buttons, x, y, deltaX, deltaY})
	}
	s.mouseX, s.mouseY, s.mouseButtons = x, y, buttons
	return nil
}

// MouseCallbackState 供有界探針觀測，不修改正式輸入與程式狀態。
func (s *MOO2StartupDOS) MouseCallbackState() (mask uint16, pending int, active bool, started, completed uint64) {
	if d := s.mouseCallback; d != nil {
		return d.mask, len(d.queue), d.active, d.started, d.completed
	}
	return
}

func (d *leMouseCallbackDispatcher) step(c *cpu386.CPU) (bool, error) {
	if ports, ok := d.dpmi.RealModeIO.(*LEOPLPorts); ok && ports.BIOSClock != nil &&
		ports.BIOSClock.protectedIRQ0 != nil && ports.BIOSClock.protectedIRQ0.active {
		return false, nil
	}
	if d.active {
		op, err := c.Bus.Read8(c.EIP)
		if err != nil || op != 0xcb {
			return false, nil
		}
		if c.Seg[cpu386.SegCS] != d.frameSelector || c.Seg[cpu386.SegSS] != d.stackSelector ||
			c.R[cpu386.ESP] != leMouseCallbackStackSize-8 || c.Descriptors[d.stackSelector] != d.stackDescriptor {
			return true, errors.New("滑鼠回呼遠返回的堆疊不符")
		}
		var frame [8]byte
		for index := range frame {
			value, ok := c.ReadSegment8(d.stackSelector, c.R[cpu386.ESP]+uint32(index))
			if !ok {
				return true, errors.New("滑鼠回呼返回框架不可讀")
			}
			frame[index] = value
		}
		if binary.LittleEndian.Uint32(frame[:4]) != leMouseCallbackReturn ||
			binary.LittleEndian.Uint32(frame[4:]) != uint32(d.frameSelector) {
			return true, errors.New("滑鼠回呼返回框架已改變")
		}
		c.R, c.Seg, c.EIP, c.EFlags = d.saved.r, d.saved.seg, d.saved.eip, d.saved.flags
		c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth = d.saved.fpuControl, d.saved.fpuStatus, d.saved.fpuStack, d.saved.fpuDepth
		d.active = false
		d.completed++
		return true, nil
	}
	if len(d.queue) == 0 || c.EFlags&cpu386.IF == 0 {
		return false, nil
	}
	event := d.queue[0]
	if !d.validTarget(c, event.target) {
		return true, errors.New("滑鼠回呼目標已無效")
	}
	if d.stackSelector == 0 {
		base, ok := d.dpmi.alloc(leMouseCallbackStackSize)
		if !ok {
			return true, errors.New("滑鼠回呼私有堆疊配置失敗")
		}
		d.stackDescriptor = cpu386.Descriptor{Base: base, Limit: leMouseCallbackStackSize - 1, Writable: true}
		d.stackSelector = d.dpmi.AllocSelector(d.stackDescriptor)
	}
	if c.Descriptors[d.stackSelector] != d.stackDescriptor {
		return true, errors.New("滑鼠回呼私有堆疊描述子已改變")
	}
	var frame [8]byte
	binary.LittleEndian.PutUint32(frame[:4], leMouseCallbackReturn)
	binary.LittleEndian.PutUint32(frame[4:], uint32(event.target.selector))
	if !c.WriteSegmentBytes(d.stackSelector, leMouseCallbackStackSize-8, frame[:]) {
		return true, fmt.Errorf("滑鼠回呼私有堆疊不可寫：%04X", d.stackSelector)
	}
	d.saved = leMouseCallbackSnapshot{c.R, c.Seg, c.EIP, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth}
	c.R[cpu386.EAX], c.R[cpu386.EBX] = uint32(event.flags), uint32(event.buttons)
	c.R[cpu386.ECX], c.R[cpu386.EDX] = uint32(event.x), uint32(event.y)
	c.R[cpu386.ESI], c.R[cpu386.EDI] = uint32(uint16(event.deltaX)), uint32(uint16(event.deltaY))
	c.Seg[cpu386.SegCS], c.Seg[cpu386.SegSS] = event.target.selector, d.stackSelector
	c.EIP, c.R[cpu386.ESP] = event.target.offset, leMouseCallbackStackSize-8
	c.EFlags &^= cpu386.IF | cpu386.DF
	d.frameSelector, d.active = event.target.selector, true
	d.queue = d.queue[1:]
	d.started++
	return true, nil
}

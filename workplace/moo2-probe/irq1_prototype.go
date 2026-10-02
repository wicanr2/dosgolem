package main

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
	"os"
)

// 規格307的可丟棄診斷；正式終態與PNG之後執行，禁止恢復玩家迴圈。
func runIRQ1Prototype(m *machine.LEMachine, h *machine.DPMIHost, target, previousVector uint64) {
	c := m.CPU
	before := *c
	if target == 0 || before.EFlags&cpu386.IF == 0 {
		fmt.Println("irq1_prototype_rejected reason=沒有已安裝入口或IF未開")
		return
	}
	selector, offset := uint16(target>>32), uint32(target)
	descriptor, ok := c.Descriptors[selector]
	if !ok || descriptor.Base != 0 {
		fmt.Println("irq1_prototype_rejected reason=入口不是可讀平坦段")
		return
	}
	if _, ok := c.ReadSegment8(selector, offset); !ok {
		return
	}
	scratch := before
	scratch.R[cpu386.EAX], scratch.R[cpu386.EBX], scratch.R[cpu386.ECX] = 0x0501, 0, 4096
	if !h.Handle(&scratch) || scratch.EFlags&cpu386.CF != 0 {
		fmt.Println("irq1_prototype_rejected reason=私有堆疊配置失敗")
		return
	}
	base := uint32(uint16(scratch.R[cpu386.EBX]))<<16 | uint32(uint16(scratch.R[cpu386.ECX]))
	ss := h.AllocSelector(cpu386.Descriptor{Base: base, Limit: 4095, Writable: true})
	memoryBefore := append([]byte(nil), m.Mem...)
	oldIn, oldOut, oldStep := c.PortIn, c.PortOut, c.StepHook
	defer func() {
		c.R, c.Seg, c.EIP, c.EFlags = before.R, before.Seg, before.EIP, before.EFlags
		c.FPUControl, c.FPUStatus, c.FPUStack, c.FPUDepth = before.FPUControl, before.FPUStatus, before.FPUStack, before.FPUDepth
		c.PortIn, c.PortOut, c.StepHook = oldIn, oldOut, oldStep
	}()
	for _, scan := range []byte{0x01, 0x81} {
		c.R, c.Seg, c.EFlags = before.R, before.Seg, before.EFlags&^(cpu386.IF|0x100)
		c.Seg[cpu386.SegCS], c.Seg[cpu386.SegSS], c.EIP, c.R[cpu386.ESP] = selector, ss, offset, 4084
		var frame [12]byte
		binary.LittleEndian.PutUint32(frame[:4], 0xffffffc0)
		binary.LittleEndian.PutUint32(frame[4:8], uint32(before.Seg[cpu386.SegCS]))
		binary.LittleEndian.PutUint32(frame[8:], before.EFlags&^(cpu386.IF|0x100))
		if !c.WriteSegmentBytes(ss, 4084, frame[:]) {
			panic("診斷堆疊不可寫")
		}
		full, inService, returned, chained := true, true, false, false
		latch, ioCount, steps := byte(0), 0, 0
		var traceError error
		var nestedFrame [12]byte
		var nestedSP uint32
		nestedPrepared := false
		logIO := func(write bool, port uint16, v uint8, supported bool) {
			if ioCount < 64 {
				fmt.Printf("irq1_prototype_io scan=%02X index=%d write=%t port=%04X value=%02X supported=%t\n", scan, ioCount, write, port, v, supported)
			}
			ioCount++
			if !supported && traceError == nil {
				traceError = fmt.Errorf("埠%04X未支援", port)
			}
		}
		c.PortIn = func(port uint16) (uint8, bool) {
			var v uint8
			supported := true
			switch port {
			case 0x60:
				if !full {
					supported = false
				} else {
					v = scan
					full = false
				}
			case 0x61:
				v = latch
			case 0x64:
				v = 4
				if full {
					v |= 1
				}
			default:
				v, supported = oldIn(port)
			}
			logIO(false, port, v, supported)
			return v, supported
		}
		c.PortOut = func(port uint16, v uint8) bool {
			supported := true
			switch port {
			case 0x61:
				latch = v
			default:
				supported = oldOut(port, v)
			}
			if port == 0x20 && v == 0x20 {
				inService = false
			}
			logIO(true, port, v, supported)
			return supported
		}
		c.StepHook = func(current *cpu386.CPU) (bool, error) {
			if oldStep != nil {
				if handled, err := oldStep(current); handled || err != nil {
					return handled, err
				}
			}
			if uint64(current.Seg[cpu386.SegCS])<<32|uint64(current.EIP) == previousVector {
				chained = true
				if os.Getenv("DOSGOLEM_MOO2_IRQ1_BIOS_PROTOTYPE") != "1" {
					return true, fmt.Errorf("進入未建模的原始預設IRQ1鏈")
				}
				if !nestedPrepared || current.Seg[cpu386.SegSS] != ss || current.R[cpu386.ESP] != nestedSP {
					return true, fmt.Errorf("BIOS診斷缺實際遠CALL框架")
				}
				for i, v := range nestedFrame {
					actual, ok := current.ReadSegment8(ss, nestedSP+uint32(i))
					if !ok || actual != v {
						return true, fmt.Errorf("BIOS診斷框架改變")
					}
				}
				code, ok := current.PortIn(0x60)
				if !ok || code != scan || m.Mem[0x417] != 0 || m.Mem[0x418] != 0 {
					return true, fmt.Errorf("BIOS診斷只接受無修飾01／81")
				}
				if m.Keyboard == nil && !machine.InstallLEBIOSKeyboard(m) {
					return true, fmt.Errorf("BIOS診斷BDA無效")
				}
				if code == 1 {
					if err := m.Keyboard.Enqueue(0x011b); err != nil {
						return true, err
					}
				}
				if !current.PortOut(0x20, 0x20) {
					return true, fmt.Errorf("BIOS診斷EOI失敗")
				}
				current.EIP = binary.LittleEndian.Uint32(nestedFrame[:4])
				current.Seg[cpu386.SegCS] = uint16(binary.LittleEndian.Uint32(nestedFrame[4:8]))
				current.EFlags = binary.LittleEndian.Uint32(nestedFrame[8:])
				current.R[cpu386.ESP] += 12
				nestedPrepared = false
				fmt.Printf("irq1_prototype_bios_return draft=true approximation=IBM_INT09 scan=%02X returned_eip=%X r=%X seg=%X flags=%X bda_queue_bytes=%X\n", scan, current.EIP, current.R, current.Seg, current.EFlags, m.Mem[0x41a:0x420])
				return true, nil
			}
			op, readOK := current.ReadSegment8(current.Seg[cpu386.SegCS], current.EIP)
			if !readOK || op != 0xcf {
				return false, nil
			}
			if current.Seg[cpu386.SegSS] != ss || current.R[cpu386.ESP] != 4084 {
				return true, fmt.Errorf("非最外層IRETD")
			}
			for i, v := range frame {
				actual, ok := current.ReadSegment8(ss, 4084+uint32(i))
				if !ok || actual != v {
					return true, fmt.Errorf("返回框架改變")
				}
			}
			if full || inService {
				return true, fmt.Errorf("未讀60h或未EOI")
			}
			returned = true
			return true, nil
		}
		fmt.Printf("irq1_prototype_begin draft=true scan=%02X target=%04X:%08X default_vector=%04X:%08X synthetic_stack=%04X:00000FF4 input_r=%X input_seg=%X input_flags=%X frame=%X\n", scan, selector, offset, uint16(previousVector>>32), uint32(previousVector), ss, c.R, c.Seg, c.EFlags, frame)
		var tail [16]string
		for ; steps < 20000 && traceError == nil && !returned; steps++ {
			bytes := make([]byte, 0, 8)
			for i := uint32(0); i < 8; i++ {
				v, ok := c.ReadSegment8(c.Seg[cpu386.SegCS], c.EIP+i)
				if !ok {
					break
				}
				bytes = append(bytes, v)
			}
			tail[steps%16] = fmt.Sprintf("step=%d cs=%04X eip=%08X r=%X seg=%X flags=%X bytes=%X", steps, c.Seg[cpu386.SegCS], c.EIP, c.R, c.Seg, c.EFlags, bytes)
			farCall := len(bytes) >= 6 && bytes[0] == 0xff && bytes[1] == 0x1d
			if farCall {
				nestedSP = c.R[cpu386.ESP] - 8
				binary.LittleEndian.PutUint32(nestedFrame[:4], c.EIP+6)
				binary.LittleEndian.PutUint32(nestedFrame[4:8], uint32(c.Seg[cpu386.SegCS]))
				binary.LittleEndian.PutUint32(nestedFrame[8:], c.EFlags)
				nestedPrepared = true
				address := binary.LittleEndian.Uint32(bytes[2:6])
				pointer := make([]byte, 0, 6)
				for i := uint32(0); i < 6; i++ {
					v, ok := c.ReadSegment8(c.Seg[cpu386.SegDS], address+i)
					if !ok {
						break
					}
					pointer = append(pointer, v)
				}
				fmt.Printf("irq1_prototype_far_call_input scan=%02X step=%d eip=%X r=%X seg=%X flags=%X source_offset=%X pointer=%X bytes=%X\n", scan, steps, c.EIP, c.R, c.Seg, c.EFlags, address, pointer, bytes)
			}
			if err := c.Step(); err != nil {
				traceError = err
			}
			if farCall {
				stack := make([]byte, 0, 12)
				for i := uint32(0); i < 12; i++ {
					v, ok := c.ReadSegment8(c.Seg[cpu386.SegSS], c.R[cpu386.ESP]+i)
					if !ok {
						break
					}
					stack = append(stack, v)
				}
				fmt.Printf("irq1_prototype_far_call_output scan=%02X step=%d eip=%X r=%X seg=%X flags=%X stack=%X error=%v\n", scan, steps, c.EIP, c.R, c.Seg, c.EFlags, stack, traceError)
			}
		}
		if !returned && traceError == nil {
			traceError = fmt.Errorf("診斷超過20000步")
		}
		fmt.Printf("irq1_prototype_end scan=%02X steps=%d returned=%t chained=%t full=%t in_service=%t latch=%02X stop=%04X:%08X r=%X seg=%X flags=%X io_count=%d error=%v\n", scan, steps, returned, chained, full, inService, latch, c.Seg[cpu386.SegCS], c.EIP, c.R, c.Seg, c.EFlags, ioCount, traceError)
		for i := max(0, steps-16); i < steps; i++ {
			fmt.Printf("irq1_prototype_tail scan=%02X %s\n", scan, tail[i%16])
		}
		changes := 0
		for i, v := range memoryBefore {
			if uint32(i) >= base && uint32(i) < base+4096 {
				continue
			}
			if m.Mem[i] != v {
				if changes < 64 {
					fmt.Printf("irq1_prototype_memory scan=%02X address_space=dosgolem_linear address=%X before=%02X after=%02X\n", scan, i, v, m.Mem[i])
				}
				changes++
			}
		}
		fmt.Printf("irq1_prototype_memory_total scan=%02X changed_bytes=%d private_stack_excluded=true\n", scan, changes)
		if traceError != nil {
			break
		}
		copy(memoryBefore, m.Mem)
	}
}

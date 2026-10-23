package machine

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// PM 版本 1–3 的狀態檔在保護模式剖面讀入時的升級（docs/spec/459-mouse-irq12-v86-delivery §5.3）。
//
// planPMUpgrade 只讀狀態檔（s.Mem、s.Ext 與解碼後的 PM），在 LoadState 的任何寫入之前決定全部結果；
// 失敗時回錯誤、機器不變。applyPMUpgrade 在 applyPM、sb8Apply 之後寫入。
//
// 讀寫的位址是實體位址：StubSeg 的樁與 IVT 在實體 0–FFFh，與 LoadState 的 Mem 一致。

// pmUpgrade 是 planPMUpgrade 的決定。
type pmUpgrade struct {
	slaveIMR, picMask uint8
	readISR           [2]bool
	rewriteIVT74      bool
	note              string // IVT[74h] 已被程式改過的紀錄
	frames            string // 分段模式下在 ring 0 堆疊上找到的 V86 iretd 框架（收據用）
}

// stateSeg 是讀狀態檔用的實體讀取（A20 照存檔的值遮罩）。
type stateSeg struct {
	s  *machineState
	pm pmFields
}

func (r stateSeg) phys8(a uint32) (uint8, bool) {
	if !r.s.A20 {
		a &^= 0x100000
	}
	if a < MemSize {
		if a < uint32(len(r.s.Mem)) {
			return r.s.Mem[a], true
		}
		return 0, false
	}
	if off := a - ExtBase; off < uint32(len(r.s.Ext)) {
		return r.s.Ext[off], true
	}
	return 0, false
}

func (r stateSeg) phys32(a uint32) (uint32, bool) {
	var v uint32
	for i := uint32(0); i < 4; i++ {
		b, ok := r.phys8(a + i)
		if !ok {
			return 0, false
		}
		v |= uint32(b) << (8 * i)
	}
	return v, true
}

// lin8 以存檔的 CR0、CR3 把線性位址轉成實體位址再讀（分頁關閉時線性即實體）。
func (r stateSeg) lin8(a uint32) (uint8, bool) {
	if r.pm.cr0&cr0PE != 0 && r.pm.cr0&cr0PG != 0 {
		pde, ok := r.phys32(r.pm.cr3&0xFFFFF000 + (a>>22)<<2)
		if !ok || pde&pteP == 0 {
			return 0, false
		}
		pte, ok := r.phys32(pde&0xFFFFF000 + (a>>12&0x3FF)<<2)
		if !ok || pte&pteP == 0 {
			return 0, false
		}
		a = pte&0xFFFFF000 | a&0xFFF
	}
	return r.phys8(a)
}

func (r stateSeg) lin32(a uint32) (uint32, bool) {
	var v uint32
	for i := uint32(0); i < 4; i++ {
		b, ok := r.lin8(a + i)
		if !ok {
			return 0, false
		}
		v |= uint32(b) << (8 * i)
	}
	return v, true
}

// planPMUpgrade 是 §5.3 升級規則第 1–3b 步的決定（只讀）。
func (m *Machine) planPMUpgrade(s *machineState, pm pmFields, sb8 *sbDMA8State) (*pmUpgrade, error) {
	r := stateSeg{s: s, pm: pm}
	up := &pmUpgrade{slaveIMR: 0xFF}
	// 第 2、2a 步：從片 IMR 取 sbDMA8State 的值（否則 FFh）後清 bit 4；主片 IMR 取 sbDMA8State 的值（否則 0）。
	if sb8 != nil {
		up.slaveIMR, up.picMask, up.readISR = sb8.SlaveIMR, sb8.PICMask, sb8.ReadISR
	}
	up.slaveIMR &^= 0x10

	// 第 3 步：BIOS 計時器樁 35 bytes 要與舊版逐位元組相同。
	base := uint32(StubSeg) * 16
	t := base + biosTimerOff
	if t+uint32(len(biosTimerCode)) > uint32(len(s.Mem)) || !bytes.Equal(s.Mem[t:t+uint32(len(biosTimerCode))], biosTimerCode[:]) {
		end := t + uint32(len(biosTimerCode))
		if end > uint32(len(s.Mem)) {
			end = uint32(len(s.Mem))
		}
		return nil, fmt.Errorf("machine: PM 版本 %d 升級：BIOS 計時器樁（%04X:%04X，35 bytes）與舊版不同，不能換成帶 EOI 的版本，"+
			"失敗即關閉：實際 % X（docs/spec/459 §5.3 第 3 步）", binary.LittleEndian.Uint16(s.PM), StubSeg, biosTimerOff, s.Mem[t:end])
	}
	// 第 3 步：返回位址不能落在舊樁 int 1Ch 之後的 0x43F–0x442（換版後會移位）。
	inOld := func(cs uint16, ip uint32) bool {
		return cs == StubSeg && ip >= biosTimerIRetOff && ip <= biosTimerIRetOff+3
	}
	if !pm.segmented {
		if inOld(s.Seg[cpu.CS], uint32(s.IP)) {
			return nil, fmt.Errorf("machine: PM 版本 %d 升級：目前 CS:IP %04X:%04X 落在舊 BIOS 計時器樁的 int 1Ch 返回位址範圍 "+
				"%04X:%04X–%04X，換版會移位，失敗即關閉（docs/spec/459 §5.3 第 3 步）",
				binary.LittleEndian.Uint16(s.PM), s.Seg[cpu.CS], s.IP, StubSeg, biosTimerIRetOff, biosTimerIRetOff+3)
		}
	} else if err := up.scanRing0(r, s, inOld); err != nil {
		return nil, err
	}
	// 第 3a 步：KbdIntercept 為真時，狀態檔內的鍵盤客體碼要是 447 的 14 bytes 或 459 的 19 bytes 版本；
	// 或是從未寫過（19 bytes 全 0，且向量 9 的 stub 是原本的 CD 09 CF），交 BootFromState 第 6 步寫入（第 5 版）；
	// 接受之後呼叫端必須接著 EnableKbdIntercept。與第 3、3b 步相同，以實體位址讀，前提是線性第 0 頁恆等映射。
	if m.KbdIntercept {
		k := base + KbdInterceptOff
		got := s.Mem[k : k+uint32(len(kbdInterceptCodeEOI))]
		v9 := base + uint32(StubOff(0x09))
		neverWritten := bytes.Equal(got, make([]byte, len(got))) && bytes.Equal(s.Mem[v9:v9+3], []byte{0xCD, 0x09, 0xCF})
		if !neverWritten && !bytes.Equal(got, kbdInterceptCodeEOI[:]) && !bytes.Equal(got[:len(kbdInterceptCode)], kbdInterceptCode[:]) {
			return nil, fmt.Errorf("machine: PM 版本 %d 升級：鍵盤攔截客體碼（%04X:%04X）不是 447 的 14 bytes 或 459 的 19 bytes 版本，"+
				"失敗即關閉：實際 % X（docs/spec/459 §5.3 第 3a 步）", binary.LittleEndian.Uint16(s.PM), StubSeg, KbdInterceptOff, got)
		}
	}
	// 第 3b 步：IVT[74h] 仍是 per-vector stub 才改。
	off, seg := binary.LittleEndian.Uint16(s.Mem[0x74*4:]), binary.LittleEndian.Uint16(s.Mem[0x74*4+2:])
	if off == StubOff(0x74) && seg == StubSeg {
		up.rewriteIVT74 = true
	} else {
		up.note = fmt.Sprintf("IVT[74h]=%04X:%04X 已被程式改過，升級時不動（docs/spec/459 §5.3 第 3b 步）", seg, off)
	}
	return up, nil
}

// scanRing0 是 §5.3 第 3 步的分段模式部分：在 ring 0 堆疊頂（SS:SP 起 64 bytes，以 dword 對齊）找 V86 的 iretd
// 框架（EFLAGS 的 VM 位元為 1），任一框架的 V86 CS:IP 落在舊樁的返回位址範圍即失敗即關閉。
// 堆疊頂本身讀不到（頁不存在、超出段界限）也失敗即關閉；找到的框架記在 frames（收據用）。
//
// JEMM 在 ring 0 出口 `popad; iretd` 之前存檔時，iretd 框架在 pushad 的 32 bytes 之上，所以不只看 SP+0。
func (up *pmUpgrade) scanRing0(r stateSeg, s *machineState, inOld func(uint16, uint32) bool) error {
	ss := r.pm.cache[cpu.SS]
	sp := uint32(s.R[cpu.SP])
	ver := binary.LittleEndian.Uint16(s.PM)
	inSeg := func(o uint32) bool { // 434 §4.1 的界限（含向下展開，JEMM 的 SS0 即是）；B=0 的 16 位元偏移
		return o+11 <= 0xFFFF && segOffOK(ss, o) && segOffOK(ss, o+11)
	}
	for k := uint32(0); k < 16; k++ {
		o := sp + 4*k
		if !inSeg(o) {
			if k == 0 {
				return fmt.Errorf("machine: PM 版本 %d 升級：分段模式 ring 0 堆疊頂 SS:SP 偏移 %X 超出界限 %X，讀不到 iretd 框架，"+
					"失敗即關閉（docs/spec/459 §5.3 第 3 步）", ver, sp, ss.Limit)
			}
			break
		}
		eip, ok1 := r.lin32(ss.Base + o)
		cs, ok2 := r.lin32(ss.Base + o + 4)
		fl, ok3 := r.lin32(ss.Base + o + 8)
		if !ok1 || !ok2 || !ok3 {
			if k == 0 {
				return fmt.Errorf("machine: PM 版本 %d 升級：分段模式 ring 0 堆疊頂（線性 %08X）讀不到，"+
					"失敗即關閉（docs/spec/459 §5.3 第 3 步）", ver, ss.Base+o)
			}
			break
		}
		if fl&(uint32(eflagsHiVM)<<16) == 0 {
			continue
		}
		up.frames += fmt.Sprintf("SP+%02X：V86 %04X:%04X EFLAGS=%08X；", 4*k, uint16(cs), uint16(eip), fl)
		if inOld(uint16(cs), eip&0xFFFF) {
			return fmt.Errorf("machine: PM 版本 %d 升級：ring 0 堆疊（SP+%02Xh）上的 V86 iretd 框架返回 %04X:%04X，落在舊 BIOS 計時器樁的 "+
				"int 1Ch 返回位址範圍，換版會移位，失敗即關閉（docs/spec/459 §5.3 第 3 步）", ver, 4*k, uint16(cs), uint16(eip))
		}
	}
	return nil
}

// applyPMUpgrade 是 §5.3 升級的寫入（第 1、2、2a、3、3b、4 步）。LoadState 在 applyPM、sb8Apply 之後呼叫。
func (m *Machine) applyPMUpgrade(up *pmUpgrade) {
	m.pic = picState{slaveIMR: up.slaveIMR, readISR: up.readISR, upgraded: true}
	m.picMask = up.picMask
	m.cbQueue = m.cbQueue[:0]
	base := uint32(StubSeg) * 16
	copy(m.Mem[base+biosTimerOff:], biosTimerCodeEOI[:])
	m.writeMouseStubs(func(a uint32, b []byte) { copy(m.Mem[a:], b) })
	if up.rewriteIVT74 {
		binary.LittleEndian.PutUint16(m.Mem[0x74*4:], Mouse74Off)
		binary.LittleEndian.PutUint16(m.Mem[0x74*4+2:], StubSeg)
	}
	m.picd.upgradeIV = up.note
	m.picd.upgradeFrames = up.frames
}

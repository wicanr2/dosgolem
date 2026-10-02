package machine

import (
	"encoding/binary"
	"fmt"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// LEBIOSClock 為預設BIOS計時服務；指令時間為明示近似，不是實機週期。
type LEBIOSClock struct {
	Micros        uint64
	Deliveries    uint64
	Pending       bool
	InService     bool
	protectedIRQ0 *leProtectedIRQ0
	credit        uint64
	generation    uint64
}

// String 排除橋接器的主機指標，讓只讀診斷不混入每次執行不同的位址。
func (b *LEBIOSClock) String() string {
	if b == nil {
		return "<nil>"
	}
	return fmt.Sprintf("&{Micros:%d Deliveries:%d Pending:%t InService:%t credit:%d generation:%d}", b.Micros, b.Deliveries, b.Pending, b.InService, b.credit, b.generation)
}

func InstallLEBIOSClock(m *LEMachine, p *LEOPLPorts) bool {
	if m == nil || m.CPU == nil || p == nil || p.BIOSClock != nil || len(m.Mem) < 0x471 {
		return false
	}
	p.BIOSClock = &LEBIOSClock{}
	previous := m.CPU.StepHook
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if err := p.AdvanceProtectedMode(c, m); err != nil {
			return true, err
		}
		if previous != nil {
			return previous(c)
		}
		return false, nil
	}
	return true
}
func (b *LEBIOSClock) advance(m *LEMachine, p *LEOPLPorts, enabled bool) error {
	if b.protectedIRQ0 != nil && b.protectedIRQ0.failed {
		return fmt.Errorf("IRQ0 橋接已失敗，不可繼續計時")
	}
	if len(m.Mem) < 0x471 {
		return fmt.Errorf("BIOS時鐘資料區不可讀寫")
	}
	b.Micros++
	reload := uint32(65536)
	if p.PIT0.Configured {
		if !p.PIT0.Loaded {
			return b.deliver(m, p, enabled)
		}
		reload = p.PIT0.Reload
	}
	if reload == 0 {
		return fmt.Errorf("PIT重載不可為零")
	}
	if b.generation != p.PIT0.Generation {
		b.credit = 0
		b.generation = p.PIT0.Generation
	}
	// 每次呼叫代表一微秒，credit 累加「一秒有幾個 PIT 計數」的定點值。
	//
	// 用 `315e6 / (reload × 264)` 這個分數，不用四捨五入的 1,193,182：
	// 輸入頻率是 `315/264` MHz ＝ 1,193,181.8181…（`PITBaseHz`），
	// 兩種寫法的比值差 1.5×10⁻⁷，長跑累積下來會偏。
	b.credit += 315_000_000
	period := uint64(reload) * 264 * 1_000_000
	if b.credit >= period {
		b.credit %= period
		b.Pending = true
	}
	return b.deliver(m, p, enabled)
}
func (b *LEBIOSClock) deliver(m *LEMachine, p *LEOPLPorts, enabled bool) error {
	if !b.Pending || !enabled || p.picMasks[0]&1 != 0 || b.InService || b.protectedIRQ0 != nil && b.protectedIRQ0.active {
		return nil
	}
	if err := b.validateDefaultBIOS(m); err != nil {
		return err
	}
	if b.protectedIRQ0 != nil {
		if handled, err := b.protectedIRQ0.dispatch(); handled || err != nil {
			return err
		}
	}
	incrementLEBIOSClockData(m)
	b.Pending = false
	b.Deliveries++
	return nil
}

// 規格 281：派送與預設結束鏈共用前提，拒絕未建模的實／保護模式 BIOS 鏈。
func (b *LEBIOSClock) validateDefaultBIOS(m *LEMachine) error {
	if len(m.Mem) < 0x471 {
		return fmt.Errorf("BIOS時鐘資料區不可讀寫")
	}
	for _, n := range []int{8, 0x1c} {
		if binary.LittleEndian.Uint32(m.Mem[n*4:]) != 0 {
			return fmt.Errorf("BIOS時鐘尚不支援客製INT%02X", n)
		}
	}
	if b.protectedIRQ0 != nil {
		for _, n := range []uint8{8, 0x1c} {
			seg, off := b.protectedIRQ0.s.DPMI.RealModeVector(n)
			if seg != 0 || off != 0 {
				return fmt.Errorf("BIOS時鐘尚不支援DPMI客製實模式INT%02X", n)
			}
		}
		vector := b.protectedIRQ0.s.dosVectors[0x1c]
		if vector != 0 && !b.protectedIRQ0.isDefault(vector, 0x1c) {
			return fmt.Errorf("BIOS時鐘尚不支援保護模式客製INT1C")
		}
		if b.protectedIRQ0.isDefault(vector, 0x1c) {
			if _, ok := b.protectedIRQ0.defaultVector(0x1c); !ok {
				return fmt.Errorf("BIOS時鐘預設INT1C已污染或不可讀")
			}
		}
	}
	return nil
}

// 呼叫端先檢查 BDA 與 BIOS 鏈；一次預設 BIOS08h 只增加一次計數。
func incrementLEBIOSClockData(m *LEMachine) {
	value := binary.LittleEndian.Uint32(m.Mem[0x46c:]) + 1
	if value >= 0x1800b0 {
		value = 0
		m.Mem[0x470]++
	}
	binary.LittleEndian.PutUint32(m.Mem[0x46c:], value)
}

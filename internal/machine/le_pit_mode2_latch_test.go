package machine

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func latchPortsFixture() *LEOPLPorts {
	return &LEOPLPorts{
		PIT0:      LEPIT0{Mode: 2, Configured: true, Loaded: true, Reload: 5966, Generation: 5},
		BIOSClock: &LEBIOSClock{generation: 5},
		Reads:     make(map[uint16]uint64), Writes: make(map[uint16]uint64),
	}
}

func TestPIT0Mode2LatchCountDomainAndFractionalPhase(t *testing.T) {
	// 每個合法 reload 的起點、最後一 tick 與完整 16 位 count 編碼。
	const unit uint64 = 264 * 1000000
	for reload := uint32(2); reload <= 65536; reload++ {
		for _, elapsed := range []uint32{0, reload - 1} {
			p := latchPortsFixture()
			p.PIT0.Reload = reload
			p.BIOSClock.credit = uint64(elapsed)*unit + unit - 1
			before := *p.BIOSClock
			if !p.Out8(0x43, 0) {
				t.Fatal("合法鎖存")
			}
			lo, ok0 := p.In8(0x40)
			hi, ok1 := p.In8(0x40)
			if !ok0 || !ok1 || uint16(lo)|uint16(hi)<<8 != uint16(reload-elapsed) || *p.BIOSClock != before || p.PIT0.readPending {
				t.Fatalf("reload=%d elapsed=%d", reload, elapsed)
			}
		}
	}
	// 避免只測 1 或 reload，遍歷其間每一個編碼。
	p := latchPortsFixture()
	p.PIT0.Reload = 65536
	for want := uint32(1); want <= 65536; want++ {
		p.BIOSClock.credit = uint64(65536-want) * unit
		if !p.Out8(0x43, 0) {
			t.Fatal("計數鎖存")
		}
		lo, _ := p.In8(0x40)
		hi, _ := p.In8(0x40)
		if uint16(lo)|uint16(hi)<<8 != uint16(want) {
			t.Fatalf("count=%d", want)
		}
	}
}

func TestPIT0Mode2LatchFreezeInterleavingAndReconfigure(t *testing.T) {
	p := latchPortsFixture()
	p.PIT0.Reload = 0x1234
	beforeClock := *p.BIOSClock
	if !p.Out8(0x43, 0) {
		t.Fatal("鎖存")
	}
	// 計數持續，重複鎖存保留原值。
	p.BIOSClock.credit = 100 * 264 * 1000000
	if !p.Out8(0x43, 0) {
		t.Fatal("忽略重複鎖存")
	}
	lo, ok := p.In8(0x40)
	if !ok || lo != 0x34 {
		t.Fatal("凍結低 byte")
	}
	if !p.Out8(0x40, 0x78) || !p.Out8(0x43, 0) {
		t.Fatal("低寫入與重複鎖存")
	}
	hi, ok := p.In8(0x40)
	if !ok || hi != 0x12 || !p.PIT0.highNext {
		t.Fatal("讀寫半筆不可混用")
	}
	if _, ok := p.In8(0x40); ok {
		t.Fatal("未鎖存直接讀取")
	}
	if !p.Out8(0x40, 0x56) || p.PIT0.Reload != 0x5678 || p.PIT0.Generation != 6 {
		t.Fatal("高寫入")
	}
	// 新世代尚未推進時使用相位零，不能帶入舊 credit。
	if !p.Out8(0x43, 0) {
		t.Fatal("新世代鎖存")
	}
	lo, _ = p.In8(0x40)
	hi, _ = p.In8(0x40)
	if lo != 0x78 || hi != 0x56 {
		t.Fatal("新世代相位")
	}
	p.BIOSClock.credit = beforeClock.credit
	if !p.Out8(0x43, 0) {
		t.Fatal("鎖存")
	}
	p.In8(0x40)
	if !p.Out8(0x43, 0x34) || p.PIT0.readPending || p.PIT0.readHighNext {
		t.Fatal("重新編程取消鎖存")
	}
	if _, ok := p.In8(0x40); ok {
		t.Fatal("取消後讀取")
	}
	want := []LEOPLPortEvent{{0x43, 0, true}, {0x43, 0, true}, {0x40, 0x34, false}, {0x40, 0x78, true}, {0x43, 0, true}, {0x40, 0x12, false}, {0x40, 0x56, true}, {0x43, 0, true}, {0x40, 0x78, false}, {0x40, 0x56, false}, {0x43, 0, true}, {0x40, 0x78, false}, {0x43, 0x34, true}}
	if !reflect.DeepEqual(p.Log, want) {
		t.Fatalf("共享埠紀錄 %+v", p.Log)
	}
}

func TestPIT0Mode2LatchRejectsWithoutEffects(t *testing.T) {
	changes := map[string]func(*LEOPLPorts){
		"缺時計":      func(p *LEOPLPorts) { p.BIOSClock = nil },
		"未設定":      func(p *LEOPLPorts) { p.PIT0.Configured = false },
		"半筆":       func(p *LEOPLPorts) { p.PIT0.Loaded = false },
		"模式3":      func(p *LEOPLPorts) { p.PIT0.Mode = 3 },
		"模式未知":     func(p *LEOPLPorts) { p.PIT0.Mode = 7 },
		"reload0":  func(p *LEOPLPorts) { p.PIT0.Reload = 0 },
		"reload1":  func(p *LEOPLPorts) { p.PIT0.Reload = 1 },
		"reload越界": func(p *LEOPLPorts) { p.PIT0.Reload = 65537 },
		"相位越界":     func(p *LEOPLPorts) { p.BIOSClock.credit = uint64(p.PIT0.Reload) * 264 * 1000000 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			p := latchPortsFixture()
			change(p)
			before := p.PIT0
			var clock LEBIOSClock
			if p.BIOSClock != nil {
				clock = *p.BIOSClock
			}
			if p.Out8(0x43, 0) || p.PIT0 != before || len(p.Log) != 0 || len(p.Writes) != 0 || len(p.Reads) != 0 {
				t.Fatal("拒絕須保留全部狀態")
			}
			if p.BIOSClock != nil && *p.BIOSClock != clock {
				t.Fatal("時計被改寫")
			}
		})
	}
	p := latchPortsFixture()
	p.Out8(0x43, 0)
	before := p.PIT0
	log := append([]LEOPLPortEvent(nil), p.Log...)
	for v := 1; v <= 255; v++ {
		if v == 0x34 || v == 0x36 {
			continue
		}
		if p.Out8(0x43, byte(v)) || p.PIT0 != before || !reflect.DeepEqual(p.Log, log) {
			t.Fatalf("未知控制字 %X", v)
		}
	}
}

func TestPIT0Mode2LatchSharedCPUModes(t *testing.T) {
	for _, realMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "保護模式", true: "實模式"}[realMode], func(t *testing.T) {
			m, p := clockFixture(t)
			m.Mem = make([]byte, 0x20000)
			m.CPU.PortOut, m.CPU.PortIn = p.Out8, p.In8
			p.Out8(0x43, 0x34)
			p.Out8(0x40, 0x4e)
			p.Out8(0x40, 0x17)
			binary.LittleEndian.PutUint32(m.Mem[0x46c:], 123)
			// 既有 DX 埠 IN 驗證共享平台；E4 另由規格 283 處理，不拷貝 timer driver。
			if realMode {
				bus := &dpmiRealBus{m: m, io: p}
				r := cpu.New(bus)
				r.Model = cpu.Model80386
				r.Seg[cpu.CS], r.IP = 0x100, 0
				r.R[cpu.DX] = 0x40
				r.R[cpu.AX] = 0xab00
				r.R[cpu.SP] = 0x7000
				r.Flags = 2
				copy(m.Mem[0x1000:], []byte{0xe6, 0x43, 0xec, 0x88, 0xc4, 0xec})
				regs, seg, flags := r.R, r.Seg, r.Flags
				var count uint16
				for i := 0; i < 4; i++ {
					if err := p.AdvanceRealMode(r, m); err != nil {
						t.Fatal(err)
					}
					if err := r.Step(); err != nil || bus.err != nil {
						t.Fatalf("實模式 %v/%v", err, bus.err)
					}
					if i == 0 {
						count = p.PIT0.readCount
					}
				}
				regs[cpu.AX] = uint16(byte(count))<<8 | uint16(byte(count>>8))
				if r.R != regs || r.Seg != seg || r.Flags != flags || r.IP != 6 {
					t.Fatal("實模式讀取狀態")
				}
			} else {
				c := m.CPU
				c.R[cpu386.EDX] = 0x40
				c.R[cpu386.EAX] = 0x9876ab00
				c.EFlags = 2
				c.EIP = 0x600
				copy(m.Mem[0x600:], []byte{0xe6, 0x43, 0xec, 0x88, 0xc4, 0xec})
				regs, seg, flags := c.R, c.Seg, c.EFlags
				var count uint16
				for i := 0; i < 4; i++ {
					if err := c.Step(); err != nil {
						t.Fatal(err)
					}
					if i == 0 {
						count = p.PIT0.readCount
					}
				}
				regs[cpu386.EAX] = 0x98760000 | uint32(byte(count))<<8 | uint32(byte(count>>8))
				if c.R != regs || c.Seg != seg || c.EFlags != flags || c.EIP != 0x606 {
					t.Fatal("保護模式讀取狀態")
				}
			}
			if p.BIOSClock.Micros != 4 || p.BIOSClock.Deliveries != 0 || p.BIOSClock.Pending || p.BIOSClock.InService || binary.LittleEndian.Uint32(m.Mem[0x46c:]) != 123 || p.PIT0.readPending || p.Reads[0x40] != 2 || p.Writes[0x43] != 2 {
				t.Fatal("共享時計／IRQ／BDA／埠紀錄被改寫")
			}
		})
	}
}

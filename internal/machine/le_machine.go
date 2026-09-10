package machine

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// LEMachine 是 DOS/4GW 已完成載入後的平坦 32-bit 執行環境。
// 它刻意不共用 real-mode Machine 的 20-bit wrap 與 IVT。
type LEMachine struct {
	Video    *LEVideo
	Keyboard *LEBIOSKeyboard
	Mem      []byte
	CPU      *cpu386.CPU
	Ports    map[uint16]uint8
	PortLog  []LEPortWrite

	// pit 解讀通道 0 的重載，與 real-mode Machine 共用同一份解碼器
	// （`pit.go`）：保護模式的遊戲一樣是寫 `0x43`／`0x40`。
	pit pit
}

// LEPortWrite 是保護模式程式的一次 byte port 輸出；Sequence 只表示先後順序，
// 不冒充硬體 cycle 或 wall-clock。
type LEPortWrite struct {
	Port     uint16
	Value    uint8
	Sequence uint64
}

func LoadLE(data []byte) (*LEMachine, error) {
	header, err := InspectLE(data)
	if err != nil {
		return nil, err
	}
	images, err := header.RelocatedObjectImages(data)
	if err != nil {
		return nil, err
	}
	if header.EIPObject == 0 || header.EIPObject > uint32(len(header.Objects)) || header.ESPObject == 0 || header.ESPObject > uint32(len(header.Objects)) {
		return nil, fmt.Errorf("machine: LE entry 或 stack object 超界")
	}
	var end uint64
	for i, object := range header.Objects {
		objectEnd := uint64(object.RelocationBase) + uint64(len(images[i]))
		if objectEnd > uint64(^uint32(0)) {
			return nil, fmt.Errorf("machine: LE object %d 位址溢位", i+1)
		}
		if objectEnd > end {
			end = objectEnd
		}
		for j := 0; j < i; j++ {
			otherStart := uint64(header.Objects[j].RelocationBase)
			otherEnd := otherStart + uint64(len(images[j]))
			if uint64(object.RelocationBase) < otherEnd && otherStart < objectEnd {
				return nil, fmt.Errorf("machine: LE objects %d 與 %d 位址重疊", j+1, i+1)
			}
		}
	}
	if end > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("machine: LE address space 太大")
	}
	m := &LEMachine{Mem: make([]byte, int(end)), Ports: make(map[uint16]uint8)}
	for i, object := range header.Objects {
		start := uint64(object.RelocationBase)
		if start > uint64(len(m.Mem)) || uint64(len(images[i])) > uint64(len(m.Mem))-start {
			return nil, fmt.Errorf("machine: LE object %d 映射超界", i+1)
		}
		copy(m.Mem[int(start):], images[i])
	}
	entryObject := header.Objects[header.EIPObject-1]
	stackObject := header.Objects[header.ESPObject-1]
	if uint64(header.EIP) >= uint64(len(images[header.EIPObject-1])) || uint64(header.ESP) > uint64(len(images[header.ESPObject-1])) {
		return nil, fmt.Errorf("machine: LE entry 或 stack offset 超界")
	}
	m.CPU = cpu386.New(m)
	// 規格008的base-0入口環境：非零selector是工具標籤，不是原版extender編號。
	m.CPU.SetDescriptor(0x08, cpu386.Descriptor{Limit: 0xffffffff})
	m.CPU.SetDescriptor(0x10, cpu386.Descriptor{Limit: 0xffffffff, Writable: true})
	m.CPU.Seg[cpu386.SegCS] = 0x08
	for _, seg := range []int{cpu386.SegDS, cpu386.SegES, cpu386.SegSS} {
		m.CPU.Seg[seg] = 0x10
	}
	m.CPU.PortOut = func(port uint16, value uint8) bool {
		m.pit.out(port, value)
		m.Ports[port] = value
		m.PortLog = append(m.PortLog, LEPortWrite{Port: port, Value: value, Sequence: uint64(len(m.PortLog))})
		return true
	}
	m.CPU.EIP = entryObject.RelocationBase + header.EIP
	m.CPU.R[cpu386.ESP] = stackObject.RelocationBase + header.ESP
	return m, nil
}

func (m *LEMachine) Read8(addr uint32) (uint8, error) {
	if uint64(addr) >= uint64(len(m.Mem)) {
		return 0, fmt.Errorf("machine: LE read 0x%X 超界", addr)
	}
	return m.Mem[addr], nil
}

func (m *LEMachine) Write8(addr uint32, value uint8) error {
	if uint64(addr) >= uint64(len(m.Mem)) {
		return fmt.Errorf("machine: LE write 0x%X 超界", addr)
	}
	m.Mem[addr] = value
	return nil
}

func (m *LEMachine) Read16(addr uint32) (uint16, error) {
	if uint64(addr)+2 > uint64(len(m.Mem)) {
		return 0, fmt.Errorf("machine: LE read16 0x%X 超界", addr)
	}
	return binary.LittleEndian.Uint16(m.Mem[addr:]), nil
}

func (m *LEMachine) Read32(addr uint32) (uint32, error) {
	if uint64(addr)+4 > uint64(len(m.Mem)) {
		return 0, fmt.Errorf("machine: LE read32 0x%X 超界", addr)
	}
	return binary.LittleEndian.Uint32(m.Mem[addr:]), nil
}

// PITDivisor 回通道 0 目前的除數；程式沒設過就回 BIOS 的預設 65,536。
func (m *LEMachine) PITDivisor() uint32 {
	if m.pit.divisor == 0 {
		return PITDefaultDivisor
	}
	return m.pit.divisor
}

// PITProgrammed 回報程式有沒有自己設過通道 0。
func (m *LEMachine) PITProgrammed() bool { return m.pit.writes > 0 }

// PITHz 回通道 0 目前的中斷頻率。
func (m *LEMachine) PITHz() float64 { return PITBaseHz / float64(m.PITDivisor()) }

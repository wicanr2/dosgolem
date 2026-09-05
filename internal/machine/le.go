package machine

import (
	"encoding/binary"
	"fmt"
)

// LEHeader 是 MZ 外殼內 Linear Executable 標頭的唯讀投影。
// 它只供能力盤點；成功解析不代表 Machine 已能執行保護模式程式。
type LEHeader struct {
	Offset            uint32
	ByteOrder         uint8
	WordOrder         uint8
	FormatLevel       uint32
	CPUType           uint16
	OSType            uint16
	ModuleFlags       uint32
	ModulePages       uint32
	EIPObject         uint32
	EIP               uint32
	ESPObject         uint32
	ESP               uint32
	PageSize          uint32
	ObjectTableOff    uint32
	ObjectCount       uint32
	ObjectPageMap     uint32
	FixupPageTable    uint32
	FixupRecordTable  uint32
	ImportModuleTable uint32
	ImportModuleCount uint32
	ImportProcTable   uint32
	DataPagesOffset   uint32
	Objects           []LEObject
}

// LEObject 保留 object table 每筆六個原始 dword。
type LEObject struct {
	VirtualSize    uint32
	RelocationBase uint32
	Flags          uint32
	PageTableIndex uint32
	PageCount      uint32
	Reserved       uint32
}

// InspectLE 以失敗即關閉方式解析 MZ 內嵌的 LE header 與 object table。
func InspectLE(data []byte) (*LEHeader, error) {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("machine: 不是含 e_lfanew 的 MZ 執行檔")
	}
	off := binary.LittleEndian.Uint32(data[0x3c:0x40])
	if off > uint32(len(data)) || uint64(off)+0xb0 > uint64(len(data)) {
		return nil, fmt.Errorf("machine: LE 標頭偏移 0x%X 超出 %d-byte 檔案", off, len(data))
	}
	b := data[off:]
	if b[0] != 'L' || b[1] != 'E' {
		return nil, fmt.Errorf("machine: MZ 新標頭不是 LE（0x%X 為 %02X %02X）", off, b[0], b[1])
	}
	u16 := func(p int) uint16 { return binary.LittleEndian.Uint16(b[p : p+2]) }
	u32 := func(p int) uint32 { return binary.LittleEndian.Uint32(b[p : p+4]) }
	h := &LEHeader{
		Offset: off, ByteOrder: b[2], WordOrder: b[3], FormatLevel: u32(4),
		CPUType: u16(8), OSType: u16(10), ModuleFlags: u32(0x10),
		ModulePages: u32(0x14), EIPObject: u32(0x18), EIP: u32(0x1c),
		ESPObject: u32(0x20), ESP: u32(0x24), PageSize: u32(0x28),
		ObjectTableOff: u32(0x40), ObjectCount: u32(0x44), ObjectPageMap: u32(0x48),
		FixupPageTable: u32(0x68), FixupRecordTable: u32(0x6c),
		ImportModuleTable: u32(0x70), ImportModuleCount: u32(0x74),
		ImportProcTable: u32(0x78), DataPagesOffset: u32(0x80),
	}
	start := uint64(off) + uint64(h.ObjectTableOff)
	size := uint64(h.ObjectCount) * 24
	if h.ObjectCount > uint32(len(data)/24) || start > uint64(len(data)) || size > uint64(len(data))-start {
		return nil, fmt.Errorf("machine: LE object table 超出檔案（offset=0x%X count=%d）", start, h.ObjectCount)
	}
	h.Objects = make([]LEObject, h.ObjectCount)
	for i := range h.Objects {
		p := int(start) + i*24
		h.Objects[i] = LEObject{
			VirtualSize:    binary.LittleEndian.Uint32(data[p:]),
			RelocationBase: binary.LittleEndian.Uint32(data[p+4:]),
			Flags:          binary.LittleEndian.Uint32(data[p+8:]),
			PageTableIndex: binary.LittleEndian.Uint32(data[p+12:]),
			PageCount:      binary.LittleEndian.Uint32(data[p+16:]),
			Reserved:       binary.LittleEndian.Uint32(data[p+20:]),
		}
	}
	return h, nil
}

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
	LastPageSize      uint32
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
	Pages             []LEPage
}

// LEPage 是 LE 專用四位元組 object page map entry。
type LEPage struct {
	Number uint32
	Flags  uint8
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
		LastPageSize:   u32(0x2c),
		ObjectTableOff: u32(0x40), ObjectCount: u32(0x44), ObjectPageMap: u32(0x48),
		FixupPageTable: u32(0x68), FixupRecordTable: u32(0x6c),
		ImportModuleTable: u32(0x70), ImportModuleCount: u32(0x74),
		ImportProcTable: u32(0x78), DataPagesOffset: u32(0x80),
	}
	if h.PageSize == 0 || h.ModulePages == 0 {
		return nil, fmt.Errorf("machine: LE page size 或 page count 是零")
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
	mapStart := uint64(off) + uint64(h.ObjectPageMap)
	mapSize := uint64(h.ModulePages) * 4
	if h.ModulePages > uint32(len(data)/4) || mapStart > uint64(len(data)) || mapSize > uint64(len(data))-mapStart {
		return nil, fmt.Errorf("machine: LE page map 超出檔案（offset=0x%X count=%d）", mapStart, h.ModulePages)
	}
	h.Pages = make([]LEPage, h.ModulePages)
	for i := range h.Pages {
		p := int(mapStart) + i*4
		h.Pages[i] = LEPage{Number: uint32(data[p])<<16 | uint32(data[p+1])<<8 | uint32(data[p+2]), Flags: data[p+3]}
	}
	return h, nil
}

// ObjectImage 重建一個尚未套 fixup 的 LE object 映像。index 使用 1-based object number。
func (h *LEHeader) ObjectImage(data []byte, index uint32) ([]byte, error) {
	if index == 0 || index > uint32(len(h.Objects)) {
		return nil, fmt.Errorf("machine: LE object number %d 超出 1..%d", index, len(h.Objects))
	}
	o := h.Objects[index-1]
	if uint64(o.VirtualSize) > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("machine: LE object %d 太大", index)
	}
	out := make([]byte, int(o.VirtualSize))
	for i := uint32(0); i < o.PageCount; i++ {
		mapIndex := uint64(o.PageTableIndex-1) + uint64(i)
		if o.PageTableIndex == 0 || mapIndex >= uint64(len(h.Pages)) {
			return nil, fmt.Errorf("machine: LE object %d 的 page map index 超界", index)
		}
		page := h.Pages[mapIndex]
		dst := uint64(i) * uint64(h.PageSize)
		if dst >= uint64(len(out)) {
			break
		}
		switch page.Flags {
		case 3: // PAGE_ZEROED
			continue
		case 0: // PAGE_VALID
		default:
			return nil, fmt.Errorf("machine: LE page flag %d 尚未支援", page.Flags)
		}
		if page.Number == 0 || page.Number > h.ModulePages {
			return nil, fmt.Errorf("machine: LE 實體 page number %d 超界", page.Number)
		}
		n := uint64(h.PageSize)
		if page.Number == h.ModulePages && h.LastPageSize != 0 {
			n = uint64(h.LastPageSize)
		}
		src := uint64(h.DataPagesOffset) + uint64(page.Number-1)*uint64(h.PageSize)
		if src > uint64(len(data)) || n > uint64(len(data))-src {
			return nil, fmt.Errorf("machine: LE page %d 資料超出檔案", page.Number)
		}
		if n > uint64(len(out))-dst {
			n = uint64(len(out)) - dst
		}
		copy(out[int(dst):int(dst+n)], data[int(src):int(src+n)])
	}
	return out, nil
}

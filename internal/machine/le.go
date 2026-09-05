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
	FixupSectionSize  uint32
	FixupChecksum     uint32
	LoaderSectionSize uint32
	LoaderChecksum    uint32
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
	FixupPageOffsets  []uint32
	Fixups            [][]LEFixup
}

// LEFixup 是尚未套用的原始 LE relocation record。
// Ordinal 依 TargetType 表示 object、import module 或 entry ordinal；NameOffset
// 只用於 import-by-name。SourceOffsets 保留 signed 16-bit 跨頁位置。
type LEFixup struct {
	SourceFlags   uint8
	TargetFlags   uint8
	SourceType    uint8
	TargetType    uint8
	SourceOffsets []int16
	Ordinal       uint32
	TargetOffset  uint32
	NameOffset    uint32
	Additive      uint32
	HasAdditive   bool
	Raw           []byte
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
		LastPageSize: u32(0x2c), FixupSectionSize: u32(0x30),
		FixupChecksum: u32(0x34), LoaderSectionSize: u32(0x38), LoaderChecksum: u32(0x3c),
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
	if err := h.inspectFixups(data); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *LEHeader) inspectFixups(data []byte) error {
	count := uint64(h.ModulePages) + 1
	start := uint64(h.Offset) + uint64(h.FixupPageTable)
	size := count * 4
	if count > uint64(len(data))/4 || start > uint64(len(data)) || size > uint64(len(data))-start {
		return fmt.Errorf("machine: LE fixup page table 超出檔案")
	}
	h.FixupPageOffsets = make([]uint32, count)
	for i := range h.FixupPageOffsets {
		p := int(start) + i*4
		h.FixupPageOffsets[i] = binary.LittleEndian.Uint32(data[p:])
		if i > 0 && h.FixupPageOffsets[i] < h.FixupPageOffsets[i-1] {
			return fmt.Errorf("machine: LE fixup page offsets 非單調")
		}
	}
	recordStart := uint64(h.Offset) + uint64(h.FixupRecordTable)
	recordBytes := uint64(h.FixupPageOffsets[len(h.FixupPageOffsets)-1])
	if recordStart > uint64(len(data)) || recordBytes > uint64(len(data))-recordStart {
		return fmt.Errorf("machine: LE fixup record table 超出檔案")
	}
	if h.ImportModuleTable != 0 {
		importStart := uint64(h.Offset) + uint64(h.ImportModuleTable)
		if recordStart+recordBytes > importStart {
			return fmt.Errorf("machine: LE fixup records 跨越 import module table")
		}
	}
	h.Fixups = make([][]LEFixup, h.ModulePages)
	for page := uint32(0); page < h.ModulePages; page++ {
		lo, hi := h.FixupPageOffsets[page], h.FixupPageOffsets[page+1]
		chunk := data[int(recordStart+uint64(lo)):int(recordStart+uint64(hi))]
		for len(chunk) != 0 {
			fixup, n, err := parseLEFixup(chunk)
			if err != nil {
				return fmt.Errorf("machine: LE fixup page %d record %d: %w", page+1, len(h.Fixups[page]), err)
			}
			h.Fixups[page] = append(h.Fixups[page], fixup)
			chunk = chunk[n:]
		}
	}
	return nil
}

func parseLEFixup(b []byte) (LEFixup, int, error) {
	var f LEFixup
	if len(b) < 2 {
		return f, 0, fmt.Errorf("record header 截斷")
	}
	f.SourceFlags, f.TargetFlags = b[0], b[1]
	f.SourceType, f.TargetType = b[0]&0x0f, b[1]&3
	if b[0]&0xc0 != 0 || (f.SourceType != 0 && f.SourceType != 2 && f.SourceType != 3 && f.SourceType != 5 && f.SourceType != 6 && f.SourceType != 7 && f.SourceType != 8) {
		return f, 0, fmt.Errorf("source flags 0x%02X 未定義", b[0])
	}
	if b[0]&0x10 != 0 && f.SourceType != 2 && f.SourceType != 3 && f.SourceType != 6 {
		return f, 0, fmt.Errorf("alias flag 不適用 source type %d", f.SourceType)
	}
	if b[1]&0x08 != 0 && (f.SourceType != 7 || (f.TargetType != 0 && f.TargetType != 3) || b[0]&0x20 != 0) {
		return f, 0, fmt.Errorf("非法 chaining 組合")
	}
	p := 2
	read8 := func() (uint32, error) {
		if p+1 > len(b) {
			return 0, fmt.Errorf("8-bit 欄位截斷")
		}
		v := uint32(b[p])
		p++
		return v, nil
	}
	read16 := func() (uint32, error) {
		if p+2 > len(b) {
			return 0, fmt.Errorf("16-bit 欄位截斷")
		}
		v := uint32(binary.LittleEndian.Uint16(b[p:]))
		p += 2
		return v, nil
	}
	read32 := func() (uint32, error) {
		if p+4 > len(b) {
			return 0, fmt.Errorf("32-bit 欄位截斷")
		}
		v := binary.LittleEndian.Uint32(b[p:])
		p += 4
		return v, nil
	}
	var sourceCount uint32
	var err error
	if b[0]&0x20 != 0 {
		sourceCount, err = read8()
	} else {
		var v uint32
		v, err = read16()
		f.SourceOffsets = []int16{int16(uint16(v))}
	}
	if err != nil {
		return f, 0, err
	}
	readOrdinal := read8
	if b[1]&0x40 != 0 {
		readOrdinal = read16
	}
	switch f.TargetType {
	case 0: // internal object
		if f.Ordinal, err = readOrdinal(); err == nil && f.SourceType != 2 {
			if b[1]&0x10 != 0 {
				f.TargetOffset, err = read32()
			} else {
				f.TargetOffset, err = read16()
			}
		}
	case 1: // import by ordinal
		if f.Ordinal, err = readOrdinal(); err == nil {
			if b[1]&0x80 != 0 {
				f.TargetOffset, err = read8()
			} else if b[1]&0x10 != 0 {
				f.TargetOffset, err = read32()
			} else {
				f.TargetOffset, err = read16()
			}
		}
	case 2: // import by name
		if f.Ordinal, err = readOrdinal(); err == nil {
			if b[1]&0x10 != 0 {
				f.NameOffset, err = read32()
			} else {
				f.NameOffset, err = read16()
			}
		}
	case 3: // internal entry table ordinal
		f.Ordinal, err = readOrdinal()
	}
	if err != nil {
		return f, 0, err
	}
	if b[1]&0x04 != 0 {
		f.HasAdditive = true
		if b[1]&0x20 != 0 {
			f.Additive, err = read32()
		} else {
			f.Additive, err = read16()
		}
		if err != nil {
			return f, 0, err
		}
	}
	if sourceCount > uint32((len(b)-p)/2) {
		return f, 0, fmt.Errorf("source list 截斷")
	}
	for i := uint32(0); i < sourceCount; i++ {
		v, e := read16()
		if e != nil {
			return f, 0, e
		}
		f.SourceOffsets = append(f.SourceOffsets, int16(uint16(v)))
	}
	f.Raw = append([]byte(nil), b[:p]...)
	return f, p, nil
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

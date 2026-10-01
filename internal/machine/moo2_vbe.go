package machine

import (
	"encoding/binary"
	"fmt"
)

// MOO2 首筆 VBE 控制器資訊是受限的合成視訊 BIOS 設定。
// 這些模式與 OEM 字串來自固定 DOSBox-X 輔助環境，不是原版遊戲資產或真機定值。
var moo2VBEModes = [...]uint16{
	0x100, 0x101, 0x102, 0x103, 0x104, 0x105, 0x106, 0x107,
	0x108, 0x109, 0x10a, 0x10b, 0x10c, 0x10d, 0x10e, 0x110,
	0x111, 0x113, 0x114, 0x116, 0x117, 0x10f, 0x112, 0x115,
	0x1f0, 0x1f1, 0x1f2, 0x151, 0x153, 0x15c, 0x159, 0x15d,
	0x15a, 0x160, 0x161, 0x162, 0x165, 0x136, 0x170, 0x172,
	0x175, 0x190, 0x201, 0x202, 0x203, 0x204, 0x205, 0x206,
	0x207, 0x208, 0x209, 0x20a, 0x213, 0xffff,
}

const (
	moo2VBEModeList = 0xc0100
	moo2VBEOEMName  = 0xc016c
)

// 固定 DOSBox-X 輔助環境對 0101h 的模式資訊；尾端保留呼叫者原值。
var moo2VBEMode0101 = [...]byte{
	0x9b, 0x00, 0x07, 0x00, 0x40, 0x00, 0x40, 0x00,
	0x00, 0xa0, 0x00, 0x00, 0x97, 0x01, 0x00, 0xc0,
	0x80, 0x02, 0x80, 0x02, 0xe0, 0x01, 0x08, 0x10,
	0x01, 0x08, 0x01, 0x04, 0x00, 0x05, 0x01, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0xe0,
}

func moo2VBEControllerInfo(h *DPMIHost, interrupt uint8, packet *[50]byte) (bool, error) {
	ax := binary.LittleEndian.Uint16(packet[28:])
	if interrupt != 0x10 || (ax != 0x4f00 && ax != 0x4f01) {
		return false, nil
	}
	if ax == 0x4f01 && binary.LittleEndian.Uint16(packet[24:]) != 0x0101 {
		return true, fmt.Errorf("MOO2 VBE 僅支援模式 0101h 資訊查詢")
	}
	if h.m == nil {
		return true, fmt.Errorf("MOO2 VBE 尚未綁定機器")
	}
	segment := binary.LittleEndian.Uint16(packet[34:])
	offset := binary.LittleEndian.Uint16(packet[0:])
	base := uint64(segment)*16 + uint64(offset)
	valid := false
	for _, block := range h.dosBlocks {
		end := uint64(block.Base) + uint64(block.Paras)*16
		if base >= uint64(block.Base) && base+256 <= end {
			valid = true
			break
		}
	}
	if !valid || base+256 > uint64(len(h.m.Mem)) {
		return true, fmt.Errorf("MOO2 VBE 目標不在活 DOS 記憶體區塊")
	}
	if base >= 0xa0000 || len(h.m.Mem) < moo2VBEOEMName+len("S3 Incorporated. Trio64")+1 {
		return true, fmt.Errorf("MOO2 VBE 低位記憶體或 BIOS ROM 不可用")
	}
	if ax == 0x4f01 {
		copy(h.m.Mem[int(base):], moo2VBEMode0101[:])
		binary.LittleEndian.PutUint16(packet[28:], 0x004f)
		binary.LittleEndian.PutUint16(packet[32:], 0x0002)
		return true, nil
	}
	if string(h.m.Mem[int(base):int(base)+4]) != "\x00\x00\x00\x00" {
		return true, fmt.Errorf("MOO2 VBE 首筆緩衝簽章與固定原版不符")
	}
	var header [20]byte
	copy(header[:4], "VESA")
	binary.LittleEndian.PutUint16(header[4:], 0x0200)
	binary.LittleEndian.PutUint32(header[6:], 0xc000016c)
	binary.LittleEndian.PutUint32(header[10:], 1)
	binary.LittleEndian.PutUint32(header[14:], 0xc0000100)
	binary.LittleEndian.PutUint16(header[18:], 32)
	for i, mode := range moo2VBEModes {
		binary.LittleEndian.PutUint16(h.m.Mem[moo2VBEModeList+i*2:], mode)
	}
	copy(h.m.Mem[moo2VBEOEMName:], "S3 Incorporated. Trio64\x00")
	copy(h.m.Mem[int(base):], header[:])
	binary.LittleEndian.PutUint16(packet[28:], 0x004f)
	binary.LittleEndian.PutUint16(packet[32:], 0x0002)
	return true, nil
}

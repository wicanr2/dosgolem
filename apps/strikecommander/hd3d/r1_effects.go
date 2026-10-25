package hd3d

import "encoding/binary"

// R1EffectsCapacity is the §9 fixed plan capacity. A successful plan is not
// permission to skip guest code: mappings, events and runtime ownership are
// deliberately outside this pure effects core.
const R1EffectsCapacity = 1536

// R1EffectsCPU preserves full architectural scalar values in original register
// order. It contains no CPU/Machine pointer or private execution state.
type R1EffectsCPU struct {
	GPR                                [8]uint32
	Seg                                [4]uint16 // ES, CS, SS, DS
	FS, GS, IP, Flags, EFlagsHi, FPUCW uint16
	Steps, Cycles                      uint64
	Halted, Segmented, DOSBoxCost      bool
	Model                              uint8
}
type R1EffectsInput struct {
	CPU           R1EffectsCPU // CPU-pre, already after the first Machine Steps increment
	ProgramSHA256 [32]byte
	Return        [4]byte
	Args          [12]byte
	Descriptor    [12]byte
	RowTable      [4]byte
	Vertices      [66]byte
}
type R1FlatCommand struct {
	XY                 [3][2]uint16
	Clip               [4]uint16 // xmin, ymin, xmax, ymax
	DestinationSegment uint16
	Color              uint8
}

// R1Effect expands in item, iteration, then low-byte-first order. Count zero
// is retained for either STOS instruction and produces no byte commit.
type R1Effect struct {
	Value                      uint32
	IP, Segment, Offset, Count uint16
	Width                      uint8
	padding                    [3]byte
}
type R1EffectsPlan struct {
	CPU                       R1EffectsCPU
	Effects                   [R1EffectsCapacity]R1Effect
	Command                   R1FlatCommand
	Instructions, CyclesDelta uint64
	ByteCommits               uint32
	EffectCount               uint16
	LastCS, LastIP            uint16
	LastOpcode                uint8
}
type R1EffectsReject uint8

const (
	R1EffectsOK R1EffectsReject = iota
	R1EffectsProgram
	R1EffectsCPUProfile
	R1EffectsStack
	R1EffectsArguments
	R1EffectsDescriptor
	R1EffectsClip
	R1EffectsTriangle
	R1EffectsPitch
	R1EffectsAddress
	R1EffectsTime
	R1EffectsCapacityExceeded
)

// R1EffectsProgramSHA returns the known full-function digest as an owned value.
// It identifies bytes only, and never grants a native execution permit.
func R1EffectsProgramSHA() [32]byte {
	return [32]byte{0xf3, 0x6e, 0x6b, 0xc5, 0x07, 0x78, 0x88, 0x20, 0x99, 0xbf, 0xff, 0x4a, 0x43, 0xaf, 0xe0, 0x63, 0x35, 0xfa, 0x3a, 0x41, 0xf5, 0xb0, 0x60, 0xab, 0x67, 0xa1, 0xe8, 0x4e, 0x81, 0x6e, 0xa7, 0xdd}
}
func (p *R1EffectsPlan) store(ip, seg, off uint16, width uint8, count uint16, value uint32) bool {
	if p.EffectCount >= R1EffectsCapacity {
		return false
	}
	if width == 1 {
		value &= 0xff
	} else if width == 2 {
		value &= 0xffff
	}
	p.Effects[p.EffectCount] = R1Effect{Value: value, IP: ip, Segment: seg, Offset: off, Width: width, Count: count}
	p.EffectCount++
	p.ByteCommits += uint32(width) * uint32(count)
	return true
}

// BuildR1Effects implements only READY 015 §9, the aligned unclipped flat-bottom
// n3 path of sub_2D5EC. Every rejection returns the entire zero plan.
func BuildR1Effects(in R1EffectsInput) (R1EffectsPlan, R1EffectsReject) {
	zero := R1EffectsPlan{}
	if in.ProgramSHA256 != R1EffectsProgramSHA() {
		return zero, R1EffectsProgram
	}
	c := in.CPU
	if c.IP != 0x000c || c.Model != 2 || c.Halted || c.Segmented || c.DOSBoxCost || c.EFlagsHi != 2 || c.Flags&0x100 != 0 || (c.Flags&0x7fd5)|2 != c.Flags {
		return zero, R1EffectsCPUProfile
	}
	sp := uint32(uint16(c.GPR[4]))
	if sp < 72 || sp > 65519 || sp%4 != 0 {
		return zero, R1EffectsStack
	}
	descriptorOff, count, vertexOff := binary.LittleEndian.Uint16(in.Args[0:]), binary.LittleEndian.Uint16(in.Args[2:]), binary.LittleEndian.Uint16(in.Args[4:])
	if count != 3 || uint32(vertexOff)+66 > 65535 {
		return zero, R1EffectsArguments
	}
	if uint32(descriptorOff)+12 > 65536 {
		return zero, R1EffectsDescriptor
	}
	dest, tableOff := binary.LittleEndian.Uint16(in.Descriptor[0:]), binary.LittleEndian.Uint16(in.Descriptor[2:])
	if uint32(tableOff)+4 > 65536 {
		return zero, R1EffectsDescriptor
	}
	var xy [3][2]uint16
	for i := range xy {
		xy[i] = [2]uint16{binary.LittleEndian.Uint16(in.Vertices[i*22:]), binary.LittleEndian.Uint16(in.Vertices[i*22+2:])}
	}
	clip := [4]uint16{binary.LittleEndian.Uint16(in.Descriptor[4:]), binary.LittleEndian.Uint16(in.Descriptor[6:]), binary.LittleEndian.Uint16(in.Descriptor[8:]), binary.LittleEndian.Uint16(in.Descriptor[10:])}
	if clip[0] > clip[2] || clip[1] > clip[3] || clip[2] > 319 || clip[3] > 199 {
		return zero, R1EffectsClip
	}
	for _, v := range xy {
		if v[0] < clip[0] || v[0] > clip[2] || v[1] < clip[1] || v[1] > clip[3] {
			return zero, R1EffectsClip
		}
	}
	if xy[1][1] != xy[2][1] || xy[0][1] >= xy[1][1] || xy[1][0] <= xy[2][0] {
		return zero, R1EffectsTriangle
	}
	// Widen before subtraction, multiplication, or address bounds.
	x0, y0 := int64(xy[0][0]), int64(xy[0][1])
	x1, x2, y1 := int64(xy[1][0]), int64(xy[2][0]), int64(xy[1][1])
	h := y1 - y0
	tab0 := int64(binary.LittleEndian.Uint16(in.RowTable[0:]))
	pitch := int64(binary.LittleEndian.Uint16(in.RowTable[2:])) - tab0
	if pitch <= 0 {
		return zero, R1EffectsPitch
	}
	if tab0+(y1+1)*pitch > 65535 || tab0+y1*pitch+int64(clip[2]) >= 65535 {
		return zero, R1EffectsAddress
	}
	sl := (x2 - x0) * 65536 / h
	sr := (x1 - x0) * 65536 / h
	initial := x0*65536 + 32768
	var k uint64
	var lastSpan int64
	for j := int64(0); j <= h; j++ {
		left, right := initial+j*sl, initial+j*sr
		n := (right >> 16) - (left >> 16) + 1
		k += uint64((n & 3) + (n >> 2))
		lastSpan = n
	}
	n := uint64(229 + 31*h)
	t := uint64(1115+122*h) + 5*k
	if c.Steps > ^uint64(0)-(n-1) || c.Cycles > ^uint64(0)-t {
		return zero, R1EffectsTime
	}
	var p R1EffectsPlan
	p.Command = R1FlatCommand{XY: xy, Clip: clip, DestinationSegment: dest, Color: in.Vertices[4]}
	p.Instructions, p.CyclesDelta = n, t
	ss := c.Seg[2]
	bp := uint16(sp - 2)
	add := func(ip uint16, off uint16, width uint8, value uint32) bool {
		return p.store(ip, ss, off, width, 1, value)
	}
	// Original ordered stack stores, including all three same-value OR bytes.
	if !add(0x000c, bp, 2, c.GPR[5]) ||
		!add(0x0010, bp-0x3a, 2, uint32(c.Seg[3])) ||
		!add(0x0011, bp-0x3c, 2, c.GPR[6]) ||
		!add(0x0012, bp-0x3e, 2, c.GPR[7]) ||
		!add(0x004e, bp-2, 2, uint32(clip[0])) ||
		!add(0x0054, bp-4, 2, uint32(clip[1])) ||
		!add(0x005a, bp-6, 2, uint32(clip[2])) ||
		!add(0x0060, bp-8, 2, uint32(clip[3])) ||
		!add(0x006b, bp-0xa, 2, uint32(pitch)) ||
		!add(0x0072, bp-0xc, 2, uint32(tab0)) ||
		!add(0x0082, bp-0x12, 2, uint32(vertexOff)) ||
		!add(0x0085, bp-0x14, 2, uint32(vertexOff)+66) ||
		!add(0x0088, bp-0x10, 2, 0) ||
		!add(0x00a9, bp-0x10, 1, 0) ||
		!add(0x00a9, bp-0x10, 1, 0) ||
		!add(0x00a9, bp-0x10, 1, 0) ||
		!add(0x00e1, bp-0x1a, 2, uint32(vertexOff)) ||
		!add(0x00e4, bp-0x1c, 2, uint32(vertexOff)) ||
		!add(0x00e7, bp-0x24, 2, uint32(y0)) ||
		!add(0x0150, bp-0x16, 2, uint32(vertexOff)) ||
		!add(0x0160, bp-0x1a, 2, uint32(vertexOff)+44) ||
		!add(0x0177, bp-0x1e, 2, uint32(h)) ||
		!add(0x0193, bp-0x30, 4, uint32(sl)) ||
		!add(0x0199, bp-0x28, 2, 0x8000) ||
		!add(0x019e, bp-0x26, 2, uint32(x0)) ||
		!add(0x01a4, bp-0x18, 2, uint32(vertexOff)) ||
		!add(0x01b4, bp-0x1c, 2, uint32(vertexOff)+22) ||
		!add(0x01cb, bp-0x20, 2, uint32(h)) ||
		!add(0x01e7, bp-0x34, 4, uint32(sr)) ||
		!add(0x01ed, bp-0x2c, 2, 0x8000) ||
		!add(0x01f2, bp-0x2a, 2, uint32(x0)) ||
		!add(0x0202, bp-0x22, 2, uint32(h)) ||
		!add(0x0263, bp-0x38, 4, uint32(in.Vertices[4])*0x01010101) {
		return zero, R1EffectsCapacityExceeded
	}
	for j := int64(0); j <= h; j++ {
		left, right := initial+j*sl, initial+j*sr
		a := left >> 16
		span := (right >> 16) - a + 1
		row := tab0 + (y0+j)*pitch
		if !add(0x0280, bp-0x42, 4, uint32(left)) ||
			!add(0x0282, bp-0x46, 4, c.GPR[7]&0xffff0000|uint32(row)) ||
			!p.store(0x02a6, dest, uint16(row+a), 1, uint16(span&3), uint32(in.Vertices[4])) ||
			!p.store(0x02ad, dest, uint16(row+a+(span&3)), 4, uint16(span>>2), uint32(in.Vertices[4])*0x01010101) ||
			!add(0x02b7, bp-0x22, 2, uint32(uint16(h-j-1))) {
			return zero, R1EffectsCapacityExceeded
		}
		if j < h-1 {
			if !add(0x02be, bp-0x1e, 2, uint32(h-j-1)) || !add(0x02c9, bp-0x20, 2, uint32(h-j-1)) {
				return zero, R1EffectsCapacityExceeded
			}
		}
	}
	p.CPU = c
	p.CPU.GPR[0] = uint32(initial + h*sl)
	p.CPU.GPR[1] = uint32(h) << 16
	p.CPU.GPR[2] = uint32(lastSpan)
	p.CPU.GPR[3] = uint32(initial + h*sr)
	p.CPU.GPR[4] = c.GPR[4]&0xffff0000 | uint32(sp+4)
	p.CPU.Seg[0] = dest
	p.CPU.Seg[1] = binary.LittleEndian.Uint16(in.Return[2:])
	p.CPU.IP = binary.LittleEndian.Uint16(in.Return[0:])
	p.CPU.Flags = (c.Flags &^ 0x0cd5) | 0x0094
	p.CPU.Steps = c.Steps + n - 1
	p.CPU.Cycles = c.Cycles + t
	p.LastCS, p.LastIP, p.LastOpcode = c.Seg[1], 0x02da, 0xcb
	return p, R1EffectsOK
}

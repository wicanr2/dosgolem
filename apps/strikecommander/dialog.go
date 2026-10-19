package strikecommander

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// 存讀檔對話框的繪圖事件（strike_commander 規格 004 §3.1、§3.2）。
// shape 繪製常式 +3A97:0000（檔案位移 40570h）在入口時，參數在 [SP+4] 起：
// 描述元近指標、x、y、圖檔遠指標（偏移、段）、條目號（RE 015 §3；dis-3A97.txt 的 [bp+6] 起）。
const (
	segShape = 0x3A97

	saveLoadSize  = 42014
	saveLoadGroup = 21 // 每組條目數；組基底 0、21、42（RE 015 §2）
)

// saveLoadSHA256 是 SAVELOAD.SHP 整檔的 SHA-256（RE 015 §1）。
var saveLoadSHA256 = mustHex("617f7270097913ea2c9841035fd201d4196d084e2408cc593eaf056d7f97ef6a")

// sigShapeEntry 是 +3A97:0000 起 16 bytes 的入口簽章（RE 015 §4 簽章表）。
var sigShapeEntry = []int{0xC8, 0x52, 0x01, 0x00, 0x56, 0x57, 0x1E, 0xFC, 0x16, 0x07, 0x1E, 0x0F, 0xA9, 0x8B, 0x76, 0x06}

func mustHex(s string) [32]byte {
	var out [32]byte
	if _, err := hex.Decode(out[:], []byte(s)); err != nil {
		panic(err)
	}
	return out
}

// ArtEvent 是一次被接受的繪圖事件（004 §3.1）。Entry、Base0 都是複本：Entry 是條目 e 的解碼陣列，
// Base0 是同一 k 的 0 組條目（條目 k）的解碼陣列，兩者尺寸都等於 §2.2 的元素尺寸 W×H。
type ArtEvent struct {
	Steps uint64
	G, K  int
	X, Y  int
	Order int
	W, H  int
	Entry []uint8
	Base0 []uint8
}

// shapeEntry 處理 IP == 0 時的 shape 繪製常式入口（004 §3.1）。依序：
//  1. CS:0000 起 16 bytes 等於入口簽章；base 已知時另要求 CS == base + 3A97h，
//     未知時以 CS − 3A97h 為候選，以 001 的 puts 簽章與其餘簽章驗證後採用；
//  2. 描述元 DS:d 的段是 FFFFh、矩形全螢幕；
//  3. 圖檔 p 起 u32 等於 42014，整檔 SHA-256 相符（每次都重算）；
//  4. g = e / 21、k = e mod 21：g 為 0、1 且 k 屬於 0 到 9 才產生事件；g == 2 記 skip；
//  5. 解碼條目 e 與條目 k，尺寸要等於 §2.2 的元素尺寸。
func (o *LabelObserver) shapeEntry(v View) {
	cs := v.CS()
	if o.baseKnown && o.baseOK {
		if cs != o.base+segShape {
			return
		}
		if !match(v, cs, 0, sigShapeEntry, 0) {
			return
		}
	} else {
		if !match(v, cs, 0, sigShapeEntry, 0) {
			return
		}
		cand := cs - segShape
		if why := o.checkBase(v, cand); why != "" {
			if !o.artBadCS[cs] {
				if o.artBadCS == nil {
					o.artBadCS = map[uint16]bool{}
				}
				o.artBadCS[cs] = true
				o.note(v.Steps(), "art", "shape 入口 %04X:0000 的候選 base %04X 驗證失敗：%s", cs, cand, why)
			}
			return
		}
		if o.baseKnown && cand != o.base {
			// 定位到不同的 base（005 §3.1 第 5 條）。
			o.dropVisible(v.Steps(), "shape 入口定位到不同的 base %04X（原 %04X），可見集合清空", cand, o.base)
		}
		o.base, o.baseKnown, o.baseOK = cand, true, true
		o.note(v.Steps(), "art", "由 shape 入口 %04X:0000 定位 base %04X", cs, cand)
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	d := rd16(v, ss, sp+4)
	x := int(int16(rd16(v, ss, sp+6)))
	y := int(int16(rd16(v, ss, sp+8)))
	pOff, pSeg := rd16(v, ss, sp+0x0A), rd16(v, ss, sp+0x0C)
	e := int(rd16(v, ss, sp+0x0E))
	if rd16(v, ds, d) != 0xFFFF || readRect(v, ds, d) != fullScreen {
		return // 裁切重畫（FCE5 等）、畫外緩衝與游標的 DB2E 都不收
	}
	base := lin(pSeg, pOff)
	if readLin32(v, base) != saveLoadSize {
		return
	}
	file := make([]byte, saveLoadSize)
	for i := range file {
		file[i] = v.Read8(base + uint32(i))
	}
	if sha256.Sum256(file) != saveLoadSHA256 {
		return
	}
	steps := v.Steps()
	g, k := e/saveLoadGroup, e%saveLoadGroup
	switch {
	case g == 2:
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d（第 2 組）未觀察，略過", e)
		return
	case g >= 3:
		return
	case k >= 10:
		return
	}
	el, _, _ := elementFor(k)
	ent, err := decodeEntry(file, e)
	if err != nil {
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d 解碼失敗：%v", e, err)
		return
	}
	b0, err := decodeEntry(file, k)
	if err != nil {
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d 解碼失敗：%v", k, err)
		return
	}
	if ent.W != el.w || ent.H != el.h || b0.W != el.w || b0.H != el.h {
		o.note(steps, "art", "SAVELOAD.SHP 條目 %d 尺寸 %d×%d（0 組 %d×%d），要 %d×%d，略過", e, ent.W, ent.H, b0.W, b0.H, el.w, el.h)
		return
	}
	o.artOrder++
	if o.OnArt != nil {
		o.OnArt(ArtEvent{Steps: steps, G: g, K: k, X: x, Y: y, Order: o.artOrder, W: ent.W, H: ent.H, Entry: ent.Pix, Base0: b0.Pix})
	}
}

// readLin32 讀線性位址 a 起的 u32。
func readLin32(v View, a uint32) uint32 {
	var b [4]byte
	for i := range b {
		b[i] = v.Read8(a + uint32(i))
	}
	return binary.LittleEndian.Uint32(b[:])
}

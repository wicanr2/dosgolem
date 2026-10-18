package strikecommander

import (
	"encoding/binary"
	"fmt"
)

// SAVELOAD.SHP 的 PAK 外層與 shape 解碼（strike_commander 規格 004 §3.2，RE 015 §3）。
//
// PAK 外層：u32 總長，接 u32 位移表；位移相對檔頭，高位元組是型別（本檔全為 0，未壓縮）。
// 條目數由第一個位移算出：(位移 0 − 4) / 4。繪圖常式 +3A97:0000 以 [p + 4(e+1)] 取第 e 個位移，
// 並以 [p+4] 的低字與 4(e+1) 比較擋掉越界條目（dis-3A97.txt 0021 到 004C）。
//
// shape：s16 right、left、top、bottom，寬 left+right+1、高 top+bottom+1。片段重複到 code = 0：
// u16 code、s16 dx、s16 dy，像素數 code>>1，起點 (left+dx, top+dy)；bit0 = 0 接原始色號，
// bit0 = 1 以子段湊滿：u8 s，長度 s>>1，s bit0 = 0 接原始色號，= 1 接 1 個色號重複。

// Shape 是一張解碼後的 shape：W×H 色號，逐列排列。Consumed 是解碼消耗的位元組數（含 8 bytes 檔頭與結尾的 0）。
type Shape struct {
	W, H     int
	Pix      []uint8
	Consumed int
}

// pakEntries 回 PAK 每個條目的 [起點, 終點)。終點是下一個條目的位移，最後一個是總長。
func pakEntries(file []byte) ([][2]int, error) {
	if len(file) < 8 {
		return nil, fmt.Errorf("PAK 長度 %d 太短", len(file))
	}
	total := int(binary.LittleEndian.Uint32(file))
	if total != len(file) {
		return nil, fmt.Errorf("PAK 總長 %d 不等於檔案長度 %d", total, len(file))
	}
	first := binary.LittleEndian.Uint32(file[4:])
	if first&0xFF000000 != 0 || first < 8 || first%4 != 0 || int(first) > len(file) {
		return nil, fmt.Errorf("PAK 第一個位移 %08X 不合法", first)
	}
	n := (int(first) - 4) / 4
	out := make([][2]int, n)
	for i := 0; i < n; i++ {
		raw := binary.LittleEndian.Uint32(file[4+4*i:])
		if raw&0xFF000000 != 0 {
			return nil, fmt.Errorf("PAK 條目 %d 的型別 %02X 不是 0（未壓縮）", i, raw>>24)
		}
		start := int(raw)
		end := total
		if i+1 < n {
			end = int(binary.LittleEndian.Uint32(file[4+4*(i+1):]) & 0xFFFFFF)
		}
		if start < int(first) || end < start || end > total {
			return nil, fmt.Errorf("PAK 條目 %d 的範圍 [%d, %d) 不合法", i, start, end)
		}
		out[i] = [2]int{start, end}
	}
	return out, nil
}

// decodeEntry 解碼 PAK 檔的第 e 個條目。
func decodeEntry(file []byte, e int) (Shape, error) {
	ents, err := pakEntries(file)
	if err != nil {
		return Shape{}, err
	}
	if e < 0 || e >= len(ents) {
		return Shape{}, fmt.Errorf("條目 %d 超出 %d 個", e, len(ents))
	}
	return decodeShape(file[ents[e][0]:ents[e][1]])
}

// decodeShape 依 RE 015 §3 解碼一張 shape。
func decodeShape(b []byte) (Shape, error) {
	p := 0
	need := func(n int) error {
		if p+n > len(b) {
			return fmt.Errorf("shape 在第 %d byte 截斷", p)
		}
		return nil
	}
	if err := need(8); err != nil {
		return Shape{}, err
	}
	right := int(int16(binary.LittleEndian.Uint16(b[0:])))
	left := int(int16(binary.LittleEndian.Uint16(b[2:])))
	top := int(int16(binary.LittleEndian.Uint16(b[4:])))
	bottom := int(int16(binary.LittleEndian.Uint16(b[6:])))
	w, h := left+right+1, top+bottom+1
	if w <= 0 || h <= 0 || w > 320 || h > 200 {
		return Shape{}, fmt.Errorf("shape 尺寸 %d×%d 不合法", w, h)
	}
	p = 8
	s := Shape{W: w, H: h, Pix: make([]uint8, w*h)}
	put := func(x, y int, c uint8) error {
		if x < 0 || x >= w || y < 0 || y >= h {
			return fmt.Errorf("shape 像素 (%d,%d) 超出 %d×%d", x, y, w, h)
		}
		s.Pix[y*w+x] = c
		return nil
	}
	for {
		if err := need(2); err != nil {
			return Shape{}, err
		}
		code := int(binary.LittleEndian.Uint16(b[p:]))
		p += 2
		if code == 0 {
			break
		}
		if err := need(4); err != nil {
			return Shape{}, err
		}
		dx := int(int16(binary.LittleEndian.Uint16(b[p:])))
		dy := int(int16(binary.LittleEndian.Uint16(b[p+2:])))
		p += 4
		n := code >> 1
		x, y := left+dx, top+dy
		if code&1 == 0 {
			if err := need(n); err != nil {
				return Shape{}, err
			}
			for i := 0; i < n; i++ {
				if err := put(x+i, y, b[p+i]); err != nil {
					return Shape{}, err
				}
			}
			p += n
			continue
		}
		for k := 0; k < n; {
			if err := need(1); err != nil {
				return Shape{}, err
			}
			sub := int(b[p])
			p++
			m := sub >> 1
			if m == 0 {
				return Shape{}, fmt.Errorf("shape 子段長度 0（第 %d byte）", p-1)
			}
			if sub&1 == 0 {
				if err := need(m); err != nil {
					return Shape{}, err
				}
				for i := 0; i < m; i++ {
					if err := put(x+k+i, y, b[p+i]); err != nil {
						return Shape{}, err
					}
				}
				p += m
			} else {
				if err := need(1); err != nil {
					return Shape{}, err
				}
				c := b[p]
				p++
				for i := 0; i < m; i++ {
					if err := put(x+k+i, y, c); err != nil {
						return Shape{}, err
					}
				}
			}
			k += m
		}
	}
	s.Consumed = p
	return s, nil
}

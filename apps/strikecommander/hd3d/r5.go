package hd3d

import "fmt"

// R5 地形三角形 23D9:0C79–116D，副程式 0943（邊設定）、0B6E（剪裁預步進）、0C07、0C30（逐列步進）、
// 0C53（深度比索引）、段函式 0072（無明暗、無剪裁）與取樣 0004。依 dis-23D9-b.txt 逐條移植。
// 0C79 的區域變數與三個邊結構（[bp−5Dh]、[bp−0BAh]、[bp−117h]，各 5Dh bytes）放在 frame 內，
// 以 byte 為單位模擬並追蹤是否初始化：讀到本次呼叫沒寫過的堆疊位置時記 fault（原版讀的是殘值）。
// 段函式 020C、04A6、0685 沒有移植（RE 025 §1.4：無動態樣本），遇到時記 fault。

func init() {
	regBranches(
		BranchDef{"R5", "R5.sort.xchg", "23D9:0CB3–0CF3", "三頂點依 y（同 y 依 x）排序有交換"},
		BranchDef{"R5", "R5.cull.x", "23D9:0D34–0D40", "三點同在 xmin 左或 xmax 右，返回"},
		BranchDef{"R5", "R5.cull.y", "23D9:0D43–0D5A", "整個在 ymin 上或 ymax 下，返回"},
		BranchDef{"R5", "R5.e.flat", "23D9:097C–0980", "0943：dy ≤ 0，邊不啟用"},
		BranchDef{"R5", "R5.e.out", "23D9:0989–099D", "0943：邊整條在視埠外"},
		BranchDef{"R5", "R5.e.on", "23D9:09A5–0B6A", "0943：邊啟用"},
		BranchDef{"R5", "R5.e.tclip", "23D9:0B39–0B5A", "0943：上剪裁預步進"},
		BranchDef{"R5", "R5.e.bclip", "23D9:0B67", "0943：下界夾到 ymax"},
		BranchDef{"R5", "R5.midrow", "23D9:0DDC–0DED", "兩條短邊都啟用：第一條 yend 減 1"},
		BranchDef{"R5", "R5.midrow.off", "23D9:0DE6", "兩條短邊都啟用且 yend ≤ ymin：關第一條"},
		BranchDef{"R5", "R5.flatLong", "23D9:0DF9–0E77", "長邊不啟用（讀邊 1 未初始化欄位）"},
		BranchDef{"R5", "R5.lr.e0left", "23D9:0EA8–0EBA／0EDC", "長邊在左"},
		BranchDef{"R5", "R5.lr.e0right", "23D9:0E94–0EA6／0EC6", "長邊在右"},
		BranchDef{"R5", "R5.d.topflat", "23D9:0F06–0F3F", "權重：上平"},
		BranchDef{"R5", "R5.d.botflat", "23D9:0F50–0F89", "權重：下平"},
		BranchDef{"R5", "R5.d.general", "23D9:0F8C–1086", "權重：一般"},
		BranchDef{"R5", "R5.seg.0072", "23D9:0072–020B", "段函式 0072：無明暗、無剪裁"},
		BranchDef{"R5", "R5.seg.020C", "23D9:020C–04A5", "段函式 020C：無明暗、有剪裁（未移植）"},
		BranchDef{"R5", "R5.seg.04A6", "23D9:04A6–0684", "段函式 04A6：有明暗、無剪裁（未移植）"},
		BranchDef{"R5", "R5.seg.0685", "23D9:0685–0942", "段函式 0685：有明暗、有剪裁（未移植）"},
		BranchDef{"R5", "R5.0072.empty", "23D9:007F–0081", "0072：右端 < 左端"},
		BranchDef{"R5", "R5.0072.idx100", "23D9:0128–012E", "0072：[13F6] ≥ 100h，不畫"},
		BranchDef{"R5", "R5.0072.s1", "23D9:018E–01A1", "0072：第 1 小段"},
		BranchDef{"R5", "R5.0072.s2", "23D9:01AE–01C3", "0072：第 2 小段"},
		BranchDef{"R5", "R5.0072.s3", "23D9:01D0–01E5", "0072：第 3 小段"},
		BranchDef{"R5", "R5.0072.s4", "23D9:01F0–0205", "0072：第 4 小段"},
		BranchDef{"R5", "R5.0072.ff", "23D9:0193–0195 等", "0072：透明 FFh 不寫"},
		BranchDef{"R5", "R5.0004.swap", "23D9:0036–003B", "0004：u 方向為主軸"},
		BranchDef{"R5", "R5.0004.zero", "23D9:0051–0053", "0004：取樣數 & FFh = 0"},
		BranchDef{"R5", "R5.chgL", "23D9:113E–114A", "左邊換到下一條"},
		BranchDef{"R5", "R5.chgR", "23D9:1157–1163", "右邊換到下一條"},
	)
}

// frame 模擬 SS 上以 bp 為基準的區域變數（位移為負）。
type frame struct {
	c    *context
	name string
	lo   int // 最小位移（負）
	b    []byte
	init []bool
}

func newFrame(c *context, name string, size int) *frame {
	return &frame{c: c, name: name, lo: -size, b: make([]byte, size), init: make([]bool, size)}
}

func (f *frame) ix(off int) int {
	i := off - f.lo
	if i < 0 || i >= len(f.b) {
		f.c.fault("%s frame 位移 %d 越界", f.name, off)
		return -1
	}
	return i
}

func (f *frame) r8(off int) byte {
	i := f.ix(off)
	if i < 0 {
		return 0
	}
	if !f.init[i] {
		// 原版讀的是呼叫前留在堆疊上的殘值，P2 沒有錄。記下來、以 0 代入繼續，
		// 群組因此標「不可重畫」（輸入不完整），但仍照常比對像素。
		f.c.noteUninit()
		f.c.uninit = append(f.c.uninit, fmt.Sprintf("seq %d %s：%s 讀未初始化的 [bp−%Xh]", f.c.seq, f.c.call.Kind, f.name, -off))
		if f.c.r5flat && !f.c.r5stepped {
			f.c.r5preUninit[fmt.Sprintf("[bp−%Xh]", -off)]++
		}
		if f.c.mut["uninitff"] { // 敏感度檢查：殘值改用 FFh
			return 0xFF
		}
		if f.c.mut["uninit5a"] {
			return 0x5A
		}
	}
	return f.b[i]
}
func (f *frame) w8(off int, v byte) {
	i := f.ix(off)
	if i < 0 {
		return
	}
	f.b[i] = v
	f.init[i] = true
}
func (f *frame) r16(off int) uint16 { return uint16(f.r8(off)) | uint16(f.r8(off+1))<<8 }
func (f *frame) w16(off int, v uint16) {
	f.w8(off, byte(v))
	f.w8(off+1, byte(v>>8))
}
func (f *frame) r32(off int) uint32 { return uint32(f.r16(off)) | uint32(f.r16(off+2))<<16 }
func (f *frame) w32(off int, v uint32) {
	f.w16(off, uint16(v))
	f.w16(off+2, uint16(v>>16))
}

// r5g 是 DS:13DA–1407 的全域。
type r5g struct {
	tblOff                 uint16 // 13DA（13DC 段）
	texOff, texSeg         uint16 // 13DE、13E0
	roww                   uint16 // 13E2
	descSeg, descTab       uint16 // 13E6、13E8
	xmin, ymin, xmax, ymax uint16 // 13EA、13EC、13EE、13F0
	shade                  uint32 // 13F2
	d13F6                  uint16
	lx, ly, lu, lv         uint16 // 13F8、13FA、13FC、13FE
	rx, ry, ru, rv         uint16 // 1400、1402、1404、1406
}

type r5 struct {
	c    *context
	k    *Call
	g    r5g
	fr   *frame
	tex  *capturedSegment
	tbl  []byte
	vofs [3]uint16 // 三個頂點的偏移（錄製順序）
}

// gs 讀頂點欄位：段一律 [bp+0Ah]，偏移 p 必須是三個參數頂點之一。
func (r *r5) gs16(p uint16, f uint16) uint16 {
	for i, o := range r.vofs {
		if o == p {
			b := r.k.Verts[i*22:]
			return uint16(b[f]) | uint16(b[f+1])<<8
		}
	}
	r.c.fault("R5 頂點指標 %04X 不在參數內", p)
	return 0
}
func (r *r5) gs32(p uint16, f uint16) uint32 {
	return uint32(r.gs16(p, f)) | uint32(r.gs16(p, f+2))<<16
}

// tw 讀 13DA 表的 word／dword（es:[si]，si 為 16 位元）。
func (r *r5) tw(si uint16) uint16 {
	o := int(uint16(si - r.g.tblOff))
	if o+1 >= len(r.tbl) {
		r.c.fault("R5 13DA 表讀出錄製範圍 +%X", o)
		return 0
	}
	return uint16(r.tbl[o]) | uint16(r.tbl[o+1])<<8
}
func (r *r5) td(si uint16) uint32 { return uint32(r.tw(si)) | uint32(r.tw(si+2))<<16 }

func (c *context) r5(k *Call) {
	r := &r5{c: c, k: k, fr: newFrame(c, "R5 0C79", 0x131)}
	// P2b：錄製器在入口讀了 SS:[SP−2−0A5h]、SS:[SP−2−9Fh]（bp = 入口 SP − 2），
	// 即 [bp−0A5h]、[bp−9Fh] 的殘值；填入 frame 當作已初始化（規格 012 第 5 版 §3.3.6）。
	if k.Aux.HasResid {
		r.fr.w16(-0xA5, k.Aux.ResA5)
		r.fr.w16(-0x9F, k.Aux.Res9F)
		c.r5residSeeded++
	}
	faults0 := len(c.faults)
	c.r5flat, c.r5stepped = false, false
	p := k.Params
	r.vofs = [3]uint16{p[1], p[3], p[5]}
	// 0C80–0CA6
	r.g.descSeg, r.g.descTab = k.Desc[0], k.Desc[1]
	r.g.xmin, r.g.ymin, r.g.xmax, r.g.ymax = k.Desc[2], k.Desc[3], k.Desc[4], k.Desc[5]
	r.g.shade = uint32(p[11]) | uint32(p[12])<<16
	r.g.texOff, r.g.texSeg = p[7], p[8]
	r.g.roww = p[9]
	r.g.tblOff = 0 // 錄製的 13DA 遠指標偏移，下面由 extra 取
	var err error
	if r.tex, err = c.capture(k.Tex.Pixels, "R5 texture"); err != nil {
		c.fault("R5 貼圖：%v", err)
		return
	}
	if r.tbl, err = k.Aux.Table, checkTable(k.Aux.Table); err != nil {
		c.fault("R5 13DA 表：%v", err)
		return
	}
	r.g.tblOff = k.Aux.TableOff
	fr := r.fr
	bpP8, bpPC, bpP10 := p[1], p[3], p[5] // [bp+8]、[bp+0Ch]、[bp+10h]

	// 0CA9–0CF3：排序
	bx, si, di := bpP8, bpPC, bpP10
	less := func(a, b uint16) bool { // a 應在 b 之後（要交換）時回 true
		ya, yb := int16(r.gs16(a, 2)), int16(r.gs16(b, 2))
		if ya > yb {
			return true
		}
		if ya < yb {
			return false
		}
		return int16(r.gs16(a, 0)) > int16(r.gs16(b, 0))
	}
	if less(bx, di) {
		bx, di = di, bx
		c.br.hit("R5.sort.xchg")
	}
	if less(si, di) {
		si, di = di, si
		c.br.hit("R5.sort.xchg")
	}
	if less(bx, si) {
		bx, si = si, bx
		c.br.hit("R5.sort.xchg")
	}
	bpP8, bpPC, bpP10 = bx, si, di
	// 0CFE–0D40
	var cl, ch byte
	for _, v := range []uint16{bx, si, di} {
		if !(int16(r.g.xmin) <= int16(r.gs16(v, 0))) {
			cl++
		}
	}
	for _, v := range []uint16{bx, si, di} {
		if !(int16(r.g.xmax) >= int16(r.gs16(v, 0))) {
			ch++
		}
	}
	fr.w16(-0x12F, uint16(cl)|uint16(ch)<<8)
	if cl == 3 || ch == 3 {
		c.br.hit("R5.cull.x")
		return
	}
	// 0D43–0D5A
	if !(int16(r.gs16(bx, 2)) <= int16(r.g.ymax)) || !(int16(r.gs16(di, 2)) >= int16(r.g.ymin)) {
		c.br.hit("R5.cull.y")
		return
	}
	// 0D5D–0D8C
	fr.w16(-0x121, r.idx0C53(r.gs32(si, 0xE), r.gs32(bx, 0xE)))
	fr.w16(-0x123, r.idx0C53(r.gs32(di, 0xE), r.gs32(si, 0xE)))
	fr.w16(-0x125, r.idx0C53(r.gs32(di, 0xE), r.gs32(bx, 0xE)))
	// 0D90–0DCB
	r.edge0943(-0x5D, fr.r16(-0x125), bpP8, bpP10)
	r.edge0943(-0xBA, fr.r16(-0x121), bpP8, bpPC)
	r.edge0943(-0x117, fr.r16(-0x123), bpPC, bpP10)
	// 0DCE–0DED
	if fr.r8(-0xBA) != 0 && fr.r8(-0x117) != 0 {
		ax := fr.r16(-0xAB)
		if int16(ax) > int16(r.g.ymin) {
			c.br.hit("R5.midrow")
			fr.w16(-0xAB, ax-1)
		} else {
			c.br.hit("R5.midrow.off")
			fr.w8(-0xBA, 0)
		}
	}
	var left, right, nextL, nextR int // bp 相對指標，0 為空
	if fr.r8(-0x5D) == 0 {            // 0DF1
		c.br.hit("R5.flatLong")
		c.r5flat = true
		c.r5flatCalls++
		fr.w8(-0xBA, 1)
		fr.w8(-0x5D, 1)
		b := bpP10
		fr.w32(-0x58, sx16(r.gs16(b, 0))<<16)
		fr.w32(-0x3C, uint32(r.gs16(b, 6))<<8)
		fr.w32(-0x1E, uint32(r.gs16(b, 8))<<8)
		left = -0xBA
		nextL = 0
		fr.w32(-0xBA+0x21, uint32(fr.r16(-0xA5))<<8) // 0E3F：[bp−0A5h]（邊 1 +15h）
		fr.w32(-0xBA+0x3F, uint32(fr.r16(-0x9F))<<8) // 0E4F：[bp−9Fh]（邊 1 +1Bh）
		right = -0x5D
		nextR = 0
		fr.w16(-0x127, fr.r16(-0x125))
		fr.w16(-0x129, 0)
	} else {
		e0, e1, e2 := -0x5D, -0xBA, -0x117
		if fr.r8(e1) != 0 { // 0E85
			if int32(fr.r32(e1+9)) > int32(fr.r32(e0+9)) { // 0E8E jg
				c.br.hit("R5.lr.e0left")
				left, nextL, right, nextR = e0, 0, e1, e2
			} else {
				c.br.hit("R5.lr.e0right")
				left, nextL, right, nextR = e1, e2, e0, 0
			}
		} else {
			if !(int32(fr.r32(e2+9)) < int32(fr.r32(e0+9))) { // 0EC0 jl
				c.br.hit("R5.lr.e0right")
				left, nextL, right, nextR = e2, 0, e0, 0
			} else {
				c.br.hit("R5.lr.e0left")
				left, nextL, right, nextR = e0, 0, e2, 0
			}
		}
		// 0EF0–
		if fr.r8(-0xBA) == 0 && fr.r16(-0xA7) == 0 {
			c.br.hit("R5.d.topflat")
			fr.w16(-0x127, fr.r16(-0x121)<<7)
			fr.w16(-0x12B, 0x4000)
			cx := fr.r16(left + 0x11)
			q := r.div16sbb(0x4000, fr.r16(-0x127), cx, "R5 0F21")
			fr.w16(-0x129, q)
			fr.w16(-0x12D, 0)
			if cxx := fr.r16(-0xA7); cxx != 0 {
				fr.w16(-0x127, fr.r16(-0x127)+uint16(int16(fr.r16(-0x129))*int16(cxx)))
			}
		} else if fr.r8(-0x117) == 0 && fr.r16(-0x104) == 0 {
			c.br.hit("R5.d.botflat")
			fr.w16(-0x127, 0x4000)
			ax := fr.r16(-0x123) << 7
			fr.w16(-0x12B, ax)
			cx := fr.r16(left + 0x11)
			q := r.div16sbb(ax, fr.r16(-0x127), cx, "R5 0F6B")
			fr.w16(-0x129, q)
			fr.w16(-0x12D, 0)
			if cxx := fr.r16(-0x104); cxx != 0 {
				fr.w16(-0x127, fr.r16(-0x127)+uint16(int16(fr.r16(-0x129))*int16(cxx)))
			}
		} else {
			c.br.hit("R5.d.general")
			fr.w16(-0x127, 0x4000)
			b, s, d := bpP8, bpPC, bpP10
			ecx := sx16(r.gs16(d, 2) - r.gs16(b, 2))
			eax := sx16(r.gs16(s, 2) - r.gs16(b, 2))
			edx := r.gs32(b, 0xE) - r.gs32(s, 0xE)
			prod := imul64(eax, edx)
			q, _ := c.idiv64(prod, int32(ecx), "R5 0FC1") // 結果在 0FE6 之後不用，只為 #DE 判斷
			_ = q
			ax := uint16(0x4000) // 0FE3；0FE6 jmp 0FFE 跳過 0FE8–0FFB
			if left != -0x5D {   // 1006–100F
				ax = -ax
				ax += 0x8000
			}
			if int16(ax) < 0x0C00 {
				ax = 0x0C00
			}
			if int16(ax) > 0x7380 {
				ax = 0x7380
			}
			fr.w16(-0x12B, ax)
			if fr.r8(-0xBA) != 0 { // 1026
				q := r.div16sbb(fr.r16(-0x12B), fr.r16(-0x127), fr.r16(-0xA9), "R5 1037")
				fr.w16(-0x129, q)
				if cxx := fr.r16(-0xA7); cxx != 0 {
					fr.w16(-0x127, fr.r16(-0x127)+uint16(int16(q)*int16(cxx)))
				}
			}
			if fr.r8(-0x117) != 0 { // 104D
				q := r.div16sbb(0x4000, fr.r16(-0x12B), fr.r16(-0x106), "R5 105D")
				fr.w16(-0x12D, q)
				if cxx := fr.r16(-0x104); cxx != 0 {
					fr.w16(-0x12B, fr.r16(-0x12B)+uint16(int16(q)*int16(cxx)))
				}
			}
			if fr.r8(-0xBA) == 0 { // 1073
				fr.w16(-0x127, fr.r16(-0x12B))
				fr.w16(-0x129, fr.r16(-0x12D))
			}
		}
	}
	// 108A–10A7：段函式
	seg := uint16(0x0072)
	alt := uint16(0x020C)
	if r.g.shade != 0 {
		seg, alt = 0x04A6, 0x0685
	}
	if fr.r16(-0x12F) != 0 {
		seg = alt
	}
	// 10AB–1167：逐列
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R5 列迴圈超過上限")
			return
		}
		s, d := left, right
		r.g.lx = uint16(fr.r32(s+5) >> 16)
		r.g.ly = fr.r16(s + 0xD)
		r.g.lu = uint16((fr.r32(s+0x21) + 0x8000) >> 16)
		r.g.lv = uint16((fr.r32(s+0x3F) + 0x8000) >> 16)
		r.g.rx = uint16(fr.r32(d+5) >> 16)
		r.g.ry = fr.r16(d + 0xD)
		r.g.ru = uint16((fr.r32(d+0x21) + 0x8000) >> 16)
		r.g.rv = uint16((fr.r32(d+0x3F) + 0x8000) >> 16)
		r.g.d13F6 = fr.r16(-0x127) >> 7
		switch seg {
		case 0x0072:
			c.br.hit("R5.seg.0072")
			r.seg0072()
		case 0x020C:
			c.br.hit("R5.seg.020C")
			c.fault("R5 段函式 020C 未移植（無樣本變體）")
			return
		case 0x04A6:
			c.br.hit("R5.seg.04A6")
			c.fault("R5 段函式 04A6 未移植（無樣本變體）")
			return
		case 0x0685:
			c.br.hit("R5.seg.0685")
			c.fault("R5 段函式 0685 未移植（無樣本變體）")
			return
		}
		if len(c.faults) > faults0 {
			return
		}
		fr.w16(-0x127, fr.r16(-0x127)+fr.r16(-0x129))
		// 1137–1167：[bp−11Dh]／[bp−11Fh] 換用後不清為 0（原版行為）
		if r.step0C30(s) { // 1139；CF = 1 表示這條邊走完
			left = nextL
			if left == 0 {
				return
			}
			c.br.hit("R5.chgL")
			fr.w16(-0x129, fr.r16(-0x12D))
		}
		if r.step0C30(d) {
			right = nextR
			if right == 0 {
				return
			}
			c.br.hit("R5.chgR")
			fr.w16(-0x129, fr.r16(-0x12D))
		}
	}
}

// div16sbb：mov ax,a；sub ax,b；sbb dx,dx；idiv cx。
func (r *r5) div16sbb(a, b, cx uint16, where string) uint16 {
	ax := a - b
	var dx uint16
	if a < b {
		dx = 0xFFFF
	}
	q, _ := r.c.idiv32(int32(uint32(dx)<<16|uint32(ax)), int16(cx), where)
	return uint16(q)
}

// idx0C53：23D9:0C53–0C78。eax、ecx 為兩個 +0Eh dword；回傳夾在 18h–E7h 的 ax。
func (r *r5) idx0C53(eax, ecx uint32) uint16 {
	dx := uint16(0x80)
	ecx += eax
	if ecx != 0 {
		q, _ := r.c.idiv64(int64(int32(eax<<8)), int32(ecx), "R5 0C61")
		dx = uint16(q)
	}
	ax := dx
	if !(int16(ax) >= 0x18) {
		ax = 0x18
	}
	if !(int16(ax) <= 0xE7) {
		ax = 0xE7
	}
	return ax
}

// edge0943：23D9:0943–0B6D。s 是邊結構的 bp 位移，idx 是 [bp+6]，va、vb 是頂點。
func (r *r5) edge0943(s int, idx uint16, va, vb uint16) {
	c, fr := r.c, r.fr
	fr.w8(s, 0)
	fr.w16(s+0xD, r.gs16(va, 2))
	fr.w16(s+0xF, r.gs16(vb, 2))
	fr.w16(s+0x13, 0)
	fr.w32(s+5, sx16(r.gs16(va, 0))<<16|0x8000)
	ax := r.gs16(vb, 2) - r.gs16(va, 2)
	if !(int16(r.gs16(vb, 2)) > int16(r.gs16(va, 2))) { // 0980 jle（sub 的有號比）
		c.br.hit("R5.e.flat")
		return
	}
	dy := ax // [bp−2]
	fr.w16(s+0x11, ax+1)
	if int16(r.gs16(va, 2)) > int16(r.g.ymax) || !(int16(r.gs16(vb, 2)) >= int16(r.g.ymin)) {
		c.br.hit("R5.e.out")
		fr.w16(s+0x13, 1)
		return
	}
	c.br.hit("R5.e.on")
	fr.w8(s, 1)
	fr.w32(s+0x17, uint32(r.gs16(vb, 6))-uint32(r.gs16(va, 6)))
	fr.w32(s+0x1D, uint32(r.gs16(vb, 8))-uint32(r.gs16(va, 8)))
	U, V := s+0x21, s+0x3F
	ecx := int32(int16(dy))
	vq, _ := c.idiv64(int64(int32(fr.r32(s+0x1D)))<<8, ecx, "R5 09E8")
	uq, _ := c.idiv64(int64(int32(fr.r32(s+0x17)))<<8, ecx, "R5 09FC")
	si := r.g.tblOff + idx*2
	for k := 0; k < 4; k++ {
		w := uint32(r.tw(si))
		fr.w32(U+4+4*k, shrd12(imul64(w, uint32(uq))))
		fr.w32(V+4+4*k, shrd12(imul64(w, uint32(vq))))
		si += 0x200
	}
	si += idx * 2
	e := uint32(uint64(r.td(si))*uint64(dy)) >> 12 // mul ecx（無號），取低 32 再 shr 12
	ax = uint16(e)
	fr.w16(U+0x18, ax)
	fr.w16(V+0x18, ax)
	cx := ax
	ax >>= 1
	fr.w16(U+0x16, ax)
	fr.w16(V+0x16, ax)
	ax = (dy + cx) >> 1
	fr.w16(U+0x1A, ax)
	fr.w16(V+0x1A, ax)
	fr.w16(U+0x1C, dy+1)
	fr.w16(V+0x1C, dy+1)
	fr.w32(U, uint32(r.gs16(va, 6))<<8)
	fr.w32(V, uint32(r.gs16(va, 8))<<8)
	fr.w16(U+0x14, 0)
	fr.w16(V+0x14, 0)
	// 0B07–0B2A
	dxq, _ := c.idiv64(int64(int32(sx16(r.gs16(vb, 0)-r.gs16(va, 0))<<16)), int32(uint32(dy)), "R5 0B27")
	fr.w32(s+9, uint32(dxq))
	// 0B2E–0B5A
	y := fr.r16(s + 0xD)
	if !(int16(y) >= int16(r.g.ymin)) {
		c.br.hit("R5.e.tclip")
		fr.w16(s+0xD, r.g.ymin)
		n := r.g.ymin - y
		fr.w16(s+0x13, n)
		fr.w32(s+5, fr.r32(s+5)+uint32(imul64(sx16(n), fr.r32(s+9))))
		r.skip0B6E(U, n)
		r.skip0B6E(V, n)
	}
	if !(int16(r.g.ymax) >= int16(fr.r16(s+0xF))) {
		c.br.hit("R5.e.bclip")
		fr.w16(s+0xF, r.g.ymax)
	}
}

func shrd12(p int64) uint32 { return uint32(uint64(p) >> 12) }

// skip0B6E：23D9:0B6E–0C06。b 是子結構，cx 是要略過的列數。
func (r *r5) skip0B6E(b int, cx uint16) {
	fr := r.fr
	ax := fr.r16(b + 0x14)
	cx += ax
	for k, lim := range []int{0x16, 0x18, 0x1A} {
		if int16(ax) < int16(fr.r16(b+lim)) && int16(cx) > int16(ax) {
			dx := fr.r16(b + lim)
			if !(int16(cx) > int16(dx)) {
				dx = cx
			}
			fr.w32(b, fr.r32(b)+uint32(imul64(sx16(dx-ax), fr.r32(b+4+4*k))))
			ax = dx
			fr.w16(b+0x14, ax)
		}
	}
	if int16(cx) > int16(ax) {
		dx := fr.r16(b + 0x1C)
		if !(int16(cx) > int16(dx)) {
			dx = cx
		}
		fr.w32(b, fr.r32(b)+uint32(imul64(sx16(dx-ax), fr.r32(b+0x10))))
		fr.w16(b+0x14, dx)
	}
}

// step0C07：23D9:0C07–0C2F。
func (r *r5) step0C07(b int) {
	fr := r.fr
	ax := fr.r16(b + 0x14)
	ecx := fr.r32(b + 4)
	if !(int16(ax) < int16(fr.r16(b+0x16))) {
		ecx = fr.r32(b + 8)
		if !(int16(ax) < int16(fr.r16(b+0x18))) {
			ecx = fr.r32(b + 0xC)
			if !(int16(ax) < int16(fr.r16(b+0x1A))) {
				ecx = fr.r32(b + 0x10)
			}
		}
	}
	fr.w32(b, fr.r32(b)+ecx)
	fr.w16(b+0x14, fr.r16(b+0x14)+1)
}

// step0C30：23D9:0C30–0C52。回傳 CF（y 舊值 ≥ yend，無號）。
func (r *r5) step0C30(b int) bool {
	fr := r.fr
	r.c.r5stepped = true
	fr.w32(b+5, fr.r32(b+5)+fr.r32(b+9))
	r.step0C07(b + 0x21)
	r.step0C07(b + 0x3F)
	ax := fr.r16(b + 0xD)
	fr.w16(b+0xD, ax+1)
	return !(ax < fr.r16(b+0xF))
}

// seg0072：23D9:0072–020B。
func (r *r5) seg0072() {
	c, g := r.c, &r.g
	ax := g.rx - g.lx
	if !(int16(g.rx) >= int16(g.lx)) { // 007F jge（sub 的有號比）
		c.br.hit("R5.0072.empty")
		return
	}
	count := ax + 1 // [bp−162h]
	// 0089–009F：取樣
	buf, n := r.sample0004(g.lu, g.lv, g.ru, g.rv)
	// 00A3–00BE
	// movzx eax,n；dec eax；shl eax,16；edx = 0；div ecx（無號，商不會溢位）
	ratio := ((uint32(n) - 1) << 16) / uint32(count)
	var loc [0x1E]byte // [bp−1Eh] 起
	put32 := func(o int, v uint32) {
		loc[o], loc[o+1], loc[o+2], loc[o+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	put16 := func(o int, v uint16) { loc[o], loc[o+1] = byte(v), byte(v>>8) }
	get16 := func(o int) uint16 { return uint16(loc[o]) | uint16(loc[o+1])<<8 }
	si := g.tblOff + g.d13F6*2
	for k := 0; k < 4; k++ {
		w := uint64(r.tw(si))
		put32(4+4*k, uint32((w*uint64(ratio))>>12)) // mul ecx；shrd eax,edx,0Ch
		si += 0x200
	}
	if !(int16(g.d13F6) < 0x100) { // 0128
		c.br.hit("R5.0072.idx100")
		return
	}
	si += g.d13F6 * 2
	e := uint32(uint64(r.td(si))*uint64(count)) >> 12 // mul ecx；shr eax,0Ch（只取低 32）
	put16(0x18, uint16(e))
	cx := uint16(e)
	put16(0x16, uint16(e)>>1)
	put16(0x1A, (count+cx)>>1)
	put32(0, 0)
	put16(0x14, 0)
	// 0164–0174
	di := c.tab(g.ly) + g.lx
	// 0178–0207
	bx := 0 // 相對 [bp−15Eh]
	write := func(nn int, bp16, dx uint16) {
		for i := 0; i < nn; i++ {
			if bx < 0 || bx >= n {
				c.fault("R5 0072 讀暫存列 [%d] 超出取樣數 %d", bx, n)
				return
			}
			al := buf[bx]
			if al == 0xFF {
				c.br.hit("R5.0072.ff")
			} else {
				c.w(di, al)
				if c.r5flat && c.r5stepped {
					c.r5flatPostWrites++
				}
				if c.r5flat {
					c.r5flatWrites++
				}
			}
			di++
			t := uint32(cx) + uint32(bp16)
			cx = uint16(t)
			bx = int(int16(uint16(bx) + dx + uint16(t>>16)))
		}
	}
	// 段 1
	if ah := int8(loc[0x16]); ah > 0 {
		c.br.hit("R5.0072.s1")
		write(int(ah), get16(4), get16(6))
	}
	// 段 2：dx = [si+0Ah]、bp = [si+4]
	if a := get16(0x18) - get16(0x16); int16(a) > 0 {
		c.br.hit("R5.0072.s2")
		write(cnt8(byte(a)), get16(4), get16(0xA))
	}
	if a := get16(0x1A) - get16(0x18); int16(a) > 0 {
		c.br.hit("R5.0072.s3")
		write(cnt8(byte(a)), get16(0xC), get16(0xE))
	}
	if a := count - get16(0x1A); int16(a) > 0 {
		c.br.hit("R5.0072.s4")
		write(cnt8(byte(a)), get16(0x10), get16(0x12))
	}
}

// cnt8：mov ah,al；…；dec ah；jne：al = 0 時跑 256 次。
func cnt8(a byte) int {
	if a == 0 {
		return 256
	}
	return int(a)
}

// sample0004：23D9:0004–0071。ax = u0、bx = v0、cx = u1、dx = v1。回傳暫存列與取樣數。
func (r *r5) sample0004(u0, v0, u1, v1 uint16) ([]byte, int) {
	c, g := r.c, &r.g
	si := g.texOff + u0 + g.roww*v0
	dvv := v1 - v0
	bx := dvv
	dx := g.roww
	if int16(dvv) < 0 { // jns（sub 結果的符號）
		dx = -dx
		bx = -bx
	}
	bx++
	ax := u1 - u0
	cx := uint16(1)
	if int16(ax) < 0 {
		cx = -cx
		ax = -ax
	}
	ax++
	if !(int16(ax) <= int16(bx)) { // 0036 cmp ax,bx；jle
		c.br.hit("R5.0004.swap")
		ax, bx = bx, ax
		dx, cx = cx, dx
	}
	major, minorStep, majorStep := bx, cx, dx
	cnt := bx & 0xFF
	acc := bx
	minor := ax
	out := make([]byte, 0, 256)
	if cnt == 0 {
		c.br.hit("R5.0004.zero")
		return out, 0
	}
	for {
		out = append(out, c.readCapture(r.tex, si))
		cnt--
		if cnt == 0 {
			break
		}
		si += majorStep
		acc -= minor
		if int16(acc) > 0 {
			continue
		}
		acc += major
		si += minorStep
	}
	return out, len(out)
}

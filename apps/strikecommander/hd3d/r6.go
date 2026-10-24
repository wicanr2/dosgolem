package hd3d

// R6 地平線 56F1:01ED–0452，副程式 00D0–014F（水平漸層帶）、0150–01EC（垂直漸層帶）。
// 依 dis-56F1-b.txt:161–492 逐條移植。DF 假設為 0（本常式不做 cld，呼叫端的狀態，見報告）。

func init() {
	regBranches(
		BranchDef{"R6", "R6.B6z", "56F1:0258–0260", "兩端點相同，直接返回"},
		BranchDef{"R6", "R6.B6h+", "56F1:0263–0297", "水平線，DX 低字 ≥ 0：S=−P、B=I"},
		BranchDef{"R6", "R6.B6h-", "56F1:0263–0297", "水平線，DX 低字 < 0：S=P、B=−I"},
		BranchDef{"R6", "R6.B6s", "56F1:0299–02CD", "斜線：S=(DX<<8)/(−DY)"},
		BranchDef{"R6", "R6.col.dyge0", "56F1:02D1–02E7", "DY 低字 ≥ 0：xchg，第一色 38BE"},
		BranchDef{"R6", "R6.col.dylt0", "56F1:02D1–02E7", "DY 低字 < 0：不 xchg，第一色 38C0"},
		BranchDef{"R6", "R6.row.eneg", "56F1:030C", "每列 e < xmin：整列第二色"},
		BranchDef{"R6", "R6.row.egtw", "56F1:0310–0313", "e > W：第一色 W 點"},
		BranchDef{"R6", "R6.row.elew", "56F1:0315", "0 ≤ e ≤ W：第一色 e 點"},
		BranchDef{"R6", "R6.row.second", "56F1:0324–0333", "第二色點數 > 0"},
		BranchDef{"R6", "R6.B6r", "56F1:0350–03CD", "|DX| < |DY|：逐列漸層"},
		BranchDef{"R6", "R6.B6c", "56F1:03D0–044D", "|DX| ≥ |DY|：逐欄漸層"},
		BranchDef{"R6", "R6.B6r.fwd", "56F1:0399–03AA", "B6r DY ≥ 0：00D0 正向"},
		BranchDef{"R6", "R6.B6r.rev", "56F1:03AC–03BB", "B6r DY < 0：00D0 反向"},
		BranchDef{"R6", "R6.B6c.fwd", "56F1:0419–042A", "B6c DX ≥ 0：0150 正向"},
		BranchDef{"R6", "R6.B6c.rev", "56F1:042C–043B", "B6c DX < 0：0150 反向"},
		BranchDef{"R6", "R6.h.skip", "56F1:00D2–00DC", "00D0 帶全在視埠外"},
		BranchDef{"R6", "R6.h.fwd", "56F1:0127–014B", "00D0 正向"},
		BranchDef{"R6", "R6.h.rev", "56F1:00FC–0125", "00D0 反向"},
		BranchDef{"R6", "R6.h.lclip", "56F1:0102–010A/012A–0132", "00D0 左剪裁"},
		BranchDef{"R6", "R6.h.rclip", "56F1:010E–0116/0136–013E", "00D0 右剪裁"},
		BranchDef{"R6", "R6.h.empty", "56F1:011A/0142", "00D0 剪裁後點數 ≤ 0"},
		BranchDef{"R6", "R6.v.skip", "56F1:0152–015E", "0150 帶全在視埠外"},
		BranchDef{"R6", "R6.v.fwd", "56F1:01B5–01E8", "0150 正向"},
		BranchDef{"R6", "R6.v.rev", "56F1:017B–01B3", "0150 反向"},
		BranchDef{"R6", "R6.v.tclip", "56F1:017E–0189/01B5–01C0", "0150 上剪裁"},
		BranchDef{"R6", "R6.v.bclip", "56F1:018B–0195/01C2–01CC", "0150 下剪裁"},
		BranchDef{"R6", "R6.v.empty", "56F1:01A4/01DB", "0150 剪裁後點數 ≤ 0"},
	)
}

// r6cs 讀 56F1 段的 byte（cs:[si]）。
func (c *context) r6cs(off uint16) byte {
	if off < 0x0008 || off > 0x00CF {
		c.note("R6 讀表出 0008–00CF：%04X", off)
	}
	if int(off) >= len(c.r6seg) {
		c.fault("R6 讀 cs:%04X 超出載入範圍", off)
		return 0
	}
	return c.r6seg[off]
}

func (c *context) r6csw(off uint16) uint16 {
	return uint16(c.r6cs(off)) | uint16(c.r6cs(off+1))<<8
}

type r6state struct {
	xmin, ymin, xmax, ymax uint16 // 38B4、38B6、38B8、38BA
	pitch                  uint16 // 38BC
}

func (c *context) r6(k *Call) {
	p := k.Params
	s := &r6state{xmin: k.Desc[2], ymin: k.Desc[3], xmax: k.Desc[4], ymax: k.Desc[5]}
	c38BE, c38C0 := k.Aux.Color0, k.Aux.Color1

	// 0201–0219：pitch = tab[ymin+1] − tab[ymin]；起點 = tab[ymin] + xmin
	s.pitch = c.tab(s.ymin+1) - c.tab(s.ymin)
	start := c.tab(s.ymin) + s.xmin
	// 021C–022F
	W := s.xmax - s.xmin + 1
	H := s.ymax - s.ymin + 1
	x0, y0, x1, y1 := p[1], p[2], p[3], p[4]

	// 0232–0242：DX = cwde(x1 − x0)；jge 依 sub 的旗標（有號比 x1 ≥ x0）
	ax := x1 - x0
	DX := sx16(ax)
	if !(int16(x1) >= int16(x0)) {
		ax = -ax
	}
	absDX := ax
	// 0245–0255：DY = cwde(y0 − y1)
	ax = y0 - y1
	DY := sx16(ax)
	if !(int16(y0) >= int16(y1)) {
		ax = -ax
	}
	absDY := ax
	// 0258–0260
	if absDX|absDY == 0 {
		c.br.hit("R6.B6z")
		return
	}
	var S, B uint32
	if uint16(DY) == 0 { // 0263 cmp word [bp-0Ch],0
		// 0269–0297
		ebx := uint32(s.pitch) << 8 // movzx、shl 8
		ax := y0 - s.ymin
		eax := uint32(imul64(sx16(ax), ebx)) // imul ebx，取低 32
		if int16(uint16(DX)) < 0 {           // 027F test word [bp-8]；js 028C
			c.br.hit("R6.B6h-")
			eax = -eax // 028C
		} else {
			c.br.hit("R6.B6h+")
			ebx = -ebx // 0286
			eax = -eax // 0289
			eax = -eax // 028C
		}
		S, B = ebx, eax
	} else {
		c.br.hit("R6.B6s")
		// 0299–02AD：eax = DX << 8；cdq；idiv(−DY)
		q, _ := c.idiv64(int64(int32(DX<<8)), int32(-DY), "R6 02AA")
		S = uint32(q)
		// 02B1–02CD
		ecx := sx16(x0) << 8
		ax := y0 - s.ymin
		eax := uint32(imul64(sx16(ax), S))
		B = ecx - eax
	}
	// 02D1–02E7：ax ← 38C0、dx ← 38BE；DY 低字非負時 xchg
	first, second := c38C0, c38BE
	neg := int16(uint16(DY)) < 0
	if c.mut["r6swap"] { // 負對照 M9：交換條件相反
		neg = !neg
	}
	if !neg {
		c.br.hit("R6.col.dyge0")
		first, second = second, first
	} else {
		c.br.hit("R6.col.dylt0")
	}

	// 02EA–033F：每列
	di := start
	si := H
	edx := B
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R6 列迴圈超過上限")
			return
		}
		rowdi := di // 02F8 push di
		bx := W
		eax := int32(edx) >> 8         // sar eax,8
		ecx := int32(uint32(s.xmin))   // movzx ecx,[38B4]
		e := uint32(eax) - uint32(ecx) // sub eax,ecx
		if eax < ecx {                 // 030C jl（有號）
			c.br.hit("R6.row.eneg")
		} else {
			cx := W
			if !(int32(e) > int32(uint32(W))) { // 0310 cmp eax,ecx（ecx = W，上位 0）；jg
				cx = uint16(e)
				c.br.hit("R6.row.elew")
			} else {
				c.br.hit("R6.row.egtw")
			}
			bx -= cx
			// 031C–0322：rep stosw（cx>>1 個 word）＋ rep stosb（CF 個）
			for i := uint16(0); i < cx>>1; i++ {
				c.ww(di, uint16(first)|uint16(first)<<8)
				di += 2
			}
			if cx&1 != 0 {
				c.w(di, first)
				di++
			}
		}
		if int16(bx) > 0 { // 0324 or bx,bx；jle
			c.br.hit("R6.row.second")
			for i := uint16(0); i < bx>>1; i++ {
				c.ww(di, uint16(second)|uint16(second)<<8)
				di += 2
			}
			if bx&1 != 0 {
				c.w(di, second)
				di++
			}
		}
		di = rowdi + s.pitch // 0335–0336
		edx += S             // 033A
		si--                 // 033E dec si；jne
		if si == 0 {
			break
		}
	}

	// 0341–034D：|DX| < |DY|（16 位元有號比）
	if int16(absDX) < int16(absDY) {
		c.br.hit("R6.B6r")
		// 0350–035E：k = (DX << 8) / DY
		k32, _ := c.idiv64(int64(int32(DX<<8)), int32(DY), "R6 035A")
		// 0362–037D
		ax := y0 - s.ymin
		prod := int32(int16(ax)) * int32(int16(uint16(DX))) // imul word [bp-8]
		q16, _ := c.idiv32(prod, int16(uint16(DY)), "R6 036C")
		xv := (uint32(int32(q16)) + sx16(x0)) << 8
		si := s.ymin
		for it := 0; int16(si) <= int16(s.ymax); it++ { // 03C7 cmp si,[38BA]；jle
			if it >= iterCap {
				c.fault("R6 B6r 迴圈超過上限")
				return
			}
			d := uint16(int32(xv) >> 8) // 0387 sar edi,8
			if DY&0x80000000 == 0 {     // 038F test dword [bp-9],80h
				c.br.hit("R6.B6r.fwd")
				c.r6h(s, si, d-0x18, d+0x17, 0)
			} else {
				c.br.hit("R6.B6r.rev")
				c.r6h(s, si, d-0x17, d+0x18, 0xFFFF)
			}
			xv -= uint32(k32)
			si++
		}
	} else {
		c.br.hit("R6.B6c")
		// 03D0–03DE：k = (DY << 8) / DX
		k32, _ := c.idiv64(int64(int32(DY<<8)), int32(DX), "R6 03DA")
		ax := x0 - s.xmin
		prod := int32(int16(ax)) * int32(int16(uint16(DY)))
		q16, _ := c.idiv32(prod, int16(uint16(DX)), "R6 03EC")
		yv := (uint32(int32(q16)) + sx16(y0)) << 8
		si := s.xmin
		for it := 0; !(int16(s.xmax) < int16(si)); it++ { // 0447 cmp [38B8],si；jl
			if it >= iterCap {
				c.fault("R6 B6c 迴圈超過上限")
				return
			}
			d := uint16(yv >> 8)    // 0407 shr edi,8（邏輯）
			if DX&0x80000000 == 0 { // 040F test dword [bp-5],80h
				c.br.hit("R6.B6c.fwd")
				c.r6v(s, si, d-0x18, d+0x17, 0)
			} else {
				c.br.hit("R6.B6c.rev")
				c.r6v(s, si, d-0x17, d+0x18, 0xFFFF)
			}
			yv -= uint32(k32)
			si++
		}
	}
}

// r6h：56F1:00D0–014F。bx = 列，cx = 起欄，dx = 終欄，ax = 方向（0 正向）。
func (c *context) r6h(s *r6state, row, lo, hi, dir uint16) {
	if int16(lo) > int16(s.xmax) || int16(hi) < int16(s.xmin) { // 00D2–00DC
		c.br.hit("R6.h.skip")
		return
	}
	si := c.r6csw(0x00C8 + (row&3)*2) // 00DE–00E5
	di := c.tab(row)                  // 00EA–00F0 ss:[38B2 + 列 × 2]
	bx := lo
	cx := uint16(0x30)
	if dir != 0 {
		c.br.hit("R6.h.rev")
		si += 0x2F
		ax := s.xmin - bx // 00FF–0104 sub；js
		if int16(ax) >= 0 {
			c.br.hit("R6.h.lclip")
			si -= ax
			cx -= ax
			bx += ax
		}
		ax = hi - s.xmax // 010C–0112 sub；jle（有號比 hi ≤ xmax）
		if !(int16(hi) <= int16(s.xmax)) {
			c.br.hit("R6.h.rclip")
			cx -= ax
			hi -= ax
		}
		di += bx
		if int16(cx) <= 0 {
			c.br.hit("R6.h.empty")
			return
		}
		for ; cx != 0; cx-- { // 011E–0123
			c.w(di, c.r6cs(si))
			si--
			di++
		}
		return
	}
	c.br.hit("R6.h.fwd")
	ax := s.xmin - bx // 0127–012C
	if int16(ax) >= 0 {
		c.br.hit("R6.h.lclip")
		si += ax
		cx -= ax
		bx += ax
	}
	ax = hi - s.xmax
	if !(int16(hi) <= int16(s.xmax)) {
		c.br.hit("R6.h.rclip")
		cx -= ax
		hi -= ax
	}
	di += bx
	if int16(cx) <= 0 {
		c.br.hit("R6.h.empty")
		return
	}
	for ; cx != 0; cx-- { // 0146–014B
		c.w(di, c.r6cs(si))
		si++
		di++
	}
}

// r6v：56F1:0150–01EC。bx = 欄，cx = 起列，dx = 終列，ax = 方向。
func (c *context) r6v(s *r6state, col, lo, hi, dir uint16) {
	if int16(lo) > int16(s.ymax) || int16(hi) < int16(s.ymin) { // 0152–015E
		c.br.hit("R6.v.skip")
		return
	}
	si := c.r6csw(0x00C8 + (col&3)*2) // 0162–0169
	di := col                         // 016E
	bx := lo
	cx := uint16(0x30)
	rev := dir != 0
	if rev {
		c.br.hit("R6.v.rev")
		si += 0x2F
	} else {
		c.br.hit("R6.v.fwd")
	}
	ax := s.ymin - bx // 017E／01B5
	if int16(ax) >= 0 {
		c.br.hit("R6.v.tclip")
		if rev {
			si -= ax
		} else {
			si += ax
		}
		cx -= ax
		bx += ax
	}
	ax = hi - s.ymax // 018B／01C2
	if !(int16(hi) <= int16(s.ymax)) {
		c.br.hit("R6.v.bclip")
		cx -= ax
		hi -= ax
	}
	di += c.tab(bx) // 0197–019D／01CE–01D4
	dx := s.pitch
	if int16(cx) <= 0 {
		c.br.hit("R6.v.empty")
		return
	}
	for ; cx != 0; cx-- { // 01A8–01B1／01DF–01E8
		c.w(di, c.r6cs(si))
		if rev {
			si--
		} else {
			si++
		}
		di += dx
	}
}

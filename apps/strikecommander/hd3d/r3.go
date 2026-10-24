package hd3d

// R3 Gouraud 1F9F:083D–0EA7。依 dis-1F9F.txt:785–1363 逐條移植。
// 頂點 +4 的字是 8.8 明暗。一般情形與換邊以 movzx 取（0A6F–0A74 等），單列以 movsx（0965、097C）。
// 抖動累加器 (ch:bp) 與 (cl:bx) 照指令移植（0B9D–0BF5、0CC8–0D1F）。

func init() {
	regBranches(
		BranchDef{"R3", "R3.B1a", "1F9F:090C–090E", "外碼全部同側，返回"},
		BranchDef{"R3", "R3.B3b", "1F9F:0921–099F", "單列（明暗以 movsx 取）"},
		BranchDef{"R3", "R3.skipL", "1F9F:09BE–09CA", "左邊跳過"},
		BranchDef{"R3", "R3.skipR", "1F9F:0A41–0A4D", "右邊跳過"},
		BranchDef{"R3", "R3.rowsMaxY", "1F9F:0AAE–0AB3", "rows = maxY − minY"},
		BranchDef{"R3", "R3.rowsYmax", "1F9F:0AB1", "rows = ymax − minY"},
		BranchDef{"R3", "R3.B3c", "1F9F:0AC0–0B22", "上剪裁預步進（x 與明暗）"},
		BranchDef{"R3", "R3.xchg", "1F9F:0B5D–0B5F／0C57–0C59", "該列左右交換（含明暗）"},
		BranchDef{"R3", "R3.B3d", "1F9F:0B4E–0C2D", "未剪裁填色"},
		BranchDef{"R3", "R3.B3d.one", "1F9F:0B74", "未剪裁：R = L，不求列內斜率"},
		BranchDef{"R3", "R3.dither0", "1F9F:0BA3–0BB8／0CCE–0CE3", "(L ^ y) & 1 = 0：cl:bx 加 0.5"},
		BranchDef{"R3", "R3.dither1", "1F9F:0BA3–0BB8／0CCE–0CE3", "(L ^ y) & 1 = 1：ch:bp 加 0.5"},
		BranchDef{"R3", "R3.oddStart", "1F9F:0BC1–0BD2／0CEC–0CFD", "起點 di 為奇數，先寫一點"},
		BranchDef{"R3", "R3.oddEnd", "1F9F:0BF2／0D1C", "尾端多一點"},
		BranchDef{"R3", "R3.B3e", "1F9F:0C48–0D57", "剪裁填色"},
		BranchDef{"R3", "R3.B3e.skip", "1F9F:0C63／0C6E", "剪裁：本列整列在視埠外"},
		BranchDef{"R3", "R3.B3e.one", "1F9F:0C86", "剪裁：R = L，不求列內斜率"},
		BranchDef{"R3", "R3.B3e.lclip", "1F9F:0D6D–0D8C", "剪裁：左剪裁預步進明暗"},
		BranchDef{"R3", "R3.B3e.rclip", "1F9F:0D8F", "剪裁：右剪裁"},
		BranchDef{"R3", "R3.lastRow", "1F9F:0C35／0D5A", "rows = 0：外插畫最後一列"},
		BranchDef{"R3", "R3.chgL", "1F9F:0D94–0E1B", "換左邊"},
		BranchDef{"R3", "R3.chgR", "1F9F:0E1E–0EA5", "換右邊"},
		BranchDef{"R3", "R3.chg0to1", "1F9F:0DB6–0DB9／0E40–0E43", "換邊 dy = 0 改 1"},
		BranchDef{"R3", "R3.shadeHi", "1F9F:0965／097C", "單列頂點明暗字 ≥ 8000h（統計，不是分支）"},
		BranchDef{"R3", "R3.shadeMixed", "1F9F:0965／097C", "單列兩端明暗符號位不同且 x 不同（M6 可觀察的條件，統計）"},
		BranchDef{"R3", "R3.shadeHiGen", "1F9F:09EC–0A21 等", "一般情形有頂點明暗字 ≥ 8000h（統計）"},
		BranchDef{"R3", "R3.shadeMixedGen", "1F9F:09EC–0A21 等", "一般情形頂點明暗符號位混合（movzx 改 movsx 可觀察的條件，統計）"},
	)
}

type gour struct {
	flat
	// R3 的記憶體配置與 R1 不同，這裡只換名字：
	// la [bp−18h]、ra [bp−1Ah]、L [bp−1Ch]、R [bp−1Eh]、lcnt [bp−20h]、rcnt [bp−22h]、
	// rows [bp−24h]、y0 [bp−26h]、hclip [bp−2Eh]、lx [bp−32h]、rx [bp−36h]、
	// ls [bp−3Ah]、rs [bp−3Eh]、lsl [bp−42h]、rsl [bp−46h]、lss [bp−4Ah]、rss [bp−4Eh]
	ls, rs   uint32
	lss, rss uint32
	m1308    uint16 // ss:[1308h]：目前列號
	m130A    uint16 // ss:[130Ah]：本列左端 x
	v52      uint32 // [bp−52h]：剪裁路徑的列內斜率
}

// shadeSlope：movzx edx,[si+4]; movzx eax,[bx+4]; sub edx,eax; shl edx,8;
// mov ax,dx; shl eax,16; sar edx,16; idiv ecx。被除數 = int32(D) << 16。
func (c *context) shadeSlope(sb, sa uint16, cnt uint16, where string) uint32 {
	var d uint32
	if c.mut["r3movsxgen"] { // 負對照：一般情形改用 movsx
		d = (sx16(sb) - sx16(sa)) << 8
	} else {
		d = (uint32(sb) - uint32(sa)) << 8
	}
	q, _ := c.idiv64(int64(int32(d))<<16, int32(uint32(cnt)<<16), where)
	return uint32(q)
}

func (c *context) r3(k *Call) {
	v := verts{c: c, b: k.Verts, start: k.Params[2]}
	var g gour
	f := &g.flat
	f.xmin, f.ymin, f.xmax, f.ymax = k.Desc[2], k.Desc[3], k.Desc[4], k.Desc[5]
	f.pitch = c.tab(1) - c.tab(0) // 0894–089C
	f.tab0 = c.tab(0)             // 08A1–08A3
	f.start = k.Params[2]
	f.end = uint16(uint32(k.Params[1])*22) + f.start
	s4 := func(p uint16) uint16 { return v.w(p, 4) }
	// 08B9–090E：外碼（與 R1 同）
	f.hclip = 0
	si, di := uint16(0x7FFF), uint16(0x8000)
	ch := byte(0x0F)
	var dx uint16
	bx := f.start
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R3 外碼迴圈超過上限")
			return
		}
		cl := byte(0)
		cl = cl<<1 | byte((v.x(bx)-f.xmin)>>15)
		cl = cl<<1 | byte((f.xmax-v.x(bx))>>15)
		f.hclip |= uint16(cl)
		cl = cl<<1 | byte((v.y(bx)-f.ymin)>>15)
		cl = cl<<1 | byte((f.ymax-v.y(bx))>>15)
		ax := v.y(bx)
		if !(int16(ax) > int16(si)) {
			si = ax
			dx = bx
		}
		if !(int16(ax) < int16(di)) {
			di = ax
		}
		ch &= cl
		bx += 22
		if bx == f.end {
			break
		}
	}
	if ch != 0 {
		c.br.hit("R3.B1a")
		return
	}
	f.L, f.R, f.y0 = dx, dx, si // 0912–0918
	if di == si {               // 091B
		c.br.hit("R3.B3b")
		bx := f.L
		sv := bx
		ax := v.x(sv)
		for it := 0; ; it++ { // 0921–093C
			if it >= iterCap {
				c.fault("R3 單列左找迴圈超過上限")
				return
			}
			if !(bx > f.start) {
				bx = f.end
			}
			bx -= 22
			if bx == f.L {
				break
			}
			if int16(ax) <= int16(v.x(bx)) {
				continue
			}
			sv = bx
			ax = v.x(sv)
		}
		bx = f.R
		dv := bx
		ax = v.x(dv)
		for it := 0; ; it++ { // 093E–0959
			if it >= iterCap {
				c.fault("R3 單列右找迴圈超過上限")
				return
			}
			bx += 22
			if !(bx < f.end) {
				bx = f.start
			}
			if bx == f.R {
				break
			}
			if int16(ax) >= int16(v.x(bx)) {
				continue
			}
			dv = bx
			ax = v.x(dv)
		}
		// 095B–099C
		f.lx = uint32(v.x(sv)) << 16
		if s4(sv)&0x8000 != 0 || s4(dv)&0x8000 != 0 {
			c.br.hit("R3.shadeHi")
			// 只有兩端符號位不同且 x 不同時，movsx／movzx 才會改變列內斜率（寫出的只有 cl）
			if (s4(sv)^s4(dv))&0x8000 != 0 && v.x(sv) != v.x(dv) {
				c.br.hit("R3.shadeMixed")
			}
		}
		if c.mut["r3movzx"] { // 負對照 M6：單列改用 movzx
			g.ls = uint32(s4(sv)) << 8
			g.rs = uint32(s4(dv)) << 8
		} else {
			g.ls = sx16(s4(sv)) << 8 // 0965 movsx edx,[si+4]；shl edx,8
			g.rs = sx16(s4(dv)) << 8 // 097C
		}
		f.rx = uint32(v.x(dv)) << 16
		f.lsl, g.lss, f.rsl, g.rss = 0, 0, 0, 0
		f.rows = 0
	} else {
		hi, lo := false, false // 統計用：一般情形的頂點明暗字符號位
		for p := f.start; p != f.end; p += 22 {
			if s4(p)&0x8000 != 0 {
				hi = true
			} else {
				lo = true
			}
		}
		if hi {
			c.br.hit("R3.shadeHiGen")
		}
		if hi && lo {
			c.br.hit("R3.shadeMixedGen")
		}
		// 09A2–0A21：左邊
		for it := 0; ; it++ {
			if it >= iterCap {
				c.fault("R3 左邊設定迴圈超過上限")
				return
			}
			bx := f.L
			f.la = bx
			s := c.prevV(f, bx)
			f.L = s
			cx := v.y(s)
			dxv := v.y(bx)
			if !(int16(dxv) >= int16(f.ymin)) && int16(cx) <= int16(f.ymin) {
				c.br.hit("R3.skipL")
				continue
			}
			cx -= dxv
			if cx == 0 {
				c.br.hit("R3.skipL")
				continue
			}
			f.lcnt = cx
			f.lsl = c.edgeSlope(v.x(s)-v.x(bx), cx, "R3 09E5")
			g.lss = c.shadeSlope(s4(s), s4(bx), cx, "R3 0A07")
			f.lx = uint32(v.x(bx))<<16 | 0x8000 // 0A0E–0A15
			g.ls = uint32(s4(bx)) << 8          // 0A18–0A21 movzx
			break
		}
		// 0A25–0AA4：右邊
		for it := 0; ; it++ {
			if it >= iterCap {
				c.fault("R3 右邊設定迴圈超過上限")
				return
			}
			bx := f.R
			f.ra = bx
			s := c.nextV(f, bx)
			f.R = s
			cx := v.y(s)
			dxv := v.y(bx)
			if !(int16(dxv) >= int16(f.ymin)) && int16(cx) <= int16(f.ymin) {
				c.br.hit("R3.skipR")
				continue
			}
			cx -= dxv
			if cx == 0 {
				c.br.hit("R3.skipR")
				continue
			}
			f.rcnt = cx
			f.rsl = c.edgeSlope(v.x(s)-v.x(bx), cx, "R3 0A68")
			g.rss = c.shadeSlope(s4(s), s4(bx), cx, "R3 0A8A")
			f.rx = uint32(v.x(bx))<<16 | 0x8000
			g.rs = uint32(s4(bx)) << 8
			break
		}
		// 0AA8–0AB5
		ax := f.ymax - f.y0
		t := di - f.ymax
		if !(int16(di) > int16(f.ymax)) {
			ax += t
			c.br.hit("R3.rowsMaxY")
		} else {
			c.br.hit("R3.rowsYmax")
		}
		f.rows = ax
		// 0AB8–0B22
		ax = f.ymin - f.y0
		if int16(f.ymin) > int16(f.y0) {
			c.br.hit("R3.B3c")
			f.rows -= ax
			cx := f.ymin
			f.y0 = f.ymin
			cx -= v.y(f.la) // 0AC9 bx = [bp−18h]
			f.lcnt -= cx
			e := uint32(cx) << 16
			f.lx += shrd16(imul64(f.lsl, e))
			g.ls += shrd16(imul64(g.lss, e))
			cx = f.ymin - v.y(f.ra) // 0AF9 bx = [bp−1Ah]
			f.rcnt -= cx
			e = uint32(cx) << 16
			f.rx += shrd16(imul64(f.rsl, e))
			g.rs += shrd16(imul64(g.rss, e))
		}
	}
	// 0B26–0B45
	di = f.tab0
	g.m1308 = f.y0
	di += f.y0 * f.pitch
	eax, ebx, ecx, edx := f.lx, f.rx, g.ls, g.rs
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R3 列迴圈超過上限")
			return
		}
		if f.hclip == 0 {
			c.r3row(&g, di, eax, ebx, ecx, edx)
		} else {
			c.r3rowClip(&g, di, eax, ebx, ecx, edx)
		}
		di += f.pitch // 0C00／0D2A
		g.m1308++     // 0C03／0D2D
		f.rows--
		if int16(f.rows) < 0 {
			return
		}
		if f.rows == 0 { // 0C35／0D5A
			c.br.hit("R3.lastRow")
			eax += f.lsl
			ecx += g.lss
			ebx += f.rsl
			edx += g.rss
			continue
		}
		f.lcnt--
		if f.lcnt == 0 { // 0D94
			c.br.hit("R3.chgL")
			bx := f.L
			f.la = bx
			si := c.prevV(f, bx)
			f.L = si
			cx := v.y(si) - v.y(bx)
			if cx == 0 {
				c.br.hit("R3.chg0to1")
				cx = 1
			}
			f.lcnt = cx
			g.lss = c.shadeSlope(s4(si), s4(bx), cx, "R3 0DDE")
			f.lsl = c.edgeSlope(v.x(si)-v.x(bx), cx, "R3 0DF7")
			ecx = uint32(s4(bx)) << 8          // 0DFE–0E03
			eax = uint32(v.x(bx))<<16 | 0x8000 // 0E07–0E0D
		} else {
			eax += f.lsl
			ecx += g.lss
		}
		f.rcnt--
		if f.rcnt == 0 { // 0E1E
			c.br.hit("R3.chgR")
			bx := f.R
			f.ra = bx
			si := c.nextV(f, bx)
			f.R = si
			cx := v.y(si) - v.y(bx)
			if cx == 0 {
				c.br.hit("R3.chg0to1")
				cx = 1
			}
			f.rcnt = cx
			g.rss = c.shadeSlope(s4(si), s4(bx), cx, "R3 0E68")
			f.rsl = c.edgeSlope(v.x(si)-v.x(bx), cx, "R3 0E81")
			edx = uint32(s4(bx)) << 8
			ebx = uint32(v.x(bx))<<16 | 0x8000
		} else {
			ebx += f.rsl
			edx += g.rss
		}
	}
}

// r3span：0B9D–0BF5（未剪裁）與 0CC8–0D1F（剪裁）共用的抖動寫入。
// cx:bx 是左端明暗（cx 整數部、bx 小數部），si 是點數，ax:dl 是列內步長（ax 小數、dl 整數低位元組）。
// clipped 時字迴圈是 dec si；jg，否則 dec si；jne。
func (c *context) r3span(g *gour, di uint16, cxv, bxv uint16, si uint16, ax uint16, dl byte, clipped bool) {
	bp := bxv // mov bp,bx
	cl := byte(cxv)
	ch := cl // mov ch,cl
	bx := bxv
	// 0BA3–0BB8：d = ((L ^ y) & 1) << 15
	d := ((g.m130A ^ g.m1308) << 15)
	if c.mut["r3dither"] { // 負對照 M5：奇偶相反
		d ^= 0x8000
	}
	if d != 0 {
		c.br.hit("R3.dither1")
	} else {
		c.br.hit("R3.dither0")
	}
	var cy uint32
	t := uint32(bp) + uint32(d) // add bp,ax
	bp, cy = uint16(t), t>>16
	ch += byte(cy) // adc ch,al（al = 0）
	d ^= 0x8000    // xor ah,80h
	t = uint32(bx) + uint32(d)
	bx, cy = uint16(t), t>>16
	cl += byte(cy)
	step := func() {
		t := uint32(bx) + uint32(ax)
		bx = uint16(t)
		cl = cl + dl + byte(t>>16)
		t = uint32(bp) + uint32(ax)
		bp = uint16(t)
		ch = ch + dl + byte(t>>16)
	}
	if di&1 != 0 { // 0BBB test di,1
		c.br.hit("R3.oddStart")
		c.w(di, cl)
		di++
		si--
		if si == 0 {
			return
		}
		step()
		bp, bx = bx, bp
		ch, cl = cl, ch
	}
	// 0BD4–0BD6：步長加倍
	t = uint32(ax) + uint32(ax)
	ax = uint16(t)
	dl = dl + dl + byte(t>>16)
	odd := si&1 != 0
	si >>= 1
	if si == 0 { // 0BDA je 0BF2
		c.br.hit("R3.oddEnd")
		c.w(di, cl)
		return
	}
	for it := 0; ; it++ { // 0BDE–0BED
		if it >= iterCap {
			c.fault("R3 字迴圈超過上限")
			return
		}
		c.ww(di, uint16(cl)|uint16(ch)<<8)
		di += 2
		step()
		si--
		if clipped {
			if !(int16(si) > 0) {
				break
			}
		} else if si == 0 {
			break
		}
	}
	if odd { // 0BEF popf；jae
		c.br.hit("R3.oddEnd")
		c.w(di, cl)
	}
}

// r3row：0B4E–0BFE（未剪裁的一列）。
func (c *context) r3row(g *gour, di uint16, eax, ebx, ecx, edx uint32) {
	c.br.hit("R3.B3d")
	if !(int32(ebx) > int32(eax)) { // 0B58 cmp ebx,eax；jg
		c.br.hit("R3.xchg")
		eax, ebx = ebx, eax
		ecx, edx = edx, ecx
	}
	a := uint16(eax >> 16) // shr eax,16
	b := uint16(ebx >> 16)
	g.m130A = a
	di += a
	bx := b - a
	si := bx
	var ax uint16
	var dl byte
	if bx == 0 { // 0B74 je 0B91：ax 仍是 L、dl 是 edx 的低位元組
		c.br.hit("R3.B3d.one")
		ax = a
		dl = byte(edx)
	} else {
		ed := edx - ecx // 0B76 sub edx,ecx
		q, _ := c.idiv64(int64(int32(ed))<<16, int32(uint32(bx)<<16), "R3 0B87")
		ax = uint16(q) // shld edx,eax,16：dx = 商的高字
		dl = byte(uint32(q) >> 16)
	}
	si++ // 0B96
	c.r3span(g, di, uint16(ecx>>16), uint16(ecx), si, ax, dl, false)
}

// r3rowClip：0C48–0D1F，以及 0D6D–0D91。
func (c *context) r3rowClip(g *gour, di uint16, eax, ebx, ecx, edx uint32) {
	c.br.hit("R3.B3e")
	f := &g.flat
	if !(int32(ebx) > int32(eax)) {
		c.br.hit("R3.xchg")
		eax, ebx = ebx, eax
		ecx, edx = edx, ecx
	}
	a := uint16(eax >> 16)
	if int16(a) > int16(f.xmax) { // 0C60
		c.br.hit("R3.B3e.skip")
		return
	}
	b := uint16(ebx >> 16)
	if int16(b) < int16(f.xmin) { // 0C6B
		c.br.hit("R3.B3e.skip")
		return
	}
	g.m130A = a
	v28, v2a := a, b
	di += a
	bx := b - a
	si := bx + 1
	if si == 1 { // 0C83 cmp si,1；je 0CA0
		c.br.hit("R3.B3e.one")
	} else {
		ed := edx - ecx
		q, _ := c.idiv64(int64(int32(ed))<<16, int32(uint32(bx)<<16), "R3 0C99")
		g.v52 = uint32(q)
	}
	// 0CA0–0CA2
	bxv := uint16(ecx)
	cxv := uint16(ecx >> 16)
	ax := f.xmin - v28 // 0CA6–0CA9 sub；jg
	if int16(f.xmin) > int16(v28) {
		c.br.hit("R3.B3e.lclip")
		// 0D6D–0D8C
		di += ax
		si -= ax
		g.m130A += ax
		p := shrd16(imul64(uint32(ax)<<16, g.v52)) // shl eax,16；imul；shrd
		t := (uint32(cxv)<<16 | uint32(bxv)) + p   // add bx,ax；adc cx,dx
		cxv, bxv = uint16(t>>16), uint16(t)
	}
	if int16(v2a) > int16(f.xmax) { // 0CB0–0CB6
		c.br.hit("R3.B3e.rclip")
		si -= v2a - f.xmax // 0D8F
	}
	// 0CBA–0CC5
	stepAx := uint16(g.v52)
	dl := byte(g.v52 >> 16)
	c.r3span(g, di, cxv, bxv, si, stepAx, dl, true)
}

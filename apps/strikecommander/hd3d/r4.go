package hd3d

// R4 貼圖 1F9F:0EA8–15DB。依 dis-1F9F-b.txt 逐條移植。
// 參數：[bp+0Eh] 貼圖偏移、[bp+10h] 貼圖段、[bp+12h] 列寬、[bp+14h] 明暗表遠指標。
// 貼圖與明暗表使用捕捉的段內 spans，索引保留16位元環繞；範圍外讀取標擷取不足。

func init() {
	regBranches(
		BranchDef{"R4", "R4.B1a", "1F9F:0F77–0F79", "外碼全部同側，返回"},
		BranchDef{"R4", "R4.B4b", "1F9F:0F8F–102F", "單列"},
		BranchDef{"R4", "R4.skipL", "1F9F:104E–105A", "左邊跳過"},
		BranchDef{"R4", "R4.skipR", "1F9F:1100–110C", "右邊跳過"},
		BranchDef{"R4", "R4.rowsMaxY", "1F9F:119C–11A1", "rows = maxY − minY"},
		BranchDef{"R4", "R4.rowsYmax", "1F9F:119F", "rows = ymax − minY"},
		BranchDef{"R4", "R4.B4c", "1F9F:11B0–1232", "上剪裁預步進（x、u、v）"},
		BranchDef{"R4", "R4.B4s", "1F9F:124C–1255", "有明暗表：內迴圈 1392"},
		BranchDef{"R4", "R4.B4n", "1F9F:1242", "無明暗表：內迴圈 13BE"},
		BranchDef{"R4", "R4.xchg", "1F9F:1288–128D", "該列左右交換"},
		BranchDef{"R4", "R4.row.skip", "1F9F:1294–12A2", "本列整列在視埠外"},
		BranchDef{"R4", "R4.row.one", "1F9F:12B0–12B2", "R = L：不求列內斜率（沿用舊的 1300–1306）"},
		BranchDef{"R4", "R4.row.duneg", "1F9F:12D6–12DF", "列內 du < 0"},
		BranchDef{"R4", "R4.row.dvneg", "1F9F:1305–130E", "列內 dv < 0"},
		BranchDef{"R4", "R4.B4e.l", "1F9F:1443–1469", "左剪裁預步進 u、v"},
		BranchDef{"R4", "R4.B4e.r", "1F9F:146C–146F", "右剪裁"},
		BranchDef{"R4", "R4.ff", "1F9F:1394–1397／13C0–13C3", "紋素 FFh 不寫"},
		BranchDef{"R4", "R4.lastRow", "1F9F:1428–1440", "rows = 0：外插畫最後一列"},
		BranchDef{"R4", "R4.chgL", "1F9F:1472–1522", "換左邊"},
		BranchDef{"R4", "R4.chgR", "1F9F:1525–15D9", "換右邊"},
		BranchDef{"R4", "R4.chg0to1", "1F9F:1497–149A／154C–154F", "換邊 dy = 0 改 1"},
	)
}

// uvSlope：movzx edx,[si+f]; movzx eax,[bx+f]; sub; shl 8; 64 位元 << 16；idiv ecx。
func (c *context) uvSlope(sb, sa uint16, cnt uint16, where string) uint32 {
	d := (uint32(sb) - uint32(sa)) << 8
	q, _ := c.idiv64(int64(int32(d))<<16, int32(uint32(cnt)<<16), where)
	return uint32(q)
}

func (c *context) r4(k *Call) {
	v := verts{c: c, b: k.Verts, start: k.Params[2]}
	tex, err := c.capture(k.Tex.Pixels, "R4 texture")
	if err != nil {
		c.fault("R4 貼圖：%v", err)
		return
	}
	var shade *capturedSegment
	shadeOn := k.Params[7] != 0 || k.Params[8] != 0 // 1245 cmp dword [bp+14h],0
	if shadeOn {
		if shade, err = c.capture(k.Tex.Shade, "R4 shade"); err != nil {
			c.fault("R4 明暗表：%v", err)
			return
		}
	}
	texOff, roww, shOff := k.Params[4], k.Params[6], k.Params[7]
	u := func(p uint16) uint16 { return v.w(p, 6) }
	vv := func(p uint16) uint16 { return v.w(p, 8) }

	var f flat // la [bp−16h]、ra [bp−18h]、L [bp−1Ah]、R [bp−1Ch]、lcnt [bp−1Eh]、rcnt [bp−20h]、rows [bp−22h]、y0 [bp−24h]
	f.xmin, f.ymin, f.xmax, f.ymax = k.Desc[2], k.Desc[3], k.Desc[4], k.Desc[5]
	f.pitch = c.tab(1) - c.tab(0)
	f.tab0 = c.tab(0)
	f.start = k.Params[2]
	f.end = uint16(uint32(k.Params[1])*22) + f.start
	// 0F24–0F79：外碼（[bp−2Eh] 之後被 lx 蓋掉，R4 不用 hclip）
	si, di := uint16(0x7FFF), uint16(0x8000)
	ch := byte(0x0F)
	var dx uint16
	bx := f.start
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R4 外碼迴圈超過上限")
			return
		}
		cl := byte(0)
		cl = cl<<1 | byte((v.x(bx)-f.xmin)>>15)
		cl = cl<<1 | byte((f.xmax-v.x(bx))>>15)
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
		c.br.hit("R4.B1a")
		return
	}
	f.L, f.R, f.y0 = dx, dx, si
	var lx, rx, lu, ru, lv, rv uint32
	var lxs, rxs, lus, rus, lvs, rvs uint32
	if di == si {
		c.br.hit("R4.B4b")
		bx := f.L
		sv := bx
		ax := v.x(sv)
		for it := 0; ; it++ { // 0F92–0FAA
			if it >= iterCap {
				c.fault("R4 單列左找迴圈超過上限")
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
		for it := 0; ; it++ { // 0FAF–0FC7
			if it >= iterCap {
				c.fault("R4 單列右找迴圈超過上限")
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
		// 0FC9–102C
		lx = uint32(v.x(sv)) << 16
		lu = uint32(u(sv)) << 8
		lv = uint32(vv(sv)) << 8
		rx = uint32(v.x(dv)) << 16
		ru = uint32(u(dv)) << 8
		rv = uint32(vv(dv)) << 8
		f.rows = 0
	} else {
		for it := 0; ; it++ { // 1032–10E0
			if it >= iterCap {
				c.fault("R4 左邊設定迴圈超過上限")
				return
			}
			bx := f.L
			f.la = bx
			s := c.prevV(&f, bx)
			f.L = s
			cx := v.y(s)
			dxv := v.y(bx)
			if !(int16(dxv) >= int16(f.ymin)) && int16(cx) <= int16(f.ymin) {
				c.br.hit("R4.skipL")
				continue
			}
			cx -= dxv
			if cx == 0 {
				c.br.hit("R4.skipL")
				continue
			}
			f.lcnt = cx
			lxs = c.edgeSlope(v.x(s)-v.x(bx), cx, "R4 1075")
			lus = c.uvSlope(u(s), u(bx), cx, "R4 1097")
			lvs = c.uvSlope(vv(s), vv(bx), cx, "R4 10B9")
			lx = uint32(v.x(bx))<<16 | 0x8000
			lu = uint32(u(bx)) << 8
			lv = uint32(vv(bx)) << 8
			break
		}
		for it := 0; ; it++ { // 10E4–1192
			if it >= iterCap {
				c.fault("R4 右邊設定迴圈超過上限")
				return
			}
			bx := f.R
			f.ra = bx
			s := c.nextV(&f, bx)
			f.R = s
			cx := v.y(s)
			dxv := v.y(bx)
			if !(int16(dxv) >= int16(f.ymin)) && int16(cx) <= int16(f.ymin) {
				c.br.hit("R4.skipR")
				continue
			}
			cx -= dxv
			if cx == 0 {
				c.br.hit("R4.skipR")
				continue
			}
			f.rcnt = cx
			rxs = c.edgeSlope(v.x(s)-v.x(bx), cx, "R4 1127")
			rus = c.uvSlope(u(s), u(bx), cx, "R4 1149")
			rvs = c.uvSlope(vv(s), vv(bx), cx, "R4 116B")
			rx = uint32(v.x(bx))<<16 | 0x8000
			ru = uint32(u(bx)) << 8
			rv = uint32(vv(bx)) << 8
			break
		}
		// 1196–11A3
		ax := f.ymax - f.y0
		t := di - f.ymax
		if !(int16(di) > int16(f.ymax)) {
			ax += t
			c.br.hit("R4.rowsMaxY")
		} else {
			c.br.hit("R4.rowsYmax")
		}
		f.rows = ax
		// 11A6–1232
		ax = f.ymin - f.y0
		if int16(f.ymin) > int16(f.y0) {
			c.br.hit("R4.B4c")
			f.rows -= ax
			cx := f.ymin
			f.y0 = f.ymin
			cx -= v.y(f.la)
			f.lcnt -= cx
			e := uint32(cx) << 16
			lx += shrd16(imul64(lxs, e))
			lu += shrd16(imul64(lus, e))
			lv += shrd16(imul64(lvs, e))
			cx = f.ymin - v.y(f.ra)
			f.rcnt -= cx
			e = uint32(cx) << 16
			rx += shrd16(imul64(rxs, e))
			ru += shrd16(imul64(rus, e))
			rv += shrd16(imul64(rvs, e))
		}
	}
	// 1236–1274
	rowDI := f.y0*f.pitch + f.tab0 // [bp−2Ah]
	if shadeOn {
		c.br.hit("R4.B4s")
	} else {
		c.br.hit("R4.B4n")
	}
	var m1300 [4]uint16 // ss:[1300–1306]，本次呼叫內先寫後讀
	var v5e, v62 uint32 // [bp−5Eh]、[bp−62h]
	eax, ebx, ecx, edx, esi, edi := lx, rx, lu, ru, lv, rv
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R4 列迴圈超過上限")
			return
		}
		c.r4row(&f, rowDI, eax, ebx, ecx, edx, esi, edi, tex, shade, shadeOn, texOff, roww, shOff, &m1300, &v5e, &v62)
		rowDI += f.pitch // 13E3–13E6
		f.rows--
		if int16(f.rows) < 0 {
			return
		}
		if f.rows == 0 { // 1428
			c.br.hit("R4.lastRow")
			eax += lxs
			ecx += lus
			esi += lvs
			ebx += rxs
			edx += rus
			edi += rvs
			continue
		}
		f.lcnt--
		if f.lcnt == 0 { // 1472
			c.br.hit("R4.chgL")
			bx := f.L
			f.la = bx
			si := c.prevV(&f, bx)
			f.L = si
			cx := v.y(si) - v.y(bx)
			if cx == 0 {
				c.br.hit("R4.chg0to1")
				cx = 1
			}
			f.lcnt = cx
			lxs = c.edgeSlope(v.x(si)-v.x(bx), cx, "R4 14B6")
			lus = c.uvSlope(u(si), u(bx), cx, "R4 14D8")
			lvs = c.uvSlope(vv(si), vv(bx), cx, "R4 14FA")
			eax = uint32(v.x(bx))<<16 | 0x8000
			ecx = uint32(u(bx)) << 8
			esi = uint32(vv(bx)) << 8
		} else {
			eax += lxs
			ecx += lus
			esi += lvs
		}
		f.rcnt--
		if f.rcnt == 0 { // 1525
			c.br.hit("R4.chgR")
			bx := f.R
			f.ra = bx
			si := c.nextV(&f, bx)
			f.R = si
			cx := v.y(si) - v.y(bx)
			if cx == 0 {
				c.br.hit("R4.chg0to1")
				cx = 1
			}
			f.rcnt = cx
			rxs = c.edgeSlope(v.x(si)-v.x(bx), cx, "R4 156B")
			rus = c.uvSlope(u(si), u(bx), cx, "R4 158D")
			rvs = c.uvSlope(vv(si), vv(bx), cx, "R4 15AF")
			edx = uint32(u(bx)) << 8
			edi = uint32(vv(bx)) << 8
			ebx = uint32(v.x(bx))<<16 | 0x8000
		} else {
			ebx += rxs
			edx += rus
			edi += rvs
		}
	}
}

// r4row：1F9F:1277–13E1 與 1443–146F（一列）。
func (c *context) r4row(f *flat, rowDI uint16, eax, ebx, ecx, edx, esi, edi uint32, tex, shade *capturedSegment, shadeOn bool,
	texOff, roww, shOff uint16, m1300 *[4]uint16, v5e, v62 *uint32) {
	if !(int32(ebx) > int32(eax)) { // 1283 cmp ebx,eax；jg
		c.br.hit("R4.xchg")
		eax, ebx = ebx, eax
		ecx, edx = edx, ecx
		esi, edi = edi, esi
	}
	a := uint16(eax >> 16)
	if int16(a) > int16(f.xmax) { // 1294
		c.br.hit("R4.row.skip")
		return
	}
	b := uint16(ebx >> 16)
	if int16(b) < int16(f.xmin) { // 129F
		c.br.hit("R4.row.skip")
		return
	}
	v26, v28 := a, b // [bp−26h]、[bp−28h]
	v66 := ecx       // [bp−66h]：左端 u
	bx := b - a
	if bx == 0 { // 12B2 je 1348
		c.br.hit("R4.row.one")
	} else {
		ebxd := uint32(bx) << 16
		// 12BA–12E4：du
		q, _ := c.idiv64(int64(int32(edx-ecx))<<16, int32(ebxd), "R4 12C7")
		*v5e = uint32(q)
		dxw, axw := uint16(uint32(q)>>16), uint16(q)
		cxw := uint16(1)
		if int16(dxw) < 0 { // 12D8 jns
			c.br.hit("R4.row.duneg")
			cxw = 0xFFFF
			if axw != 0 { // cmp ax,1；sbb dx,−1
				dxw++
			}
		}
		cxw += dxw
		duInt, cxU := dxw, cxw
		// 12E6–1311：dv
		q2, _ := c.idiv64(int64(int32(edi-esi))<<16, int32(ebxd), "R4 12F6")
		*v62 = uint32(q2)
		dxv, axv := uint16(uint32(q2)>>16), uint16(q2)
		cxv := roww
		if int16(dxv) < 0 {
			c.br.hit("R4.row.dvneg")
			cxv = -roww
			if axv != 0 {
				dxv++
			}
		}
		axp := uint16(int16(roww) * int16(dxv)) // imul dx（取 ax）
		d := duInt + axp
		m1300[0] = d
		m1300[1] = d + cxv
		e := cxU + axp
		m1300[2] = e
		m1300[3] = e + cxv
		// 1334–1344
		if int16(f.xmin) > int16(v26) {
			c.br.hit("R4.B4e.l")
			cl := f.xmin - v26
			v26 += cl
			sh := uint32(cl) << 16
			v66 += shrd16(imul64(*v5e, sh))
			esi += shrd16(imul64(*v62, sh))
		}
		if int16(v28) > int16(f.xmax) {
			c.br.hit("R4.B4e.r")
			v28 -= v28 - f.xmax
		}
	}
	// 1348–138D
	ecxL := esi
	vint := uint16(esi >> 16)
	si := vint*roww + texOff + uint16(v66>>16)
	di := rowDI + v26
	cnt := v28 - v26
	dxa := uint16(v66)
	axs := uint16(*v5e)
	if int32(*v5e) < 0 { // 1376 jns
		axs = -axs
		dxa = ^dxa
	}
	bps := uint16(*v62)
	cxa := uint16(ecxL)
	if int32(*v62) < 0 {
		bps = -bps
		cxa = ^cxa
	}
	cnt++ // 1389 inc bx；push bx
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("R4 內迴圈超過上限")
			return
		}
		bl := c.readCapture(tex, si) // 1392／13BE mov bl,[si]
		if bl == 0xFF && !c.mut["r4noff"] {
			c.br.hit("R4.ff")
		} else if shadeOn {
			c.w(di, c.readCapture(shade, uint16(uint16(bl)+shOff)))
		} else {
			c.w(di, bl)
		}
		di++
		t := uint32(dxa) + uint32(axs) // add dx,ax；adc bx,bx
		dxa = uint16(t)
		bxi := t >> 16
		t = uint32(cxa) + uint32(bps) // add cx,bp；adc bx,bx
		cxa = uint16(t)
		bxi = bxi*2 + t>>16
		si += m1300[bxi] // add bx,bx；add si,ss:[bx+1300h]
		cnt--            // dec word [esp]；jg
		if !(int16(cnt) > 0) {
			break
		}
	}
}

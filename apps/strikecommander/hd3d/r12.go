package hd3d

// R1 單色 1F9F:000C–0415 與 R2 查表 1F9F:0416–083C。
// 依 dis-1F9F.txt:7–784 逐條移植。兩支的邊設定、單列、上剪裁、換邊逐指令相同
// （RE 025 §1.6 正規化比對），共用 flatEdges；位址先寫 R1、括號內為 R2。
// 對齊重入（000C–0047／0416–0451）由錄製排除，這裡從 0048（0452）起。

func init() {
	for _, r := range []string{"R1", "R2"} {
		regBranches(
			BranchDef{r, r + ".B1a", "1F9F:00DB–00DD（04E5–04E7）", "外碼全部同側，返回"},
			BranchDef{r, r + ".B1b", "1F9F:00EE–014A（04F8–0554）", "單列（minY = maxY）"},
			BranchDef{r, r + ".skipL", "1F9F:0169–0175（0573–057F）", "左邊跳過（ymin 之上或 dy = 0）"},
			BranchDef{r, r + ".skipR", "1F9F:01BD–01C9（05C7–05D3）", "右邊跳過"},
			BranchDef{r, r + ".rowsMaxY", "1F9F:01FB–0200（0605–060A）", "maxY ≤ ymax：rows = maxY − minY"},
			BranchDef{r, r + ".rowsYmax", "1F9F:01FE（0608）", "maxY > ymax：rows = ymax − minY"},
			BranchDef{r, r + ".B1c", "1F9F:020D–024F（0617–0659）", "上剪裁預步進"},
			BranchDef{r, r + ".xchg", "1F9F:028C／02F2（0688／0704）", "該列左右交換"},
			BranchDef{r, r + ".lastRow", "1F9F:02DB／034D（06EB／0773）", "rows = 0：外插畫最後一列"},
			BranchDef{r, r + ".chgL", "1F9F:0361–03B9（0788–07E0）", "換左邊"},
			BranchDef{r, r + ".chgR", "1F9F:03BB–0413（07E2–083A）", "換右邊"},
			BranchDef{r, r + ".chg0to1", "1F9F:0381–0384／03DB–03DE", "換邊 dy = 0 改 1"},
			BranchDef{r, r + ".B1e.skip", "1F9F:02FC–0304（070E–0716）", "剪裁路徑：本列整列在視埠外"},
			BranchDef{r, r + ".B1e.lclip", "1F9F:0311／0357（0723／077E）", "剪裁路徑：左剪裁"},
			BranchDef{r, r + ".B1e.rclip", "1F9F:0316／035D（0728／0784）", "剪裁路徑：右剪裁"},
		)
	}
	regBranches(
		BranchDef{"R1", "R1.B1d", "1F9F:0280–02D4", "未剪裁填色：閉區間 R−L+1 點"},
		BranchDef{"R1", "R1.B1e", "1F9F:02E6–0355", "剪裁填色：夾後閉區間"},
		BranchDef{"R2", "R2.B2d", "1F9F:067A–06E4", "未剪裁：R−L 點（[L, R−1]）"},
		BranchDef{"R2", "R2.B2d.none", "1F9F:069E–06A0", "未剪裁：R−L ≤ 1，不寫"},
		BranchDef{"R2", "R2.B2d.odd", "1F9F:06A4–06AB", "未剪裁：奇數先寫一點"},
		BranchDef{"R2", "R2.B2e", "1F9F:06F6–0771", "剪裁：夾後閉區間"},
		BranchDef{"R2", "R2.B2e.one", "1F9F:0731–0733", "剪裁：夾後 1 點，不寫"},
		BranchDef{"R2", "R2.B2e.odd", "1F9F:0735–073C", "剪裁：奇數先寫一點"},
	)
}

// verts 讀頂點陣列：指標 p 是頂點段內的 16 位元偏移，start 是陣列起點（[bp+0Ah]）。
type verts struct {
	c     *context
	b     []byte
	start uint16
}

func (v verts) w(p uint16, f uint16) uint16 {
	o := int(uint16(p - v.start + f))
	if o+1 >= len(v.b) {
		v.c.fault("頂點讀出錄製範圍：%04X+%d", p, f)
		return 0
	}
	return uint16(v.b[o]) | uint16(v.b[o+1])<<8
}
func (v verts) x(p uint16) uint16 { return v.w(p, 0) }
func (v verts) y(p uint16) uint16 { return v.w(p, 2) }

// flat 是 R1、R2 的共同狀態（[bp−xx] 的變數）。
type flat struct {
	xmin, ymin, xmax, ymax uint16 // [bp−2]、[bp−4]、[bp−6]、[bp−8]
	pitch, tab0            uint16 // [bp−0Ah]、[bp−0Ch]
	start, end             uint16 // [bp−12h]、[bp−14h]
	hclip                  uint16 // [bp−10h]
	la, ra                 uint16 // [bp−16h]、[bp−18h]：目前左／右邊的起點
	L, R                   uint16 // [bp−1Ah]、[bp−1Ch]：目前左／右邊的終點
	lcnt, rcnt             uint16 // [bp−1Eh]、[bp−20h]
	rows                   uint16 // [bp−22h]
	y0                     uint16 // [bp−24h]
	lx, rx                 uint32 // [bp−28h]、[bp−2Ch]
	lsl, rsl               uint32 // [bp−30h]、[bp−34h]
}

func (c *context) prevV(f *flat, p uint16) uint16 {
	if !(p > f.start) { // ja（無號）
		p = f.end
	}
	return p - 22
}

func (c *context) nextV(f *flat, p uint16) uint16 {
	p += 22
	if !(p < f.end) { // jb（無號）
		p = f.start
	}
	return p
}

// edgeSlope：mov dx,[si]; sub dx,[bx]; shl edx,16; mov ax,dx; shl eax,16; sar edx,16; idiv ecx。
// 被除數 = int16(Δx) × 2^32，除數 = int32(cnt << 16)。
func (c *context) edgeSlope(dxw uint16, cnt uint16, where string) uint32 {
	q, _ := c.idiv64(int64(int16(dxw))<<32, int32(uint32(cnt)<<16), where)
	return uint32(q)
}

// flatSetup：1F9F:0048–024F（0452–0659）。回傳 false 表示返回不畫。
func (c *context) flatSetup(k *Call, f *flat, v verts, rn string) bool {
	f.xmin, f.ymin, f.xmax, f.ymax = k.Desc[2], k.Desc[3], k.Desc[4], k.Desc[5]
	// 0063–0072：pitch = tab[1] − tab[0]、Tab0 = tab[0]
	f.pitch = c.tab(1) - c.tab(0)
	f.tab0 = c.tab(0)
	// 0075–0085：lds bx,[bp+0Ah]；end = n × 22 + start（mul 取低字）
	f.start = k.Params[2]
	f.end = uint16(uint32(k.Params[1])*22) + f.start
	// 0088–00DD：外碼
	f.hclip = 0
	si, di := uint16(0x7FFF), uint16(0x8000)
	ch := byte(0x0F)
	var dx uint16
	bx := f.start
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("%s 外碼迴圈超過上限", rn)
			return false
		}
		cl := byte(0)
		cl = cl<<1 | byte((v.x(bx)-f.xmin)>>15) // add ax,ax；adc cl,cl：結果的符號位
		cl = cl<<1 | byte((f.xmax-v.x(bx))>>15)
		f.hclip |= uint16(cl) // or byte [bp−10h],cl
		cl = cl<<1 | byte((v.y(bx)-f.ymin)>>15)
		cl = cl<<1 | byte((f.ymax-v.y(bx))>>15)
		ax := v.y(bx)
		if !(int16(ax) > int16(si)) { // 00C3 cmp ax,si；jg
			si = ax
			dx = bx
		}
		if !(int16(ax) < int16(di)) { // 00CB cmp ax,di；jl
			di = ax
		}
		ch &= cl
		bx += 22
		if bx == f.end {
			break
		}
	}
	if ch != 0 { // 00DB
		c.br.hit(rn + ".B1a")
		return false
	}
	f.L, f.R, f.y0 = dx, dx, si // 00E1–00E7
	if di == si {               // 00EA
		c.br.hit(rn + ".B1b")
		// 00EE–010B：往前找最小 x（嚴格小才取代）
		bx := f.L
		sv := bx
		ax := v.x(sv)
		for it := 0; ; it++ {
			if it >= iterCap {
				c.fault("%s 單列左找迴圈超過上限", rn)
				return false
			}
			if !(bx > f.start) {
				bx = f.end
			}
			bx -= 22
			if bx == f.L {
				break
			}
			if int16(ax) <= int16(v.x(bx)) { // 0105 cmp ax,[bx]；jle
				continue
			}
			sv = bx
			ax = v.x(sv)
		}
		// 010B–0126：往後找最大 x
		bx = f.R
		dv := bx
		ax = v.x(dv)
		for it := 0; ; it++ {
			if it >= iterCap {
				c.fault("%s 單列右找迴圈超過上限", rn)
				return false
			}
			bx += 22
			if !(bx < f.end) {
				bx = f.start
			}
			if bx == f.R {
				break
			}
			if int16(ax) >= int16(v.x(bx)) { // 0122 cmp ax,[bx]；jge
				continue
			}
			dv = bx
			ax = v.x(dv)
		}
		// 0128–0147
		f.lx = uint32(v.x(sv)) << 16
		f.rx = uint32(v.x(dv)) << 16
		f.lsl, f.rsl, f.rows = 0, 0, 0
		return true
	}
	// 014D–01A1：左邊
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("%s 左邊設定迴圈超過上限", rn)
			return false
		}
		bx := f.L
		f.la = bx
		s := c.prevV(f, bx)
		f.L = s
		cx := v.y(s)
		dxv := v.y(bx)
		if !(int16(dxv) >= int16(f.ymin)) && int16(cx) <= int16(f.ymin) { // 0169–0171
			c.br.hit(rn + ".skipL")
			continue
		}
		cx -= dxv // 0173 sub cx,dx；je
		if cx == 0 {
			c.br.hit(rn + ".skipL")
			continue
		}
		f.lcnt = cx
		f.lsl = c.edgeSlope(v.x(s)-v.x(bx), cx, rn+" 0190")
		f.lx = uint32(v.x(bx))<<16 | 0x8000 // 0197–019E
		break
	}
	// 01A1–01F2：右邊
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("%s 右邊設定迴圈超過上限", rn)
			return false
		}
		bx := f.R
		f.ra = bx
		s := c.nextV(f, bx)
		f.R = s
		cx := v.y(s)
		dxv := v.y(bx)
		if !(int16(dxv) >= int16(f.ymin)) && int16(cx) <= int16(f.ymin) {
			c.br.hit(rn + ".skipR")
			continue
		}
		cx -= dxv
		if cx == 0 {
			c.br.hit(rn + ".skipR")
			continue
		}
		f.rcnt = cx
		f.rsl = c.edgeSlope(v.x(s)-v.x(bx), cx, rn+" 01E4")
		f.rx = uint32(v.x(bx))<<16 | 0x8000
		break
	}
	// 01F5–0202：rows = min(ymax, maxY) − minY
	ax := f.ymax - f.y0
	t := di - f.ymax
	if !(int16(di) > int16(f.ymax)) { // 01FE jg（sub di,[bp−8] 的有號比）
		ax += t
		c.br.hit(rn + ".rowsMaxY")
	} else {
		c.br.hit(rn + ".rowsYmax")
	}
	f.rows = ax
	// 0205–024F：上剪裁
	ax = f.ymin - f.y0
	if int16(f.ymin) > int16(f.y0) { // 020B jle（有號比 ymin ≤ y0）
		c.br.hit(rn + ".B1c")
		f.rows -= ax
		cx := f.ymin
		f.y0 = f.ymin
		cx -= v.y(f.la)
		f.lcnt -= cx
		f.lx += shrd16(imul64(f.lsl, uint32(cx)<<16))
		cx = f.ymin - v.y(f.ra)
		f.rcnt -= cx
		f.rx += shrd16(imul64(f.rsl, uint32(cx)<<16))
	}
	return true
}

// chgLeft：1F9F:0361–03B0（0788–07D6）。回傳新的 eax（左 x）。
func (c *context) chgLeft(f *flat, v verts, rn string) uint32 {
	c.br.hit(rn + ".chgL")
	bx := f.L
	f.la = bx
	si := c.prevV(f, bx)
	f.L = si
	cx := v.y(si) - v.y(bx)
	if cx == 0 { // cmp cx,1；adc cx,0
		c.br.hit(rn + ".chg0to1")
		cx = 1
	}
	f.lcnt = cx
	f.lsl = c.edgeSlope(v.x(si)-v.x(bx), cx, rn+" 03A0")
	return uint32(v.x(bx))<<16 | 0x8000 // 03A7–03AD
}

// chgRight：1F9F:03BB–0409（07E2–0830）。回傳新的 ebx（右 x）。
func (c *context) chgRight(f *flat, v verts, rn string) uint32 {
	c.br.hit(rn + ".chgR")
	bx := f.R
	f.ra = bx
	si := c.nextV(f, bx)
	f.R = si
	cx := v.y(si) - v.y(bx)
	if cx == 0 {
		c.br.hit(rn + ".chg0to1")
		cx = 1
	}
	f.rcnt = cx
	f.rsl = c.edgeSlope(v.x(si)-v.x(bx), cx, rn+" 03FA")
	return uint32(v.x(bx))<<16 | 0x8000
}

// r12：mode 1 = R1，mode 2 = R2。
func (c *context) r12(k *Call, mode int) {
	rn := "R1"
	if mode == 2 {
		rn = "R2"
	}
	v := verts{c: c, b: k.Verts, start: k.Params[2]}
	var f flat
	if !c.flatSetup(k, &f, v, rn) {
		return
	}
	// R1 0253–0263：色 = 頂點 0 的 +4 低位元組
	color := byte(v.w(k.Params[2], 4))
	// R2 065D：fs = [bp+10h]；表偏移 [bp+0Eh]（錄製的 256 bytes 已從表偏移起）
	tbl := k.Table
	// 0267–0270：di = Tab0 + y0 × pitch
	di := f.tab0 + f.y0*f.pitch
	eax, ebx := f.lx, f.rx
	for it := 0; ; it++ {
		if it >= iterCap {
			c.fault("%s 列迴圈超過上限", rn)
			return
		}
		// 0280／02E6（067A／06F6）
		lo, hi := eax, ebx
		if !(int32(hi) > int32(lo)) { // cmp edx,eax；jg
			lo, hi = hi, lo
			c.br.hit(rn + ".xchg")
		}
		a := uint16(lo >> 16) // shr eax,16（邏輯）
		d := uint16(hi >> 16)
		if f.hclip == 0 {
			if mode == 1 {
				c.br.hit("R1.B1d")
				cx := d - a + 1 // 0296–029A
				if c.mut["r1noright"] {
					cx-- // 負對照 M3：少畫右端
				}
				c.fillN(di+a, cx, color)
			} else {
				c.br.hit("R2.B2d")
				cx := d - a // 0692–0694，沒有 inc
				if c.mut["r2closed"] {
					cx++ // 負對照 M4：改成閉區間
				}
				c.r2span(di+a, cx, tbl, false)
			}
		} else {
			if mode == 1 {
				c.br.hit("R1.B1e")
			} else {
				c.br.hit("R2.B2e")
			}
			if int16(a) > int16(f.xmax) || int16(d) < int16(f.xmin) { // 02FC–0304
				c.br.hit(rn + ".B1e.skip")
			} else {
				cx := d - a + 1 // 0306–030A
				ddi := di + a   // 030B add edi,eax
				t := a - f.xmin // 030E sub ax,[bp−2]；jl
				if int16(a) < int16(f.xmin) {
					c.br.hit(rn + ".B1e.lclip")
					ddi -= t // 0357 sub di,ax
					cx += t  // 0359 add cx,ax
				}
				t2 := d - f.xmax // 0313 sub dx,[bp−6]；jg
				if int16(d) > int16(f.xmax) {
					c.br.hit(rn + ".B1e.rclip")
					cx -= t2
				}
				if mode == 1 {
					c.fillN(ddi, cx, color)
				} else {
					c.r2span(ddi, cx, tbl, true)
				}
			}
		}
		di += f.pitch // 02B4
		f.rows--      // 02B7 dec；js；je
		if int16(f.rows) < 0 {
			return
		}
		if f.rows == 0 {
			c.br.hit(rn + ".lastRow")
			eax += f.lsl
			ebx += f.rsl
			continue
		}
		f.lcnt--
		if f.lcnt == 0 {
			eax = c.chgLeft(&f, v, rn)
		} else {
			eax += f.lsl
		}
		f.rcnt--
		if f.rcnt == 0 {
			ebx = c.chgRight(&f, v, rn)
		} else {
			ebx += f.rsl
		}
	}
}

// fillN：rep stosb（cx & 3）＋ rep stosd（cx >> 2），合起來是 cx 個連續 byte。
func (c *context) fillN(di uint16, cx uint16, color byte) {
	n := cx & 3
	for i := uint16(0); i < n; i++ {
		c.w(di, color)
		di++
	}
	for i := uint16(0); i < cx>>2; i++ {
		if di > 0xFFFC {
			c.fault("stosd 跨段尾 di=%04X", di)
		}
		for j := 0; j < 4; j++ {
			c.w(di, color)
			di++
		}
	}
}

// r2span：R2 內迴圈。clipped=false 為 069C–06BC，true 為 072D–074D。
// 每點 al = fs:[表偏移 + 緩衝原值]；word 寫入前兩點都先讀。
func (c *context) r2span(di uint16, cx uint16, tbl []byte, clipped bool) {
	look := func(b byte) byte {
		if len(tbl) < 256 {
			c.fault("R2 表未錄")
			return 0
		}
		return tbl[b]
	}
	odd := cx&1 != 0
	cx >>= 1 // shr cx,1
	if !clipped {
		if cx == 0 { // 06A0 je
			c.br.hit("R2.B2d.none")
			return
		}
		if odd { // 06A2 jae 不跳
			c.br.hit("R2.B2d.odd")
			c.w(di, look(c.r(di)))
			di++
			if cx == 0 { // 06AB jcxz
				return
			}
		}
	} else {
		if odd { // 0731 jae 不跳
			if cx == 0 { // 0733 je
				c.br.hit("R2.B2e.one")
				return
			}
			c.br.hit("R2.B2e.odd")
			c.w(di, look(c.r(di)))
			di++
		} else if cx == 0 {
			// 夾後 0 點：jae 跳到 073E，cx = 0 時 dec cx 會環繞（原版會寫 65536 個 word）。
			c.fault("R2 剪裁點數 0 進入字迴圈")
			return
		}
	}
	for ; cx != 0; cx-- { // 06AD–06BC／073E–074D
		lo := look(c.r(di))
		hi := look(c.r(di + 1))
		c.ww(di, uint16(lo)|uint16(hi)<<8)
		di += 2
	}
}

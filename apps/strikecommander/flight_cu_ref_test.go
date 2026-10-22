package strikecommander

import "fmt"

// refFlightObserver 是 cuTracker 抽出前（dosgolem-sc eaafe2e 的 flight.go）FlightObserver 的程式碼複本，
// 只把型別名換成 refFlightObserver、刪去註解、存取方法併成單行，作為 strike_commander 規格 012 §3.2.2 抽出驗收第 2、5 項的參考實作。
// 共用的套件函式（match、readRows、readLabelString、flightBand、flightInk、inkInBox、sameFlight 等）不複製。
// 不得修改：它是差分測試的期望來源。
type refFlightObserver struct {
	OnCommit func(FlightCommit)

	resolve func(Kind, string) (string, int, bool)

	baseKnown, baseOK bool
	base              uint16
	lastVerify        uint64

	cur *flightPuts
	plq *flightPlaque

	pending   []FlightText
	cands     []FlightText
	candValid bool
	candCU    uint64
	visible   []FlightText
	gen       uint64
	lastSig   uint64

	vga bool

	cuIn, cuAwait bool
	cuSteps       uint64
	cuSS, cuSP    uint16
	cuIP          uint16

	firstRun   int
	counts     FlightCounts
	log        []Event
	logDropped int
}

func newRefFlightObserver(resolve func(Kind, string) (string, int, bool)) *refFlightObserver {
	return &refFlightObserver{resolve: resolve}
}

func (o *refFlightObserver) Base() (uint16, bool)      { return o.base, o.baseKnown && o.baseOK }
func (o *refFlightObserver) Visible() []FlightText     { return append([]FlightText(nil), o.visible...) }
func (o *refFlightObserver) Generation() uint64        { return o.gen }
func (o *refFlightObserver) Pending() []FlightText     { return append([]FlightText(nil), o.pending...) }
func (o *refFlightObserver) Candidates() []FlightText  { return append([]FlightText(nil), o.cands...) }
func (o *refFlightObserver) CUState() (bool, bool)     { return o.cuIn, o.cuAwait }
func (o *refFlightObserver) VGAWindow() bool           { return o.vga }
func (o *refFlightObserver) Log() ([]Event, int)       { return append([]Event(nil), o.log...), o.logDropped }
func (o *refFlightObserver) Counts() FlightCounts      { return o.counts }
func (o *refFlightObserver) NoteCUFrameMismatch(n int) { o.counts.CUFrameMismatch += n }
func (o *refFlightObserver) NoteFirstFrameBlocked(n int) {
	o.counts.FirstFrameBlocked += n
	if n > 0 {
		o.firstRun++
		if o.firstRun > o.counts.FirstFrameBlockedMaxRun {
			o.counts.FirstFrameBlockedMaxRun = o.firstRun
		}
	} else {
		o.firstRun = 0
	}
}

func (o *refFlightObserver) note(steps uint64, kind, format string, a ...any) {
	if len(o.log) >= maxLog {
		o.logDropped++
		return
	}
	o.log = append(o.log, Event{Steps: steps, Kind: kind, Detail: fmt.Sprintf(format, a...)})
}

func (o *refFlightObserver) Observe(v FlightView) {
	switch v.IP() {
	case ipFPuts:
		o.putsEntry(v)
	case ipFPutc:
		if o.cur != nil {
			o.putc(v)
		}
	case ipFPutsRet:
		if o.cur != nil {
			o.putsReturn(v)
		}
	case ipPlaque:
		if o.baseOK {
			o.plaqueEntry(v)
		}
	case ipPlaqueRet:
		if o.plq != nil {
			o.plaqueReturn(v)
		}
	case ipCUDiff, ipCUPlain:
		if o.baseOK {
			o.cuEntry(v)
		}
	case ipCUDiffRet, ipCUPlainRet:
		if o.baseOK {
			o.cuReturn(v)
		}
	case ipMU:
		if o.baseOK {
			o.mu(v)
		}
	}
	if len(o.visible) != 0 && o.baseOK {
		if s := v.Steps(); s-o.lastSig >= SigCheckSteps {
			o.lastSig = s
			if why := o.checkBase(v, o.base); why != "" {
				o.note(s, "sig", "base %04X 的 %s 簽章不符，可見集合、候選、待提交與 CU 狀態清空，停用", o.base, why)
				o.baseOK = false
				o.lastVerify = s
				o.reset()
			}
		}
	}
}

func (o *refFlightObserver) reset() {
	if len(o.visible) != 0 {
		o.gen++
	}
	o.visible, o.pending, o.cands, o.candValid = nil, nil, nil, false
	o.cur, o.plq = nil, nil
	o.vga = false
	o.cuIn, o.cuAwait, o.cuSteps, o.cuSS, o.cuSP, o.cuIP = false, false, 0, 0, 0, 0
}

func (o *refFlightObserver) verifyRows(v View, base uint16) string {
	for _, r := range flightBaseRows {
		if !match(v, base+r.seg, r.off, r.bytes, 0) {
			return r.name
		}
	}
	return ""
}

func (o *refFlightObserver) checkBase(v View, base uint16) string {
	if !match(v, base+segFText, ipFPuts, sigPutsEntry, 0) {
		return "puts 入口"
	}
	return o.verifyRows(v, base)
}

func (o *refFlightObserver) putsEntry(v FlightView) {
	cs := v.CS()
	if !match(v, cs, ipFPuts, sigPutsEntry, 0) {
		return
	}
	o.counts.HitPuts++
	steps := v.Steps()
	base := cs - segFText
	switch {
	case !o.baseKnown || base != o.base:
		if o.baseKnown {
			o.note(steps, "base", "puts 入口定位到不同的 base %04X（原 %04X）", base, o.base)
		}
		o.reset()
		o.base, o.baseKnown, o.lastVerify = base, true, steps
		if why := o.verifyRows(v, base); why != "" {
			o.baseOK = false
			o.note(steps, "base", "base %04X 停用：%s 簽章不符", base, why)
		} else {
			o.baseOK = true
			o.note(steps, "base", "base %04X 啟用", base)
		}
	case !o.baseOK && steps-o.lastVerify >= flightReverifySteps:
		o.lastVerify = steps
		if why := o.verifyRows(v, base); why == "" {
			o.baseOK = true
			o.reset()
			o.note(steps, "base", "base %04X 重驗成功，啟用", base)
		}
	}
	if !o.baseOK {
		o.counts.SkipBase++
		return
	}
	if o.vga {
		o.counts.SkipVGA++
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	kind, ok := o.classify(v, ss)
	if !ok {
		o.counts.SkipCaller++
		return
	}
	obj := rd16(v, ss, sp+4)
	str := rd16(v, ss, sp+6)
	key, why := readLabelString(v, ds, str)
	if why != "" {
		o.counts.SkipString++
		return
	}
	catKey, k, ok := o.resolve(kind, key)
	if !ok {
		o.counts.SkipResolve++
		return
	}
	if o.cur != nil {
		o.note(steps, "puts", "上一筆 %q 的 puts 沒有返回，捨棄", o.cur.t.Key)
	}
	yp := int(rd16(v, ds, obj+objY))
	t := FlightText{Key: key, CatKey: catKey, Kind: kind, K: k, DS: ds, Steps: steps,
		Seg: rd16(v, ds, obj+objSeg), List: rd16(v, ds, obj+objList)}
	t.B0, t.BH = flightBand(kind, yp)
	t.Before = readRows(v, ds, t.Seg, t.List, t.B0, t.B0+t.BH)
	o.cur = &flightPuts{t: t, obj: obj, ss: ss, sp: sp, yPrime: yp}
}

func (o *refFlightObserver) classify(v View, ss uint16) (Kind, bool) {
	p := v.BP()
	text := o.base + segFText
	if rd16(v, ss, p+2) != ipFFmtRet || rd16(v, ss, p+4) != text {
		return "", false
	}
	q := rd16(v, ss, p)
	r, s := rd16(v, ss, q+2), rd16(v, ss, q+4)
	switch {
	case s == o.base+segFHUD && r == ipFHUDRet:
		if match(v, s, r-uint16(len(sigHUDCall)), sigHUDCall, text) {
			return KindHUD, true
		}
	case s == o.base+segFMFD && r == ipFMFDRet:
		if match(v, s, r-uint16(len(sigMFDCall)), sigMFDCall, text) {
			return KindMFD, true
		}
	}
	return "", false
}

func (o *refFlightObserver) putc(v FlightView) {
	if v.CS() != o.base+segFText {
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	if rd16(v, ss, sp+4) != o.cur.obj {
		return
	}
	if len(o.cur.t.Xs) == 0 {
		o.cur.t.Y = int(rd16(v, ds, o.cur.obj+objY))
	}
	o.cur.t.Xs = append(o.cur.t.Xs, int(rd16(v, ds, o.cur.obj+objX)))
}

func (o *refFlightObserver) putsReturn(v FlightView) {
	if v.CS() != o.base+segFText || v.SS() != o.cur.ss || v.SP() != o.cur.sp {
		return
	}
	c := o.cur
	o.cur = nil
	steps := v.Steps()
	t := c.t
	t.X1 = int(rd16(v, v.DS(), c.obj+objX))
	if len(t.Xs) != len(t.Key) {
		o.counts.SkipVoid++
		o.note(steps, "void", "%q 的 putc %d 次，字串 %d 字，作廢", t.Key, len(t.Xs), len(t.Key))
		return
	}
	t.X0 = t.Xs[0]
	if t.Y != c.yPrime {
		o.counts.SkipVoid++
		o.note(steps, "void", "%q 第一次 putc 的 Y %d 不等於 puts 入口的 y′ %d，作廢", t.Key, t.Y, c.yPrime)
		return
	}
	t.After = readRows(v, v.DS(), t.Seg, t.List, t.B0, t.B0+t.BH)
	x0, y0, x1, y1 := t.InkBox()
	t.Ink = flightInk(t.Key, t.Before, t.After, t.B0, x0, y0, x1, y1)
	if !inkInBox(t.Ink) {
		o.counts.SkipInk++
		o.note(steps, "ink", "墨跡框核對不通過：%s", t.Ink)
		return
	}
	o.addPending(steps, t)
}

func (o *refFlightObserver) addPending(steps uint64, t FlightText) {
	for i := 0; i < len(o.pending); i++ {
		p := o.pending[i]
		if p.Kind == t.Kind && p.CatKey == t.CatKey && p.Y == t.Y && p.Anchor() == t.Anchor() {
			o.pending = append(o.pending[:i], o.pending[i+1:]...)
			o.counts.SkipDedup++
			i--
		}
	}
	if len(o.pending) >= maxFlightPending {
		o.counts.SkipOverflow++
		o.note(steps, "pending", "待提交清單滿 %d 筆，捨棄最舊的 %q", maxFlightPending, o.pending[0].Key)
		o.pending = append(o.pending[:0], o.pending[1:]...)
	}
	o.pending = append(o.pending, t)
}

func (o *refFlightObserver) plaqueEntry(v FlightView) {
	cs := v.CS()
	if v.Read8(lin(cs, ipPlaque)) != 0x55 {
		return
	}
	if !match(v, cs, ipPlaque, sigPlaqueHead, 0) || !match(v, cs, ipPlaqueLoop, sigPlaqueLoop, 0) {
		return
	}
	o.counts.HitPlaque++
	steps := v.Steps()
	if o.plq != nil {
		o.note(steps, "plaque", "上一筆橫幅 %q 沒有返回，捨棄", o.plq.t.Key)
		o.plq = nil
	}
	if o.vga {
		o.counts.SkipVGA++
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	desc := rd16(v, ss, sp+6)
	x := int(rd16(v, ss, sp+8))
	y := int(int16(rd16(v, ss, sp+0x0A)))
	off, seg := rd16(v, ss, sp+0x0C), rd16(v, ss, sp+0x0E)
	key, why := readLabelString(v, seg, off)
	if why != "" {
		o.counts.SkipString++
		return
	}
	if r := readRect(v, ds, desc); rd16(v, ds, desc) != 0xFFFF || r.x0 > r.x1 || r.x1 > 319 || r.y0 > r.y1 || r.y1 > 199 {
		o.counts.SkipDesc++
		o.note(steps, "plaque", "橫幅 %q 的描述元 DS:%04X 不是畫外緩衝或 rect 超出畫面，略過", key, desc)
		return
	}
	catKey, k, ok := o.resolve(KindPlaque, key)
	if !ok {
		o.counts.SkipResolve++
		return
	}
	t := FlightText{Key: key, CatKey: catKey, Kind: KindPlaque, K: k, X0: x, Y: y, DS: ds, Steps: steps,
		Seg: rd16(v, ds, desc), List: rd16(v, ds, desc+2)}
	t.B0, t.BH = flightBand(KindPlaque, y)
	t.Before = readRows(v, ds, t.Seg, t.List, t.B0, t.B0+t.BH)
	o.plq = &flightPlaque{t: t, cs: cs, ss: ss, sp: sp}
}

func (o *refFlightObserver) plaqueReturn(v FlightView) {
	if v.CS() != o.plq.cs || v.SS() != o.plq.ss || v.SP() != o.plq.sp {
		return
	}
	c := o.plq
	o.plq = nil
	steps := v.Steps()
	t := c.t
	t.X1 = int(v.AX())
	t.After = readRows(v, t.DS, t.Seg, t.List, t.B0, t.B0+t.BH)
	x0, y0, x1, y1 := t.InkBox()
	t.Ink = flightInk(t.Key, t.Before, t.After, t.B0, x0, y0, x1, y1)
	if !inkInBox(t.Ink) {
		o.counts.SkipInk++
		o.note(steps, "ink", "墨跡框核對不通過：%s", t.Ink)
		return
	}
	o.addPending(steps, t)
}

func (o *refFlightObserver) jemmOK(v View) bool {
	return v.CS() == o.base+segFJemm && match(v, o.base+segFJemm, 0x0090, sigJemm, 0)
}

func (o *refFlightObserver) checkTimeout(steps uint64) {
	if o.cuIn && steps-o.cuSteps >= cuTimeoutSteps {
		o.cuIn = false
		o.counts.Timeouts++
		o.note(steps, "cu", "第 %d 步的 CU 入口超過 %d 步沒有返回，逾時", o.cuSteps, cuTimeoutSteps)
	}
}

func (o *refFlightObserver) cuEntry(v FlightView) {
	if !o.jemmOK(v) {
		return
	}
	ip := v.IP()
	if ip == ipCUDiff {
		o.counts.HitCUDiff++
	} else {
		o.counts.HitCUPlain++
	}
	if v.CX()>>8 != 0 {
		o.counts.CUCHNonZero++
		return
	}
	steps := v.Steps()
	ss, sp := v.SS(), v.SP()
	o.checkTimeout(steps)
	if o.cuIn {
		if ss == o.cuSS && sp == o.cuSP && steps-o.cuSteps < cuDupSteps {
			o.counts.EntriesDuplicate++
			return
		}
		o.counts.EntriesVoided++
		o.note(steps, "cu", "第 %d 步的 CU 入口（SS:SP %04X:%04X）沒有返回就有新入口（%04X:%04X），舊的作廢", o.cuSteps, o.cuSS, o.cuSP, ss, sp)
		o.cuIn = false
	}
	kept := o.pending[:0:0]
	for _, t := range o.pending {
		if steps-t.Steps >= flightStaleSteps {
			o.counts.SkipStale++
			o.note(steps, "stale", "%q（第 %d 步入口）早於 CU %d 步以上，捨棄", t.Key, t.Steps, flightStaleSteps)
			continue
		}
		t.Ref = readRows(v, t.DS, t.Seg, t.List, t.B0, t.B0+t.BH)
		kept = append(kept, t)
	}
	if o.candValid {
		o.counts.CandidatesReplaced++
		if o.cuAwait {
			o.counts.CandidatesReplacedAwaiting++
		}
		o.note(steps, "cu", "第 %d 步 CU 的候選 %d 筆尚未升格，被取代", o.candCU, len(o.cands))
	}
	o.cands, o.candValid, o.candCU = kept, true, steps
	o.pending = nil
	o.cuAwait = false
	o.vga = false
	o.cuIn, o.cuSteps, o.cuSS, o.cuSP, o.cuIP = true, steps, ss, sp, ip
}

func (o *refFlightObserver) cuReturn(v FlightView) {
	if v.CS() != o.base+segFJemm {
		return
	}
	ip := v.IP()
	if ip == ipCUDiffRet {
		o.counts.HitRetDiff++
	} else {
		o.counts.HitRetPlain++
	}
	steps := v.Steps()
	o.checkTimeout(steps)
	if !o.cuIn {
		o.counts.ReturnsIgnored++
		return
	}
	want := uint16(ipCUDiffRet)
	if o.cuIP == ipCUPlain {
		want = ipCUPlainRet
	}
	if ip != want || v.SS() != o.cuSS || v.SP() != o.cuSP {
		o.counts.ReturnsSSMismatch++
		o.note(steps, "cu", "CU 返回 %04X SS:SP %04X:%04X 與入口 %04X %04X:%04X 不配對", ip, v.SS(), v.SP(), o.cuIP, o.cuSS, o.cuSP)
		return
	}
	o.cuIn, o.cuAwait = false, true
	o.counts.ReturnsAccepted++
}

func (o *refFlightObserver) mu(v FlightView) {
	if !o.jemmOK(v) {
		return
	}
	o.counts.HitMU++
	if v.CX() == 0 {
		if len(o.pending) != 0 {
			o.note(v.Steps(), "vga", "MU CX=0：HMA 視窗指向 VGA，待提交 %d 筆清空", len(o.pending))
		}
		o.vga = true
		o.pending = nil
		return
	}
	o.vga = false
}

func (o *refFlightObserver) Frame(indexed []byte, steps uint64) {
	o.checkTimeout(steps)
	if o.cuIn {
		o.counts.CUFrames++
		return
	}
	if !o.cuAwait {
		return
	}
	prev := o.visible
	o.visible = o.cands
	o.cands, o.candValid, o.cuAwait = nil, false, false
	if !sameFlight(prev, o.visible) {
		o.gen++
	}
	o.lastSig = steps
	o.counts.Promotions++
	if o.OnCommit != nil {
		c := FlightCommit{CUSteps: o.candCU, FrameSteps: steps, Generation: o.gen}
		for _, t := range o.visible {
			c.Texts = append(c.Texts, t.stripped())
		}
		o.OnCommit(c)
	}
}

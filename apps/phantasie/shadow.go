package phantasie

import (
	"github.com/wicanr2/dosgolem/xlate"
)

// 內容定址的頁面影子（docs/spec/002 §4）。影子與語言無關：只存事件編號、垂直位移與已不可見的 x 範圍。

const (
	// MaxKnownShadows 是 known 的容量（淘汰最久沒用到者）。
	MaxKnownShadows = 32
	// MaxEmptyShadows 是 empties 的容量（先進先出）。
	MaxEmptyShadows = 64
)

// ShadowEntry 是影子的一筆（002 §4）。
type ShadowEntry struct {
	ID     string   // 事件編號 g<N>；由 records[ID] 取得 EventRecord
	DY     int      // 疊字的 Y 相對於 records[ID].Row×8 的位移
	Hidden []xrange // 事件矩形內已不可見的像素 x 範圍
}

// Shadow 依 Layer.Stamps 的順序（疊序）。
type Shadow []ShadowEntry

type shadowState uint8

const (
	shadowUnknown shadowState = iota
	shadowEmpty
	shadowKnown
)

type knownShadow struct {
	s    Shadow
	used uint64
}

type shadowStore struct {
	known   map[uint64]*knownShadow
	tick    uint64
	empties map[uint64]struct{}
	fifo    []uint64
}

func newShadowStore() *shadowStore {
	return &shadowStore{known: map[uint64]*knownShadow{}, empties: map[uint64]struct{}{}}
}

// reset 清空 known 與 empties（視訊模式改變，002 §5）。
func (st *shadowStore) reset() {
	st.known = map[uint64]*knownShadow{}
	st.empties = map[uint64]struct{}{}
	st.fifo = nil
}

// get 取得影子：known 內的非空影子、empties 內的空影子、否則未知。get 與 set 都算用到。
func (st *shadowStore) get(h uint64) (Shadow, shadowState) {
	if k, ok := st.known[h]; ok {
		st.tick++
		k.used = st.tick
		return k.s, shadowKnown
	}
	if _, ok := st.empties[h]; ok {
		return nil, shadowEmpty
	}
	return nil, shadowUnknown
}

func (st *shadowStore) setKnown(h uint64, s Shadow) {
	st.dropEmpty(h)
	st.tick++
	st.known[h] = &knownShadow{s: s, used: st.tick}
	if len(st.known) > MaxKnownShadows {
		var oldest uint64
		var ou uint64
		first := true
		for k, v := range st.known {
			if first || v.used < ou {
				oldest, ou, first = k, v.used, false
			}
		}
		delete(st.known, oldest)
	}
}

func (st *shadowStore) dropEmpty(h uint64) {
	if _, ok := st.empties[h]; !ok {
		return
	}
	delete(st.empties, h)
	for i, v := range st.fifo {
		if v == h {
			st.fifo = append(st.fifo[:i], st.fifo[i+1:]...)
			break
		}
	}
}

func (st *shadowStore) addEmpty(h uint64) {
	delete(st.known, h)
	if _, ok := st.empties[h]; ok {
		return
	}
	st.empties[h] = struct{}{}
	st.fifo = append(st.fifo, h)
	if len(st.fifo) > MaxEmptyShadows {
		delete(st.empties, st.fifo[0])
		st.fifo = st.fifo[1:]
	}
}

// referenced 回 known 內所有條目引用的事件編號（004 §5：records 的保留規則）。
func (st *shadowStore) referenced() map[string]struct{} {
	out := map[string]struct{}{}
	for _, k := range st.known {
		for _, e := range k.s {
			out[e.ID] = struct{}{}
		}
	}
	return out
}

// shadowFromLayer 依 Layer.Stamps 順序，對每個不同的 Key 產生一筆條目（002 §4）。
// 同一事件各疊字的 DY 不一致時該事件不入影子（shadow_skip）；records 已不存在者同樣略過。
func (o *Overlay) shadowFromLayer() Shadow {
	var s Shadow
	for _, key := range o.groupKeys() {
		rec := o.records[key]
		if rec == nil {
			o.C.Inc("shadow_lost")
			continue
		}
		stamps := o.groupStamps(key)
		dy, ok := groupDY(stamps, rec)
		if !ok {
			o.C.Inc("shadow_skip")
			continue
		}
		s = append(s, ShadowEntry{ID: key, DY: dy, Hidden: hiddenOf(rec, stamps)})
	}
	return s
}

// effRow 是條目的有效列：records[ID].Row + DY/8。
func (o *Overlay) effRow(e ShadowEntry) (int, bool) {
	rec := o.records[e.ID]
	if rec == nil {
		return 0, false
	}
	return (rec.Row*8 + e.DY) / 8, true
}

// restoreShadow 由影子還原（002 §4）：先清空 Layer，再依影子順序以 shadowLang 重新解析與排版，
// 疊字直接附加到 Layer.Stamps（保留疊序，不經 Layer.Add 的覆蓋判斷），State=Pending，
// 並登記到有效性閘門（001 §8：閘門只對這條路徑執行）。
func (o *Overlay) restoreShadow(s Shadow) {
	o.Layer.Clear(0, 0, screenW, screenH)
	lang := o.langs[o.shadowLang]
	if lang == nil {
		return
	}
	rs := &Resolver{Cat: lang.Cat, Wide: lang.Wide, Font: lang.Font}
	for _, e := range s {
		rec := o.records[e.ID]
		if rec == nil {
			o.C.Inc("shadow_lost")
			continue
		}
		res := rs.Resolve(rec)
		if !res.OK {
			o.C.Inc("shadow_lost")
			continue
		}
		line, _, err := layoutLine(res.Zh, res.Center, availH(rec.Col, len(rec.Text)), lang.Wide)
		if err != nil {
			o.C.Inc("shadow_lost")
			continue
		}
		o.hits[e.ID] = res.Hits
		for _, st := range buildStamps(e.ID, rec.Col*8, rec.Row*8+e.DY, line, lang.Wide, lang.Font, e.Hidden) {
			o.Layer.Stamps = append(o.Layer.Stamps, st)
			o.gate[st] = struct{}{}
		}
	}
	o.C.Inc("shadow_restore")
}

// OnSave1 是 save1 完成（螢幕 → 緩衝區 1）：h 是螢幕 4000h bytes 的雜湊（002 §4）。
func (o *Overlay) OnSave1(h uint64) {
	s := o.shadowFromLayer()
	if len(s) == 0 {
		o.sh.addEmpty(h)
		return
	}
	o.sh.setKnown(h, s)
}

// OnLoad 是 load1、load2 完成（緩衝區 → 螢幕）：h 是入口時對應緩衝區內容的雜湊。
func (o *Overlay) OnLoad(h uint64) {
	s, st := o.sh.get(h)
	switch st {
	case shadowKnown:
		o.restoreShadow(s)
	case shadowEmpty:
		o.Layer.Clear(0, 0, screenW, screenH)
	default:
		o.Layer.Clear(0, 0, screenW, screenH)
		o.sh.addEmpty(h)
	}
}

// OnRow24 是 row24 完成：hOld 是入口時緩衝區 1 的雜湊，hNew 是把第 24 列換成螢幕內容後的雜湊（002 §4）。
func (o *Overlay) OnRow24(hOld, hNew uint64) {
	sOld, st := o.sh.get(hOld)
	if st == shadowUnknown {
		return
	}
	var ns Shadow
	for _, e := range sOld {
		if r, ok := o.effRow(e); ok && r == 24 {
			continue
		}
		ns = append(ns, e)
	}
	for _, e := range o.shadowFromLayer() {
		if r, ok := o.effRow(e); ok && r == 24 {
			ns = append(ns, e)
		}
	}
	if len(ns) == 0 {
		o.sh.addEmpty(hNew)
		return
	}
	o.sh.setKnown(hNew, ns)
}

// OnInt10 是 int86 的 INT 10h 完成（002 §5；其他中斷號由呼叫端排除）。矩形與夾邊的次序依 dosgolem 規格
// 250-cga-int10-scroll-and-palette §3.1。
func (o *Overlay) OnInt10(ax, bx, cx, dx uint16) {
	ah, al := byte(ax>>8), byte(ax)
	switch ah {
	case 0x00:
		o.Layer.Clear(0, 0, screenW, screenH)
		o.sh.reset()
		o.C.Inc("op_int10_00")
	case 0x06, 0x07:
		o.C.Inc("op_int10_" + hex2(ah))
		top, left, bottom, right := int(cx>>8), int(cx&0xFF), int(dx>>8), int(dx&0xFF)
		if top > bottom || left > right {
			return
		}
		if bottom > 24 {
			bottom = 24
		}
		if right > 39 {
			right = 39
		}
		if top > bottom || left > right {
			return
		}
		x0, y0, x1, y1 := left*8, top*8, (right+1)*8, (bottom+1)*8
		rows := bottom - top + 1
		if al == 0 || int(al) >= rows {
			o.clearRect(x0, y0, x1, y1)
			return
		}
		if ah == 0x06 {
			o.Layer.Scroll(x0, y0, x1, y1, -8*int(al))
		} else {
			o.Layer.Scroll(x0, y0, x1, y1, 8*int(al))
		}
	case 0x0B:
		o.C.Inc("op_int10_0B")
		for _, s := range o.Layer.Stamps {
			s.State = xlate.Pending
		}
	}
}

func hex2(b byte) string {
	const d = "0123456789ABCDEF"
	return string([]byte{d[b>>4], d[b&15]})
}

// OnInvert 是 invert 或 invert2 完成：與矩形 [col×8, row×8, (col+width)×8, (row+1)×8) 相交的疊字，
// 其整個事件組都改 Pending（002 §5）。dim 為 true 時是 invert2，另計 dim。
func (o *Overlay) OnInvert(row, col, width int, dim bool) {
	if dim {
		o.C.Inc("op_invert2")
		o.C.Inc("dim")
	} else {
		o.C.Inc("op_invert")
	}
	x0, y0, x1, y1 := col*8, row*8, (col+width)*8, (row+1)*8
	hit := map[string]struct{}{}
	for _, s := range o.Layer.Stamps {
		sx0, sy0, sx1, sy1 := s.Rect()
		if sx0 < x1 && x0 < sx1 && sy0 < y1 && y0 < sy1 {
			hit[s.Key] = struct{}{}
		}
	}
	for _, s := range o.Layer.Stamps {
		if _, ok := hit[s.Key]; ok {
			s.State = xlate.Pending
		}
	}
}

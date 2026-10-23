package strikecommander

// 入口重看（strike_commander 規格 009 第 6 版 §3.2.1 第 5 條、§3.2.2 第 8 條）。
//
// IRQ 在受監看入口（puts 0345、putc 02AC、橫幅 0182）送達時，該指令沒有執行，返回後同一 PC、同一 SS:SP 會再被觀察。
// 入口真的改了觀察器狀態時記 pend；下一個觀察點（任何 PC）若送達計數已增加，就撤銷那次入口的改動。
// 送達計數以 viewDeliveries 取得；不提供 Deliveries 的視圖恆回 0，規則不觸發（行為與第 5 版相同）。

// 重看的入口種類（FlightRedeliverEvent.Kind）。
const (
	RedelivPutc   = "putc"
	RedelivPuts   = "puts"
	RedelivPlaque = "plaque"

	maxRedelivered = 4096
)

// FlightRedeliverEvent 是一次被撤銷的入口（§3.11 第 6 版）。Steps 是第一次觀察的步數；Key 是所屬字串的觀察字串。
type FlightRedeliverEvent struct {
	Steps uint64
	Kind  string
	Key   string
}

// flightPend 是一次待確認的入口（§3.2.1 第 5 條第 2 款）。
type flightPend struct {
	kind   string
	ss, sp uint16
	d0     uint64
	steps  uint64
	key    string
}

// markPend 在入口改了觀察器狀態之後呼叫。
func (o *FlightObserver) markPend(v FlightView, kind, key string) {
	o.pend = flightPend{kind: kind, ss: v.SS(), sp: v.SP(), d0: viewDeliveries(v), steps: v.Steps(), key: key}
	o.pendOn = true
}

// checkPend 是 §3.2.1 第 5 條第 3 款：在 Observe 的 IP 分派之前，送達計數增加就撤銷 pend 那次入口的改動。
// 兩種情形都清 pend；撤銷時 cur／plq 已是 nil 就不做其他事。
func (o *FlightObserver) checkPend(v FlightView) {
	p := o.pend
	o.pendOn = false
	if viewDeliveries(v) <= p.d0 {
		return
	}
	switch p.kind {
	case RedelivPutc:
		if o.cur == nil || len(o.cur.t.Xs) == 0 {
			return
		}
		o.cur.t.Xs = o.cur.t.Xs[:len(o.cur.t.Xs)-1]
		if len(o.cur.t.Xs) == 0 {
			o.cur.t.Y = 0
		}
		o.counts.PutcRedelivered++
	case RedelivPuts:
		if o.cur == nil {
			return
		}
		o.cur = nil
		o.counts.PutsRedelivered++
	case RedelivPlaque:
		if o.plq == nil {
			return
		}
		o.plq = nil
		o.counts.PlaqueRedelivered++
	default:
		return
	}
	o.note(p.steps, "redeliv", "%s 入口（%q，SS:SP %04X:%04X）在第 %d 步之前有送達（%d → %d），指令沒有執行，撤銷等重看",
		p.kind, p.key, p.ss, p.sp, v.Steps(), p.d0, viewDeliveries(v))
	if len(o.redeliv) >= maxRedelivered {
		o.redelivDropped++
		return
	}
	o.redeliv = append(o.redeliv, FlightRedeliverEvent{Steps: p.steps, Kind: p.kind, Key: p.key})
}

// Redelivered 回被撤銷入口的事件複本與因上限（4096 筆）捨棄的筆數（§3.11 第 6 版）。
func (o *FlightObserver) Redelivered() ([]FlightRedeliverEvent, int) {
	return append([]FlightRedeliverEvent(nil), o.redeliv...), o.redelivDropped
}

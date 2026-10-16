package cpu

import "errors"

// V86 事件與回 V86（docs/spec/435-v86-hooks-and-idt-delivery §2.1、§2.2、§2.5）。
//
// V86 下需要經保護模式 IDT 遞送的事件（軟體中斷、例外）由核心以 v86Event 記下，Step 把 IP 設回本道指令起點後
// 回 ErrV86Event；遞送（堆疊切換、框架、載入閘）由 Machine 做。分段模式（ring 0）的 iretd 彈出 VM=1 時，核心
// 讀出 9 個值存進 V86Return 後回 ErrV86Return，狀態載入由 Machine 做。

// ErrV86Event 是 V86 事件的哨兵（435 §2.2）：IP 已設回本道指令起點，事件以 V86Event() 讀取。
var ErrV86Event = errors.New("cpu: V86 事件，經保護模式 IDT 遞送交給 Machine（docs/spec/435 §2）")

// ErrV86Return 是 ring 0 iretd 回 V86 的哨兵（435 §2.5）：IP 已設回本道指令起點，彈出值以 V86Return() 讀取。
var ErrV86Return = errors.New("cpu: 分段模式 iretd 彈出 VM=1，回 V86 交給 Machine（docs/spec/435 §2.5）")

// V86EventKind 是事件種類（435 §2.1 的表）。種類決定遞送時做哪些檢查。
type V86EventKind uint8

const (
	// V86SoftInt 是 V86 的 CD nn：做 IOPL 檢查與閘 DPL 檢查，框架 EIP 為下一道。
	V86SoftInt V86EventKind = iota + 1
	// V86Into 是 INTO（CE，OF=1）：同 V86SoftInt，向量 4。
	V86Into
	// V86Int3 是 INT 3（CC）：不做 IOPL 檢查（DOSBox-X NoIOPLCheck），做閘 DPL 檢查，向量 3。
	V86Int3
	// V86External 是外部中斷（IRQ0、IRQ1）：不做 IOPL、DPL 檢查，框架 EIP 為遞送當下的 IP。
	V86External
	// V86Exception 是 CPU 例外（#DE、#BR、#GP）：不做 IOPL、DPL 檢查，框架 EIP 為指令起點。
	V86Exception
)

// V86Event 是一個待遞送的 V86 事件（435 §2.2）。Reason 保留 436 的診斷字串。
type V86Event struct {
	Kind      V86EventKind
	Vector    uint8
	HasError  bool
	ErrorCode uint32
	FrameEIP  uint32
	Reason    string
}

// V86Return 是 ring 0 iretd 回 V86 時從 SS:SP 彈出的 9 個值（435 §2.5 第 1 步的順序）。
type V86Return struct {
	EIP, CS, EFLAGS, ESP, SS, ES, DS, FS, GS uint32
}

// V86Event 回最近一次 ErrV86Event 的事件副本。內容在下一次 Step 前有效。
func (c *CPU) V86Event() V86Event { return c.evOut }

// V86Return 回最近一次 ErrV86Return 的彈出值。內容在下一次 Step 前有效。
func (c *CPU) V86Return() V86Return { return c.v86Ret }

// v86Placeholder 是 v86Event 設在 c.fault 的佔位錯誤（預先配置的單一值）。Step 看到 c.ev 就改回 ErrV86Event，
// 這個值不會流出核心，也不會被改寫。
var v86Placeholder = &Error{Reason: "V86 事件佔位（docs/spec/435 §2.2）"}

// v86Event 記下 V86 事件（435 §2.2）：事件存在 c.ev，同時設佔位錯誤，由 Step 的 fault 分支統一處理。
// 本道指令已先記了一般 failf 時以先記的為準，不記事件。呼叫點在任何寫入之前。
func (c *CPU) v86Event(ev V86Event) {
	if c.fault != nil {
		return
	}
	c.evBuf = ev
	c.ev = &c.evBuf
	c.fault = v86Placeholder
}

// v86GP0 記 #GP(0) 事件（435 §2.1）：框架 EIP 為指令起點，錯誤碼 0。
func (c *CPU) v86GP0(reason string) {
	c.v86Event(V86Event{Kind: V86Exception, Vector: 13, HasError: true, FrameEIP: uint32(c.opIP), Reason: reason})
}

// v86ReturnRead 是 435 §2.5 第 1–3 步：分段模式、NT=0、CPL 0 的 66 CF。以 SS 快取讀 SS:SP 起的 dword
// （位移 16 位元環繞，434 §4.1 的界限檢查在 read32），彈出的 EFLAGS 有 VM=1 時再讀 6 個；檢查通過後存進
// c.v86Ret，IP 設回起點，回 ErrV86Return。只讀不寫。
func (c *CPU) v86ReturnRead(op uint8) error {
	c.stackCheck()
	sp := c.R[SP]
	at := func(k int) uint32 { return c.read32(SS, uint32(sp+uint16(4*k))) }
	var r V86Return
	r.EIP, r.CS, r.EFLAGS = at(0), at(1), at(2)
	if c.fault != nil {
		return nil // 界限或 B=1 的失敗由 Step 回報
	}
	if r.EFLAGS&(eflagsVM) == 0 {
		return c.errf(op, "IRETD：分段模式彈出的 EFLAGS %08X 的 VM=0（同特權或外層返回屬後續），失敗即關閉（docs/spec/435 §2.5）", r.EFLAGS)
	}
	r.ESP, r.SS, r.ES, r.DS, r.FS, r.GS = at(3), at(4), at(5), at(6), at(7), at(8)
	if c.fault != nil {
		return nil
	}
	if r.EIP>>16 != 0 {
		return c.errf(op, "IRETD：回 V86 彈出的 EIP %08X 高 16 位元不為 0，失敗即關閉（刻意從嚴，docs/spec/435 §2.5）", r.EIP)
	}
	if r.EFLAGS>>18 != 0 {
		return c.errf(op, "IRETD：回 V86 彈出的 EFLAGS %08X 的 bit 18–31 不為 0，失敗即關閉（刻意從嚴，docs/spec/435 §2.5）", r.EFLAGS)
	}
	c.v86Ret = r
	c.IP = c.opIP
	return ErrV86Return
}

// eflagsVM 是 EFLAGS 的 VM 位元（bit 17）。
const eflagsVM = 1 << 17

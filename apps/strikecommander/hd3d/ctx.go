// 1 倍整數重畫核心，契約為規格 012 READY 第 7 版 §3.3。
// 移植來源是 tools/m6replay 的 P4 證據碼，原位址註記保留。
// 產品移植自 tools/m6replay；依 READY 012 §3.3 與原反組譯（workplace/m6/dis-*.txt），
// 每段註明執行期位址範圍。
package hd3d

import (
	"fmt"
	"sort"
)

const (
	bufLin  = 0x100000 // 畫外緩衝線性起點（FFFF:0010，RE 024 §2、RE 025 §3.1）
	bufSize = 64000
	iterCap = 64000 // §3.3.1：每次呼叫迴圈上限
)

// Ctx 是一個畫格群組的 1 倍畫布與統計。
type context struct {
	buf           [bufSize]byte
	written       [bufSize]bool
	writer        [bufSize]uint16
	seq           uint16 // 目前呼叫的序號（calls.tsv 的 seq）
	seg           uint16 // 目前呼叫的描述元段
	call          *Call
	list          []uint16
	objectWritten [bufSize]bool

	oob    int      // 越界寫入
	faults []string // idiv 溢位、迴圈上限、讀出界等（標群組不可重畫）
	notes  []string // 不影響可重畫的附註
	uninit []string // 讀到本次呼叫沒寫過的堆疊（原版讀殘值，P2 未錄）：群組標不可重畫，但繼續比對
	// R5 平三角形（0DF9）的檢查：第一次 0C30 之前讀到的未初始化位置、0C30 之後的寫入數
	r5flat, r5stepped                           bool
	r5preUninit                                 map[string]int
	r5flatCalls, r5flatWrites, r5flatPostWrites int
	br                                          *Branches
	mut                                         map[string]bool
	r6seg                                       []byte // 56F1:0000–0452（由 STRIKE.EXE 讀出）
	capShort                                    int    // 擷取不足
	tabReal, tabAssumed                         int    // 列表讀實值／照假設的次數（P2b）
	r5residSeeded                               int    // 以錄到的殘值填入的 R5 呼叫數（P2b）
	r10Patch, r10PreBad                         int    // 黑箱補丁點數、Before 與畫布不符點數（P2b）
	// RE 025b：本次呼叫的畫布寫入數，與第一次未初始化讀取時的寫入數。
	// 呼叫結束時兩者相等（讀到殘值之後沒有再寫任何點）的未初始化讀取另列 uninitAfterLast。
	callWrites, callWritesAtUninit int
	callUninitStart                int // c.uninit 中本次呼叫的起點
	callHasUninit                  bool
	uninitAfterLast                []string
}

// beginCall／endCall 包住每次呼叫（RE 025b 的未初始化讀取分類）。
func (c *context) beginCall() {
	c.callWrites, c.callHasUninit, c.callUninitStart = 0, false, len(c.uninit)
}

func (c *context) noteUninit() {
	if !c.callHasUninit {
		c.callHasUninit, c.callWritesAtUninit = true, c.callWrites
	}
}

// 「讀殘值後不再寫」固定依 READY §3.3.1 移出不可重畫條件。
func (c *context) endCall() {
	if c.callHasUninit && c.callWrites == c.callWritesAtUninit {
		c.uninitAfterLast = append(c.uninitAfterLast, c.uninit[c.callUninitStart:]...)
		c.uninit = c.uninit[:c.callUninitStart]
	}
}

func (c *context) fault(f string, a ...any) {
	s := fmt.Sprintf("seq %d %s：", c.seq, c.call.Kind) + fmt.Sprintf(f, a...)
	c.faults = append(c.faults, s)
}

func (c *context) note(f string, a ...any) {
	s := fmt.Sprintf("seq %d %s：", c.seq, c.call.Kind) + fmt.Sprintf(f, a...)
	c.notes = append(c.notes, s)
}

// idx 把 es:di（es = 描述元段）換成畫布索引。
func (c *context) idx(di uint16) int {
	return int(di) - int(c.list[0])
}

// w 寫一個 byte 到 es:di。
func (c *context) w(di uint16, v byte) {
	i := c.idx(di)
	if i < 0 || i >= bufSize {
		c.oob++
		return
	}
	c.buf[i] = v
	c.written[i] = true
	c.writer[i] = c.seq
	if c.call != nil && c.call.Object {
		c.objectWritten[i] = true
	}
	c.callWrites++
}

// ww 寫一個 word 到 es:di（低位元組在 di）。位移 FFFFh 的 word 在真實／V86 模式會出例外，記錄。
func (c *context) ww(di uint16, v uint16) {
	if di == 0xFFFF {
		c.fault("word 寫入跨段尾 di=FFFF")
	}
	c.w(di, byte(v))
	c.w(di+1, byte(v>>8))
}

// r 讀 es:[di]（R2 讀畫布現值）。
func (c *context) r(di uint16) byte {
	i := c.idx(di)
	if i < 0 || i >= bufSize {
		c.fault("讀畫外緩衝越界 di=%04X", di)
		return 0
	}
	if !c.written[i] {
		c.note("讀到未寫的點 (%d,%d)", i%320, i/320)
	}
	return c.buf[i]
}

// tab reads the actual captured list. There is no assumed 320-byte pitch.
func (c *context) tab(y uint16) uint16 {
	if y > 200 || int(y) >= len(c.list) {
		c.capShort++
		c.fault("列表讀出界 y=%d", y)
		return 0
	}
	c.tabReal++
	return c.list[y]
}

// idiv64 模擬 32 位元 idiv（edx:eax ÷ r/m32）：商向零截斷，商超出 int32 或除數 0 時是 #DE。
func (c *context) idiv64(dividend int64, divisor int32, where string) (q, r int32) {
	if divisor == 0 {
		c.fault("%s idiv 除數 0", where)
		return 0, 0
	}
	qq := dividend / int64(divisor)
	if qq > 0x7FFFFFFF || qq < -0x80000000 {
		c.fault("%s idiv 商溢位 %d/%d", where, dividend, divisor)
		return 0, 0
	}
	return int32(qq), int32(dividend % int64(divisor))
}

// idiv32 模擬 16 位元 idiv（dx:ax ÷ r/m16）。
func (c *context) idiv32(dividend int32, divisor int16, where string) (q, r int16) {
	if divisor == 0 {
		c.fault("%s idiv16 除數 0", where)
		return 0, 0
	}
	qq := dividend / int32(divisor)
	if qq > 0x7FFF || qq < -0x8000 {
		c.fault("%s idiv16 商溢位 %d/%d", where, dividend, divisor)
		return 0, 0
	}
	return int16(qq), int16(dividend % int32(divisor))
}

// 32 位元乘法取 64 位元乘積（imul r/m32）。
func imul64(a, b uint32) int64 { return int64(int32(a)) * int64(int32(b)) }

// shrd eax,edx,16 的結果：64 位元乘積的位元 16..47。
func shrd16(p int64) uint32 { return uint32(uint64(p) >> 16) }

func sx16(v uint16) uint32 { return uint32(int32(int16(v))) }

// ---- 分支命中表（§3.4.3 branches.tsv） ----

type BranchDef struct {
	Routine, ID, Addr, Desc string
}

type Branches struct {
	n map[string]int
}

func NewBranches() *Branches { return &Branches{n: map[string]int{}} }

func (b *Branches) hit(id string) {
	if _, ok := branchIndex[id]; !ok {
		panic("未登記的分支 " + id)
	}
	b.n[id]++
}

func (b *Branches) add(o *Branches) {
	for k, v := range o.n {
		b.n[k] += v
	}
}

var branchIndex = map[string]BranchDef{}

func regBranches(defs ...BranchDef) {
	for _, d := range defs {
		branchIndex[d.ID] = d
	}
}

func sortedBranchIDs() []string {
	var ids []string
	for k := range branchIndex {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	return ids
}

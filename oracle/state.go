package oracle

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// State 是一份機器狀態的快照（`docs/spec/005` §5）。
//
// 用途是**從同一個狀態展開多個變體**：開機到防拷畫面要四千兩百萬道指令，
// 走到棋盤還要更多。rich2 的 D33／D34「走到罕見畫面很貴，最後改存檔資料」
// 就是這個問題。
//
// ⚠ **不落地成檔案。** 這裡面是原版的整份記憶體映像，寫到磁碟等於散布它。
type State struct {
	// machine 那一半（記憶體、CPU、**所有內部時鐘**）由 machine 自己拍。
	// 時鐘漏一個就會讓還原之後的計時器中斷不再送，見 machine.Snapshot 的說明。
	m *machine.Snapshot

	mouse dos.Mouse
	stdin []byte
	// 開過的檔要一起存，否則 Restore 之後 Opened() 的路標會回到過去。
	opened []string
}

// mem 讓同一個套件裡的搜尋拿得到快照的記憶體。
func (s *State) mem() []uint8 { return s.m.Mem() }

// Save 深拷貝目前的狀態。1 MB 記憶體，大約 1 毫秒。
func (o *Oracle) Save() *State {
	s := &State{
		m:      o.m.Snapshot(),
		mouse:  o.d.Mouse,
		stdin:  append([]byte(nil), o.d.Stdin...),
		opened: append([]string(nil), o.d.Opened...),
	}
	// Polls 是切片，共用底層陣列會讓快照跟著之後的執行變動。
	s.mouse.Polls = append([]dos.Poll(nil), o.d.Mouse.Polls...)
	s.mouse.Sets = append([]dos.Poll(nil), o.d.Mouse.Sets...)
	return s
}

// Restore 把機器倒回某個快照。
//
// ⚠ **開著的檔不會倒回去。** 檔案位置是作業系統的狀態，不在這份快照裡；
// 快照之後才開的檔會留著開著。實務上沒問題（程式開檔就讀完），
// 但要知道這個邊界。
func (o *Oracle) Restore(s *State) {
	o.m.Restore(s.m)
	o.d.Mouse = s.mouse
	o.d.Mouse.Polls = append([]dos.Poll(nil), s.mouse.Polls...)
	o.d.Mouse.Sets = append([]dos.Poll(nil), s.mouse.Sets...)
	o.d.Stdin = append([]byte(nil), s.stdin...)
	o.d.Opened = append([]string(nil), s.opened...)
	o.d.Exited = false
}

// Regs 是進到某支常式時的暫存器快照。
//
// **搭配 `OnCall` 用**：把判準從像素換成參數。「AI 選了哪個指令」
// 不必從畫面猜——攔住分派器，它自己會說。
type Regs struct {
	AX, BX, CX, DX, SI, DI, BP, SP uint16
	DS, ES, SS, CS, IP             uint16
	Flags                          uint16
}

// Regs 讀目前的暫存器。
func (o *Oracle) Regs() Regs {
	c := o.m.CPU
	return Regs{
		AX: c.R[cpu.AX], BX: c.R[cpu.BX], CX: c.R[cpu.CX], DX: c.R[cpu.DX],
		SI: c.R[cpu.SI], DI: c.R[cpu.DI], BP: c.R[cpu.BP], SP: c.R[cpu.SP],
		DS: c.Seg[cpu.DS], ES: c.Seg[cpu.ES], SS: c.Seg[cpu.SS],
		CS: c.Seg[cpu.CS], IP: c.IP, Flags: c.Flags,
	}
}

// CallRegs 是呼叫前要擺好的暫存器。零值表示不動。
type CallRegs struct {
	AX, BX, CX, DX, SI, DI, BP uint16
	DS, ES                     uint16
	// Set 標出上面哪幾個要真的寫進去——**0 是合法值**，
	// 不能拿「等於零」當「沒指定」。
	SetAX, SetBX, SetCX, SetDX, SetSI, SetDI, SetBP, SetDS, SetES bool
}

// CallNear 把 CPU 直接擺到原版某一支 **near** 常式的入口。
//
// ⭐ **原版當函式庫用。** 有些畫面只有在遊戲自己決定要開的時候才會出現
// （戰術畫面要等遭遇），而那可能是四億道指令之後。擺好參數直接叫下去，
// 同一個畫面五分鐘就到得了。
//
// ⚠ **返回位址是假的。** 只推一個 near 返回位址 `ret`，常式回來時就跳到
// 那裡，而那一跳會比原本多吃掉呼叫端堆疊上的一個字。所以這一支適合
// 「叫下去之後就在裡面」的用途（開畫面、進迴圈），**不適合當函式呼叫**。
//
// ⚠ **near 不能跨段**：`addr.Seg` 必須就是常式所在的段，函式回來時
// CS 不會變。要叫 far 的用 `FarCall`。
func (o *Oracle) CallNear(addr Addr, ret uint16, r CallRegs) {
	c := o.m.CPU
	sp := c.R[cpu.SP] - 2
	c.R[cpu.SP] = sp
	o.m.Write16(cpu.Addr(c.Seg[cpu.SS], sp), ret)
	set := func(dst *uint16, v uint16, ok bool) {
		if ok {
			*dst = v
		}
	}
	set(&c.R[cpu.AX], r.AX, r.SetAX)
	set(&c.R[cpu.BX], r.BX, r.SetBX)
	set(&c.R[cpu.CX], r.CX, r.SetCX)
	set(&c.R[cpu.DX], r.DX, r.SetDX)
	set(&c.R[cpu.SI], r.SI, r.SetSI)
	set(&c.R[cpu.DI], r.DI, r.SetDI)
	set(&c.R[cpu.BP], r.BP, r.SetBP)
	set(&c.Seg[cpu.DS], r.DS, r.SetDS)
	set(&c.Seg[cpu.ES], r.ES, r.SetES)
	c.Seg[cpu.CS], c.IP = addr.Seg, addr.Off
}

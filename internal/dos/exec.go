package dos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EXEC（`AH=4Bh AL=00h`）、TSR（`AH=31h`）、回傳碼（`AH=4Dh`）與
// 監督佇列。規格：`docs/spec/008`（TSR 與行程模型）、`docs/spec/009`
// （EXEC 與程式鏈），兩份都是 READY，量測證據附在規格裡。

// procFrame 是一個被 EXEC 暫停的父行程（`008` §2）。
type procFrame struct {
	r        [8]uint16
	seg      [4]uint16
	ip, fl   uint16
	psp      uint16
	freeSeg  uint16 // 進子行程之前的 freeSeg，回來時不必用（LIFO 回收）
}

// Queued 是監督佇列裡的一支待跑程式（`009` §4）。
type Queued struct {
	Name string
	Args string
}

// ExecRecord 是一次 EXEC／監督載入的紀錄。
//
// **「殼鏈走到哪一跳」唯一的直接答案。** 沒有它的話，MAIN.EXE 沒跑起來
// 與「殼根本沒 EXEC 它」看起來一樣。
type ExecRecord struct {
	Name  string // 呼叫端給的檔名
	Base  string // 實際開到的檔（basename）
	PSP   uint16
	TSR   bool   // 用 AH=31h 常駐
	Keep  uint16 // TSR 保留的段數（DX）
	Exit  uint8  // 離開碼；0xFF ＝ 還沒結束
}

// Enqueue 排入一支待跑程式。行程疊空了（第一支程式結束或常駐）之後
// 由服務層推出來跑（`009` §4）。
func (d *DOS) Enqueue(name, args string) {
	d.queue = append(d.queue, Queued{Name: name, Args: args})
}

// exec 是 `AH=4Bh`。**只實作 AL=00h**（`009` §1）。
func (d *DOS) exec(c *cpu.CPU) {
	if al(c) != 0x00 {
		d.note(0x21, 0x4B, al(c))
		c.R[cpu.AX] = 1 // Invalid function
		setCarry(c)
		return
	}
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	pb := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.BX])
	envSeg := d.M.Read16(pb)
	tailOff := d.M.Read16(pb + 2)
	tailSeg := d.M.Read16(pb + 4)
	d.spawn(c, name, envSeg, tailSeg, tailOff)
}

// spawn 載入並跳到一支子程式。失敗時設 CF 與 AX，行程疊不變。
func (d *DOS) spawn(c *cpu.CPU, name string, envSeg, tailSeg, tailOff uint16) {
	path := d.resolve(name)
	if path == "" {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2 // File not found
		setCarry(c)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}

	// 子行程 PSP ＝ freeSeg+1（freeSeg 那格是假 MCB，與 AH=48h 一致）。
	need := uint16((len(data)+15)/16) + 0x10 + 1 // 上限估計；MZ 去掉檔頭會更小
	psp := d.freeSeg + 1
	if psp+need > machine.MemTop {
		c.R[cpu.AX] = 8 // 記憶體不足
		setCarry(c)
		return
	}

	// 壓父行程框。此時 IP 已指到 int 21h 的下一道，存起來的就是
	// 正確的接續點。
	f := procFrame{r: c.R, seg: c.Seg, ip: c.IP, fl: c.Flags, psp: d.curPSP, freeSeg: d.freeSeg}

	prog, err := d.M.LoadProgramAt(psp, data)
	if err != nil {
		d.Missing = append(d.Missing, fmt.Sprintf("%s（%v）", name, err))
		c.R[cpu.AX] = 8
		setCarry(c)
		return
	}
	d.stack = append(d.stack, f)
	d.M.WriteMCB(d.freeSeg, psp, prog.EndSeg-d.freeSeg)

	// PSP 欄位（`009` §2.4）。
	base := uint32(psp) * 16
	d.M.Write16(base+0x16, f.psp) // 父行程 PSP
	if envSeg == 0 {              // 繼承父行程的環境段
		envSeg = d.M.Read16(uint32(f.psp)*16 + 0x2C)
	}
	d.M.Write16(base+0x2C, envSeg)
	d.copyCmdTail(base, tailSeg, tailOff)

	// 切 CPU。DS=ES=子 PSP，其餘通用暫存器歸零（真 DOS 不保證它們的值，
	// 歸零是決定性選擇；AX=0 表示驅動器代號有效）。
	c.Seg[cpu.CS], c.IP = prog.CS, prog.IP
	c.Seg[cpu.SS], c.R[cpu.SP] = prog.SS, prog.SP
	c.Seg[cpu.DS], c.Seg[cpu.ES] = psp, psp
	for i := range c.R {
		c.R[i] = 0
	}
	c.R[cpu.SP] = prog.SP

	d.ExecLog = append(d.ExecLog, ExecRecord{
		Name: name, Base: filepath.Base(path), PSP: psp, Exit: 0xFF,
	})
	d.curPSP = psp
	d.freeSeg = prog.EndSeg
	clearCarry(c)
}

// copyCmdTail 把 EXEC 參數區塊指的命令列尾拷進子 PSP+80h。
func (d *DOS) copyCmdTail(psp uint32, tailSeg, tailOff uint16) {
	if tailSeg == 0 && tailOff == 0 {
		d.M.WriteBytes(psp+0x80, []byte{0, 0x0D})
		return
	}
	src := cpu.Addr(tailSeg, tailOff)
	n := d.M.Read8(src)
	if n > 126 {
		n = 126
	}
	d.M.Write8(psp+0x80, n)
	for i := uint8(0); i < n; i++ {
		d.M.Write8(psp+0x81+uint32(i), d.M.Read8(src+1+uint32(i)))
	}
	d.M.Write8(psp+0x81+uint32(n), 0x0D)
}

// terminate 結束目前行程（`AH=4Ch`／`AH=31h`／`int 20h` 共用）。
//
// tsr 為真表示走 `AH=31h` 的常駐語意，keep 是它的 DX
// （從 PSP 起保留的段數）；非 TSR 兩者都給 false／0。
func (d *DOS) terminate(c *cpu.CPU, code uint8, tsr bool, keep uint16) {
	d.lastExit = uint16(code)
	// 記到**目前行程**那一筆——不是最後一筆。殼結束時最後一筆是它的
	// 子行程（早就 TSR 了），寫過去會把殼的離開碼記到 FMDRV 頭上。
	for i := len(d.ExecLog) - 1; i >= 0; i-- {
		if d.ExecLog[i].PSP == d.curPSP && d.ExecLog[i].Exit == 0xFF {
			d.ExecLog[i].Exit = code
			d.ExecLog[i].TSR = tsr
			d.ExecLog[i].Keep = keep
			break
		}
	}

	if len(d.stack) > 0 {
		// 彈回父行程（`009` §2 的「回傳」）。
		f := d.stack[len(d.stack)-1]
		d.stack = d.stack[:len(d.stack)-1]
		c.R, c.Seg, c.IP, c.Flags = f.r, f.seg, f.ip, f.fl
		if tsr {
			// TSR：常駐區保留。bump 配置器沒有洞的觀念，所以
			// **只能往前推不能往回收**（`008` §2.1）——
			// TSR 之前 AH=48h 拿走並留在常駐程式名下的區塊跟著保留。
			if k := d.curPSP + keep; k > d.freeSeg {
				d.freeSeg = k
			}
		} else {
			// 非 TSR：整個子行程（含它 AH=48h 拿走的）LIFO 回收。
			d.freeSeg = d.curPSP - 1
		}
		d.curPSP = f.psp
		clearCarry(c)
		return
	}

	// 疊底行程結束。非 TSR 把記憶體回收到它的 PSP 前一格，
	// 讓監督佇列的下一支從這裡開始。
	if tsr {
		if k := d.curPSP + keep; k > d.freeSeg {
			d.freeSeg = k
		}
	} else if d.curPSP > 0 {
		d.freeSeg = d.curPSP - 1
	}

	// 監督佇列（`009` §4）：有排就跑下一支，沒有才算程式結束。
	if len(d.queue) > 0 {
		q := d.queue[0]
		d.queue = d.queue[1:]
		// 疊底沒有父行程可壓；用一個假的參數區塊語意直接 spawn。
		// spawn 失敗（檔案找不到）時靜靜跳過會讓鏈斷得不明不白，
		// 所以失敗就直接當整台結束，Missing 裡有名字。
		before := len(d.Missing)
		d.spawnQueued(c, q)
		if len(d.Missing) > before {
			d.Exited, d.ExitCode = true, 2
			c.Halted = true
		}
		return
	}
	d.Exited, d.ExitCode = true, code
	c.Halted = true
}

// spawnQueued 從監督佇列推出一支程式當新的疊底。
// 沒有父行程框可壓（結束時疊是空的）；命令列尾由 Args 直接給。
func (d *DOS) spawnQueued(c *cpu.CPU, q Queued) {
	path := d.resolve(q.Name)
	if path == "" {
		d.Missing = append(d.Missing, q.Name)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		d.Missing = append(d.Missing, q.Name)
		return
	}
	need := uint16((len(data)+15)/16) + 0x10 + 1
	psp := d.freeSeg + 1
	if psp+need > machine.MemTop {
		d.Missing = append(d.Missing, q.Name+"（記憶體不足）")
		return
	}
	prog, err := d.M.LoadProgramAt(psp, data)
	if err != nil {
		d.Missing = append(d.Missing, fmt.Sprintf("%s（%v）", q.Name, err))
		return
	}
	d.M.WriteMCB(d.freeSeg, psp, prog.EndSeg-d.freeSeg)
	base := uint32(psp) * 16
	d.M.Write16(base+0x16, psp) // 疊底的父行程是自己

	// 命令列尾：長度 ＋ 內容 ＋ CR（`009` §4）。
	args := []byte(q.Args)
	if len(args) > 126 {
		args = args[:126]
	}
	d.M.Write8(base+0x80, uint8(len(args)))
	d.M.WriteBytes(base+0x81, args)
	d.M.Write8(base+0x81+uint32(len(args)), 0x0D)

	c.Seg[cpu.CS], c.IP = prog.CS, prog.IP
	c.Seg[cpu.SS], c.R[cpu.SP] = prog.SS, prog.SP
	c.Seg[cpu.DS], c.Seg[cpu.ES] = psp, psp
	for i := range c.R {
		c.R[i] = 0
	}
	c.R[cpu.SP] = prog.SP
	c.Halted = false

	d.ExecLog = append(d.ExecLog, ExecRecord{
		Name: q.Name, Base: filepath.Base(path), PSP: psp, Exit: 0xFF,
	})
	d.curPSP = psp
	d.freeSeg = prog.EndSeg
}

// tsr 是 `AH=31h`：常駐並結束（`008` §3）。
func (d *DOS) tsr(c *cpu.CPU) {
	d.terminate(c, al(c), true, c.R[cpu.DX])
}

// getExitCode 是 `AH=4Dh`（`009` §3）：AL ＝ 離開碼，AH ＝ 0（正常結束）。
// **可重複讀，不清掉**——清了會讓第二次讀到 0，一個看起來合理但假的值。
func (d *DOS) getExitCode(c *cpu.CPU) {
	c.R[cpu.AX] = d.lastExit & 0xFF
	clearCarry(c)
}

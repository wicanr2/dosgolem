package dos

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EXEC（`AH=4Bh`）、TSR（`AH=31h`）、結束（`AH=4Ch`／`int 20h`）、
// 回傳碼（`AH=4Dh`）與監督佇列。
//
// 規格：`docs/spec/007`（EXEC 與 EMMXXXX0，證據是 GIN3.COM 的反組譯
// bytes 與 probe 軌跡）、`docs/spec/008`（TSR 與行程模型）、
// `docs/spec/009`（EXEC 與程式鏈）、`docs/spec/010`（結束時的記憶體回收）。
//
// **記憶體內容不做快照還原**——隔離靠 bump 配置器（子程式載在父程式之上），
// 子程式對視訊記憶體／IVT 的寫入保留，這符合真 DOS（畫面不會因程式結束
// 而消失）。代價是子程式踩父程式空間不會被擋。**但配置游標會在子程式
// 結束時退回**——那是所有權回收，不是內容還原，兩件事不一樣。

// procFrame 是一個被 EXEC 暫停的父行程（`docs/spec/008` §2）。
type procFrame struct {
	r      [8]uint16
	seg    [4]uint16
	ip, fl uint16
	psp    uint16 // 父行程的 PSP（回來時還原 curPSP）
	// handleBase 是進子行程之前的 nextHandle：子行程結束時把 >= 它的
	// handle 全部關掉。真 DOS 的 AH=4Ch 會關掉該 PSP 名下的所有檔案，
	// 漏關的症狀是「跑久了開不了檔」——而且錯誤碼是 04h（handle 用盡），
	// 完全不指向這裡。
	handleBase uint16
	freeSeg    uint16 // 進子行程之前的配置游標
	// ivt 是 DOS 替呼叫端保管的三個向量（22h Terminate、23h Ctrl-Break、
	// 24h Critical Error）。子行程一定會改，不還原的話父行程的
	// Ctrl-Break 處理常式指到已經被回收的記憶體。
	ivt [3][2]uint16
}

// savedVectors 是 EXEC 前後要保管的向量號。
var savedVectors = [3]uint8{0x22, 0x23, 0x24}

// Queued 是監督佇列裡的一支待跑程式（`docs/spec/009` §4）。
type Queued struct {
	Name string
	Args string
}

// ExecRecord 是一次 EXEC／監督載入的紀錄。
//
// **「殼鏈走到哪一跳」唯一的直接答案。** 沒有它的話，MAIN.EXE 沒跑起來
// 與「殼根本沒 EXEC 它」看起來一樣。
type ExecRecord struct {
	Name string // 呼叫端給的檔名
	Base string // 實際開到的檔（basename）
	PSP  uint16
	TSR  bool   // 用 AH=31h 常駐
	Keep uint16 // TSR 保留的段數（DX）
	Exit uint8  // 離開碼；0xFF ＝ 還沒結束
}

// Enqueue 排入一支待跑程式。行程疊空了（第一支程式結束或常駐）之後
// 由服務層推出來跑（`docs/spec/009` §4）。
func (d *DOS) Enqueue(name, args string) {
	d.queue = append(d.queue, Queued{Name: name, Args: args})
}

// exec 是 `AH=4Bh`。AL=00h 是載入並執行，AL=03h 是載入 overlay。
func (d *DOS) exec(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	pb := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.BX])
	switch al(c) {
	case 0x00:
		// 參數區 ES:BX：+0 env（0＝繼承）、+2/+4 尾巴、+6/+8 FCB1、+A/+C FCB2
		// （bytes 證據：GIN3.COM 0215–023C）。
		d.spawn(c, name, execParams{
			envSeg:  d.M.Read16(pb),
			tailOff: d.M.Read16(pb + 2),
			tailSeg: d.M.Read16(pb + 4),
			fcb1Off: d.M.Read16(pb + 6),
			fcb1Seg: d.M.Read16(pb + 8),
			fcb2Off: d.M.Read16(pb + 10),
			fcb2Seg: d.M.Read16(pb + 12),
		})
	case 0x03:
		// overlay：參數區是 +0 載入段、+2 relocation factor。
		d.loadOverlay(c, name, d.M.Read16(pb), d.M.Read16(pb+2))
	default:
		d.note(0x21, 0x4B, al(c))
		c.R[cpu.AX] = 1 // Invalid function
		setCarry(c)
	}
}

// execParams 是 EXEC 參數區塊拆出來的欄位。
type execParams struct {
	envSeg           uint16
	tailSeg, tailOff uint16
	fcb1Seg, fcb1Off uint16
	fcb2Seg, fcb2Off uint16
}

// imageParags 估算一支程式載進來要幾個段（含 PSP 與前面那格假 MCB）。
//
// MZ 走檔頭（準確），其他當 .COM 用檔案長度。**在動任何狀態之前算**——
// 載到一半才發現記憶體不夠會留下半套 PSP。
func imageParags(data []byte) uint16 {
	if n, err := machine.MZImageParags(data); err == nil {
		return uint16(n) + 0x11
	}
	return uint16((len(data)+15)/16) + 0x11
}

// spawn 載入並跳到一支子程式。失敗時設 CF 與 AX，行程疊不變。
func (d *DOS) spawn(c *cpu.CPU, name string, p execParams) {
	data, path, ok := d.readProgram(c, name)
	if !ok {
		return
	}

	// 子行程 PSP ＝ freeSeg+1（freeSeg 那格是假 MCB，與 AH=48h 一致）。
	psp := d.freeSeg + 1
	need := imageParags(data)
	if avail := uint16(machine.MemTop) - d.freeSeg; need > avail {
		c.R[cpu.AX] = 8 // 記憶體不足
		c.R[cpu.BX] = avail
		setCarry(c)
		return
	}

	// 壓父行程框。此時 IP 已指到 int 21h 的下一道，存起來的就是
	// 正確的接續點。
	f := procFrame{
		r: c.R, seg: c.Seg, ip: c.IP, fl: c.Flags,
		psp: d.curPSP, handleBase: d.nextHandle, freeSeg: d.freeSeg,
	}
	for i, n := range savedVectors {
		f.ivt[i][0] = d.M.Read16(uint32(n) * 4)
		f.ivt[i][1] = d.M.Read16(uint32(n)*4 + 2)
	}

	prog, err := d.M.LoadProgramAt(psp, data)
	if err != nil {
		d.Missing = append(d.Missing, fmt.Sprintf("%s（%v）", name, err))
		c.R[cpu.AX] = 8
		setCarry(c)
		return
	}
	d.procStack = append(d.procStack, f)
	d.M.WriteMCB(d.freeSeg, false, psp, prog.EndSeg-d.freeSeg)

	// PSP 欄位（`docs/spec/009` §2.4）。
	base := uint32(psp) * 16
	d.M.Write16(base+0x16, f.psp) // 父行程 PSP
	envSeg := p.envSeg
	if envSeg == 0 { // 繼承父行程的環境段
		envSeg = d.M.Read16(uint32(f.psp)*16 + 0x2C)
	}
	d.M.Write16(base+0x2C, envSeg)
	d.copyCmdTail(base, p.tailSeg, p.tailOff)
	// FCB 原樣各抄 16 bytes。
	for i := uint32(0); i < 16; i++ {
		d.M.Write8(base+0x5C+i, d.M.Read8(cpu.Addr(p.fcb1Seg, p.fcb1Off)+i))
		d.M.Write8(base+0x6C+i, d.M.Read8(cpu.Addr(p.fcb2Seg, p.fcb2Off)+i))
	}

	d.enterProgram(c, prog)
	d.ExecLog = append(d.ExecLog, ExecRecord{
		Name: name, Base: filepath.Base(path), PSP: psp, Exit: 0xFF,
	})
	d.Opened = append(d.Opened, name)
	clearCarry(c)
}

// loadOverlay 是 `AH=4Bh AL=03h`：把映像放到指定的段，什麼都不切。
//
// overlay 是被 far call 進去的程式碼，**不建 PSP、不動 CS:IP**。
// relocation factor 與載入段是兩個獨立參數，DOS 讓呼叫端分別指定。
func (d *DOS) loadOverlay(c *cpu.CPU, name string, loadSeg, relocFactor uint16) {
	data, path, ok := d.readProgram(c, name)
	if !ok {
		return
	}
	if err := d.M.LoadOverlay(data, loadSeg, relocFactor); err != nil {
		d.Missing = append(d.Missing, fmt.Sprintf("%s（%v）", name, err))
		c.R[cpu.AX] = 8
		setCarry(c)
		return
	}
	d.Opened = append(d.Opened, name)
	d.ExecLog = append(d.ExecLog, ExecRecord{
		Name: name, Base: filepath.Base(path), PSP: loadSeg, Exit: 0xFF,
	})
	clearCarry(c)
}

// readProgram 解析檔名並讀進來。失敗時已經設好 CF 與 AX。
func (d *DOS) readProgram(c *cpu.CPU, name string) ([]byte, string, bool) {
	path := d.resolve(name)
	if path == "" {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2 // File not found
		setCarry(c)
		return nil, "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		d.Missing = append(d.Missing, name)
		c.R[cpu.AX] = 2
		setCarry(c)
		return nil, "", false
	}
	return data, path, true
}

// enterProgram 把 CPU 切到剛載好的程式。
//
// DS=ES=子 PSP，其餘通用暫存器歸零（真 DOS 不保證它們的值，歸零是
// 決定性選擇；AX=0 表示驅動器代號有效）。
func (d *DOS) enterProgram(c *cpu.CPU, prog *machine.Program) {
	for i := range c.R {
		c.R[i] = 0
	}
	c.Seg[cpu.CS], c.IP = prog.CS, prog.IP
	c.Seg[cpu.SS], c.R[cpu.SP] = prog.SS, prog.SP
	c.Seg[cpu.DS], c.Seg[cpu.ES] = prog.PSPSeg, prog.PSPSeg
	d.curPSP = prog.PSPSeg
	d.freeSeg = prog.EndSeg
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

	if len(d.procStack) > 0 {
		// 彈回父行程（`docs/spec/009` §2 的「回傳」）。
		f := d.procStack[len(d.procStack)-1]
		d.procStack = d.procStack[:len(d.procStack)-1]

		// 子行程開的檔要關掉，DOS 保管的三個向量要還原。
		if !tsr {
			d.closeHandlesFrom(f.handleBase)
		}
		for i, n := range savedVectors {
			d.M.Write16(uint32(n)*4, f.ivt[i][0])
			d.M.Write16(uint32(n)*4+2, f.ivt[i][1])
		}

		c.R, c.Seg, c.IP, c.Flags = f.r, f.seg, f.ip, f.fl
		if tsr {
			// TSR：常駐區保留。bump 配置器沒有洞的觀念，所以
			// **只能往前推不能往回收**（`docs/spec/008` §2.1）——
			// TSR 之前 AH=48h 拿走並留在常駐程式名下的區塊跟著保留。
			if k := d.curPSP + keep; k > d.freeSeg {
				d.freeSeg = k
			}
		} else {
			// 非 TSR：整個子行程（含它 AH=48h 拿走的）LIFO 回收。
			// 真 DOS 的 AH=4Ch 會釋放該 PSP 擁有的所有 MCB；bump 配置器
			// 只升不降的話，下一支 EXEC 進來的程式會被載到不該有的高段
			// ——而且它自己完全察覺不到，只有在向 DOS 要不到記憶體時
			// 才顯現（`docs/spec/010` §1）。
			d.freeSeg = f.freeSeg
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

	// 監督佇列（`docs/spec/009` §4）：有排就跑下一支，沒有才算程式結束。
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

// closeHandlesFrom 關掉編號 >= base 的所有 handle。
func (d *DOS) closeHandlesFrom(base uint16) {
	for h, hh := range d.handles {
		if h >= base {
			if hh.f != nil {
				hh.f.Close()
			}
			delete(d.handles, h)
		}
	}
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
	psp := d.freeSeg + 1
	if need := imageParags(data); psp+need > machine.MemTop {
		d.Missing = append(d.Missing, q.Name+"（記憶體不足）")
		return
	}
	prog, err := d.M.LoadProgramAt(psp, data)
	if err != nil {
		d.Missing = append(d.Missing, fmt.Sprintf("%s（%v）", q.Name, err))
		return
	}
	d.M.WriteMCB(d.freeSeg, false, psp, prog.EndSeg-d.freeSeg)
	base := uint32(psp) * 16
	d.M.Write16(base+0x16, psp) // 疊底的父行程是自己

	// 命令列尾：長度 ＋ 內容 ＋ CR（`docs/spec/009` §4）。
	args := []byte(q.Args)
	if len(args) > 126 {
		args = args[:126]
	}
	d.M.Write8(base+0x80, uint8(len(args)))
	d.M.WriteBytes(base+0x81, args)
	d.M.Write8(base+0x81+uint32(len(args)), 0x0D)

	d.enterProgram(c, prog)
	c.Halted = false

	d.ExecLog = append(d.ExecLog, ExecRecord{
		Name: q.Name, Base: filepath.Base(path), PSP: psp, Exit: 0xFF,
	})
}

// tsr 是 `AH=31h`：常駐並結束（`docs/spec/008` §3）。
func (d *DOS) tsr(c *cpu.CPU) {
	d.terminate(c, al(c), true, c.R[cpu.DX])
}

// getExitCode 是 `AH=4Dh`（`docs/spec/009` §3）：AL ＝ 離開碼、
// AH ＝ 結束方式（0 ＝ 正常）。
//
// ⚠ **AH 一定要清 0**——GIN3.COM 拿整個 AX 比較（`or ax,ax`／`cmp ax,1`），
// AH 留垃圾會讓「回碼 0」讀成非 0。
//
// **可重複讀，不清掉**——清了會讓第二次讀到 0，一個看起來合理但假的值。
func (d *DOS) getExitCode(c *cpu.CPU) {
	c.R[cpu.AX] = d.lastExit & 0xFF
	clearCarry(c)
}

// isEMMDevice 回 basename 是不是 EMM 驅動的字元裝置名。
// 開啟它成功 ＝ EMS 驅動存在（GIN3.COM 01D9–01F0 就是這樣偵測的）。
func isEMMDevice(name string) bool {
	base := name
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' || base[i] == '\\' || base[i] == ':' {
			base = base[i+1:]
			break
		}
	}
	return strings.EqualFold(base, "EMMXXXX0")
}

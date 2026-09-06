package dos

import (
	"os"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// `int 21h`（`docs/spec/004` §2）。
//
// 這張表是**跑出來的**，不是照手冊列的：`rich2/docs/re/005` 那一輪把
// `RUN.EXE` 跑到資產全部載完（`DATA.PAK`／`PART1.PAK`／`SAVE_7.DSK`／
// `RICHA.RIX` 都開了）。沒列到的功能號照原則 3 記一筆，不要靜靜地放行。

func (d *DOS) int21(c *cpu.CPU) {
	fn := ah(c)
	if d.CallTrace != nil {
		rec := CallRec{AH: fn, AL: al(c), ESIn: c.Seg[cpu.ES], BXIn: c.R[cpu.BX]}
		defer func() {
			rec.ESOut, rec.BXOut = c.Seg[cpu.ES], c.R[cpu.BX]
			d.CallTrace = append(d.CallTrace, rec)
		}()
	}
	switch fn {
	case 0x4B: // EXEC
		d.exec(c)

	case 0x00, 0x4C:
		d.exit(c, al(c))

	case 0x01, 0x07, 0x08:
		d.conIn(c, fn)

	case 0x02, 0x06:
		d.conOut(c, fn)

	case 0x0B: // 查有沒有按鍵：AL=FFh 有、AL=00h 沒有
		if len(d.Stdin) == 0 {
			setAL(c, 0x00)
		} else {
			setAL(c, 0xFF)
		}
		clearCarry(c)

	case 0x09: // 輸出 $ 結尾的字串
		addr := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX])
		for i := 0; i < 1024; i++ {
			ch := d.M.Read8(addr + uint32(i))
			if ch == '$' {
				break
			}
			d.Console = append(d.Console, ch)
		}
		clearCarry(c)

	case 0x19: // 取目前磁碟機
		// 不實作的話 AL 是垃圾，遊戲把它拼進路徑就變成 `A:\…`，
		// 而 open 還是會成功（我們按檔名解析），**錯誤完全不顯現**。
		setAL(c, d.Drive)

	case 0x1A: // 設 DTA。收下就好，我們不用 FCB 那一套。
		clearCarry(c)

	case 0x1C: // 取指定磁碟機的配置資訊
		// AL ＝ 每叢集磁區數、CX ＝ 磁區大小、DX ＝ 叢集數、DS:BX → 媒體識別碼。
		// 給一組自洽的 1.2 MB 軟碟數字：程式拿它算「還有多少空間可以存檔」，
		// 回 0 會被讀成「磁碟滿了」。媒體識別碼指到樁段的一個位元組。
		setAL(c, 1)
		c.R[cpu.CX] = 512
		c.R[cpu.DX] = 2400
		c.Seg[cpu.DS] = machine.StubSeg
		c.R[cpu.BX] = 0
		d.M.Write8(uint32(machine.StubSeg)*16, 0xF9) // 1.2 MB 軟碟
		clearCarry(c)

	case 0x25: // 設中斷向量 ← DS:DX
		// **一定要真的寫進去。** 程式用它裝自己的 8087 模擬處理常式
		// （`INT 34h`–`3Dh`）；不寫的話所有浮點運算都落空，
		// 而 BASIC 的金錢運算全靠浮點。
		d.M.Write16(uint32(al(c))*4, c.R[cpu.DX])
		d.M.Write16(uint32(al(c))*4+2, c.Seg[cpu.DS])
		clearCarry(c)

	case 0x29: // 把檔名解析進 FCB
		d.parseFilename(c)

	case 0x2A: // 取系統日期 → CX:DH:DL
		c.R[cpu.CX] = 1993
		c.R[cpu.DX] = 1<<8 | 1
		setAL(c, 5) // 星期五
		clearCarry(c)

	case 0x2C: // 取系統時間 → CH:CL:DH:DL
		c.R[cpu.CX] = uint16(d.Now.Hour)<<8 | uint16(d.Now.Min)
		c.R[cpu.DX] = uint16(d.Now.Sec)<<8 | uint16(d.Now.Hundredth)
		clearCarry(c)

	case 0x67: // 設 handle 數上限
		// 我們沒有 handle 上限，收下就好。回成功是誠實的：
		// 程式要的是「之後開得了這麼多檔」，而那確實成立。
		clearCarry(c)

	case 0x30: // 取 DOS 版本
		c.R[cpu.AX] = 0x0005 // 5.0
		clearCarry(c)

	case 0x33: // Ctrl-Break 檢查旗標
		c.R[cpu.DX] = 0
		clearCarry(c)

	case 0x35: // 取中斷向量 → ES:BX
		c.R[cpu.BX] = d.M.Read16(uint32(al(c)) * 4)
		c.Seg[cpu.ES] = d.M.Read16(uint32(al(c))*4 + 2)
		clearCarry(c)

	case 0x36: // 取磁碟剩餘空間
		c.R[cpu.AX] = 8     // 每叢集磁區數
		c.R[cpu.BX] = 20000 // 可用叢集
		c.R[cpu.CX] = 512   // 每磁區位元組
		c.R[cpu.DX] = 40000 // 總叢集
		clearCarry(c)

	case 0x3D:
		d.open(c)
	case 0x3E:
		d.close(c)
	case 0x3F:
		d.read(c)
	case 0x40:
		d.write(c)
	case 0x42:
		d.seek(c)

	case 0x43: // 取／設檔案屬性
		d.fileAttr(c)

	case 0x44: // IOCTL
		if al(c) == 0x00 { // 取裝置資訊：bit7 = 0 表示是檔案
			c.R[cpu.DX] = uint16(d.Drive)
		}
		c.R[cpu.AX] = 0
		clearCarry(c)

	case 0x47: // 取目前目錄 → DS:SI（64 bytes）
		addr := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.SI])
		buf := []byte(d.Dir)
		if len(buf) > 63 {
			buf = buf[:63]
		}
		d.M.WriteBytes(addr, append(buf, 0))
		c.R[cpu.AX] = 0x0100
		clearCarry(c)

	case 0x48:
		d.alloc(c)
	case 0x49:
		d.release(c)
	case 0x4A:
		d.setBlock(c)

	case 0x52: // 取 DOS 內部結構表（list of lists）→ ES:BX
		c.Seg[cpu.ES] = machine.LOLSeg
		c.R[cpu.BX] = 0x10
		clearCarry(c)

	default:
		// 原則 1：**不要動 AX**。一開始寫 AX=0 會把「設中斷向量」迴圈的
		// 計數清掉，`AH` 變成 0 就被當成「結束程式」——程式因此提早死掉。
		d.note(0x21, fn, al(c))
		clearCarry(c)
	}
}

// conIn 是 `AH=01h`（有回顯）／`07h`（無回顯、不理 Ctrl-Break）／
// `08h`（無回顯、理 Ctrl-Break）的主控台輸入。字元回在 `AL`。
//
// ⚠ **真 DOS 的這三個都是阻塞的**：佇列空就停在那裡等人按鍵，一道指令都不走。
// 步進式的執行器停不下來，只能返回一個值，而**返回什麼都是在說謊**——
// 差別只在說得多難聽：
//
//   - 不動 `AX`（落到 default 的舊行為）＝ 餵殘留的垃圾當按鍵。
//     程式拿到一個沒人按過的鍵，通常不合法，於是重來一次，
//     結果是每十幾道指令一次的緊迴圈。
//   - 餵 `StdinFill` ＝ 假裝有人按了那個鍵。比垃圾更糟，因為它看起來像對的。
//
// 這裡選 `AL=0`：**0 不是任何一個可打出來的鍵**，程式會繼續等，
// 與真 DOS 的「還沒有人按」語意最接近。空轉照樣發生，但那是誠實的空轉——
// 同時 `KeyWaits` 會把它數出來，外面就分得出「在等鍵盤」與「在做事」。
//
// 要讓它往下走就餵鍵（probe 的 `-keys`、oracle 的 `SendKeys`），不是加大 `-steps`。
func (d *DOS) conIn(c *cpu.CPU, fn uint8) {
	if len(d.Stdin) == 0 {
		d.KeyWaits++
		if !d.NonBlockingKeys {
			// 阻塞：把 CS:IP 退回這道 INT，讓它下一步重跑。
			// 程式因此一道指令都不往前走，而 machine.Step 的 tick()
			// 照常推進計時器——背景動畫會繼續播，與真 DOS 相同。
			//
			// ⚠ 用 c.Rewind() 不要自己算 IP−2：前綴會讓指令長度不是 2。
			d.Blocked = true
			c.Rewind()
			return
		}
		setAL(c, 0)
		clearCarry(c)
		return
	}
	d.Blocked = false
	ch := d.Stdin[0]
	d.Stdin = d.Stdin[1:]
	setAL(c, ch)
	if fn == 0x01 { // 只有 AH=01h 回顯
		d.Console = append(d.Console, ch)
	}
	clearCarry(c)
}

// conOut 是 `AH=02h`／`AH=06h` 的主控台輸出。
//
// ⚠ **字元在 `DL`，不是 `AL`。** 第一版對 `AH=06h` 讀了 `AL`，收到的全是
// 垃圾，害 `RUN.EXE` 的錯誤訊息整個漏掉——印字元的路徑是
// `mov dx,ax / mov ah,6 / int 21h`。
func (d *DOS) conOut(c *cpu.CPU, fn uint8) {
	ch := uint8(c.R[cpu.DX])
	if fn == 0x06 && ch == 0xFF {
		// `DL=0FFh` 是「直接主控台**輸入**」，不是輸出。
		// 沒有按鍵時要回 ZF=1。
		setAL(c, 0)
		c.SetFlags(c.Flags | cpu.ZF)
		return
	}
	d.Console = append(d.Console, ch)
	clearCarry(c)
}

// fileAttr 是 `AH=43h`。**只實作 `AL=00h`（取屬性）**，那是目前唯一被用到的。
//
// 智冠《三國演義》在解壓資料之前查一次屬性；沒實作的話落到 default，
// `CX` 是殘留值、CF 清掉看起來像成功——程式拿到一個亂七八糟的屬性字組，
// 然後**以離開碼 255 結束**，而中途一個錯誤訊息都沒有。
//
// `AL=01h`（設屬性）與其他子功能故意不實作，讓 probe 繼續把它們列出來。
// 「實作實際用到的，其餘保持可見」比「先全部填掉」好：填掉的那一刻
// 就分不出「沒人用」與「用了但我們做錯」。
func (d *DOS) fileAttr(c *cpu.CPU) {
	if al(c) != 0x00 {
		d.note(0x21, 0x43, al(c))
		clearCarry(c)
		return
	}
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	path := d.resolve(name)
	if path == "" {
		// 找不到：CF=1、AX=2（file not found）。
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	// 一律回 `20h`（archive），**不回 `01h`（read-only）**。
	//
	// 我們把素材目錄唯讀掛載，但那是這個測試架構的性質，不是原版執行環境的。
	// 回 read-only 會讓程式據此改變行為（例如拒絕寫存檔），
	// 那就是把工具的實作細節洩漏進被觀測對象——量到的就不是原版了。
	c.R[cpu.CX] = 0x20
	clearCarry(c)
}

// exec 是 `AH=4Bh`。**只實作 `AL=03h`（載入 overlay）**。
//
// `AL=00h`（載入並執行子程序）需要另一套 PSP 與返回機制，目前沒有觀測到
// 任何程式用它；照原則保持「未實作」讓 probe 繼續列出來。
//
// 參數區塊在 ES:BX：word 0 是載入段，word 2 是重定位加數。
// **兩個是獨立的參數**，多數程式給相同的值但不保證。
func (d *DOS) exec(c *cpu.CPU) {
	if al(c) != 0x03 {
		d.note(0x21, 0x4B, al(c))
		clearCarry(c)
		return
	}
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	path := d.resolve(name)
	if path == "" {
		c.R[cpu.AX] = 2 // file not found
		setCarry(c)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.R[cpu.AX] = 2
		setCarry(c)
		return
	}
	pb := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.BX])
	loadSeg := d.M.Read16(pb)
	relocFactor := d.M.Read16(pb + 2)

	// ⚠ 參數區塊要在載入**之前**抄起來。overlay 常常就載在參數區塊
	// 所在的那一段，載完再讀會讀到剛寫進去的映像——那會讓診斷輸出
	// 看起來像「程式傳了一段機器碼當參數」，把人帶往完全錯的方向。
	rec := OverlayLoad{Name: name, Seg: loadSeg, Reloc: relocFactor, Size: len(data),
		PBSeg: c.Seg[cpu.ES], PBOff: c.R[cpu.BX],
		CallCS: c.Seg[cpu.CS], CallIP: c.IP, Steps: d.M.Steps}
	// INT 指令本身 2 byte，所以 CallIP-2 是它的起點；往前再留 22 byte
	// 看參數是怎麼備好的。
	for i := 0; i < 32; i++ {
		rec.CallSite[i] = d.M.Read8(cpu.Addr(c.Seg[cpu.CS], c.IP-24+uint16(i)))
	}
	for i := 0; i < 8; i++ {
		rec.PBRaw[i] = d.M.Read8(pb + uint32(i))
	}
	if err := d.M.LoadOverlay(data, loadSeg, relocFactor); err != nil {
		// **載入失敗要說出來。** overlay 沒載進去而回成功的話，程式會
		// far call 進一片空白，然後在幾百萬道指令之後死在一個與這裡
		// 毫無關聯的位址上。
		d.Console = append(d.Console, []byte("\n[dosgolem] overlay "+name+"："+err.Error()+"\n")...)
		c.R[cpu.AX] = 8
		setCarry(c)
		return
	}
	d.Overlays = append(d.Overlays, rec)
	clearCarry(c)
}

// parseFilename 是 `AH=29h`：把 `DS:SI` 的檔名解析進 `ES:DI` 的 FCB。
//
// 這一版只做**實際被用到的部分**：填磁碟機代號與 8.3 名稱、回報有沒有萬用字元、
// 並把 `SI` 推到解析完的位置。不做的部分（`AL` 的各種控制位元、
// 保留原有欄位）在目標程式上沒有觀測到。
//
// ⚠ **`SI` 一定要推進。** 呼叫端常常是「解析一個、再解析下一個」的迴圈；
// 不動 `SI` 的話它會解析同一個名字直到天荒地老，而**沒有任何錯誤訊息**。
func (d *DOS) parseFilename(c *cpu.CPU) {
	src := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.SI])
	dst := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DI])

	// 跳過前置空白與 tab。
	off := uint16(0)
	for {
		ch := d.M.Read8(src + uint32(off))
		if ch != ' ' && ch != '\t' {
			break
		}
		off++
	}

	// 磁碟機代號：有 "X:" 就用它，否則 0（＝目前磁碟）。
	drive := uint8(0)
	if b := d.M.Read8(src + uint32(off) + 1); b == ':' {
		dl := d.M.Read8(src + uint32(off))
		if dl >= 'a' && dl <= 'z' {
			dl -= 32
		}
		if dl < 'A' || dl > 'Z' {
			setAL(c, 0xFF) // 磁碟機代號無效
			return
		}
		drive = dl - 'A' + 1
		off += 2
	}
	d.M.Write8(dst, drive)

	// 名稱 8 格、副檔名 3 格，都用空白補齊——FCB 的版面。
	wildcard := false
	fill := func(at uint32, n int, stop func(uint8) bool) {
		i := 0
		for ; i < n; i++ {
			ch := d.M.Read8(src + uint32(off))
			if ch == 0 || stop(ch) {
				break
			}
			if ch == '*' {
				// 萬用字元 '*' 把剩下的格子填成 '?'。
				for ; i < n; i++ {
					d.M.Write8(at+uint32(i), '?')
				}
				wildcard = true
				off++
				return
			}
			if ch == '?' {
				wildcard = true
			}
			if ch >= 'a' && ch <= 'z' {
				ch -= 32 // FCB 一律大寫
			}
			d.M.Write8(at+uint32(i), ch)
			off++
		}
		for ; i < n; i++ {
			d.M.Write8(at+uint32(i), ' ')
		}
	}
	fill(dst+1, 8, func(ch uint8) bool { return ch == '.' || ch == ' ' || ch == '\\' })
	if d.M.Read8(src+uint32(off)) == '.' {
		off++
	}
	fill(dst+9, 3, func(ch uint8) bool { return ch == ' ' })

	// ⚠ SI 要推到解析完的位置，否則呼叫端的迴圈會原地打轉。
	c.R[cpu.SI] += off
	if wildcard {
		setAL(c, 1)
	} else {
		setAL(c, 0)
	}
}

// setBlock 是 `AH=4Ah`，**記憶體探測**（`docs/spec/004` §1.2）。
//
//	56C4  bx = 0FFFFh    ; 故意要求 0FFFFh 段
//	56C7  ah = 4Ah
//	56C9  int 21h
//	56CC  jae 錯誤       ; ★ 成功就跳「錯誤」
//	56CE  ah = 4Ah       ; 用 DOS 在 BX 回的實際大小再要一次
//	56D3  jb  錯誤       ; 這次失敗才算錯誤
//
// 一律清 CF 報成功的話**第一次呼叫就掉進錯誤路徑**——那是
// `DOS memory-arena error` 的真正根因，連續三輪調 MCB 佈局都無效。
func (d *DOS) setBlock(c *cpu.CPU) {
	want := c.R[cpu.BX]
	blk := c.Seg[cpu.ES]
	avail := uint16(machine.MemTop) - blk
	if want > avail {
		c.R[cpu.BX] = avail
		c.R[cpu.AX] = 8 // 記憶體不足
		setCarry(c)
		return
	}
	d.Resizes = append(d.Resizes, ResizeCall{Seg: blk, Want: want, FreeSeg: d.freeSeg})
	// 程式調整自己的 PSP 區塊之後，後面那塊才是可配置的空間。
	//
	// ⚠ **要跟著降，不能只升。** 第一版寫成 `if blk+want+1 > d.freeSeg`，
	// 只在變大時更新。那讓「記憶體探測」變成單向的災難：程式先要
	// `want=9EFFh`（能拿多少拿多少）探出上限，freeSeg 被推到 A000h
	// ＝ 可配置區歸零；接著程式縮回它真正要的大小，freeSeg 卻留在 A000h。
	// 之後每一次配置都失敗，而**失敗的地方離這裡很遠**——
	// 智冠《三國演義》是在載 overlay 時拿到一個荒謬的載入段
	// （0110h，正好蓋在自己的映像上），然後死在 overlay 自己的 C runtime。
	//
	// DOS 的語意是「區塊邊界移到這裡」，升降都算。
	if blk == machine.PSPSeg {
		d.freeSeg = blk + want + 1
		// 邊界移動了，arena 的基底也要跟著移。已經配出去的區塊還在用，
		// 不能直接丟；沒有任何存活區塊時才重建。
		live := false
		for _, b := range d.arena {
			if !b.free {
				live = true
				break
			}
		}
		if !live {
			d.arena = nil
		}
	}
	// arena 內的區塊走真正的 resize（規格 009）。不在 arena 內的
	// （PSP、映像本體）維持原本的行為：那條路是記憶體探測協定，
	// 上面的註解記著為什麼不能一律報成功。
	if d.arena != nil && d.resize(c, blk, want) {
		return
	}
	clearCarry(c)
}

// initArena 在第一次用到時把 [freeSeg, MemTop) 建成一個自由區塊。
//
// 延後到這裡是因為 freeSeg 會被 `AH=4Ah` 的 PSP 縮小改寫——
// 程式先縮自己的區塊，後面那塊才是可配置的。太早建會把還不屬於
// 我們的空間算進去。
func (d *DOS) initArena() {
	if d.arena != nil {
		return
	}
	base := d.freeSeg
	if base >= uint16(machine.MemTop) {
		d.arena = []memBlock{}
		return
	}
	d.arena = []memBlock{{seg: base, size: uint16(machine.MemTop) - base - 1, free: true}}
}

// largestFree 是目前最大的一塊自由空間（資料段數）。
func (d *DOS) largestFree() uint16 {
	var max uint16
	for _, b := range d.arena {
		if b.free && b.size > max {
			max = b.size
		}
	}
	return max
}

// alloc 是 `AH=48h`：首次適配。
//
// ⚠ **第一版是單向的 bump 配置器，而 `AH=49h` 是空操作。** 那組合對
// 「配置一次就用到結束」的程式沒問題，但任何配置／釋放交替的迴圈都會
// 單調吃光 640 KB。智冠《三國演義》的解壓階段配置 9,925 次、釋放 9,921 次，
// 淨值只有 4 塊——在舊實作下它撞到上限然後以離開碼 255 結束，
// **而且沒有任何錯誤訊息**（`docs/spec/009` §1）。
func (d *DOS) alloc(c *cpu.CPU) {
	d.initArena()
	want := c.R[cpu.BX]
	for i := range d.arena {
		b := &d.arena[i]
		if !b.free || b.size < want {
			continue
		}
		// 切得出一塊有意義的剩餘（至少 1 段 MCB ＋ 1 段資料）才切，
		// 否則整塊給出去——切出 0 段的區塊只會讓表變長。
		if b.size >= want+2 {
			rest := memBlock{seg: b.seg + want + 1, size: b.size - want - 1, free: true}
			b.size = want
			b.free = false
			d.arena = append(d.arena, memBlock{})
			copy(d.arena[i+2:], d.arena[i+1:])
			d.arena[i+1] = rest
		} else {
			b.free = false
		}
		c.R[cpu.AX] = d.arena[i].seg + 1
		clearCarry(c)
		return
	}
	c.R[cpu.AX] = 8 // 記憶體不足
	c.R[cpu.BX] = d.largestFree()
	setCarry(c)
}

// release 是 `AH=49h`：標記為自由**並合併相鄰的自由區塊**。
//
// 合併是必要的不是優化：不合併的話九千次配置／釋放會把空間切成
// 九千個碎片，最後一樣配不出大塊——換一種方式撞同一面牆。
func (d *DOS) release(c *cpu.CPU) {
	d.initArena()
	seg := c.Seg[cpu.ES]
	for i := range d.arena {
		if d.arena[i].seg+1 != seg {
			continue
		}
		d.arena[i].free = true
		d.coalesce()
		clearCarry(c)
		return
	}
	// 不認識的區塊：**照實回錯誤**。悄悄成功會把「釋放了不屬於自己的東西」
	// 藏起來，而那是真 DOS 會抓的錯（AX=9，MCB 位址無效）。
	c.R[cpu.AX] = 9
	setCarry(c)
}

// coalesce 把相鄰的自由區塊併起來。被吃掉的那一塊連它的 MCB 段一起回收。
func (d *DOS) coalesce() {
	for i := 0; i+1 < len(d.arena); {
		if d.arena[i].free && d.arena[i+1].free {
			d.arena[i].size += d.arena[i+1].size + 1
			d.arena = append(d.arena[:i+1], d.arena[i+2:]...)
			continue
		}
		i++
	}
}

// resize 是 `AH=4Ah` 落在 arena 內的區塊時的處理。
func (d *DOS) resize(c *cpu.CPU, seg, want uint16) bool {
	for i := range d.arena {
		b := &d.arena[i]
		if b.seg+1 != seg {
			continue
		}
		switch {
		case want <= b.size:
			// 縮小：後半段切出來標為自由，再與後面合併。
			if b.size >= want+2 {
				rest := memBlock{seg: b.seg + want + 1, size: b.size - want - 1, free: true}
				b.size = want
				d.arena = append(d.arena, memBlock{})
				copy(d.arena[i+2:], d.arena[i+1:])
				d.arena[i+1] = rest
				d.coalesce()
			}
			clearCarry(c)
		case i+1 < len(d.arena) && d.arena[i+1].free && b.size+1+d.arena[i+1].size >= want:
			// 放大：吃掉後面那塊自由區塊（連它的 MCB 段）。
			b.size += 1 + d.arena[i+1].size
			d.arena = append(d.arena[:i+1], d.arena[i+2:]...)
			// 吃太多就把多的再吐回去。
			if b.size >= want+2 {
				rest := memBlock{seg: b.seg + want + 1, size: b.size - want - 1, free: true}
				b.size = want
				d.arena = append(d.arena, memBlock{})
				copy(d.arena[i+2:], d.arena[i+1:])
				d.arena[i+1] = rest
				d.coalesce()
			}
			clearCarry(c)
		default:
			max := b.size
			if i+1 < len(d.arena) && d.arena[i+1].free {
				max += 1 + d.arena[i+1].size
			}
			c.R[cpu.BX] = max
			c.R[cpu.AX] = 8
			setCarry(c)
		}
		return true
	}
	return false
}

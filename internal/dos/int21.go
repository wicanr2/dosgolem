package dos

import (
	"fmt"

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
	// **失敗一律留下錯誤碼給 `AH=59h`。** DOS 的慣例是「CF=1 表示 AX 是
	// 錯誤碼」，所以在這裡收一次就涵蓋每一條失敗路徑——各支自己記的話，
	// 漏掉的那幾支會讓程式問到更早之前那一次的原因，然後照著錯的原因
	// 決定要重試還是放棄。
	//
	// `AH=59h` 自己不會落進來（它清 CF），所以問完不會把答案洗掉。
	if fn != 0x59 {
		defer func() {
			if c.Flags&cpu.CF != 0 {
				d.lastErr = c.R[cpu.AX]
			}
		}()
	}
	if d.CallTrace != nil {
		rec := CallRec{Step: d.M.Steps, AH: fn, AL: al(c), ESIn: c.Seg[cpu.ES], BXIn: c.R[cpu.BX]}
		defer func() {
			rec.ESOut, rec.BXOut = c.Seg[cpu.ES], c.R[cpu.BX]
			d.CallTrace = append(d.CallTrace, rec)
		}()
	}
	switch fn {
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

	case 0x0A: // 緩衝輸入：DS:DX ＝ [0]最大長度 [1]實際長度 [2..]內容
		// **格式要照 DOS 的**：位元組 0 是呼叫端填的容量（含 CR），
		// 位元組 1 是我們填的實際字元數，內容從位元組 2 開始，結尾補 CR。
		// 少填位元組 1 的話呼叫端讀到的長度是它自己上一次留下的值——
		// 那多半是 0（看起來像「使用者什麼都沒輸入」）或一個過大的數字
		// （於是它把緩衝區後面的垃圾當成輸入）。
		d.bufferedInput(c)

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

	case 0x0D: // flush disk buffers：我們的寫檔根本不做（Wrote 清單），
		// 沒有可 flush 的東西——靜默收下（`docs/spec/009` §3），不是「沒實作」。
		clearCarry(c)

	case 0x19: // 取目前磁碟機
		// 不實作的話 AL 是垃圾，遊戲把它拼進路徑就變成 `A:\…`，
		// 而 open 還是會成功（我們按檔名解析），**錯誤完全不顯現**。
		setAL(c, d.Drive)

	case 0x1A: // 設 DTA ← DS:DX
		d.dtaSeg, d.dtaOff = c.Seg[cpu.DS], c.R[cpu.DX]
		clearCarry(c)

	case 0x1B, 0x1C: // 取（預設／指定）磁碟機的配置資訊
		// `AL` ＝ 每叢集磁區數、`CX` ＝ 每磁區位元組、`DX` ＝ 總叢集，
		// **`DS:BX` 要指到媒體識別位元組**。
		//
		// 這一項不能用「宣告成功但不動暫存器」的預設處置：呼叫端拿到的
		// `DS:BX` 是它自己傳進來的值，讀出來的媒體位元組是垃圾，
		// 而 `AL = 0xFF`（無效磁碟機）與任意垃圾值都可能讓它判定「沒有磁碟」
		// 然後停在等待迴圈裡——**沒有錯誤訊息，只有 CPU 一直在跑**。
		//
		// 數字要自洽而且夠大：程式拿它算「還有多少空間可以存檔」，
		// 回 0 會被讀成「磁碟滿了」。8 × 512 × 40000 ≈ 163 MB。
		//
		// 媒體位元組放 BDA 的程式間通訊區（`0040:00F0`，16 bytes，
		// 我們沒有別的東西用它）。`0xF8` ＝ 固定磁碟，與 `AH=19h` 回的
		// 預設磁碟機（C:）一致。⚠ **不要放在 StubSeg**：那一段是
		// 每個向量的 stub，寫進去等於把 `int 00h` 的處理常式改掉。
		const mediaAt = 0x0040*16 + 0xF0
		d.M.Write8(mediaAt, 0xF8)
		setAL(c, 8)         // 每叢集磁區數
		c.R[cpu.CX] = 512   // 每磁區位元組
		c.R[cpu.DX] = 40000 // 總叢集
		c.Seg[cpu.DS], c.R[cpu.BX] = 0x0040, 0x00F0
		clearCarry(c)

	case 0x25: // 設中斷向量 ← DS:DX
		// **一定要真的寫進去。** 程式用它裝自己的 8087 模擬處理常式
		// （`INT 34h`–`3Dh`）；不寫的話所有浮點運算都落空，
		// 而 BASIC 的金錢運算全靠浮點。
		d.M.Write16(uint32(al(c))*4, c.R[cpu.DX])
		d.M.Write16(uint32(al(c))*4+2, c.Seg[cpu.DS])
		d.VecSets = append(d.VecSets, VecSet{Int: al(c),
			Seg: c.Seg[cpu.DS], Off: c.R[cpu.DX], Step: d.M.Steps})
		clearCarry(c)

	case 0x29: // 把檔名解析進 FCB
		d.parseFilename(c)

	case 0x2A: // 取系統日期 → CX:DH:DL
		c.R[cpu.CX] = 1993
		c.R[cpu.DX] = 1<<8 | 1
		setAL(c, 5) // 星期五
		clearCarry(c)
	case 0x2C: // 取系統時間 → CH:CL:DH:DL
		t := d.clock()
		c.R[cpu.CX] = uint16(t.Hour)<<8 | uint16(t.Min)
		c.R[cpu.DX] = uint16(t.Sec)<<8 | uint16(t.Hundredth)
		clearCarry(c)

	case 0x67: // 設 handle 數上限
		// 程式要的是「之後開得了這麼多檔」。真的照做——上限同時也是
		// **號碼配置的邊界**（見 `allocHandle`），收下卻不改的話
		// 程式以為自己有 40 格、實際只拿得到 20 個號碼。
		if want := c.R[cpu.BX]; want > d.MaxHandles {
			d.MaxHandles = want
		}
		clearCarry(c)

	case 0x2F: // 取 DTA → ES:BX
		c.Seg[cpu.ES], c.R[cpu.BX] = d.dtaSeg, d.dtaOff

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

	case 0x3C:
		d.create(c)
	case 0x3D:
		d.open(c)
	case 0x3E:
		d.close(c)
	case 0x3F:
		d.read(c)
	case 0x40:
		d.write(c)
	case 0x41:
		d.unlink(c)
	case 0x42:
		d.seek(c)
	case 0x43: // 取／設檔案屬性（`docs/spec/008` §4）
		d.fileAttr(c)

	case 0x44: // IOCTL
		if al(c) == 0x00 { // 取裝置資訊：bit7 = 0 表示是檔案
			c.R[cpu.DX] = uint16(d.Drive)
		} else {
			// 其他子功能沒實作——記下來，別讓「成功」假象藏住。
			d.note(0x21, 0x44, al(c))
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
		d.syncMCB()
	case 0x49:
		d.release(c)
		d.syncMCB()
	case 0x4A:
		d.setBlock(c)
		d.syncMCB()
	case 0x31: // TSR 並結束（`docs/spec/008`）
		d.tsr(c)
	case 0x4B: // EXEC（`docs/spec/007` §2／`docs/spec/009`：AL=00h 與 AL=03h）
		d.exec(c)
	case 0x4D: // 取子行程回傳碼（`docs/spec/009` §3）
		d.getExitCode(c)

	case 0x51, 0x62: // 取目前行程的 PSP → BX
		// **這是 `KI.EXE` 的第一道指令。** 不實作的話 BX 是呼叫端留下的值，
		// 接著 `mov ds,bx / mov al,ds:80h` 就把垃圾當成命令列長度。
		c.R[cpu.BX] = machine.PSPSeg
		clearCarry(c)
	case 0x4E:
		d.findFirst(c)
	case 0x4F: // Find Next（`docs/knowledge-base/010`）
		d.findNext(c)

	case 0x39: // 建目錄
		d.mkdir(c)
	case 0x3A: // 刪目錄
		d.rmdir(c)
	case 0x3B: // 切目錄
		d.chdir(c)
	case 0x45: // 複製 handle
		d.dupHandle(c)
	case 0x46: // 強制複製 handle（dup2／重導向）
		d.dup2Handle(c)
	case 0x56: // 更名
		d.renameFile(c)
	case 0x57: // 取／設檔案日期時間
		d.fileTime(c)
	case 0x59: // 取延伸錯誤資訊
		d.extendedError(c)
	case 0x5B: // 建立新檔（已存在就失敗）
		d.createNew(c)
	case 0x68, 0x6A: // commit file
		d.commitFile(c)

	case 0x52: // 取 DOS 內部結構表（list of lists）→ ES:BX
		c.Seg[cpu.ES] = machine.LOLSeg
		c.R[cpu.BX] = 0x10
		clearCarry(c)

	case 0x38: // 取國別資訊（`docs/spec/010`）
		d.country(c)

	case 0x63: // DOS/V：DBCS 前導位元組表（`docs/spec/010` §2）
		d.dbcs(c)

	default:
		// 原則 1：**不要動 AX**。一開始寫 AX=0 會把「設中斷向量」迴圈的
		// 計數清掉，`AH` 變成 0 就被當成「結束程式」——程式因此提早死掉。
		d.noteCPU(c, 0x21, fn, al(c))
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
	d.noteKey(fmt.Sprintf("int21-AH%02X", fn), ch)
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
// availFrom 回從 seg 起還剩幾段可用。
//
// ⚠ **一定要夾在 0**。直接寫 `MemTop - seg` 的話，指標越過 `MemTop` 之後
// uint16 環繞成 0FFFFh，於是探測**永遠成功**——「把記憶體配光」的迴圈因此
// 一路配到 0FFFFh 段再繞回低位，把整台機器的記憶體覆蓋掉。
func availFrom(seg uint16) uint16 {
	if seg >= uint16(machine.MemTop) {
		return 0
	}
	return uint16(machine.MemTop) - seg
}

func (d *DOS) setBlock(c *cpu.CPU) {
	want := c.R[cpu.BX]
	blk := c.Seg[cpu.ES]
	avail := availFrom(blk)
	if want > avail {
		d.MemOps = append(d.MemOps, MemOp{Fn: 0x4A, BX: want, ES: blk,
			AX: avail, Step: d.M.Steps})
		c.R[cpu.BX] = avail
		c.R[cpu.AX] = 8 // 記憶體不足
		setCarry(c)
		return
	}
	d.MemOps = append(d.MemOps, MemOp{Fn: 0x4A, BX: want, ES: blk,
		AX: 0, Step: d.M.Steps, OK: true})
	d.Allocs = append(d.Allocs, AllocOp{Step: d.M.Steps, Fn: 0x4A, Want: want, Seg: blk, OK: true})
	rec := ResizeCall{Seg: blk, Want: want, FreeSeg: d.freeSeg, CS: c.Seg[cpu.CS], IP: c.IP}
	rec.Before, rec.InArena = d.blockSize(blk)
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
	// ⚠ **判準是「目前這個行程的 PSP」，不是主程式的 PSP。** 只認
	// machine.PSPSeg 的話，EXEC 起來的子行程把自己的區塊撐大之後
	// freeSeg 停在它的映像結尾，接下來的 AH=48h 就**把子行程自己的
	// 記憶體再配一次出去**——配到的緩衝區蓋在它的堆疊上，讀個檔就把
	// 返回位址換成檔案內容，然後 retf 到一個看起來很像程式碼的地方。
	// （源平合戰的 OPEN.EXE：AH=4Ah 撐到 8340 段、擁有到 28A3h，
	// 而 AH=48h 從 1A50h 配下去，讀 LOGO.GP 蓋掉堆疊。）
	if blk == d.curPSP {
		// ⚠ **這裡沒有檢查 arena 裡別人已配走的區塊。**
		//
		// 試過加上檢查（放大時上限取「第一個已配置區塊的起點」，
		// 那是比較接近 DOS 語意的做法），結果讓智冠《三國演義》**更早死**：
		// 探測回報上限 5150h 之後，程式當場判定記憶體不足、
		// 連 `AH=4Bh` 都沒發出就印 `R6005 - not enough memory on exec`。
		// 修正前它至少走到讀取資料容器。
		//
		// 兩種行為哪一個才是真 DOS，**目前沒有證據**——要拿 DOSBox-X
		// 當交叉 oracle 量一次。在那之前保留原本的寬鬆做法：
		// 它讓目標程式走得更遠，而走得更遠才有更多可觀測的東西。
		// 這是刻意的取捨，不是疏漏。
		d.setPSPBlock(blk + want + 1)
	}
	// arena 內的區塊走真正的 resize（規格 009）。不在 arena 內的
	// （PSP、映像本體）維持原本的行為：那條路是記憶體探測協定，
	// 上面的註解記著為什麼不能一律報成功。
	if d.arena != nil && d.resize(c, blk, want) {
		rec.After, _ = d.blockSize(blk)
		rec.OK = c.Flags&cpu.CF == 0
		d.Resizes = append(d.Resizes, rec)
		return
	}
	clearCarry(c)
	rec.After, _ = d.blockSize(blk)
	rec.OK = true
	d.Resizes = append(d.Resizes, rec)
}

// blockSize 查 arena 裡以 seg 為資料起點的區塊有多少段。
func (d *DOS) blockSize(seg uint16) (uint16, bool) {
	for _, b := range d.arena {
		if b.seg+1 == seg {
			return b.size, true
		}
	}
	return 0, false
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

// setPSPBlock 把可配置區的起點移到 newFree，並讓 arena 跟著動。
//
// ⚠ **只更新 freeSeg 是不夠的。** 第一版那樣寫，程式把自己的 PSP 區塊
// 縮小之後 freeSeg 確實降下來了，但 arena 還停在舊的基底——
// `[newFree, 舊基底)` 這段新釋出的記憶體**從來沒有進到區塊表裡**。
//
// 症狀離現場很遠：智冠《三國演義》的 overlay 在解壓後要 3 個段
// （48 bytes，環境指標陣列），arena 裡一個自由區塊都沒有，配置失敗，
// MSC 啟動碼因此印 `R6009 - not enough space for environment` 並以 255 離開。
// 從外面看像「overlay 壞了」，實際上是配置器少記了 190 KB。
func (d *DOS) setPSPBlock(newFree uint16) {
	if d.arena == nil {
		d.freeSeg = newFree
		return
	}
	base := d.arena[0].seg
	switch {
	case newFree < base && base-newFree >= 2:
		// 縮小：[newFree, base) 回到可配置區。至少要 2 段才放得下
		// 一個 MCB ＋ 一段資料。
		d.arena = append([]memBlock{{seg: newFree, size: base - newFree - 1, free: true}},
			d.arena...)
		d.coalesce()
	case newFree > base:
		// 放大：程式要回前面那段。**只吃自由的部分**——
		// 前端已經配出去的區塊不能收回，那會讓別人手上的指標失效。
		for len(d.arena) > 0 && d.arena[0].free && d.arena[0].seg+1+d.arena[0].size <= newFree {
			d.arena = d.arena[1:]
		}
		if len(d.arena) > 0 && d.arena[0].free && d.arena[0].seg < newFree {
			shrink := newFree - d.arena[0].seg
			if d.arena[0].size > shrink {
				d.arena[0].seg = newFree
				d.arena[0].size -= shrink
			}
		}
	}
	d.freeSeg = newFree
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
		d.noteMem(c, 0x48, want, d.arena[i].seg+1, d.arena[i].size, true)
		return
	}
	c.R[cpu.AX] = 8 // 記憶體不足
	c.R[cpu.BX] = d.largestFree()
	setCarry(c)
	d.noteMem(c, 0x48, want, 0, d.largestFree(), false)
}

// noteMem 記一筆配置器帳。MemTrace 是 nil 就什麼都不做。
func (d *DOS) noteMem(c *cpu.CPU, op uint8, want, seg, got uint16, ok bool) {
	if d.MemTrace == nil {
		return
	}
	d.MemTrace = append(d.MemTrace, MemCall{
		Step: d.M.Steps, Op: op, Want: want, Seg: seg, Got: got, OK: ok,
		CS: c.Seg[cpu.CS], IP: c.IP, DS: c.Seg[cpu.DS], ES: c.Seg[cpu.ES],
	})
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
		size := d.arena[i].size
		d.arena[i].free = true
		d.coalesce()
		clearCarry(c)
		d.noteMem(c, 0x49, 0, seg, size, true)
		return
	}
	// 不認識的區塊：**照實回錯誤**。悄悄成功會把「釋放了不屬於自己的東西」
	// 藏起來，而那是真 DOS 會抓的錯（AX=9，MCB 位址無效）。
	c.R[cpu.AX] = 9
	setCarry(c)
	d.noteMem(c, 0x49, 0, seg, 0, false)
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

// syncMCB 把配置器的區塊表發布成客體記憶體裡的 MCB 鏈。
//
// **配置器的狀態只活在 Go 這一側是不夠的。** DOS 程式看得到 MCB，
// 而且會走它：智冠《三國演義》在載入下一個模組之前，從自己的 PSP
// 沿著鏈往上加，算出「我總共構得到多少段」再拿去要記憶體
// （`docs/spec/009` 附錄五，反組譯在 `0583:3068`）。鏈沒同步的話
// 那個數字與事實無關，而**程式不會因此報錯**——它只是拿錯的數字
// 去做決定，然後在很遠的地方失敗。
//
// 鏈的形狀：`PSPSeg−1` 是程式自己的區塊（涵蓋 PSP ＋ 映像），
// 接著是 arena 的每一塊，最後一塊掛 `Z`。arena 相鄰無空隙
// （每塊佔 1 段 MCB ＋ size 段資料），所以直接照順序寫就是一條合法的鏈。
func (d *DOS) syncMCB() {
	d.M.WriteMCB(machine.PSPSeg-1, len(d.arena) == 0, machine.PSPSeg,
		d.freeSeg-machine.PSPSeg)
	for i, b := range d.arena {
		owner := uint16(machine.PSPSeg)
		if b.free {
			owner = 0
		}
		d.M.WriteMCB(b.seg, i == len(d.arena)-1, owner, b.size)
	}
	if len(d.arena) == 0 {
		// 還沒配置過：可配置區整塊掛成自由的鏈尾。
		d.M.WriteMCB(d.freeSeg, true, 0, uint16(machine.MemTop)-d.freeSeg-1)
		d.M.WriteMCB(machine.PSPSeg-1, false, machine.PSPSeg, d.freeSeg-machine.PSPSeg)
	}
}

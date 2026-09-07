package dos

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 第二批補洞（`docs/knowledge-base/010-dos-int21-coverage.md` 的 P2）：
// `AH=0Ch`、`32h`、`34h`、`37h`、`58h`、`5Ah`、`5Ch`、`60h`、`6Ch`。
//
// 這一批的共通點是**它們不是檔案操作，而是「程式在問這台機器長什麼樣」**。
// 問不到答案的程式不會報錯，它會照沒有初始化的暫存器往下走——
// 所以每一支寧可回一個保守但誠實的值，也不要落到 default 不動 AX。

// flushAndInput 是 `AH=0Ch`：清掉鍵盤緩衝區，再執行 `AL` 指定的輸入功能。
//
// 用途是「把使用者在上一個畫面亂按的鍵丟掉，再問一次」。**清掉是重點**：
// 不清的話，那些殘留的鍵會被下一個提示直接吃掉，玩家看到的是選單自己跳過去。
//
// ⚠ **`Stdin` 不清。** BDA 環與 `d.Keys` 是模擬的鍵盤，清掉對得起語意；
// `Stdin` 是我們自己的回放管道（probe 的 `-keys`、oracle 的 `SendKeys`），
// 它裝的是「這一輪要餵的整串輸入」。跟著清掉的話，程式只要呼叫一次 `AH=0Ch`
// 就會把後面所有預備好的按鍵吃光，而回放會停在一個看起來像「程式不動了」的
// 地方。這是與真 DOS 的明確差異，記在 `docs/spec/184`。
func (d *DOS) flushAndInput(c *cpu.CPU) {
	d.M.FlushKeys()
	d.Keys = nil
	// **直接叫下層，不要遞迴回 `int21`**：那會讓同一次呼叫在 `CallTrace`
	// 裡出現兩筆，而看 trace 的人會以為程式真的叫了兩次。
	switch sub := al(c); sub {
	case 0x01, 0x07, 0x08:
		setAH(c, sub)
		d.conIn(c, sub)
	case 0x06:
		setAH(c, sub)
		d.conOut(c, sub)
	case 0x0A:
		d.bufferedInput(c)
	default:
		// 其他 AL 只做清空。真 DOS 也是這樣（AL 不是那五個就只 flush）。
		setAL(c, 0)
		clearCarry(c)
	}
}

// driveParams 是 `AH=32h`：取磁碟參數區塊（DPB）。
//
// **回 AL=FFh（無效磁碟機）而不是偽造一份 DPB。** DPB 裡面是 FAT 起始磁區、
// 每叢集磁區數、根目錄項數、裝置驅動程式的 far 指標——程式拿到之後會
// 沿著那個指標走進驅動程式，或直接照那些數字算磁區位置。我們沒有真的
// 檔案系統，填出來的每一個欄位都是編的，而**編得越像，程式走得越遠才炸**。
// 回 FFh 的話它就地放棄，錯誤停在問的地方。
//
// 會踩到這裡的多半是磁碟工具與安裝程式；`Unimplemented` 會把它數出來。
func (d *DOS) driveParams(c *cpu.CPU) {
	d.note(0x21, 0x32, al(c))
	setAL(c, 0xFF)
	clearCarry(c)
}

// inDOSFlag 是 `AH=34h`：ES:BX 指向 InDOS 旗標。
//
// TSR 用它判斷「現在能不能安全地叫 DOS」：非 0 表示 DOS 正在處理別的請求，
// 這時再進去會踩壞它的內部堆疊。
//
// 我們的 `int 21h` 是一次做完的，沒有可以被打斷的中間狀態——任何人讀得到
// 這個旗標的時刻，DOS 都不在裡面，所以值恆為 0。**指標要指到真的記憶體**：
// 回 0000:0000 的話，程式寫回去（有些 TSR 會自己加減）就把 `int 00h` 的
// 向量改掉了，而症狀是除以零的時候跳進一片亂碼。
func (d *DOS) inDOSFlag(c *cpu.CPU) {
	d.M.Write8(uint32(machine.StubSeg)*16+inDOSOff, 0)
	c.Seg[cpu.ES] = machine.StubSeg
	c.R[cpu.BX] = inDOSOff
	clearCarry(c)
}

// switchChar 是 `AH=37h`：取／設「選項字元」（DOS 的 `/`、Unix 的 `-`）。
//
// 程式用它決定要把命令列裡的 `/S` 當成選項還是當成路徑。回垃圾的話，
// 拿到的可能是 `\`——於是它把自己的選項當成目錄名，開檔全部失敗。
func (d *DOS) switchChar(c *cpu.CPU) {
	switch al(c) {
	case 0x00: // 取
		setAL(c, 0)
		c.R[cpu.DX] = c.R[cpu.DX]&0xFF00 | uint16(d.SwitchChar)
	case 0x01: // 設
		d.SwitchChar = uint8(c.R[cpu.DX])
		setAL(c, 0)
	case 0x02, 0x03:
		// 「裝置可用性」：DOS 2.x 的遺跡，3.0 之後沒有作用。
		// 回 DL=0 ＝「必須加 `\DEV\` 前綴」，與 DOS 5 相同。
		setAL(c, 0)
		if al(c) == 0x02 {
			c.R[cpu.DX] = c.R[cpu.DX] & 0xFF00
		}
	default:
		setAL(c, 0xFF)
	}
	clearCarry(c)
}

// allocStrategyCall 是 `AH=58h`：取／設記憶體配置策略與 UMB 連結狀態。
//
// 記憶體吃緊的程式會把策略切成 best fit 再配大塊，配完切回來。**設完要
// 真的生效**（見 `pickBlock`）：只記不用的話，`AH=48h` 給的段位址與程式
// 依策略算出來的預期不同，而它多半是拿那個位址去比大小、算距離的。
func (d *DOS) allocStrategyCall(c *cpu.CPU) {
	switch al(c) {
	case 0x00: // 取策略
		c.R[cpu.AX] = d.allocStrategy
		clearCarry(c)
	case 0x01: // 設策略
		v := c.R[cpu.BX]
		// 低半位元組是 fit 型別（0／1／2），高位元組選記憶體區
		// （00 ＝ 只用傳統、40h ＝ 高位優先、80h ＝ 只用高位）。
		if v&0x0F > 2 || (v&^uint16(0x0F) != 0 && v&^uint16(0x0F) != 0x40 && v&^uint16(0x0F) != 0x80) {
			d.fail(c, 1) // 無效功能
			return
		}
		if v&^uint16(0x0F) != 0 {
			// 我們的配置池只有傳統記憶體（`initArena`），UMB 是 XMS
			// 那邊另外一組（`AH=10h`）。記一筆，別讓「要求高位記憶體
			// 卻拿到傳統記憶體」這件事沒有痕跡。
			d.note(0x21, 0x58, 0x81)
		}
		d.allocStrategy = v
		c.R[cpu.AX] = v
		clearCarry(c)
	case 0x02: // 取 UMB 連結狀態
		if d.umbLink {
			setAL(c, 1)
		} else {
			setAL(c, 0)
		}
		clearCarry(c)
	case 0x03: // 設 UMB 連結狀態
		d.umbLink = c.R[cpu.BX]&1 != 0
		if d.umbLink {
			d.note(0x21, 0x58, 0x83) // 同上：連起來了但配置池沒變
		}
		clearCarry(c)
	default:
		d.fail(c, 1)
	}
}

// createTemp 是 `AH=5Ah`：在 DS:DX 給的目錄底下建一個檔名唯一的檔，
// **並把產生的檔名接在那個路徑後面寫回去**。
//
// 寫回去是這支的重點：呼叫端接著就拿同一個緩衝區當檔名去 `AH=41h` 刪掉它。
// 不寫回的話它刪到的是那個目錄本身的名字（多半失敗），而暫存檔就一直留著。
//
// 檔名用流水號不用時間：`AH=5Ah` 在真 DOS 是拿系統時鐘編的，
// 那會讓同一份輸入每次得到不同的檔案清單。
func (d *DOS) createTemp(c *cpu.CPU) {
	dir := d.readCString(c.Seg[cpu.DS], c.R[cpu.DX], 128)
	var name string
	for {
		name = fmt.Sprintf("TMP%05d", d.tempCount)
		d.tempCount++
		if d.resolve(name) == "" {
			break
		}
	}
	full := dir + name
	if len(full) > 126 {
		d.fail(c, 3) // 路徑找不到
		return
	}
	d.M.WriteBytes(cpu.Addr(c.Seg[cpu.DS], c.R[cpu.DX]), append([]byte(full), 0))
	d.create(c) // 讀的就是我們剛寫回去的那個字串
}

// lockRegion 是 `AH=5Ch`：鎖定／解鎖檔案的一段。
//
// 這一層只有一個行程，沒有第二個人來搶——所以鎖一定拿得到。**照樣要
// 驗 handle**：handle 不對還回成功的話，程式會以為自己鎖住了一段其實
// 不存在的檔案，接著放心地寫下去。
//
// 記一筆是因為「鎖得到」在單行程底下是恆真的，換成多行程就不是了；
// 真的踩到共用檔案的程式要看得見這個假設。
func (d *DOS) lockRegion(c *cpu.CPU) {
	if _, ok := d.handles[c.R[cpu.BX]]; !ok {
		d.fail(c, 6) // 無效 handle
		return
	}
	d.note(0x21, 0x5C, al(c))
	clearCarry(c)
}

// trueName 是 `AH=60h`：把 DS:SI 的路徑正規化成完整路徑寫進 ES:DI。
//
// 安裝程式拿它把使用者打的相對路徑變成絕對路徑再存進設定檔。回原字串
// （或什麼都不寫）的話，設定檔裡留下的是相對路徑，而下一次從別的目錄
// 啟動就找不到了——症狀出現在**下一次執行**，離這裡很遠。
func (d *DOS) trueName(c *cpu.CPU) {
	in := d.readCString(c.Seg[cpu.DS], c.R[cpu.SI], 128)
	if in == "" {
		d.fail(c, 3) // 路徑找不到
		return
	}
	out := d.canonical(in)
	if len(out) > 127 {
		d.fail(c, 3)
		return
	}
	d.M.WriteBytes(cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DI]), append([]byte(out), 0))
	c.R[cpu.AX] = c.R[cpu.AX] & 0xFF00 // DOS 把 AL 清成 0
	clearCarry(c)
}

// canonical 把一個 DOS 路徑攤成 `X:\A\B\C`。
//
// 全部轉大寫（FAT 不分大小寫，而程式會拿結果去做字串比對）、`.` 丟掉、
// `..` 往上退一層、多餘的分隔符併掉。退到根目錄之上就停在根目錄——
// 真 DOS 也是這樣，不會回錯誤。
func (d *DOS) canonical(in string) string {
	s := strings.ToUpper(strings.ReplaceAll(in, "/", `\`))
	drive := byte('A' + d.Drive)
	if len(s) >= 2 && s[1] == ':' {
		if s[0] >= 'A' && s[0] <= 'Z' {
			drive = s[0]
		}
		s = s[2:]
	}
	var parts []string
	if !strings.HasPrefix(s, `\`) {
		// 相對路徑：接在目前目錄後面。
		parts = splitPath(d.Dir)
	}
	for _, seg := range splitPath(s) {
		switch seg {
		case ".":
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, seg)
		}
	}
	return string(drive) + `:\` + strings.Join(parts, `\`)
}

func splitPath(s string) []string {
	var out []string
	for _, seg := range strings.Split(s, `\`) {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// extendedOpen 是 `AH=6Ch`：一支呼叫同時表達「有就開／有就砍掉重建／
// 沒有就建」，並回報**實際做了哪一件**（CX）。
//
// CX 是重點：程式靠它分辨「這是我剛建的新檔」與「這是上次留下的檔」。
// 不填的話它讀到殘值，可能把一個已經有內容的存檔當成空白的。
//
// 參數（注意檔名在 **DS:SI**，不是 DS:DX）：
//
//	BX  開檔模式（低三位 ＝ 存取方式，與 `AH=3Dh` 的 AL 相同）
//	CX  建檔時的屬性
//	DX  動作：低半位元組 ＝ 檔案存在時（0 失敗／1 開啟／2 覆寫），
//	    高半位元組 ＝ 檔案不存在時（0 失敗／1 建立）
func (d *DOS) extendedOpen(c *cpu.CPU) {
	name := d.readCString(c.Seg[cpu.DS], c.R[cpu.SI], 128)
	exists := d.resolve(name) != ""
	action := c.R[cpu.DX]

	// `open`／`create` 都從 DS:DX 讀檔名，這裡先把 SI 借過去再還回來。
	savedDX, savedAX := c.R[cpu.DX], c.R[cpu.AX]
	c.R[cpu.DX] = c.R[cpu.SI]
	defer func() { c.R[cpu.DX] = savedDX }()

	var taken uint16
	switch {
	case exists && action&0x0F == 1: // 開啟既有的
		c.R[cpu.AX] = savedAX&0xFF00 | c.R[cpu.BX]&0x07
		d.open(c)
		taken = 1
	case exists && action&0x0F == 2: // 砍掉重建
		d.create(c)
		taken = 3
	case !exists && action&0xF0 == 0x10: // 建立新的
		d.create(c)
		taken = 2
	case exists:
		d.fail(c, 80) // 檔案已存在
		return
	default:
		d.fail(c, 2) // 找不到檔
		return
	}
	if c.Flags&cpu.CF != 0 {
		// 失敗時 AX 已經是錯誤碼；`fail` 沒走到，這裡補記一次。
		d.lastErr = c.R[cpu.AX]
		return
	}
	c.R[cpu.CX] = taken
	clearCarry(c)
}

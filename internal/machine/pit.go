package machine

// PITBaseHz 是 PC 的 8253／8254 輸入時脈。
//
// NTSC 彩色副載波 `315/88` MHz ×4 ＝ 14.318181… MHz，再 ÷12 ＝
// `315/264` MHz ＝ **1,193,181.8181… Hz**。文獻常寫成 1,193,182——那是
// 四捨五入，差 3×10⁻⁷，一般沒差；但拿來跟別處的常數對拍時會差出來，
// 所以這裡用**精確值**。
//
// **這是硬體的常數，不是任何一支程式的知識。** 每個程式自己寫進去的是**除數**，
// 所以「這個遊戲跑幾 Hz」＝ `PITBaseHz / 除數`——不要把某一支遊戲的頻率寫死在
// 程式碼裡：《大富翁2》寫 17,000（70.187 Hz），《臥龍傳》的 `YNSOUND.COM`
// 寫 4,096（291.3 Hz），開機預設是 65,536（18.2 Hz）。
const PITBaseHz = 315e6 / 264

// PITDefaultDivisor 是 BIOS 開機時的除數：0 在 8254 上代表 65,536。
const PITDefaultDivisor = 65536

// pit 記通道 0 目前的除數。**只解讀寫入，不推進時鐘**——dosgolem 的時間由
// 指令數驅動（`IRQ0Every`），這裡回答的是「程式要求多快」，不是「現在幾點」。
type pit struct {
	divisor  uint32 // 0 ＝ 還沒被程式設過
	writes   uint64 // 完成過幾次重載，判「有沒有被設過」用
	access   uint8  // 命令位元組的 bits 5–4
	lo       uint8
	haveLo   bool
	selected bool // 上一個命令位元組選的是不是通道 0
}

// out 解讀一次埠寫入，回報有沒有吃下去。
//
// 8254 的命令位元組：bits 7–6 通道、5–4 存取方式（01 只低、10 只高、
// 11 先低後高；00 是鎖存，不改設定）、3–1 模式、0 BCD。
// **模式與 BCD 不影響除數**，所以這裡不挑模式——挑了就會漏掉用模式 2 的程式。
func (p *pit) out(port uint16, v uint8) bool {
	switch port {
	case 0x43:
		p.selected = v>>6 == 0
		if !p.selected {
			return false
		}
		if acc := (v >> 4) & 3; acc != 0 { // 00 ＝ 鎖存，不動設定
			p.access, p.haveLo = acc, false
		}
		return true
	case 0x40:
		if !p.selected {
			return false
		}
		switch p.access {
		case 1: // 只寫低位元組
			p.set(uint32(v))
		case 2: // 只寫高位元組
			p.set(uint32(v) << 8)
		default: // 先低後高
			if !p.haveLo {
				p.lo, p.haveLo = v, true
				return true
			}
			p.haveLo = false
			p.set(uint32(p.lo) | uint32(v)<<8)
		}
		return true
	}
	return false
}

func (p *pit) set(d uint32) {
	if d == 0 {
		d = PITDefaultDivisor // 8254：寫 0 代表 65,536
	}
	p.divisor, p.writes = d, p.writes+1
}

// PITDivisor 回通道 0 目前的除數；程式沒設過就回 BIOS 的預設 65,536。
func (m *Machine) PITDivisor() uint32 {
	if m.pit.divisor == 0 {
		return PITDefaultDivisor
	}
	return m.pit.divisor
}

// PITProgrammed 回報程式有沒有自己設過通道 0。
//
// **要區分「量到 18.2 Hz」與「沒被設過所以是預設值」**——兩者的數字一樣，
// 但一個是結論、一個是「還沒發生」。
func (m *Machine) PITProgrammed() bool { return m.pit.writes > 0 }

// PITHz 回通道 0 目前的中斷頻率。
func (m *Machine) PITHz() float64 { return PITBaseHz / float64(m.PITDivisor()) }

// parityStepsPerSecond 是**對拍那條路**標定的機器速度，單位是指令／秒。
//
// ⚠ 它與 machine.StepsPerSecond()（`speaker.go`）不是同一個數：那一支
// 把 DefaultIRQ0Every 當成「分頻 65,536 時的間隔」（＝主線的定義，
// 見 recalcIRQ0 依 PITDefaultDivisor 縮放），算出來約 3.0 M 指令／秒；
// 這裡把它當成「rich2 那個 17,000 分頻下的間隔」，約 11.6 M。兩份標定
// 差 3.85 倍，是兩條分支各自的模型，**還沒有裁決哪一個對**。
//
// 它不是量出來的硬體規格，而是從對拍定住的 `DefaultIRQ0Every` 反推的：
// 那個常數是「rich2 的一個計時刻 ＝ 165,000 道指令」，而 rich2 把除數設成
// 17,000（`PITHz` ＝ 70.187 Hz），所以 165,000 × 70.187 ≈ 11.58 M 指令／秒
// ——落在 386DX-33／486SX 的量級，與這款遊戲的年代相符。
//
// 有了它，**別的除數也換算得出來**：程式把除數改成別的值時，一刻該有幾道
// 指令跟著變，而不是繼續沿用某一款遊戲的數字。
const parityStepsPerSecond = DefaultIRQ0Every * PITBaseHz / 17000

// PITStepsPerTick 回「除數 d 的一個計時刻等於幾道指令」。
//
// d ＝ 17,000（rich2）剛好回 `DefaultIRQ0Every`；除數愈小刻愈密，
// 指令數等比例變少。
func PITStepsPerTick(d uint32) uint64 {
	if d == 0 {
		d = PITDefaultDivisor
	}
	n := parityStepsPerSecond * float64(d) / PITBaseHz
	if n < 1 {
		return 1
	}
	return uint64(n + 0.5)
}

// PITStepsPerTick 回**目前**這個除數的一刻等於幾道指令。
//
// ⚠ 它是「程式要求的速度」，不是機器現在跑的速度——後者是 `IRQ0Every`。
// 兩者要一致得由呼叫端自己設（`m.IRQ0Every = m.PITStepsPerTick()`），
// **不自動跟隨**：既有的錄影與對拍都釘在固定的 `IRQ0Every` 上，
// 讓機器隨遊戲寫入而改速度會把它們全部作廢。
func (m *Machine) PITStepsPerTick() uint64 { return PITStepsPerTick(m.PITDivisor()) }

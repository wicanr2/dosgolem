package rich2

import (
	"math"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 路過銀行。
//
// ⚠ **是「路過」不是「停在」。** 銀行在逐格結算那條路徑上處理，
// 不在落地分派裡（`rich2/docs/re/031` §1）——dosgolem 實測落在銀行格
// 一張選單都不會開（`rich2/docs/re/186` §5）。要觸發它得**走過**那一格。
//
// 控制流與位址見 `rich2/docs/re/186`。
const (
	// IDABankSlider 是**金額滑桿**（`0CED:45FA`）。
	//
	// 三個參數 `(x, y, 上限)`，回傳 32 位金額在 `DX:AX`。
	// **上限就是來源欄的金額**——存款時是現金、領款時是存款。
	//
	// ⚠ 只有人類玩家會走到這裡（`ds:1034h == 2`）；
	// 電腦玩家走另一條分支（`docs/spec/006` R9：存一半、領六成）。
	IDABankSlider = 0x214CA

	// IDABankTitle 是滑桿上面那個標題框（`0CED:9941`，六個參數）。
	// 文字索引是 `334 + 選擇`——334「存上金額」、335「提出金額」。
	IDABankTitle = 0x26811
)

// 銀行用到的全域（`rich2/docs/re/186` §2）。
const (
	// VarBankChoice 是選單的結果：**0 存款、1 領款、2 放棄**。
	//
	// 它同時是 `11A2h` 的**欄索引**——所以「來源欄」與「選了什麼」
	// 是同一個數字，那不是巧合，是原版的寫法。
	VarBankChoice = 0x0874
	// VarBankAmount 是滑桿選出來的金額（32 位，低字在這裡）。
	VarBankAmount = 0x063A
	// VarHumanFlag 是「現在這位是人類玩家」的判準（值 2）。
	VarHumanFlag = 0x1034
)

// 選單的三個選項，值就是 `VarBankChoice`。
const (
	BankDeposit  = 0 // 存款：現金 → 存款
	BankWithdraw = 1 // 領款：存款 → 現金
	BankGiveUp   = 2 // 放棄：不動錢（`0x17889` 判 >= 2 直接返回）
)

// BankSlider 是一次金額滑桿的觀測。
type BankSlider struct {
	Step   uint64
	X, Y   int   // 滑桿畫在哪
	Max    int32 // **上限 ＝ 來源欄的金額**
	Choice int   // 進來時的 `ds:874h`
	Amount int32 // 選出來的金額（回傳之後才有）
	Done   bool

	Want int32 // 自動操作要的金額（Answer 回傳的）
	Pos0 int   // 換算出來的把手位移（0..100）
}

// BankLog 收集整場的銀行操作。
type BankLog struct {
	All []BankSlider
	cur *BankSlider

	answer func(*BankSlider) int32
	armed  bool // 這一支滑桿還沒送出 Enter
	laps   int  // 這一支滑桿跑過幾圈
}

// Open 回目前有沒有一個滑桿在等輸入。
func (l *BankLog) Open() *BankSlider {
	if l == nil {
		return nil
	}
	return l.cur
}

// WatchBank 掛上攔截。**要在 Run／Click 之前叫。**
//
// 出口攔的是 `0x17969`（`ds:63Ah = ds:AA6h`），那一刻金額已經寫好了。
// 攔滑桿的 `retf` 也可以，但那要知道它在哪；攔呼叫端的下一個動作更穩。
func WatchBank(o *oracle.Oracle) *BankLog {
	log := &BankLog{}

	o.OnCall(o.IDA(IDABankSlider), func(o *oracle.Oracle) {
		a := basic.CallArgs(o, 3)
		// 第三個參數是 32 位的上限，但 `CallArgs` 一個參數只讀一個 word。
		// 高字在指標 +2 的地方——**傳址的參數是指標，不是值**，
		// 所以要自己讀第二個 word。
		hi := o.Word(o.DS(o.StackWord(2) + 2))
		log.All = append(log.All, BankSlider{
			Step:   o.Steps(),
			X:      int(int16(a[0])),
			Y:      int(int16(a[1])),
			Max:    int32(uint32(hi)<<16 | uint32(a[2])),
			Choice: int(int16(o.Word(o.DS(VarBankChoice)))),
		})
		log.cur = &log.All[len(log.All)-1]
		log.armed, log.laps = false, 0
		if log.answer == nil {
			return
		}
		want := log.answer(log.cur)
		if want < 0 {
			return
		}
		pos := log.cur.Pos(want)
		x, y := log.cur.Point(pos)
		log.cur.Want, log.cur.Pos0 = want, pos
		// **只移滑鼠，不送鍵。** 迴圈每一圈都重讀滑鼠，把手會自己跟過來；
		// Enter 由 WatchSliderInput 在第二圈送（見 Answer 的註解）。
		o.MoveMouse(x, y)
		log.armed = true
	})

	o.OnCall(o.IDA(IDABankAmountStored), func(o *oracle.Oracle) {
		if log.cur == nil {
			return
		}
		log.cur.Amount = BankAmount(o)
		log.cur.Done = true
		log.cur = nil
	})

	return log
}

// IDABankAmountStored 是 `ds:63Ah` 寫好之後的第一個位址（`0x17977` 的 jmp）。
const IDABankAmountStored = 0x17977

// BankChoice 讀目前的選單結果。
func BankChoice(o *oracle.Oracle) int { return int(int16(o.Word(o.DS(VarBankChoice)))) }

// BankAmount 讀滑桿選出來的金額（32 位）。
func BankAmount(o *oracle.Oracle) int32 {
	lo := uint32(o.Word(o.DS(VarBankAmount)))
	hi := uint32(o.Word(o.DS(VarBankAmount + 2)))
	return int32(hi<<16 | lo)
}

// 滑桿的幾何與離開條件（`rich2/docs/re/186` §4.3、§4.6，都是 confirmed）。
//
// 滑桿本體 `0x2313F` 從三個參數算出這些：
//
//	[bp-1Ch] = x − 78                 ; 滑鼠 X 的下界
//	[bp-1Eh] = x − 73                 ; ★ 左端（把手位移 0 的地方）
//	[bp-20h] = x − 23                 ; 把手初值（左端 + 50）
//	[bp-18h] = y + 22                 ; 滑鼠 Y 的下界（**不含**）
//	[bp-1Ah] = y + 26
//
// 命中判定是四道（`0x2325E`–`0x23297`）：Y 要落在 `(y+22, y+43)` 之間、
// X 要 `>= x−78` 且 `<= 左端+140`；通過之後**把手直接等於滑鼠 X**，
// 再夾進 `左端 .. 左端+100`。
const (
	// SliderLeftOffset 是左端相對於傳進去的 x：左端 ＝ x − 73。
	SliderLeftOffset = -73
	// SliderTravel 是把手的行程（像素），與 `÷ 100.0` 的除數互相印證。
	SliderTravel = 100
	// SliderBandTop／SliderBandBottom 是滑鼠 Y 的有效範圍，相對於傳進去的 y。
	// 判斷式是嚴格不等式，所以有效的是 `y+23 .. y+42`。
	SliderBandTop    = 23
	SliderBandBottom = 42
)

// Point 回「要讓把手停在第 pos 格」該把滑鼠放在哪。
//
// pos 是 0..100 的位移，不是金額。y 取有效帶的正中間，
// 免得差一個像素就整個判定不成立。
func (s BankSlider) Point(pos int) (x, y int) {
	if pos < 0 {
		pos = 0
	}
	if pos > SliderTravel {
		pos = SliderTravel
	}
	return s.X + SliderLeftOffset + pos, s.Y + (SliderBandTop+SliderBandBottom)/2
}

// Pos 把金額換算成把手位移（0..100）。
//
// 原版的算式是**反過來的**：`金額 = 位移 × (上限 ÷ 100)`，而且
// `fistp` 是四捨五入到最近的整數。所以這裡取最接近的位移，
// 呼叫端要驗的是「原版算回來的金額」而不是「我想要的金額」——
// 上限不是 100 的倍數時兩者本來就會差一點。
func (s BankSlider) Pos(amount int32) int {
	if s.Max <= 0 {
		return 0
	}
	pos := int((float64(amount)*SliderTravel)/float64(s.Max) + 0.5)
	if pos < 0 {
		pos = 0
	}
	if pos > SliderTravel {
		pos = SliderTravel
	}
	return pos
}

// AmountAt 回「把手停在第 pos 格時原版會算出的金額」。
//
// 照抄原版的捨入點：級距先算成**單精度**（`0x23156` 的 `fstp DWORD`），
// 相乘之後才 `fistp` 成整數。用 float64 全程算會在上限大的時候差 1。
func (s BankSlider) AmountAt(pos int) int32 {
	stepF32 := float32(float64(s.Max) / 100.0)
	return int32(math.Round(float64(float32(pos) * stepF32)))
}

// Answer 掛上「滑桿一開就自動拉到某個金額再確認」。
//
// 回傳要的金額；回傳負數表示不理它（讓呼叫端自己處理）。
//
// **走的是正常玩家路徑**：把滑鼠移到對應的把手位置讓原版自己算金額，
// 再送 Enter——那正是原版自己的離開條件（`rich2/docs/re/186` §4.6）。
//
// ⚠ **Enter 不能在滑桿一開就送。** `ds:1094h` 只有 `0CED:6759` 會寫，
// 而迴圈是「讀鍵 → 移把手 → 算金額 → 比對 Enter」。開場那一次 `6759`
// （`0x23226`）會先把 Enter 吃掉，於是迴圈裡比對到的是舊值，
// **看起來像「送了 Enter 但原版不理」**。所以這裡等到迴圈跑過一圈
// （把手已經移到位、金額已經算好）才送。
func (l *BankLog) Answer(f func(*BankSlider) int32) {
	if l != nil {
		l.answer = f
	}
}

// WatchSliderInput 掛上滑桿的自動操作。**要和 WatchBank 一起用。**
//
// 分成兩個攔截點，理由見 Answer：
//
//	0x214CA  滑桿一開 → 算出把手位置，移滑鼠（WatchBank 做的）
//	0x23230  迴圈頭   → 跑過一圈之後送 Enter
func WatchSliderInput(o *oracle.Oracle, log *BankLog) {
	o.OnCall(o.IDA(IDASliderLoop), func(o *oracle.Oracle) {
		if log == nil || log.cur == nil || !log.armed {
			return
		}
		log.laps++
		// 第一圈把手才會移到位、金額才算得出來（`0x2333F`）。
		// 第二圈開頭送 Enter，這一圈的 `6759` 就讀得到。
		if log.laps < 2 {
			return
		}
		o.ClearInput()
		o.Type("\r")
		log.armed = false
	})
}

// IDASliderLoop 是滑桿主迴圈的頭（`0x23230`：`cmp [bp-38h], 0`）。
//
// `[bp-38h]` 是離開旗標，Enter 與 ESC 都把它設成 1。
const IDASliderLoop = 0x23230


package rich2

import (
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
}

// BankLog 收集整場的銀行操作。
type BankLog struct {
	All []BankSlider
	cur *BankSlider
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

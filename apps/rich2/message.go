package rich2

import (
	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/dosgolem/runtime/basic"
)

// 訊息框：全檔共用的對話／提示出口。
//
// 選單擋路的問題解掉之後（`selector.go`），落地格巡迴還是會停住，
// 而且那一次**不是選單擋著**——ESC 會處理選單。剩下的嫌疑是訊息框：
// 它只等一個按鍵、不走選擇器，所以 `WatchSelectors` 看不到它
// （`rich2/docs/playtest/054` §3.2）。
//
// `288CF` 是那個共用出口，**22 個呼叫端**遍佈主程式區與副程式區
// （`rich2/docs/re/115` §2，confirmed）：拍賣、黑市、賭場、事件提示
// 共用同一個框。所以攔它一次就看得到全部。
const (
	// IDAMessageBox 是訊息框的進入點（`0CED:B9FF` 的目標）。
	//
	// `288CC` 的 `jmp` 是 3 bytes，剛好結束在這裡；`288CF` 本身是
	// `mov cx, 46h`（BASIC 的框架配置，70 bytes 區域變數），
	// 與選擇器的 `mov cx, 16h` 同一個形狀。
	IDAMessageBox = 0x288CF

	// MessageArgs 是它的參數個數（`rich2/docs/re/115` 標 confirmed）。
	MessageArgs = 5
)

// Message 是一次訊息框的觀測。
//
// 五個參數 `rich2/docs/re/115` 標 confirmed，但那一份只把**第 1 個與第 5 個**
// 講清楚，中間三個沒有寫成表。所以這裡只命名那兩個，其餘原樣留在 `Args`
// ——硬取名字等於把推論當結論。
type Message struct {
	Step uint64
	Args [MessageArgs]int

	// Dialog 是對話框編號（第 1 參數），拿去查版面表 `14B0h`
	// （`docs/re/115` §2a）。**它就是「這是哪一個框」的識別碼。**
	Dialog int

	// SelfClose 是「這個框會不會自己關掉」（第 5 參數 ＝ 0）。
	//
	//	[bp+6] == 0 → 停一下（70）之後把整螢幕備份貼回去，框自己消失
	//	非 0        → 留著，由呼叫端負責善後
	//
	// （`docs/re/115` §2b，confirmed；法院那個呼叫端傳 0，拍賣傳變數。）
	//
	// **這正是「這個框會不會擋路」的判準**——自己會關的不必理它。
	SelfClose bool
}

// MessageLog 收集整場的訊息框。
type MessageLog struct {
	All     []Message
	dismiss string
}

// WatchMessages 掛上攔截。**要在 Run／Click 之前叫。**
func WatchMessages(o *oracle.Oracle) *MessageLog {
	log := &MessageLog{}
	o.OnCall(o.IDA(IDAMessageBox), func(o *oracle.Oracle) {
		a := basic.CallArgs(o, MessageArgs)
		m := Message{Step: o.Steps()}
		for i := range m.Args {
			m.Args[i] = int(int16(a[i]))
		}
		m.Dialog = m.Args[0]
		m.SelfClose = m.Args[MessageArgs-1] == 0
		log.All = append(log.All, m)
		if log.dismiss != "" {
			o.Type(log.dismiss)
		}
	})
	return log
}

// Dismiss 讓接下來每一個訊息框一開就自動送出關閉的按鍵。
//
// ⚠ **關閉它要按什麼還沒查。** 這裡不預設 Enter 也不預設 ESC——
// 呼叫端自己給，並且在測試裡記錄哪一種真的讓遊戲往下走。
// 猜一個然後看起來能動，會把「剛好也接受這個鍵」寫成結論。
func (l *MessageLog) Dismiss(keys string) { l.dismiss = keys }

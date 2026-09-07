// Package dos 是 DOS 與 BIOS 服務層：`int 21h`、`int 10h`、`int 33h`、`int 16h`。
//
// 規格在 `docs/spec/004-dos-bios-services.md`（§2／§3／§4 都是 READY）。
//
// 三條貫穿全部的原則（`docs/spec/004` §1），每一條都是用一整輪換來的：
//
//  1. **未實作的服務只宣告成功，不碰呼叫端沒要求改的暫存器。**
//     清 `AX` 會讓「設中斷向量」的迴圈把 `AH` 變成 0 ＝ 結束程式。
//  2. **有些服務的「失敗」才是正確答案。** `AH=4Ah` 是記憶體探測。
//  3. **沒建的東西不會抱怨，只會給 0，而 0 是合法值。**
package dos

import (
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// Time 是回給 `int 21h AH=2Ch` 的時刻。
//
// **固定值。** 時間會餵進亂數種子，浮動的話同一組按鍵每次跑到不同畫面
// （`rich2/CLAUDE.md` §8：截圖驗收要帶固定 seed）。
//
// **預設全 0，這是刻意與原版那邊的固定種子版對齊的。**
// `rich2/tools/patch_seed.py` 把 `TIMER` 內部的 `mov ah,2Ch / int 21h`
// 換成 `xor cx,cx / xor dx,dx`；我們這邊不改 binary，讓 `AH=2Ch` 直接
// 回 CX=DX=0，效果相同。兩邊的 `RANDOMIZE TIMER` 因此拿到同一個種子，
// 逐點對拍才有意義（`docs/spec/001` MVP-B）。
type Time struct{ Hour, Min, Sec, Hundredth uint8 }

// clock 是 `AH=2Ch` 回報的時刻：Now 當基準，再加上 PIT tick 推出來的時間。
//
// ⚠ **時鐘不能是固定值。** 第一版 `AH=2Ch` 直接回 `Now`，於是任何
// 「等 N 個百分之一秒」的迴圈都轉不出來——智冠《三國演義》的識別畫面
// 就卡在這裡，六百一十二萬次呼叫問同一個時刻。從外面看是「程式還活著」，
// 與「在算很久」分不開。
//
// 推進的依據是 **PIT tick 不是牆上的時間**：時鐘掛在指令數上，
// 同樣的輸入永遠得到同樣的時刻，對拍才是決定性的
// （與 `machine.tick` 的模型一致）。18.2 Hz ＝ 一個 tick 約 5.4925 個
// 百分之一秒。
func (d *DOS) clock() Time {
	base := (uint64(d.Now.Hour)*3600+uint64(d.Now.Min)*60+uint64(d.Now.Sec))*100 +
		uint64(d.Now.Hundredth)
	// 5493/1000 ≈ 100/18.2，用整數算避免浮點進到決定性的路徑上。
	total := (base + d.M.Ticks*5493/1000) % (24 * 3600 * 100)
	return Time{
		Hour:      uint8(total / 360000),
		Min:       uint8(total / 6000 % 60),
		Sec:       uint8(total / 100 % 60),
		Hundredth: uint8(total % 100),
	}
}

// Mouse 是滑鼠狀態。座標用**像素**存，回報時才乘上 XScale
// （mode 13h 的標準驅動水平回報 0–639，`rich2/docs/re/182` §3）。
type Mouse struct {
	X, Y    uint16
	Buttons uint16
	// Press／Release 是 AX=5／AX=6 的統計，**逐鍵分開**（0 ＝ 左、1 ＝ 右、
	// 2 ＝ 中），讀走就歸零。
	//
	// ⚠ **`AX=5` 的 `BX` 是輸入（問哪一個鍵），不只是輸出。**
	// 不分鍵的話「按左鍵」會被問右鍵的那一次先領走——而《臥龍傳》的等待迴圈
	// （IDA `0x121E9`）**先問右鍵**，於是每一次左鍵都被讀成右鍵，
	// 畫面上看起來像「點了沒反應」或「點什麼都是取消」。
	Press, Release [3]uint16
	// PressAt／ReleaseAt 是該鍵最後一次按下／放開的座標。
	// `AX=5`／`AX=6` 回的是**那一刻**的位置，不是現在的位置。
	PressAt, ReleaseAt [3][2]uint16
	// XScale 是水平的虛擬座標倍率。0 表示依視訊模式自動決定
	// （320 寬 → 2、640 寬 → 1），這是預設；設非 0 就強制用那個值。
	XScale uint16
	// Polls 記下每一次 AX=3 回報出去的東西。**這是分辨「輸入沒送到」與
	// 「送到了但答錯」的唯一辦法**——兩者的畫面表現一模一樣。
	Polls []Poll

	// Sets 記下每一次 AX=4（程式自己設游標位置）。
	// 程式設過之後我們注入的位置就被蓋掉了，而症狀是「滑鼠移不動」。
	Sets []Poll

	// Calls 是每個 int 33h 功能號被叫了幾次。
	// **診斷「點了沒反應」的第一步**：先確認遊戲到底在讀哪一支。
	Calls map[uint16]int

	// 座標範圍（`AX=7`／`AX=8` 設的，**虛擬座標**）。
	// MaxX／MaxY 為 0 表示遊戲還沒設過，那就不夾。
	//
	// ⚠ **夾制是遊戲行為的一部分**：《臥龍傳》開機把範圍設成
	// 0–27Fh × 0–18Fh，真機上送畫面外的座標會被夾回邊界。
	// 不夾的話「點在畫面外」這種邊界測試會得到相反的結論。
	//
	// **收下就好會漏掉資訊**：程式用範圍宣告它期待的座標系，
	// 兩邊對不上時游標與命中判定會整個偏移，而畫面看起來完全正常。
	MinX, MaxX, MinY, MaxY uint16

	// PressQ 是每個按鍵被查詢的次數（不論回報 0 或非零）——
	// 用來分辨「遊戲查的是別顆鍵」與「遊戲沒查」。
	PressQ [3]uint64

	// PressReads 記下每一次「AX=5／6 真的回報了非零次數」——**送不進去與
	// 遊戲不理會是兩件事**，沒有這個清單就分不開。
	PressReads []PressRead

	// Handler 是 `AX=000Ch` 登記的事件處理常式（`docs/spec/009`）。
	//
	// **這一支要真的呼叫。**《臥龍傳》靠它維持畫面上那隻手：一次四千萬道
	// 指令的跑分裡 `AX=3` 只有 5 次，而 `AX=5` 有 240 萬次——
	// 游標幾乎全靠事件常式重畫。
	Handler struct {
		Seg, Off, Mask uint16
		Set            bool
	}

	// Events 是實際送出去的回呼（Buttons 欄位放事件旗標）。
	// **0 次與「遊戲不看事件」長得一樣**，所以要留清單不是只留計數。
	Events []Poll
}

// PressRead 是一次回報出去的按鍵統計。
type PressRead struct {
	Fn     uint16 // 5 ＝ 按下、6 ＝ 放開
	Button int
	Count  uint16
	X, Y   uint16
	Step   uint64
	CS, IP uint16 // 呼叫端
}

// Poll 是一次 `AX=3` 的回報內容。
type Poll struct {
	X, Y, Buttons uint16
	Step          uint64
}

// DOS 是服務層。用 Install 掛到機器上。
type DOS struct {
	M *machine.Machine

	// Root 是原版素材的目錄（玩家自備）。**本專案不含任何原版檔案。**
	Root string

	// Now 是固定時刻，Mouse 是滑鼠狀態，Font 是 DOS/V 字型服務。
	Now   Time
	Mouse Mouse
	Font  Font

	// Sound 記下 `int 61h`（音源 TSR）每個 command 被叫了幾次。
	// **這一輪不模擬音源**（`docs/spec/008` §6），只留下「走到了沒」。
	Sound map[uint8]int

	// Console 收 `AH=02h`／`06h`／`09h` 與 `int 10h AH=0Eh` 印出來的字。
	// **錯誤訊息走這條**，收不到就等於什麼都不知道。
	Console []byte

	// Calls 是每一種 (中斷, AH) 的呼叫次數；nil 表示不記。
	Calls map[Call]int

	// Stdin 是 `AH=3Fh` 讀 handle 0 時要餵的位元組；空的時候餵 StdinFill。
	//
	// ⚠ **不能回「讀到 0 個」**（等同 EOF）：主程式會當成輸入結束、
	// 還原中斷向量然後 exit（`rich2/docs/re/005`「死點修掉了」）。
	Stdin     []byte
	StdinFill uint8

	// StdinEmptyReadsZero 讓「佇列空」回報**讀到 0 個位元組**，
	// 而不是餵一個 StdinFill。
	//
	// **為什麼需要它**：編譯後 BASIC 的 `INKEY$` 空轉是
	// `while INKEY$ = "" : wend`（`rich2/docs/re/153` 的 `0x2CB42`：
	// `while ds:1094h == ds:2232h`，後者是空字串）。餵一個 `00` 會讓
	// `INKEY$` 拿到 `CHR$(0)`——**非空字串**，於是空轉立刻結束。
	// 症狀是**所有「按任意鍵繼續」的畫面一閃而過**：畫出來了、
	// 也真的畫對了，但幾十萬道指令之後就被下一次重繪蓋掉，
	// 而外面用粗取樣根本看不到它存在過。
	//
	// ⚠ **預設 false，不要全域打開。** 主程式在別的地方會把「讀到 0 個」
	// 當成 EOF，然後還原中斷向量並結束（那正是 StdinFill 存在的理由）。
	// 這個開關是給「我現在就是要讓某張畫面停住」用的，
	// 用完要關掉。
	StdinEmptyReadsZero bool

	// KeyWaits 數「佇列空的時候被要求讀一個鍵」發生了幾次
	// （`AH=01h`／`07h`／`08h`）。
	//
	// **為什麼要數它**：真 DOS 的這幾個功能是**阻塞**的——呼叫一次就停在
	// 那裡等人按鍵。dosgolem 是步進式的，停不下來，只能立刻返回；
	// 於是程式在同一個迴圈裡空轉，從外面看是「跑滿指令上限、程式還活著」，
	// 與「跑掛了」「在算很久」長得一模一樣。
	//
	// 數出來就分得開了：KeyWaits 很大 ＝ **它在等鍵盤，不是在做事**，
	// 該餵鍵而不是加大 `-steps`。
	KeyWaits int

	// NonBlockingKeys 關掉阻塞語意，讓 `AH=01h`／`07h`／`08h` 在佇列空時
	// 回 `AL=0` 繼續跑（舊行為）。
	//
	// **零值 ＝ false ＝ 阻塞 ＝ 真 DOS 的語意**：忘記設的人得到的是對的那一邊，
	// 不是一個安靜說謊的模型。規格 `docs/spec/008`。
	NonBlockingKeys bool

	// Blocked 表示這一步停在阻塞式輸入上（佇列空）。取到鍵時清掉。
	// 上層可以據此停下來、餵鍵、再繼續。
	Blocked bool

	// Drive 是 `AH=19h` 的目前磁碟（0 ＝ A:、1 ＝ B:、**2 ＝ C:**），
	// Dir 是 `AH=47h` 的目前目錄。
	//
	// ⚠ **預設一定要是 C:（硬碟）。** 回 A: 的話程式判定自己是從磁片跑，
	// 停在「Please put Disk#2 in A: and put Disk#3 in B:」等按鍵——
	// 而在接上 `AH=40h` 之前，那個畫面在主控台上是**一片空白**。
	Drive uint8
	Dir   string

	// Unimplemented 記下每一個沒實作的功能號被叫了幾次。
	//
	// **「宣告成功」本身也會說謊**：該填的緩衝區沒填就是垃圾，症狀出現在
	// 很後面而且完全不指向這裡。所以要數、要印（`docs/spec/004` §1.3）。
	Unimplemented map[Call]int

	// Opened 是開過的檔（依序），Missing 是找不到的。兩份都是診斷用。
	Opened  []string
	Missing []string

	// OnOpen 在每一次成功開檔之後叫一次（參數是檔名，不含路徑）。
	//
	// **這是「把一段執行期產物框到某個檔上」唯一的錨。** 例：音樂對拍要
	// 「只有這一首的暫存器串」——開機會連放好幾首而且是接起來的，
	// 事後從序列裡找分界只能用猜的。掛在開檔那一刻就不必猜。
	OnOpen func(name string)

	// Reads 是每一次成功的讀檔（`AH=3Fh`），依序。
	//
	// **「這個檔被讀進哪裡、讀了多長」是解資料格式的起點**，也是
	// 「緩衝區有沒有蓋到別的東西」唯一直接的證據——DOS 服務的寫入走
	// WriteBytes，繞過 Write8，WatchWrites 看不到它。
	Reads []ReadOp

	// Allocs 是每一次記憶體配置（`AH=48h`）與縮放（`AH=4Ah`）。
	// 「這塊緩衝區是誰給的」——配置器給錯位址時，症狀會出現在
	// 很遠的地方（緩衝區蓋到堆疊、蓋到別人的資料）。
	Allocs []AllocOp

	// EMSOps 是每一次 EMS 的配置與映射（`docs/spec/014`）。
	// 「這一頁什麼時候被映到哪」——資料放 EMS 的程式，內容不對的時候
	// 只能從映射序列回推。
	EMSOps []EMSOp

	// XMSMoves 是每一次 `AH=0Bh`（move EMB）。字型從 XMS 取的程式，
	// 「取錯位移」的症狀是**某些字畫不出來**，其餘完全正常。
	XMSMoves []XMSMove

	// Wrote 記下「程式想寫檔」的每一次。**我們不寫**（原版素材唯讀），
	// 但安靜地報成功會讓「存檔壞掉」查不出來。
	Wrote []Write

	// VecSets 記下每一次 `AH=25h` 設中斷向量。
	// 除錯「中斷跳進垃圾」的第一手：向量是誰在什麼時候設成那個值的。
	VecSets []VecSet

	// FileOps 記下每一次檔案讀取與定位（AH=3Fh／AH=42h）的檔名與偏移。
	//
	// 回答「這個檔案是整份載入還是按需取用」——那決定了要去記憶體找資料，
	// 還是去看它讀了哪些偏移。字型就是後者：整份 GRAPH.IMG 從來沒有進過
	// 記憶體，遊戲每畫一個字就 seek 過去讀 30 bytes
	// （`~/cht/logh3/docs/re/07`）。
	FileOps []FileOp

	// PalOps 記下每一次 int 10h AH=10h 的子功能與暫存器。
	// 診斷「圖形對了但顏色全錯」用：這一支的 AL 分支語意各不相同，
	// 接錯的話不會報錯，只會把別的東西寫進 DAC。
	PalOps []PalOp

	// MemOps 記下每一次 AH=48h／49h／4Ah 記憶體操作的輸入與結果。
	// 除錯「模組載到沒人配置過的位址」用：先確定配置器到底發過哪些段。
	MemOps []MemOp

	// Exited 為真表示程式呼叫了 `AH=4Ch`／`AH=00h`；ExitCode 是它的回傳碼。
	Exited   bool
	ExitCode uint8

	// ExecLog 是每一次 EXEC／監督載入的紀錄（`docs/spec/009`）。
	// **「殼鏈走到哪一跳」唯一的直接答案。**
	ExecLog []ExecRecord

	// Overlays 記每一次成功的 overlay 載入。
	//
	// **這是「程式載了哪些模組」的唯一觀測點。** overlay 沒有 PSP、
	// 不動 CS:IP，從暫存器與開檔清單都看不出它發生過。
	Overlays []OverlayLoad

	// Resizes 記每一次 AH=4Ah，用來查「可配置區起點是怎麼被決定的」。
	Resizes []ResizeCall

	// CallTrace 記每一次 int 21h 進入與離開時的 ES:BX。
	//
	// 用途只有一個：分辨「程式自己設的 ES:BX」與「某個服務把它改壞了」。
	// 服務回傳時動到不該動的暫存器，症狀會出現在很後面的另一個呼叫上。
	CallTrace []CallRec

	// MemTrace 記每一次配置／釋放。非 nil 才記（筆數可達上萬）。
	MemTrace []MemCall

	// KeyReads 記每一次按鍵被取走。永遠記——筆數是按鍵數，很少。
	KeyReads []KeyRead

	handles map[uint16]*handle
	// MaxHandles 是這個程序同時開得了幾個檔（含 0–4 的標準 handle）。
	// DOS 的預設是 20，`AH=67h` 可以調高。
	//
	// ⚠ **上限不是拿來擋人的，是號碼配置的邊界。** MSC 的低階 I/O
	// 用 handle 當索引查自己的表，表和 JFT 一樣大；handle 超出範圍時
	// `fopen` 會**先開成功再把它關掉並回 NULL**，看起來像開檔失敗，
	// 實際上是號碼太大。
	MaxHandles uint16
	freeSeg    uint16

	// 行程模型（`docs/spec/008` §2、`docs/spec/009` §2）：
	// procStack 是被 EXEC 暫停的父行程，curPSP 是目前行程的 PSP，
	// lastExit 給 AH=4Dh，queue 是監督佇列。
	procStack []procFrame
	curPSP    uint16
	lastExit  uint16
	queue     []Queued

	// XMS（`docs/spec/011`）：EMB 的內容放 Go 端。
	emb     map[uint16][]byte
	nextEMB uint16

	// EMS（`docs/spec/014`）：邏輯頁的內容放 Go 端，page frame 在
	// 1 MB 空間裡的 D000h 段。
	ems *ems

	// arena 是 [freeSeg, MemTop) 這段的區塊表，依段位址排序、首尾相接、
	// 沒有空隙。每個區塊佔 1 段的假 MCB ＋ size 段的資料，
	// 交給程式的是 seg+1。規格 `docs/spec/009`。
	//
	// **它取代了早期的「bump ＋ 洞清單」**：那一版不合併相鄰的洞，
	// 而且不把狀態發布到客體記憶體，會走 MCB 鏈的程式因此看到一份
	// 與事實無關的地圖。
	//
	// nil 表示還沒初始化；第一次配置時用當時的 freeSeg 建起來，
	// 這樣測試裡先設 freeSeg 再用的寫法仍然成立。
	arena []memBlock
}

// AllocOp 是一次記憶體配置或縮放。Fn 是 48h、49h 或 4Ah。
type AllocOp struct {
	Step uint64
	Fn   uint8
	Want uint16 // 要幾段
	Seg  uint16 // 48h：配到的段；4Ah：被縮放的區塊
	OK   bool
}

// EMSOp 是一次 EMS 操作（`AH=43h` 配置／`44h` 映射／`45h` 釋放）。
type EMSOp struct {
	Step    uint64
	Fn      uint8
	Handle  uint16
	Logical uint16 // 44h：邏輯頁（FFFFh ＝ 解除映射）
	Phys    uint8  // 44h：實體頁 0–3
	Pages   int    // 43h：配了幾頁
	Status  uint8  // 回傳的 AH
}

// XMSMove 是一次 XMS 的 move（`AH=0Bh`）。handle 0 表示常規記憶體，
// 此時位移欄是 far 指標。
type XMSMove struct {
	Step           uint64
	Len            uint32
	SrcH, DstH     uint16
	SrcOff, DstOff uint32
	// Bits 是搬過去的資料裡有幾個 1。0 表示搬了一片空白。
	Bits int
}

// ReadOp 是一次讀檔（`AH=3Fh`）。Seg:Off 是緩衝區。
type ReadOp struct {
	Step     uint64
	Name     string
	Handle   uint16
	Seg, Off uint16
	Want     uint16
	Got      int
}

// Write 是一次被擋下來的寫檔。
type Write struct {
	Name string
	N    int
}

// VecSet 是一次 `AH=25h` 設中斷向量。
type VecSet struct {
	Int      uint8
	Seg, Off uint16
	Step     uint64
}

// FileOp 是一次檔案操作。
//
// **開檔清單只說「開過什麼」，說不出「要求讀哪一段、拿到多少」。**
// 遊戲抱怨某個項目找不到時，要分辨「它算錯位移」與「我們回錯資料」
// 就得看這個。
//
// 也回答「這個檔案是整份載入還是按需取用」——那決定了要去記憶體找資料，
// 還是去看它讀了哪些偏移。字型就是後者：整份 GRAPH.IMG 從來沒有進過
// 記憶體，遊戲每畫一個字就 seek 過去讀 30 bytes。
type FileOp struct {
	Step   uint64
	Op     string // open／seek／read／write／close
	Fn     uint8  // 對應的 int 21h AH（0 ＝ 不適用）
	Handle uint16
	Name   string
	Arg    int64 // 呼叫端要求的量：seek 的位移、read 的 CX
	Pos    int64 // seek：定位後的位置；read：讀取起點
	Len    int   // read／write：實際的位元組數；<0 是錯誤碼
}

// PalOp 是一次 int 10h AH=10h 呼叫。
type PalOp struct {
	AL             uint8
	BX, CX, DX, ES uint16
	Step           uint64
}

// MemOp 是一次記憶體配置操作（AH=48h/49h/4Ah）的記錄。
type MemOp struct {
	Fn     uint8
	BX, ES uint16 // 輸入：段落數／區塊段
	AX     uint16 // 結果：配置到的段或錯誤碼
	Step   uint64
	OK     bool
}

// Call 是一次沒實作的服務呼叫：哪一個中斷、AH、AL。
type Call struct {
	Int, AH, AL uint8
}

func (c Call) String() string {
	return fmt.Sprintf("int %02Xh AH=%02X AL=%02X", c.Int, c.AH, c.AL)
}

// New 造一個服務層。root 是原版素材目錄。
func New(m *machine.Machine, root string) *DOS {
	return &DOS{
		M: m, Root: root,
		Now:           Time{}, // 全 0：與原版的固定種子版對齊，見 Time 的說明
		Mouse:         Mouse{Calls: map[uint16]int{}},
		Font:          DefaultFont(),
		Sound:         map[uint8]int{},
		Drive:         2, // C:，見 Drive 欄位的說明
		Dir:           "RICH2",
		Unimplemented: map[Call]int{},
		handles:       map[uint16]*handle{},
		MaxHandles:    20, // DOS 預設；AH=67h 可調
		ems:           newEMS(),
	}
}

// Install 把服務層掛到機器的中斷鉤子上。**要在 LoadEXE 之後叫**——
// 它會記下映像後面的第一個可配置段。
func (d *DOS) Install() {
	d.freeSeg = d.M.FreeSeg
	d.arena = nil // 第一次配置時用當時的 freeSeg 建起來
	d.curPSP = machine.PSPSeg
	d.installFont()
	d.M.CPU.IntHook = d.handle
}

// handle 是中斷分派。回 true 表示「處理完了」，CPU 不走向量表。
//
// ⚠ **只在向量還指著我們的 stub 時才接手。** 程式會用 `AH=25h` 裝自己的
// 處理常式——最重要的是 `INT 34h`–`3Dh`（binary 自帶的 Microsoft 浮點
// 模擬器，全檔 876 個呼叫）。那些一定要讓 CPU 真的跳過去，
// 攔下來的話**所有浮點運算都會落空**，而 BASIC 的金錢運算全靠它。
//
// 這條規則也順便處理了計時器（`int 08h`／`1Ch`）與 Ctrl-Break（`int 23h`）：
// 程式裝了誰就跑誰的，不必逐個列白名單。
func (d *DOS) handle(c *cpu.CPU, n uint8) bool {
	if seg := d.M.Read16(uint32(n)*4 + 2); seg != machine.StubSeg {
		return false // 程式自己裝了處理常式
	}
	// Calls 記每一種 (中斷, AH) 呼叫過幾次。**只記不改行為**——
	// 「它到底在做什麼」在沒有畫面可看的時候只剩這個問得到。
	if d.Calls != nil {
		d.Calls[Call{Int: n, AH: uint8(c.R[cpu.AX] >> 8)}]++
	}
	switch n {
	case 0xF2: // int 21h 的 trampoline（`docs/spec/004` §2.1）：TSR chain 進來的
		d.int21(c)
		d.fixStackedCF(c)
	case 0xF3:
		d.int10(c)
		d.fixStackedCF(c)
	case 0xF4:
		d.int15(c)
		d.fixStackedCF(c)
	case 0xF5: // XMS driver entry 的 trampoline（`docs/spec/011`）
		d.xmsCall(c)
		d.fixStackedCF(c)
	case 0xF6: // EMS 的 trampoline（`docs/spec/014`）。**EMS 不用 CF**，
		// 狀態在 AH，所以不呼叫 fixStackedCF。
		d.emsCall(c)
	case 0x2F: // XMS 偵測（AH=43h）
		d.int2F(c)
	case 0x08, 0x1C:
		// 計時器中斷鏈上的空 stub。向量還在 StubSeg ＝ 沒人裝，
		// 它的語意就是 IRET——**不記一筆**，否則每次 tick 都會
		// 洗出一筆假的「未實作」（BIOS int 08h stub 每 tick 都
		// 轉呼 int 1Ch，見 machine.initVectors）。
	case 0x21:
		d.int21(c)
	case 0x10:
		d.int10(c)
	case 0x33:
		d.int33(c)
	case 0x16:
		d.int16(c)
	case 0x11:
		// 取設備清單：就是 BDA 那一格（`docs/spec/003` §1）。
		// 不實作的話 AX 保持呼叫端傳進來的值，而顯示卡欄位是垃圾。
		c.R[cpu.AX] = d.M.Read16(0x0040*16 + 0x10)
	case 0x12:
		// 取常規記憶體大小，單位 KB。
		c.R[cpu.AX] = d.M.Read16(0x0040*16 + 0x13)
	case 0x13:
		d.int13(c)
	case 0x15:
		d.int15(c)
	case 0x1A:
		d.int1A(c)
	case 0x61:
		d.int61(c)
	case machine.IntCallbackReturn:
		// 回呼跑完了，把整份 CPU 狀態還原（`docs/spec/009` §3.1）。
		if !d.M.FinishCallback() {
			// **不要吞掉。** 沒有回呼在跑卻收到哨兵，表示有人踩到
			// `StubSeg:CallbackRetOff`，那是個 bug 不是雜訊。
			d.note(machine.IntCallbackReturn, 0, 0)
		}
	case intFontFull:
		d.fontGlyph(c, true)
	case intFontHalf:
		d.fontGlyph(c, false)
	case 0x20:
		d.exit(c, 0)
	case 0x67:
		d.int67(c)
	default:
		d.note(n, uint8(c.R[cpu.AX]>>8), uint8(c.R[cpu.AX]))
		clearCarry(c)
	}
	return true
}

func (d *DOS) exit(c *cpu.CPU, code uint8) {
	// 行程疊非空時是子程式結束：回傳碼記下來、控制權還父程式，不停機
	// （`docs/spec/007` §2／`docs/spec/009` §2）。
	d.terminate(c, code, false, 0)
}

// note 記一筆沒實作的呼叫。
func (d *DOS) note(intNo, ah, al uint8) {
	d.Unimplemented[Call{Int: intNo, AH: ah, AL: al}]++
}

// fixStackedCF 把服務結果的 CF 寫進**堆疊上的旗標框**。
//
// ⚠ 走 trampoline（`CD Fx / CF`）進來的時候，服務結束後 CPU 會 IRET——
// 旗標從堆疊框彈回來，我們對 `c.Flags` 的修改整個被蓋掉。
// 症狀是「TSR 落腳之後 EXEC 一律回 CF」：殼因此印
// 「FMDRV.COM : cannot execute.」然後帶著 65h 離開——服務本身做對了，
// 只有旗標到不了（源平合戰，`docs/spec/004` §2.1）。
func (d *DOS) fixStackedCF(c *cpu.CPU) {
	at := cpu.Addr(c.Seg[cpu.SS], c.R[cpu.SP]+4)
	w := d.M.Read16(at)
	if c.Flags&cpu.CF != 0 {
		w |= cpu.CF
	} else {
		w &^= cpu.CF
	}
	d.M.Write16(at, w)
}

// UnimplementedReport 把統計排成可讀的清單，次數多的在前面。
//
// **收工前一定要看它。** 「跑得動」與「跑得動但行為不對」的差別就在這裡。
func (d *DOS) UnimplementedReport() []string {
	type row struct {
		c Call
		n int
	}
	rows := make([]row, 0, len(d.Unimplemented))
	for c, n := range d.Unimplemented {
		rows = append(rows, row{c, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].c.String() < rows[j].c.String()
	})
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%s ×%d", r.c, r.n))
	}
	return out
}

// Close 關掉所有還開著的檔。
func (d *DOS) Close() {
	for _, h := range d.handles {
		if h.f != nil {
			h.f.Close()
		}
	}
	d.handles = map[uint16]*handle{}
}

// ---- 旗標與暫存器的半邊 --------------------------------------------------

func setCarry(c *cpu.CPU)   { c.SetFlags(c.Flags | cpu.CF) }
func clearCarry(c *cpu.CPU) { c.SetFlags(c.Flags &^ cpu.CF) }

// setAL／setAH 只動 AX 的一半，不碰另一半——原則 1。
func setAL(c *cpu.CPU, v uint8) { c.R[cpu.AX] = c.R[cpu.AX]&0xFF00 | uint16(v) }
func setAH(c *cpu.CPU, v uint8) { c.R[cpu.AX] = c.R[cpu.AX]&0x00FF | uint16(v)<<8 }

func setBL(c *cpu.CPU, v uint8) { c.R[cpu.BX] = c.R[cpu.BX]&0xFF00 | uint16(v) }
func setBH(c *cpu.CPU, v uint8) { c.R[cpu.BX] = c.R[cpu.BX]&0x00FF | uint16(v)<<8 }

func ah(c *cpu.CPU) uint8 { return uint8(c.R[cpu.AX] >> 8) }
func al(c *cpu.CPU) uint8 { return uint8(c.R[cpu.AX]) }
func bl(c *cpu.CPU) uint8 { return uint8(c.R[cpu.BX]) }
func bh(c *cpu.CPU) uint8 { return uint8(c.R[cpu.BX] >> 8) }

// ArenaDump 把配置器目前的區塊表印成一行一塊，供診斷用。
func (d *DOS) ArenaDump() []string {
	out := make([]string, 0, len(d.arena)+1)
	out = append(out, fmt.Sprintf("freeSeg=%04X arena=%d 塊（nil=%v）",
		d.freeSeg, len(d.arena), d.arena == nil))
	for _, b := range d.arena {
		st := "已配置"
		if b.free {
			st = "自由"
		}
		out = append(out, fmt.Sprintf("  seg=%04X size=%04X %s", b.seg, b.size, st))
	}
	return out
}

// memBlock 是配置器的一個區塊。seg 是假 MCB 的位置，資料從 seg+1 開始。
type memBlock struct {
	seg  uint16
	size uint16 // 資料段數，不含 MCB
	free bool
}

// OverlayLoad 是一次 `AH=4Bh AL=03` 的載入紀錄。
type OverlayLoad struct {
	Name  string
	Seg   uint16
	Reloc uint16
	Size  int

	// PBSeg/PBOff 是參數區塊的位置，PBRaw 是它前 8 個 byte。
	// 載入段看起來不合理時，要能分辨「程式真的這樣要求」與
	// 「我們讀錯了地方」——沒有這個就只能猜。
	PBSeg, PBOff uint16
	PBRaw        [8]byte

	// CallCS/CallIP 是呼叫端（INT 之後的下一道指令）。
	// 參數看起來不合理時要能直接跳去反組譯那裡。
	CallCS, CallIP uint16

	// Steps 是載入發生在第幾道指令。要看「載完之後發生什麼」就靠它
	// 把 -steps 停在正確的位置。
	Steps uint64

	// CallSite 是呼叫端 INT 指令前後的位元組（前 24、後 8）。
	//
	// 呼叫端常常是執行期搬到高位段的 stub，**檔案裡找不到**，
	// 事後也可能被覆蓋。要看它就得在呼叫發生的當下抄。
	CallSite [32]byte
}

// MemCall 是一次配置器呼叫（`AH=48h` 配置／`AH=49h` 釋放）的逐筆帳。
//
// 存在的理由是**總量對不對答不了「哪一次開始偏離」**。原版跑到
// `DATA5.GRP` 那一層時要 `BX=FFFF`（探測上限），拿到 122 KB 就收工；
// 當下它自己握著約 373 KB 而整趟一次都沒釋放。要判斷是「真 DOS 底下
// 它也拿這麼多」還是「我們給多了」，只能一筆一筆比對要求與回應。
type MemCall struct {
	Step   uint64
	Op     uint8  // 0x48 配置、0x49 釋放
	Want   uint16 // 要幾段（0x49 時無意義）
	Seg    uint16 // 成功時給出去的段；0x49 時是 ES
	Got    uint16 // 失敗時回報的最大自由段數
	OK     bool
	CS, IP uint16 // 呼叫端
	DS, ES uint16
}

// ResizeCall 是一次 `AH=4Ah` 的紀錄。
//
// Before／After 是這個區塊在 arena 裡調整前後的段數（`InArena` 為假時無意義）。
// **要求的大小不等於區塊最後的大小**：程式可以一路把同一塊撐大，
// 而只看 `AH=48h` 的要求量會漏掉這一段成長。
type ResizeCall struct {
	Seg, Want, FreeSeg uint16
	Before, After      uint16
	InArena, OK        bool
	CS, IP             uint16
}

// KeyRead 是一次「把按鍵從佇列取走」的紀錄。
//
// 存在的理由是**餵進去的鍵不見了的時候，看不出是誰吃的**。
// 同一個佇列有三條出口（`int 21h AH=01/07/08`、`AH=3Fh` 讀 handle 0、
// `int 16h AH=00/10`），而「程式沒反應」既可能是沒收到、
// 也可能是被另一條路提前取走。
type KeyRead struct {
	Step uint64
	Via  string // "int21-AH08"／"int21-3F"／"int16-AH00"
	Key  uint8
}

// CallRec 是一次 int 21h 的暫存器快照。
type CallRec struct {
	Step         uint64
	AH, AL       uint8
	ESIn, BXIn   uint16
	ESOut, BXOut uint16
}

// int61 是松崗 DOS/V 版的音源 TSR（`YNSOUND.COM`）。
//
// **只記錄不模擬**（`docs/spec/008` §6）。對拍比的是畫面，
// 音訊 parity 在臥龍傳專案那邊用錄音比過。
func (d *DOS) int61(c *cpu.CPU) {
	d.Sound[ah(c)]++
	// AH=0Ch 是「登記時鐘回呼」：`DS:DX` 是一支 `retf` 結尾的常式，
	// `AL=1` 表示取消（臥龍傳專案 `docs/re/61` §2）。
	//
	// ⭐ **這一支不接的話遊戲時鐘不會走**，而畫面完全正常——
	// 日期永遠停在第一天，兩層節流的等待迴圈也永遠等不到。
	// 驅動把 PIT 設成 4660.9 Hz、分頻 16 之後回呼，＝ 291.30 Hz，
	// 剛好是 BIOS tick（18.206 Hz）的 16 倍。
	if ah(c) == 0x0C {
		if al(c) == 1 {
			d.M.ClearPeriodicFarCall()
		} else {
			every := d.M.IRQ0Every / soundTickDivisor
			if every == 0 {
				every = 1
			}
			d.M.SetPeriodicFarCall(c.Seg[cpu.DS], c.R[cpu.DX], every)
		}
		clearCarry(c)
		return
	}
	// AH=0Ah 回旗標。回 0 ＝ 沒有任何旗標，是安全的預設；
	// **但它要留在未實作清單裡**，不要安靜地變成「有旗標」。
	if ah(c) == 0x0A {
		d.note(0x61, 0x0A, al(c))
		setAL(c, 0)
	}
	clearCarry(c)
}

// soundTickDivisor 是「音效驅動的回呼比 BIOS tick 快幾倍」。
//
// 291.30 ÷ 18.206 ＝ 16，而那正是驅動裡 `cs:0B6Ah` 的分頻值——
// **兩個獨立來源給同一個 16**。
const soundTickDivisor = 16

func bcd(v uint8) uint8 { return v/10<<4 | v%10 }

func setCH(c *cpu.CPU, v uint8) { c.R[cpu.CX] = c.R[cpu.CX]&0x00FF | uint16(v)<<8 }
func setCL(c *cpu.CPU, v uint8) { c.R[cpu.CX] = c.R[cpu.CX]&0xFF00 | uint16(v) }
func setDH(c *cpu.CPU, v uint8) { c.R[cpu.DX] = c.R[cpu.DX]&0x00FF | uint16(v)<<8 }
func setDL(c *cpu.CPU, v uint8) { c.R[cpu.DX] = c.R[cpu.DX]&0xFF00 | uint16(v) }

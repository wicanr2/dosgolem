package phantasie

// 共用型別（docs/spec/001 §3.3、§5；003 §5）。本檔是各檔案之間的契約，實作的人不要在別處重複定義。

// Kind 是指標所屬的種類（003 §5.1）。預設拒絕：不在表內的指標一律 KindOther，不查任何 catalog。
type Kind uint8

const (
	KindOther   Kind = iota // 玩家與名冊記錄、位置描述緩衝區、未知：不查表
	KindStatic              // 靜態字串區與目前載入的 overlay 資料區：查 ui
	KindBuffer              // 堆疊與行緩衝區 DS:638E 起：查 prose
	KindMonster             // 怪物記錄（%s 引數）：查 ui
	KindTown                // 城鎮名（%s 引數）：查 ui
)

func (k Kind) String() string {
	switch k {
	case KindStatic:
		return "static"
	case KindBuffer:
		return "buffer"
	case KindMonster:
		return "monster"
	case KindTown:
		return "town"
	}
	return "other"
}

// ArgStr 是一個 %s 引數（或 strcat 追加的字串）：指標、擷取當下的內容（最多 80 bytes，到 NUL 為止，
// 不含 NUL）、種類、以及它若是組句緩衝區時的關聯。內容以 Go string 保存位元組（可含 80h 以上的位元組）。
type ArgStr struct {
	Ptr     uint16
	Content string
	Kind    Kind
	Piece   *Piece
}

// Piece 是遞迴的格式化單元（001 §3.3）：不含 Col、Row。
// Literal 是沒有轉換的 Piece 的字面文字（頂層取 EventRecord.Text，組句 Piece 取其組合字串）。
type Piece struct {
	Fmt     string
	FmtKind Kind
	Literal string
	Args    [12]uint16
	ArgStrs []ArgStr // 依格式字串中 %s 轉換的出現順序
	Appends []ArgStr // 組句之後以 strcat 追加的字串（依追加順序）
}

// EventRecord 是 25A5 一次呼叫在 A 時擷取的全部資料（001 §3.3）。之後不再讀原版記憶體。
type EventRecord struct {
	ID      string // g<N>
	Step    uint64
	SP, BP  uint16 // SP 為 0 表示未擷取（RegionTable.Classify 不套用堆疊規則）
	Caller  uint16
	Overlay string // L 記錄的目前載入 overlay 名稱

	Col, Row int
	Text     string // DS:3A36 的 NUL 結尾字串，取 [:40]；位元組
	FmtPtr   uint16
	Format   string // 格式字串（最多 64 bytes）
	FmtKind  Kind
	Args     [12]uint16
	ArgStrs  []ArgStr
	Composed *Piece // 整句組句關聯；非空時 Resolve 以它為準
	// Cells 是事件矩形內每個原版格的字元（Text 逐位元組），供字模遮罩使用；P 類修補時與 Text 同步改寫。
	Cells []byte
}

// Reason 是 Result.OK=false 的原因（001 §5）。
type Reason uint8

const (
	WhyNone         Reason = iota // OK=true
	WhyProtected                  // 手冊對照提示（003 §9）
	WhyBadFormat                  // 未支援規格、尾端 %
	WhyNoKey                      // 缺譯
	WhyBlankMissing               // 譯文為空字串
	WhyPassthrough                // 原文沒有字母，不是缺譯
	WhyIdentity                   // 恆等模板且沒有任何引數被替換
	WhyBadArg                     // %s 引數含非 20h 至 7Eh 的位元組、字組不足、帶 \c 的資料引數
)

func (r Reason) String() string {
	return [...]string{"ok", "protected", "badformat", "nokey", "blank_missing", "passthrough", "identity", "badarg"}[r]
}

// Result 是 Resolve 的結果（001 §5）。
type Result struct {
	Zh     []rune   // 譯文（尚未排版）；不含置中標記
	Center bool     // 整句置中（003 §8）
	Key    string   // 缺譯時是應補的鍵（只在來源是格式字串或靜態字串時填入）
	Hits   []string // 命中的 catalog 鍵集合（prose 為 h: 摘要鍵）
	Missed []string // 查不到的靜態、怪物、城鎮名引數
	OK     bool
	Why    Reason
}

// CenterMark 是 catalog 載入後置中標記在譯文開頭的表示（檔案內是開頭的 \c）。
const CenterMark = ''

// Lookup 是 Resolve 對 catalog 的唯一要求。譯文開頭可帶 CenterMark；空字串表示尚未翻譯（Resolve 視同缺）。
// ok=false 表示沒有這個鍵。實作要對鍵規範化由呼叫端負責（Resolve 傳入的已是規範化文字）。
type Lookup interface {
	UI(key string) (translation string, ok bool)
	Prose(digestKey string) (translation string, ok bool) // digestKey 是 "h:" 加 12 位十六進位
	Protected(normalized string) bool
}

// WideFunc 回字元在目前字型是不是全形（16 像素寬，2 h）；由字型寬度表決定，不用碼點範圍（001 §6）。
type WideFunc func(r rune) bool

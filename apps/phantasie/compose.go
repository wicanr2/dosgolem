package phantasie

import "strings"

// 組句關聯（docs/spec/001 §3.4）。S 掛點記錄 sprintf，T 掛點記錄 strcat 追加，
// A 時以「目的位址加組合字串與 DS 目前內容逐位元組相等」判定一個指標是不是組句緩衝區。

// MaxSprintfRecs 是 sprintf 記錄的容量（最近 64 次，以 dest 為鍵只留最新一筆）。
const MaxSprintfRecs = 64

// StrReader 讀 DGROUP 內以 NUL 結尾的字串：回內容（不含 NUL）與是否讀滿 max 仍無 NUL。
type StrReader func(ptr uint16, max int) (content string, truncated bool)

type sprintfRec struct {
	step     uint64
	dest     uint16
	piece    Piece  // Literal 存組合字串
	combined string // 組合字串（截到第一個 NUL）
}

// ComposeStore 是 sprintf 記錄與關聯檢查。
type ComposeStore struct {
	recs map[uint16]*sprintfRec
}

func NewComposeStore() *ComposeStore {
	return &ComposeStore{recs: map[uint16]*sprintfRec{}}
}

// nulCut 截到第一個 NUL。
func nulCut(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		return s[:i]
	}
	return s
}

// pieceContents 取 Piece 的各 %s 引數內容。
func pieceContents(p *Piece) []string {
	out := make([]string, len(p.ArgStrs))
	for i, a := range p.ArgStrs {
		out[i] = a.Content
	}
	return out
}

// PieceCombined 是 Piece 的組合字串：FormatEnglish 截到第一個 NUL，接上各追加字串（001 §3.4）。
// 格式錯誤回 ok=false。
func PieceCombined(p *Piece) (string, bool) {
	s, err := FormatEnglish(p.Fmt, p.Args[:], pieceContents(p))
	if err != nil {
		return "", false
	}
	var b strings.Builder
	b.WriteString(nulCut(s))
	for _, a := range p.Appends {
		b.WriteString(a.Content)
	}
	return b.String(), true
}

func clonePiece(p *Piece) *Piece {
	c := *p
	c.ArgStrs = append([]ArgStr(nil), p.ArgStrs...)
	c.Appends = append([]ArgStr(nil), p.Appends...)
	return &c
}

// Miss 是 Match 失敗的原因，對應 001 §9 的 composed_miss_* 計數。
type Miss uint8

const (
	MissNone    Miss = iota
	MissDest         // dest 沒有記錄
	MissPrefix       // 記錄的組合字串是目前字串的前綴（疑似有未追蹤的追加）
	MissContent      // 內容不符
	MissPercent      // 目前字串含 %
)

// Match 檢查指標 p 是否命中某筆記錄：dest 相同、組合字串等於目前內容（雙方截到第一個 NUL），且內容不含 %。
// 命中回該記錄的 Piece 快照（之後記錄被覆寫或追加不影響快照）。
func (c *ComposeStore) Match(p uint16, cur string) (*Piece, Miss) {
	r := c.recs[p]
	if r == nil {
		return nil, MissDest
	}
	cur = nulCut(cur)
	if strings.IndexByte(cur, '%') >= 0 {
		return nil, MissPercent
	}
	if cur != r.combined {
		if r.combined != "" && strings.HasPrefix(cur, r.combined) {
			return nil, MissPrefix
		}
		return nil, MissContent
	}
	return clonePiece(&r.piece), MissNone
}

// OnSprintf 在 S 入口呼叫：記錄 sprintf(dest, fmt, ...)。args 是格式字串之後的 12 個字組，strs 是各 %s 引數
// （Ptr、Content、Kind 已填）。對每個 %s 引數指標查現有記錄，成立就把當下記錄存成該引數的 Piece（巢狀關聯在 S 時建立）。
// 格式錯誤或字串引數不足時不記錄，並移除該 dest 的舊記錄（內容即將被覆寫，舊記錄必不符）。
func (c *ComposeStore) OnSprintf(step uint64, dest, fmtPtr uint16, fmtKind Kind, format string, args [12]uint16, strs []ArgStr) {
	p := Piece{Fmt: format, FmtKind: fmtKind, Args: args}
	p.ArgStrs = make([]ArgStr, len(strs))
	for i, a := range strs {
		if kindMayBeBuffer(a.Kind) {
			if pc, _ := c.Match(a.Ptr, a.Content); pc != nil {
				a.Piece = pc
			}
		}
		p.ArgStrs[i] = a
	}
	combined, ok := PieceCombined(&p)
	if !ok {
		delete(c.recs, dest)
		return
	}
	p.Literal = combined
	c.recs[dest] = &sprintfRec{step: step, dest: dest, piece: p, combined: combined}
	if len(c.recs) > MaxSprintfRecs {
		var oldest *sprintfRec
		for _, r := range c.recs {
			if oldest == nil || r.step < oldest.step {
				oldest = r
			}
		}
		delete(c.recs, oldest.dest)
	}
}

// kindMayBeBuffer：只有 buffer 種類的指標可能是組句緩衝區。
func kindMayBeBuffer(k Kind) bool { return k == KindBuffer }

// AppendResult 是 OnStrcat 的結果。
type AppendResult uint8

const (
	AppendNoRec    AppendResult = iota // dest 沒有記錄
	AppendMismatch                     // 有記錄但組合字串不等於 DS 目前內容（含 IRQ0 雙觸發的第二次）
	AppendDone
)

// OnStrcat 在 T 入口呼叫：strcat(dest, src)。dest 有記錄且組合字串等於 DS 目前內容（cur）時，把來源字串存入 Appends。
// src.Content 為空時不追加（不改變組合字串，也避免 IRQ0 雙觸發重複記錄空追加）。
func (c *ComposeStore) OnStrcat(dest uint16, cur string, src ArgStr) AppendResult {
	r := c.recs[dest]
	if r == nil {
		return AppendNoRec
	}
	if nulCut(cur) != r.combined {
		return AppendMismatch
	}
	if src.Content == "" {
		return AppendDone
	}
	r.piece.Appends = append(r.piece.Appends, src)
	r.combined += src.Content
	r.piece.Literal = r.combined
	return AppendDone
}

// MissCounter 是 Miss 對應的計數器名稱。
func MissCounter(m Miss) string {
	switch m {
	case MissDest:
		return "composed_miss_dest"
	case MissPrefix:
		return "composed_miss_prefix"
	case MissContent:
		return "composed_miss_content"
	case MissPercent:
		return "composed_miss_percent"
	}
	return ""
}

// Associate 在 A 時對事件做關聯檢查（001 §3.4）：
//  1. FmtPtr 命中記錄 → Composed。
//  2. 25A5 自己的每個 %s 引數指標命中記錄 → 該引數的 Piece。
//  3. Composed 內巢狀引數的 Piece 驗證其指標目前內容仍等於該 Piece 的組合字串，不符就清掉。
//
// 只對 buffer 種類的指標檢查；miss 計數只算 buffer 種類的指標（靜態字串不是緩衝區，不算缺陷）。
// read 讀 DGROUP 的目前內容。回傳各種 miss 的次數與成功組句數。
func (c *ComposeStore) Associate(rec *EventRecord, read StrReader) (composed int, misses map[Miss]int) {
	misses = map[Miss]int{}
	if rec.FmtKind == KindBuffer {
		cur, _ := read(rec.FmtPtr, 80)
		if pc, m := c.Match(rec.FmtPtr, cur); pc != nil {
			rec.Composed = validateNested(pc, read)
			composed++
		} else {
			misses[m]++
		}
	}
	for i := range rec.ArgStrs {
		a := &rec.ArgStrs[i]
		if a.Kind != KindBuffer || a.Piece != nil {
			continue
		}
		if pc, m := c.Match(a.Ptr, a.Content); pc != nil {
			a.Piece = validateNested(pc, read)
		} else {
			misses[m]++
		}
	}
	return composed, misses
}

// validateNested 複製 Piece 樹，清掉目前內容已不等於組合字串的巢狀 Piece。
func validateNested(p *Piece, read StrReader) *Piece {
	out := clonePiece(p)
	for i := range out.ArgStrs {
		a := &out.ArgStrs[i]
		if a.Piece == nil {
			continue
		}
		want, ok := PieceCombined(a.Piece)
		cur, _ := read(a.Ptr, 80)
		if !ok || nulCut(cur) != want {
			a.Piece = nil
			continue
		}
		a.Piece = validateNested(a.Piece, read)
	}
	return out
}

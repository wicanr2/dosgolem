package main

import "github.com/wicanr2/dosgolem/apps/buckrogers"

// storyASCIIState 是規格 031 §3.4 在 receipt runner legacy 劇情路徑的取得狀態。
// 它與 LiveRuntime 分開計數：搜尋函式相同（buckrogers.FindOriginalASCII），
// 回掃計數與 60 格節流由 runner 自己持有。收據只記取得與否、搜尋次數與 step，
// 不記任何字模位元組。
type storyASCIIState struct {
	Found bool `json:"found"`
	// Searches 是實際掃描記憶體的次數。
	Searches int `json:"searches"`
	// Step 是取得成功的 step（未取得為 0）。
	Step uint64 `json:"step,omitempty"`
	// Presenters 是成功換字型的劇情 presenter 數；SetFontErrors 是失敗而沿用原字型的數。
	Presenters    int `json:"presenters,omitempty"`
	SetFontErrors int `json:"set_font_errors,omitempty"`

	frames   uint64 // 已觀測的回掃數
	tryFrame uint64 // 上次搜尋時的回掃數 + 1（0 = 從未搜尋）
}

// search 依 60 格節流掃描一次；成功時回傳字形表（只在記憶體中使用）。
func (s *storyASCIIState) search(m buckrogers.MemReader, step uint64) ([]byte, bool) {
	if s.Found || s.tryFrame != 0 && s.frames < s.tryFrame-1+60 {
		return nil, false
	}
	s.tryFrame = s.frames + 1
	s.Searches++
	table, ok := buckrogers.FindOriginalASCII(m, buckrogers.OrigASCIIScanLimit)
	if !ok {
		return nil, false
	}
	s.Found, s.Step = true, step
	return table, true
}

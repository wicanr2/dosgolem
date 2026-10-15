package main

import "github.com/wicanr2/dosgolem/internal/machine"

// 傾印與診斷輸出的讀取（docs/spec/432-linear-memory-and-paging §3.4、§6）。
//
// 這些讀取只是為了印出來，不得有副作用：轉換開啟時用 Peek（客體線性視角，不產生
// 分頁錯誤紀錄、不填 TLB、不寫 accessed）；轉換關閉時維持現行的 Read8／Read16，
// 輸出與 432 之前逐位元組相同。

func peek8(m *machine.Machine, a uint32) uint8 {
	if m.Translating() {
		return m.Peek8(a)
	}
	return m.Read8(a)
}

func peek16(m *machine.Machine, a uint32) uint16 {
	if m.Translating() {
		return m.Peek16(a)
	}
	return m.Read16(a)
}

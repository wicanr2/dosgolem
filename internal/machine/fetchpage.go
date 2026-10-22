package machine

// 取指令頁快取的 Machine 端（docs/spec/455-fetch-page-cache；cpu.FetchPager）。
//
// 轉換開啟（V86、分段模式）時 CPU.Code 停用，每個指令位元組都經 Bus.Read8 → readLin →
// 分頁轉換 → readPhysRaw。FetchRead8 先照原樣做那一次 Read8，再判斷該線性頁之後的讀取
// 能否等於直接索引一塊 4 KB 的實體記憶體；能的話把那塊交給 CPU，CPU 在同一頁內取指令就不再
// 呼叫 Bus。快取只記「轉換」，位元組每次都從 Mem／Ext 現讀，所以自我修改碼照常生效。
//
// 只在轉換開啟時裝上：syncCodeFastPath 在 xlat 為真時 SetFetchPager(m)，否則 SetFetchPager(nil)，
// 讓實模式（含 A20 開）的取指令照舊直接走 Bus.Read8，不多一層 FetchRead8（455 §3.3）。
//
// 交出頁面的條件（全部成立）：
//   - m.xlat 為真（轉換關閉時 Read8 另有 HMA、20 位元遮罩與 CPU.Code 的路徑，不碰）；
//   - 沒有讀取監看（onRead == nil）：readPhysRaw 會對監看範圍呼叫回呼；
//   - 分頁開啟時，這一頁在微快取讀取份有目前世代、目前身分（cplUser）的條目，也就是剛才那次
//     Read8 轉換成功；同世代、同身分再讀同一頁，xlatPage 走 TLB 命中、沒有副作用（paging.go）；
//   - 套 A20 之後的實體頁整塊落在 Mem 的 [0, A0000h) 或 [C0000h, 1 MB)，或整塊落在 Ext 內。
//     A0000h–BFFFFh 一律不交（平面模式讀取會載入 VGA latch）。A20 遮罩只動 bit 20，同一頁的
//     位址遮罩結果一致。頁尾以 uint64 計算：實體頁 FFFFF000h 的頁尾 1_0000_0000h 不得環繞成 0。
//
// 作廢（CPU.FlushFetch）：
//   - invalidateTLB（tlbGen 改變：SetCR0 的 PE/PG、SetCR3、SetA20、SetProtectedModeProfile、applyPM）；
//   - syncCodeFastPath（每次 recalcPM，以及 SetA20、WatchReads、LoadState、Restore；SetFetchPager 內含清空）；
//   - Ext 配置（SetExtendedMemoryKB；配置前 Ext 為空、不會交出 Ext 頁，屬防禦性作廢）。
//     Machine 的 Mem 不重新配置（dpmi.go 的擴充屬 LEMachine）。
//     LoadState、Restore 以 copy 寫入既有 Mem／Ext，slice 不變，但兩者都會經 syncCodeFastPath 作廢。
func (m *Machine) FetchRead8(lin uint32) (uint8, []byte) {
	v := m.Read8(lin)
	if !m.xlat || m.onRead != nil {
		return v, nil
	}
	phys := lin &^ 0xFFF
	if m.paging {
		p := lin >> 12
		e := &m.utlbR[p&(utlbSize-1)]
		if e.page != p || e.gen != m.tlbGen || e.user != m.cplUser {
			return v, nil
		}
		phys = e.phys
	}
	if !m.a20 {
		phys &^= 0x100000
	}
	lo, end := uint64(phys), uint64(phys)+0x1000
	switch {
	case end <= vgaLo || (lo >= 0xC0000 && end <= MemSize):
		if end <= uint64(len(m.Mem)) {
			return v, m.Mem[lo:end:end]
		}
	case lo >= ExtBase:
		if off := lo - ExtBase; off+0x1000 <= uint64(len(m.Ext)) {
			return v, m.Ext[off : off+0x1000 : off+0x1000]
		}
	}
	return v, nil
}

// flushFetch 作廢 CPU 的取指令頁快取（CPU 尚未建立時不做事）。
func (m *Machine) flushFetch() {
	if m.CPU != nil {
		m.CPU.FlushFetch()
	}
}

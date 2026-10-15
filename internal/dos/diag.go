package dos

// 診斷回溯的讀取（docs/spec/432-linear-memory-and-paging §6、§10 第 8 項）。
//
// 診斷回溯（noteKeyWord、noteMissingAccess、overlay 載入紀錄的 CallSite、noteCPU）只是
// 記錄，**不得有副作用**：轉換開啟時，Read8 可能產生分頁錯誤紀錄、填 TLB、寫 accessed，
// 這會讓「多看一眼」改變客體之後看到的狀態與 Step 的結果。所以轉換開啟時改用 Peek 讀
// （位址一律由 SvcAddr 計算）；轉換關閉時維持現行的 Read8／Read16，行為不變
// （Peek 對平面 VGA 視窗與讀取監看的行為不同，改了會讓轉換關閉的路徑變動）。

func (d *DOS) diagRead8(a uint32) uint8 {
	if d.M.Translating() {
		return d.M.Peek8(a)
	}
	return d.M.Read8(a)
}

func (d *DOS) diagRead16(a uint32) uint16 {
	if d.M.Translating() {
		return d.M.Peek16(a)
	}
	return d.M.Read16(a)
}

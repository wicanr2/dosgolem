# 018 — 所有記憶體寫入都要走 `Write8`

日期：2026-09-08
狀態：**READY**
前置：[`009`](009-planar-vga.md) §1（planar 寫入路徑）。
動機：《銀河英雄傳說III SP》追「誰把行星的擁有者改掉」時，`-watch`
說那個位元組在載入之後停在 `0`，而同一個位址倒出來是 `2`
（`~/cht/logh3/docs/re/58`）。監看不是報錯，是**沒報**。

---

## 1. 證據

`Machine.Write8` 是唯一有掛勾的寫入路徑：

```go
func (m *Machine) Write8(a uint32, v uint8) {
	a &= 0xFFFFF
	if m.watchLo <= a && a <= m.watchHi && m.Mem[a] != v {
		m.onWrite(a, m.Mem[a], v)
	}
	m.Mem[a] = v
	if a >= 0xA0000 && a < 0xB0000 && m.planarVideo() {
		m.planarWrite(a-0xA0000, v)
	}
}
```

`Write16` 與 `WriteBytes` **直接寫 `m.Mem`**，兩個掛勾都不觸發。

用得到它們的地方不是邊緣情況：

| 呼叫端 | 做什麼 |
|---|---|
| `internal/dos/files.go` INT 21h `AH=3F`（讀檔）| 把檔案內容填進呼叫者的緩衝區 |
| `internal/dos/ems.go` map page | 把一頁 EMS 換進分頁框 |
| `internal/dos/bios.go`、`int21.go` | BIOS 資料區、中斷向量 |

也就是說：**遊戲從檔案讀進來的每一個位元組、每一次 EMS 換頁，
對監看都是隱形的。** 而那正是「表格是怎麼被填出來的」最常見的兩條路。

`WatchWrites` 的註解寫著「CPU 的每一次寫入都走 Write8，所以 16 位寫入
會來兩次」——這句話對 CPU 成立，讀者會理解成「所有寫入都看得到」，
但服務層不走 CPU。

## 2. 語意

`Write16(a, v)` ＝ 兩次 `Write8`（低位在前，位址各自 wrap）。
`WriteBytes(a, b)` ＝ 逐位元組 `Write8`。

掛勾的語意不變：**只有值真的改變才通知**，所以重複寫同一個值仍然不洗版。

planar 影像掛勾同理——服務層若把資料寫進 `A0000`–`AFFFF`，
現在也會走 planar 路徑。真機上 DOS 讀檔寫進顯示記憶體就是要經過
VGA 的寫入邏輯的，繞過去才是錯的。

## 3. 影響

- 監看不再漏報。這是主要目的。
- 服務層寫進顯示記憶體時行為改變（原本繞過 planar 邏輯）。
  本作沒有觀察到這種寫法，但語意上這是修正不是回歸。
- 成本：`WriteBytes` 每個位元組多一次區間比較。載入期的量級是
  數百 KB，可忽略。

## 4. 驗收

- 單元測試：`WriteBytes` 與 `Write16` 落在監看區間內時要通知，
  值沒變則不通知。
- 收據：改動後重跑所有收據（`~/cht/logh3/tools/rerun_receipts.sh`），
  逐位元組相同才算過。

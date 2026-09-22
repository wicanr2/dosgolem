# 229 — Buck Rogers 六行 story-fill 診斷

狀態：**READY（僅 content-safe 診斷旗標；非 runtime overlay）。**
日期：2026-09-22
前置：`223-buck-rogers-story-content-safe-diagnostics.md`。

## 缺口與範圍

`223` 的 `-story-fill-trace` 固定 rows 17–21，不能證明第四頁六行安全矩形
`[8,320)×[136,184)` 的最早 pre-execution fill。此 DRAFT 僅擴充 receipt 的讀取
範圍；不接 catalog、watcher、renderer、RGBA layer、遊戲規則或存檔。

## 可丟棄診斷契約

`-story-fill-rows` 僅允許 `5` 或 `6`，預設 `5`。top row 固定 17、水平區間固定
`[8,320)`：五行為 rows 17–21，六行為 rows 17–22。非法列數失敗即關閉。
`-story-fill-trace` 啟用時，receipt 額外輸出非機密整數 `story_fill_rows`；每筆
write 仍只有 step、CS:IP、ES:DI、CX，絕不輸出原文、VRAM bytes、答案、state 或畫面。

## READY 證據審查

既有 `223-buck-rogers-story-content-safe-diagnostics.md` 已 CONFORMED 的五行 helper
證明 observer 放在原版執行前、只讀 `ES:DI/CX`，且既有 receipt 不含原文或 VRAM
bytes。這次不改 address、執行時序、寫入、輸入或資料結構：只將已存在的 bounded
Mode 13h half-open 計算以明示常數 `5/6` 參數化。row22-only 與 row23-only 的
邊界是算術可重現的 diagnostic input，沒有推測原版語意；非法 rows 失敗即關閉。
因此證據足以授權**這一個** receipt CLI 診斷，不授權任何第四頁覆繪。

## READY 驗收

單元測試必須保持五行既有 boundary；在六行模式中 row22-only span 必命中、row23-only
span 必不命中，且非法 rows 拒絕。固定 commit 後以既有合法第四頁 state 及一筆正常
BIOS Enter 雙重重播，收據須逐 byte 相同；再以 Mode 13h half-open span 對完整六行
rectangle 重算最早相交。這是 DRAFT 量測，不使第四頁 catalog 或 adapter 變 READY。

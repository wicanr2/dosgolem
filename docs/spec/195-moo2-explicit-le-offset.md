# 195 — 明示偏移的 LE 載入探針

狀態：**CONFORMED**（明示 LE 標頭解析及零基址載入；MOO2 真入口見 197）
日期：2026-09-30

## 證據與範圍

MOO2 1.31 `ORION2.EXE` SHA-256 為 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，大小 2,612,010 bytes。MZ 的 `0x3C..0x3F` 是 `00 00 B4 09`，按標準 `e_lfanew` 解為超界 `0x09B40000`；原始檔案偏移 `0x292E4` 則是 `4C 45 00 00`。僅在本機合成副本將該四個 MZ bytes 改為 `E4 92 02 00` 後，既有 `InspectLE` 完整解析 2 objects、365 pages、51,363 筆 internal fixup；重定位後兩個 object 雜湊見 MOO2 專案 `docs/re/dosgolem-moo2-intake-20260930.md`。這是格式證據，不是執行或玩法證據。

## 契約

- 新增 `InspectLEAt(data, offset)` 與 `LoadLEAt(data, offset)`：呼叫者明示檔案偏移。仍須驗證 MZ、偏移範圍、LE 簽章與既有全部 object／page／fixup 界限；不掃描 `LE` 字串、不猜測 offset，也不修改輸入 bytes。
- 原有 `InspectLE`、`LoadLE` 繼續使用 `e_lfanew`，現有 FD2 收據不變。
- `cmd/leprobe -offset 0x...` 使用明示入口，預設仍讀 `e_lfanew`。輸出必須標示 offset 是呼叫者明示；`-execute-entry-prefix` 的 FD2 專用檢查不得拿 MOO2 使用。
- MOO2 本機固定雜湊的真檔探針必須能直接讀取 `0x292E4`，取得與合成標頭副本相同的 object 與 fixup 摘要。此時仍須列 `execution_support=partial`，不能寫成對拍通過。

## 驗收

合成 fixture 將 `e_lfanew` 設為超界，明示有效 LE offset 應成功，預設入口與錯誤 offset 應失敗；`LoadLEAt` 不改原 bytes，映像與有效預設入口一致。真檔只在本機唯讀掛載，核對 SHA-256 後執行 `leprobe -offset 0x292E4`。相關測試與探針通過後才標 `CONFORMED`。

驗收結果：`go test ./internal/machine ./cmd/leprobe` 通過；未改動的 MOO2 原版直接解析出 2 objects、365 pages、51,363 筆 fixup，與合成標頭探針完全一致。`LoadLEAt` 可建立入口 `0x10FF18`、堆疊 `0x1CDCD0` 的機器。這只符合本規格的載入範圍。

## 2026-09-30 勘誤

MOO2 在原檔 `0x26654` 還有內嵌 MZ，資料頁偏移相對該 MZ；本規格的 `LoadLEAt` 以原檔零點載入了錯誤頁面。前述 `0x10FF18` 只有 LE 標頭入口**數值**正確，入口**內容**及 object 雜湊不得再當原版執行收據。明示 LE 標頭解析及 fixup 表盤點仍成立；正確資料頁基址、真入口 bytes 與重跑結果見 [197](197-bound-mz-le-file-base.md)。

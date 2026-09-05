# 067 — 386 空資料 selector 載入

狀態：**CONFORMED**  
日期：2026-09-06  
依據：Intel 386 保護模式資料 segment 的空 selector 契約；FD2
[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md) 提供實際消費點。

- selector 0 可載入 DS、ES、FS、GS，代表後續不得解參照的空資料 segment；
  不需在 descriptor map 建立假描述子。
- SS 與 CS 不得透過此規則接受空 selector。
- 任何以空 selector 讀寫記憶體仍由既有 descriptor 檢查拒絕。
- 固定 FD2 第三回呼以 `POP FS` 還原進入時保存的零值，之後才能正常拆框並返回。

驗收包含空 FS 載入成功、空 selector 解參照失敗與固定原版第三回呼返回。

2026-09-06：空 FS 載入、不可解參照及固定第三回呼還原測試通過。

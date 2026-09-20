# Buck Rogers 未實作 DOS／BIOS 服務收據

狀態：**CONFORMED**

本規格補足 `031-buck-rogers-scratch-save-and-roster-lifecycle` 的診斷缺口：完整技能配置後，
圖示確認、`SAVE A?` 的 `YES` 及 `ADD CHARACTER TO TEAM` 都已由正常玩家路徑對齊，但
scratch 沒有 DOS write、名冊仍空。現有 `buckrogers-text-receipt` 不輸出
`DOS.Unimplemented`，因此尚不能排除原版呼叫了未實作服務後繼續執行。

這是唯讀診斷能力；不授權修改 DOS 服務語意、偽造成功結果、寫入 pristine original 或
散布原版內容。

## 輸入與輸出契約

- 新增可選 `-unimplemented`。未指定時，既有 JSON schema 與執行行為不變。
- 指定時，收據加入 `unimplemented` 字串陣列；內容直接取自 dosgolem 既有
  `DOS.UnimplementedReport()`，只含 interrupt／AH／AL identity 與呼叫次數，排序規則沿用
  `internal/dos` 的權威實作。
- 不輸出暫存器、路徑、原版 bytes、記憶體內容或文字；空集合省略。
- 此旗標不得清除、補做或改寫任何服務，也不得改變 CPU、DOS、檔案或畫面狀態。

## 原版實驗與證據等級

- 已證實：完整配置路徑的後 26 筆 dispatcher identity 與 spec 031 的 `YES`→加入角色分支
  逐筆相同；checkpoint 亦證實 `YES` 及 `ADD CHARACTER TO TEAM` 確實被選取。
- 已證實：現有 scratch-backed 收據只記錄 `CHARS.DAX` read／seek／close，沒有 AH=40h
  write，檔案雜湊等於 pristine。
- 未知：是否有 `FileOps` 未涵蓋的未實作服務；本規格只核准把既有統計顯示出來。
- 未知：若存在未實作服務，它是否是空名冊根因；仍須以呼叫邊界、輸入與結果另行證實。

## 驗收

- 單元測試固定旗標關閉時省略欄位、開啟且集合為空時省略欄位，以及非空報告完整保留。
- 從相同 state、正常 BIOS 排程與空白 scratch 重播完整技能配置及保存／名冊流程；收據必須
  能明確顯示是否存在未實作服務。
- 既有 receipt、Buck Rogers packages、全部正式 package tests、`go vet` 與相關 race
  detector 通過後，方可升為 CONFORMED。

## 停止線

- 報告為空時，排除「已記錄的未實作服務」分支，回到原版記憶體狀態與角色資格證據。
- 報告非空時，先建立最小可重現實驗與 DRAFT 服務規格；不得由服務代號直接推論根因或
  實作平台行為。

## 符合性紀錄（2026-09-21）

- `-unimplemented` 已實作為純觀測旗標；關閉、空集合與排序後非空報告的單元測試通過。
- 完整 80／40 點配置、`SAVE A? YES` 與加入角色路徑由兩個空白隔離 scratch 正式重播；
  兩份 1,457-event JSON 逐 byte 相同，SHA-256 為
  `a8c4cc190108c4107eae14a78238a88532334fdf8d32b85ef8071b81c996ed47`。
- 兩份收據的 `unimplemented` 與 `writes` 皆為空；因此已排除 dosgolem 有記錄但未實作的
  DOS／BIOS 服務是本路徑空名冊的原因，沒有據此修改平台語意。
- 全部正式 packages test／vet 與 Buck Rogers／receipt race detector 通過。

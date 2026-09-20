# Buck Rogers 職業選擇生命週期收據

狀態：**CONFORMED**

本規格沿用 `cmd/buckrogers-text-receipt` 與固定 state，以三次 Enter 抵達職業選擇，再分別
排入 Down→Up 或 Escape。不得加入翻譯、繪圖、倍率預設、direct-entry 或角色規則修改。

## 固定契約

- state 起點 #99,999,999；Enter：#100,010,000、#100,240,000、#100,400,000。
- Down／Up：#100,650,000／#100,720,000；Escape：#100,650,000；停止 #102,000,000。
- Down→Up 必須在既有 22 筆前綴後依序產生第一職業 normal、第二職業 selected、第二職業
  normal、第一職業 selected；Escape 必須先產生第一職業 normal，再重建功能選單七筆事件。
- caller 使用 runtime `segment:offset`；所有事件以完整長度、SHA-256、caller、色號及座標比對。
- 兩條路徑各重播兩次，JSON 與 64,000-byte indexed framebuffer 必須逐 byte 相同。

## 驗收

- 專案 inventory 與 verifier 對 schema、次序、step、identity、排程、雜湊或 framebuffer
  漂移失敗即關閉。
- 全部正式 packages test／vet 與相關 race detector 通過後，才能改標 CONFORMED。
- 本規格只證實預設種族／性別下前兩個職業的移動，以及 Escape 返回功能選單；其餘未知。

## 符合性紀錄（2026-09-21）

- Down→Up 與 Escape 各重播兩次，pair 內 JSON／framebuffer 逐 byte 相同。
- JSON SHA-256 分別為 `650ad5b1…6696`、`653fc53d…6c51`；framebuffer 分別為
  `3a61cb54…012c`、`b08623d2…3a3`。
- 專案 61 項 Python 測試與正式 verifier 通過；dosgolem 正式測試、vet、race 見提交前收據。

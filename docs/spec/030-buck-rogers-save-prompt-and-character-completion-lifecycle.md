# Buck Rogers 儲存詢問與角色建立完成生命週期

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 從固定 state 經正常角色建立路徑到儲存詢問，量測字母
`Y`、預設 `NO` 與選取 `YES` 的結果。這是診斷規格，不授權改寫存檔、角色資料或遊戲流程。

## 固定輸入

- 共用前綴沿用 spec 029，並於 #106,000,000 Enter、#106,200,000 `Y` 進入儲存詢問。
- 字母 `Y` 分支：#107,000,000 `Y`；停止於 #113,000,000，預期 298 筆事件。
- 預設 `NO` 分支：#107,000,000 `N`；停止於 #113,000,000，預期 305 筆事件。
- 選取 `YES` 分支：#107,000,000 Left、#107,200,000 Enter；停止於 #113,000,000，預期
  305 筆事件。
- `NO` 與 `YES` 各使用由同一 pristine original 建立的全新 writable overlay；每條分支從
  相同 state 獨立重跑兩次。不得 direct-entry 或記憶體注入。

## 生命週期契約

- 儲存詢問是選項介面，不是 `Y`／`N` 對話框：直接字母 `Y` 沒有事件與像素變化。
- 直接字母 `N` 接受預設 `NO` 並回到功能選單。
- Left 改變選取 framebuffer，但沒有 dispatcher 事件；後續 Enter 接受 `YES` 並回到同一
  功能選單。
- 兩條接受分支的 overlay 完整 manifest 必須逐 byte 等於 pristine manifest；本實測路徑
  沒有檔案副作用，不得把選項文字外推成已寫磁碟。
- 清冊只保存 content-safe identity、位置、色彩與推論等級，不保存原版英文全文。

## 驗收

- 三條正式分支各兩次 JSON、64,000-byte indexed framebuffer 逐 byte 相同。
- verifier 固定 state、原版程式、命令、298-event 儲存詢問基線、完整 BIOS 排程、新增事件
  及 overlay manifest；任一漂移皆失敗即關閉。
- 專案正反例、真實 verifier、全部正式 dosgolem packages test／vet 及相關 race detector
  通過後，本規格才能升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- 字母 `Y`、預設 `NO` 與選取 `YES` 三條分支各從相同 state 重播兩次；每對 JSON 與
  framebuffer 逐 byte 相同，事件數依序為 298／305／305。
- `NO` 與 `YES` 各自兩份 overlay manifest 皆逐 byte 等於 pristine manifest；兩者終點
  framebuffer 亦逐 byte 相同，穩定停在功能選單。
- 專案 92 項測試與真實 verifier 通過；dosgolem 正式 packages test／vet 與 Buck Rogers
  相關 race detector 全數通過。

# Buck Rogers 角色姓名輸入生命週期

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 從固定 state 經四次 Enter、`N` 接受能力值，到姓名提示
後量測兩條正常 BIOS 輸入分支：`A`→`B`→Backspace 的編輯路徑，以及 `A`→Enter 的確認路徑。
這是診斷規格，不授權改寫姓名、長度、允許字元、角色資料或存檔格式。

## 固定輸入

- 共用前綴：#100,010,000、#100,240,000、#100,400,000、#100,650,000 為 Enter；
  #101,400,000 為 `N`（scan `0x31`／ASCII `0x6E`）。
- 編輯分支：#102,050,000 `A`、#102,150,000 `B`、#102,250,000 Backspace；停止於
  #103,000,000，預期 185 筆事件。
- 確認分支：#102,050,000 `A`、#102,150,000 Enter；停止於 #103,000,000，預期 226 筆事件。
- 每條分支從同一 state 獨立重跑兩次；不得 direct-entry 或寫入原版記憶體。

## 生命週期契約

- `A`、`B` 各透過既有 dispatcher 產生一筆長度 1 的回顯事件，座標依輸入位置遞增。
- Backspace 不產生 dispatcher 完成事件，但終點 framebuffer 必須證明第二字元已清除、第一字元
  仍保留；不能因事件清冊沒有 Backspace 就宣稱按鍵無效。
- 非空姓名後 Enter 進入職業技能點配置畫面；正式收據只固定第一個穩定終點，不繼續配置。
- Escape 探針只重印姓名提示、終點 framebuffer 不變；空字串 Backspace 也沒有事件或畫面
  差異。兩者只保存為負面證據，不宣稱 Escape 具有取消語意。

## 驗收

- 編輯與確認分支各兩次 JSON、64,000-byte indexed framebuffer 逐 byte 相同。
- verifier 固定 state、原版程式、命令及 183-event 姓名提示基線 SHA-256，逐筆驗證新增事件、
  輸入排程與終點畫面；任一漂移皆失敗即關閉。
- 確認分支的玩家輸入回顯與下一畫面事件必須分開分類；正式檔案不保存原版英文全文。
- 專案正反例、真實 verifier、全部正式 dosgolem packages test／vet 及相關 race detector
  通過後才能升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- 編輯與確認分支各從相同 state 重播兩次；每對 JSON 與 framebuffer 逐 byte 相同。
- 編輯分支 185 events，收據／畫面 SHA-256 為 `863440bc…e9e3`／`bd3d7799…eec6`；確認
  分支 226 events，分別為 `52e50460…1d3d`／`a1cd728c…95f7`。
- 專案 79 項測試與真實 verifier 通過；全部正式 dosgolem packages test／vet 與 Buck Rogers
  相關 race detector 通過。

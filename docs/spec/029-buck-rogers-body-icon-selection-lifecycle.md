# Buck Rogers 角色身體圖示選擇生命週期

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 從固定 state 經正常角色建立路徑進入角色身體圖示選擇
畫面，量測方向選取、拒絕圖示與確認圖示至儲存詢問畫面。這是診斷規格，不授權改寫角色
資料、圖示資產、存檔格式或遊戲流程。

## 固定輸入

- 共用前綴沿用 spec 028，並於 #103,600,000 Escape、#103,800,000 `Y` 進入圖示畫面。
- 移動分支：#106,000,000 Right；停止於 #108,000,000，預期 297 筆事件。
- 拒絕分支：#106,000,000 Enter、#106,200,000 `N`；停止於 #109,000,000，預期 302 筆事件。
- 確認分支：#106,000,000 Enter、#106,200,000 `Y`；停止於 #109,000,000，預期 298 筆事件。
- 每條分支從同一 state 獨立重跑兩次；不得 direct-entry 或記憶體注入。

## 生命週期契約

- Right 改變選取圖示並重畫底部選取指示；完整 framebuffer 必須不同於基線。
- Enter 顯示圖示確認提示；`N` 重建圖示畫面及底部指示，終點逐 byte 等於圖示基線。
- Enter→`Y` 清除圖示畫面並顯示儲存詢問，作為本階段下一個穩定玩家可見邊界。
- 清冊只保存 content-safe identity、位置、色彩與推論等級，不保存原版英文全文。

## 驗收

- 三條正式分支各兩次 JSON、64,000-byte indexed framebuffer 逐 byte 相同。
- verifier 固定 state、原版程式、命令、296-event 圖示基線、完整 BIOS 排程與新增事件；
  拒絕分支另須逐 byte 回到基線，任一漂移皆失敗即關閉。
- 專案正反例、真實 verifier、全部正式 dosgolem packages test／vet 及相關 race detector
  通過後，本規格才能升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- 移動、拒絕與確認三條分支各從相同 state 重播兩次；每對 JSON 與 framebuffer 逐 byte
  相同，事件數依序為 297／302／298。
- 拒絕終點逐 byte 等於圖示基線；確認終點穩定停在儲存詢問畫面。
- 專案 90 項測試與真實 verifier 通過；排除既有未納版控 `workplace/` probe 後，dosgolem
  正式 packages test／vet 與 Buck Rogers 相關 race detector 全數通過。

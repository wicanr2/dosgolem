# Buck Rogers 職業技能動作選單與確認離開生命週期

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 從固定 state 經正常角色建立路徑到達職業技能配置畫面，
量測一次加點後以 Right 選擇減點並還原，以及仍有未用點數時 Escape→`Y` 進入技術技能配置
畫面。這是診斷規格，不授權改寫點數、技能規則、角色資料或存檔格式。

## 固定輸入

- 共用前綴：#100,010,000、#100,240,000、#100,400,000、#100,650,000 為 Enter；
  #101,400,000 為 `N`；#102,050,000 為 `A`；#102,150,000 為 Enter。
- 可逆減點分支：#102,700,000 Enter 加點、#102,800,000 Right、#102,900,000 Enter；停止於
  #104,000,000，預期 236 筆事件。
- 確認離開分支：#102,700,000 Escape、#102,900,000 `Y`；停止於 #105,000,000，預期
  289 筆事件。
- 每條分支從同一 state 獨立重跑兩次；不得 direct-entry、記憶體注入或自動分配其餘點數。

## 生命週期契約

- 預設動作 Enter 先合法加一點；Right 本身沒有 dispatcher 完成事件，但後續 Enter 以五筆
  重畫事件把同一技能列及剩餘點數還原。終點 framebuffer 必須逐 byte 等於既有零點／Right
  動作選取畫面，不能只以事件數宣稱可逆。
- 預設動作的 Left 會環回離開位置；加點後 Enter 顯示同一未用點數確認 identity。此路徑只作
  輔助定位，不列為正式分支，亦不把不可見反白文字當譯文。
- 尚有 6 點未用時 Escape 顯示確認提示；`Y` 後必須完成下一個技術技能配置畫面的 62 筆
  content-safe 事件並停在穩定終點。確認提示與下一畫面必須分開分類。
- 正式檔案只保存長度、SHA-256、caller、色彩、座標及 step，不保存原版英文全文或動態數值。

## 驗收

- 兩條正式分支各兩次 JSON、64,000-byte indexed framebuffer 逐 byte 相同。
- verifier 固定 state、原版程式、命令、226-event 基線、完整 BIOS 排程與新增事件；減點終點
  另與既有 Right 選取畫面逐 byte 比對，任一漂移皆失敗即關閉。
- 專案正反例、真實 verifier、全部正式 dosgolem packages test／vet 及相關 race detector
  通過後，本規格才能升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- 可逆減點與確認離開兩條分支各自從相同 state 重播兩次；每對 JSON 與 framebuffer 逐 byte
  相同。
- 減點分支 236 events，收據／畫面 SHA-256 為 `b513aae8…3cd3`／`5dc661e4…195`，且終點
  逐 byte 等於既有零點／Right 選取畫面；離開分支 289 events，分別為
  `68c586e4…f5b1`／`6bf7f9bb…432a`。
- 專案 85 項測試與真實 verifier 通過；全部正式 dosgolem packages test／vet 與 Buck Rogers
  相關 race detector 通過。

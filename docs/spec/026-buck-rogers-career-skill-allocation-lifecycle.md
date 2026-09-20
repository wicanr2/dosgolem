# Buck Rogers 職業技能點配置生命週期

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 從固定 state 經正常角色建立路徑到達職業技能點配置畫面，
量測技能列選取、一次合法加點，以及仍有點數時 Escape→`N` 拒絕離開。這是診斷規格，不授權
改寫點數、bonus、total、職業規則、角色資料或存檔格式。

## 固定輸入

- 共用前綴：#100,010,000、#100,240,000、#100,400,000、#100,650,000 為 Enter；
  #101,400,000 為 `N`（scan `0x31`／ASCII `0x6E`）；#102,050,000 為 `A`；
  #102,150,000 為 Enter。
- 選取分支：#102,700,000 為 Down（scan `0x50`／ASCII `0x00`）；停止於 #103,000,000，
  預期 234 筆事件。
- 加點分支：#102,700,000 為 Enter；停止於 #103,000,000，預期 231 筆事件。
- 拒絕離開分支：#102,700,000 為 Escape，#102,900,000 為 `N`；停止於 #104,000,000，
  預期 227 筆事件。
- 每條分支從同一 state 獨立重跑兩次；不得 direct-entry、寫入原版記憶體或自動配置其餘點數。

## 生命週期契約

- Down 將第一列取消選取並選取第二列；正式清冊固定兩列各四筆重畫事件的次序、座標、色彩與
  identity。
- 預設動作下 Enter 合法增加第一項技能點數一次；正式清冊固定選取列四筆重畫與畫面頂端
  剩餘點數一筆重畫。動態數值只以 identity 與像素終點驗證，不列為譯文。
- 尚有點數時 Escape 顯示底部確認提示；輸入 `N` 後回到與操作前逐 byte 相同的職業技能點
  配置畫面。正式清冊只保存該提示的 content-safe identity，不保存英文全文。
- `+`／`-` 探針未產生事件或畫面變化；Left／Right 只改變底部動作選取。它們保存為探索證據，
  不外推成已完成的加減規則。

## 驗收

- 三條分支各兩次 JSON、64,000-byte indexed framebuffer 逐 byte 相同。
- verifier 固定 state、原版程式、命令及 226-event 配置畫面基線 SHA-256，逐筆驗證新增事件、
  BIOS 排程與終點畫面；任一漂移皆失敗即關閉。
- 專案正反例、真實 verifier、全部正式 dosgolem packages test／vet 及相關 race detector通過後，
  本規格才能升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- 選取、加點與拒絕離開三條分支各自從相同 state 重播兩次；每對 JSON 與 framebuffer 逐 byte
  相同。
- 選取分支 234 events，收據／畫面 SHA-256 為 `655984fa…5487`／`ddf66e0c…c6cbd`；加點
  分支 231 events，分別為 `559023d6…2898`／`3428db0d…c808`；拒絕離開分支 227 events，
  分別為 `cd76a0e8…af3`／`a1cd728c…95f7`。
- 專案 82 項測試與真實 verifier 通過；全部正式 dosgolem packages test／vet 與 Buck Rogers
  相關 race detector 通過。

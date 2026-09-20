# Buck Rogers 重擲提示輸入與動態欄位生命週期

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 以正常 BIOS 鍵盤輸入，從固定 state 經四次 Enter 到達
角色資料頁後，分別送出 `Y` 與 `N`，量測重擲及接受分支的 guarded post-call 事件與終點
framebuffer。這是診斷規格，不授權翻譯、覆繪、改寫亂數或角色規則。

## 固定輸入

- state 起點 #99,999,999；四次 Enter 固定排在 #100,010,000、#100,240,000、
  #100,400,000、#100,650,000。
- 分支鍵固定排在 #101,400,000：`Y` 為 scan `0x15`／ASCII `0x79`，`N` 為 scan
  `0x31`／ASCII `0x6E`；停止點 #102,000,000。
- 每條分支必須從同一 state 獨立重跑兩次，不得 direct-entry、寫入原版記憶體或挑選骰值。

## 事件與亂數契約

- `Y` 分支預期共 149 筆事件：沿用前 118 筆基線，新增七筆能力值、摘要／技能重畫及同一
  重擲提示；終點仍停在提示畫面。
- `N` 分支預期共 183 筆事件：沿用前 118 筆基線，重新建立角色資料內容後進入下一個姓名
  輸入提示邊界；本規格不輸入姓名、不追入後續子系統。
- 新增事件固定 step、length、原文 SHA-256、runtime `segment:offset` caller、bg／fg、
  row／column；正式資料不保存英文全文。
- 相同 snapshot 的兩次 `Y` 結果相同只證明原版亂數狀態包含在 snapshot 且本路徑可重播；
  不宣稱已辨識 seed、亂數公式、呼叫次數或自然開局的一般骰序。

## 驗收

- `Y` 與 `N` 各兩次 JSON、64,000-byte indexed framebuffer 逐 byte 相同。
- verifier 固定 state、原版程式與命令 SHA-256、五鍵排程、事件數、每筆 step／identity、
  終點 framebuffer；任一漂移皆失敗即關閉。
- Space／Escape 探針只重印原提示且 framebuffer 不變，作為未接受按鍵的輔助負面證據；
  它們不納入正式分支收據。
- 專案正反例測試、真實 verifier、全部正式 dosgolem packages test／vet 及相關 race
  detector 通過後，才可把本規格升為 CONFORMED。

## 符合性紀錄（2026-09-21）

- `Y` 與 `N` 各從相同 state 獨立重播兩次；每對 JSON 與 framebuffer 均逐 byte 相同。
- `Y` 為 149 events，收據／畫面 SHA-256 分別為 `132cf007…d5eb5`／`03d9bf1f…0f97`；
  `N` 為 183 events，分別為 `f9ed6a8e…7bff`／`55da7c0e…a6bb`。
- 專案 76 項測試與真實 verifier 通過；全部正式 dosgolem packages test／vet 及 Buck Rogers
  相關 race detector 通過。

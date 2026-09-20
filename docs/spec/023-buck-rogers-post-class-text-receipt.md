# Buck Rogers 確認職業後文字收據

狀態：**CONFORMED**

本規格核准 `buckrogers-text-receipt` 以正常 BIOS 鍵盤輸入，從既有固定 state 依序送出四次
Enter，量測確認預設職業後的角色資料／重擲能力值畫面。這是診斷與證據規格，不授權翻譯、
覆繪、改寫角色規則或選定 2×／3×。

## 固定輸入

- state 起點固定為 #99,999,999，Enter 排在 #100,010,000、#100,240,000、#100,400,000、
  #100,650,000，停止於 #102,000,000。
- 必須從同一 state 獨立重跑兩次；不得 direct-entry、修改記憶體、略過選單或重擲到「看起來
  合理」才保存。
- 收據固定 state、`START.EXE`、`GAME.OVR` 及命令來源 SHA-256。

## 事件契約

- 正式 JSON 必須恰有 118 筆 guarded post-call 完成事件：前 22 筆沿用既有正常路徑證據，
  後 96 筆由專案 `text/post-class-events.tsv` 固定 exact identity 與 step。
- 每筆新事件固定 length、原文 SHA-256、runtime `segment:offset` caller、bg／fg、row／column；
  正式證據不得保存英文全文。
- 96 筆依實際輸出區分靜態標籤、動態角色值、能力值、職業技能、重畫及重擲提示；分類只描述
  可見輸出角色，不推論角色規則或亂數演算法。
- fixed state 的兩次輸出相同，只證明此 snapshot 含足以重生本次結果的狀態；不宣稱已辨識
  原版 seed，也不宣稱其他 state 會產生相同能力值。

## 驗收與符合性紀錄（2026-09-21）

- 兩份 JSON 逐 byte 相同，SHA-256 為
  `3c55e98804c752a817d80c66b20e0bac59992d32280582786790affc888b0a42`。
- 兩份 64,000-byte indexed framebuffer 逐 byte 相同，SHA-256 為
  `1f3b81946cc058d89a8ecfd9ca5aab8e7fb2aaa5d95c2c2ecbd49c219f51d4dd`。
- 專案 verifier 對 schema、四鍵排程、事件數、次序、step、identity、輸入雜湊與 framebuffer
  漂移採失敗即關閉；正反例及真實收據均通過。
- 本規格不新增 runtime 功能；全部正式套件測試、`go vet` 與相關 race detector 通過後維持
  CONFORMED。

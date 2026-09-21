# 211 — Buck Rogers 技術技能配置執行期繁中顯示請求

狀態：CONFORMED

## 範圍與證據

由職業技能頁 Escape→`Y` 的正常玩家路徑，技術技能配置畫面有四個固定標題、
13 個技能名稱、52 個動態數值事件，以及第一列選取色重畫。Down 路徑另證實第一列
一般色重畫與第二列選取色重畫。其中「單項技能上限」與「點數／加值／總計」和
職業技能頁是同一 exact identities，直接共享既有 career catalog。技術 catalog 只新增
17 個不重複 identities：兩個專屬標題、13 個一般技能列與前兩列 selected variants。

技能名稱逐筆反查使用者本機中文手冊 `SCAN0352_012.jpg` 第 19–20 頁；正式 TSV 只保存
繁中譯文與 content-safe identity，不保存可還原原作的英文全文或掃描圖。

## 契約

1. resolver 同時比對長度、SHA-256、caller、背景、前景、row 與 column；不使用字串相似度。
2. normal／selected variants 是不同 identity，但同一技能必須共用 text key。
3. 動態剩餘點數、points／bonus／total 與未實測 selected variants 全部 miss。
4. 譯文只產生 `DisplayRequest`；不進入原版記憶體、比較、技能點規則、輸入或存檔。
5. 沒有安全矩形時不得啟用技術技能繁中像素覆繪。
6. 啟用 technical catalog 時必須同時提供 career catalog；共享標題不重複收錄，合併後仍是
   一個 immutable exact resolver。

## 驗收

- 正式 TSV 驗證 17 個不重複 identities、連續 sequence、來源 identity、雙向 text-key coverage、
  中文手冊術語、動態欄邊界與 normal／selected 共用 key。
- base 與 Down 各做無技術 catalog control 與技術 catalog，每種各雙重重播。
- presentation 欄位外 JSON 與 64,000-byte framebuffer 必須等於 control；請求順序必須符合
  入場與 Down 的原版重畫順序。
- Go 正反例、完整正式套件 test／vet、本輪改動套件 race 與專案回歸通過後升
  CONFORMED。

## READY 審查

所有新增 identity 都來自第 47 階段 confirmed 正常路徑清冊；13 個技能的繁中譯名均能在
中文手冊原圖唯一對應。兩個共享標題的 identity 與譯文均已由 spec
209-buck-rogers-career-skill-runtime-display-requests CONFORMED；成對依賴可在命令列失敗即關閉。
本規格不改玩法、畫面幾何或產品預設倍率，可重新進入實作。

## 實作回退發現

首次 catalog 合併在正常入場路徑失敗即關閉：技術頁的「單項技能上限」與「點數／加值／
總計」兩筆事件，長度、SHA-256、caller、色號與座標全部和職業技能頁相同，因此是
同一 exact identity，不能在兩個 catalog 重複定義。規格曾退回 DRAFT；現已刪除重複定義、
將共享標題依賴寫成失敗即關閉契約，並完成新一輪 READY 審查。

## CONFORMED 收據

- `LoadTechnicalSkillCatalog` 與 `-technical-skill-events`／`-technical-skill-translations` 已接入既有
  exact watcher；後兩旗標必須與 career catalog 一起提供。
- base 雙重重播為 289 events／32 total requests／257 misses；Down 為 297／34／263。
- Down 新增第一列 normal 與第二列 selected 兩筆 request，順序符合原版重畫。
- control 與 catalog 的 events、BIOS keys、停止點及 64,000-byte framebuffer 逐位元相同。
- 專案 131 項 Python 測試、正式 Go 套件 test／vet 與本輪套件 race 通過。

# 209 — Buck Rogers 職業技能配置執行期繁中顯示請求

狀態：CONFORMED

## 範圍與證據

由姓名 `A`→Enter 的正常路徑，職業技能配置畫面共有四個固定標題、八個技能名稱、36 個動態
數值事件，以及第一列選取色重畫。Down 路徑另證實第一列一般色重畫與第二列選取色重畫。
正式 catalog 收錄 14 個 exact identities：四標題、八一般技能列、注意力 selected、無重力
行動 selected；其餘數字事件維持 miss。

## 契約

1. resolver 同時比對長度、SHA-256、caller、背景、前景、row、column；normal／selected 為
   不同 identity，但相同技能共用 text key。
2. 技能譯名必須逐筆等於既有 `character-sheet.zh-TW.tsv` 正式詞彙；畫面標題使用已核准的
   runtime-interface 譯文，不讓譯文進入技能點規則或輸入。
3. base 路徑產生 13 個職業技能 request；Down 路徑再產生注意力 normal 重畫與無重力 selected
   兩筆 request。姓名提示 request 由既有 catalog 獨立產生。
4. 動態剩餘點數、points、bonus、total 及未收錄的選取 variant 全部 miss，不模糊匹配。
5. 本規格只建立 `DisplayRequest`；沒有安全矩形時不得啟用繁中像素覆繪。

## 驗收

- 正式 TSV 驗證來源 identity、連續 sequence、唯一 key、孤兒／漏譯、文字治理及技能術語漂移。
- base 正常路徑為 226 events、14 total requests、212 misses；Down 為 234 events、16 requests、
  218 misses。兩路各雙重重播，raw framebuffer 與無 catalog control 一致。
- Go 正反例、完整 test、vet、race detector 通過後升為 CONFORMED。

## READY 審查

所有收錄 identity 都來自 confirmed 正常玩家路徑清冊；技能譯名已有正式手冊來源 catalog，動態
數值位置與選取色 variant 亦有獨立證據。本規格不改規則、輸入或畫面幾何，可以進入實作。

## CONFORMED 收據

- `LoadCareerSkillCatalog` 與 `-career-skill-events`／`-career-skill-translations` 已接入既有 exact
  watcher；未提供安全矩形時不啟用 overlay。
- base 雙重重播為 226 events／14 total requests／212 misses；Down 為 234／16／218。後者
  新增注意力 normal 與無重力 selected 兩筆 request，順序符合原版重畫。
- catalog 與 control 的 events、BIOS keys、停止點及 64,000-byte framebuffer 逐位元相同。
- 正式 events／譯文 SHA-256 分別為
  `3d0bde60cfe3fb4bc9db9853ca969a9062c8b56a29a7dd147a8f59122b3e0613`、
  `96624a20a2f6e4b90d33d6447066400a137e6574fd82811624962867a97a4161`。

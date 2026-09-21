# 207 — Buck Rogers 姓名提示執行期繁中覆繪

狀態：CONFORMED

## 證據

spec 206 已 CONFORMED 的固定提示事件位於 row 24／column 0，原文長度 16，故原版文字範圍為
`[0,128)×[192,200)`。已證實的第一個玩家姓名回顯位於 row 24／column 17，即 x=136；提示
範圍與輸入欄之間另有一個 8-pixel 空格。繁中「角色姓名：」為五個 Unicode 字元，可置於
16-cell 單列矩形，不需擴張、截斷或移動輸入欄。

## 契約

1. 唯一安全矩形固定為 x=0、y=192、width=128、height=8；draw anchor 與左上角相同，容量
   16 cells、單列、`single-line-reject`。
2. runtime catalog 與 rectangle 必須成對提供；任何缺表、孤兒 key、identity／幾何漂移、
   缺字或非 2／3 倍率均失敗即關閉。
3. presenter 只改輸出 RGBA，不改 indexed framebuffer、palette、文字事件、BIOS 輸入、玩家
   姓名 buffer 或存檔。
4. 輸入 `A` 後，固定提示 stamp 必須仍存在；動態 `A` 使用原版 x=136 像素，不由 overlay
   清除、替換或覆蓋。

## 驗收

- base 與輸入 `A` 各有無覆繪 control，2×／3× 各雙重重播。
- 各倍率 JSON／RGBA／baseline／raw framebuffer 決定性一致；presentation metadata 以外的
  JSON projection 等於 control。
- RGBA 差異只能位於縮放後的 `[0,128)×[192,200)`；矩形外零差異，且輸入欄 x≥136 零差異。
- 正式資料測試、Go 正反例、完整測試、vet 與相關 race detector 通過後才升 CONFORMED。

## READY 審查

事件 identity、完整譯文、原版文字範圍、玩家輸入左界與既有整數倍率 renderer 均已有已證實
證據。本規格不改輸入或遊戲語意，也不選定產品預設倍率，可以進入實作。

## CONFORMED 收據

- `-name-prompt-rects` 已成對接入既有 presenter；`ValidateMenuOverlayCoverage` 對 request／rect
  event-key 集合做雙向檢查，缺少與孤兒矩形皆失敗。
- base 及輸入 `A` 在 2×／3× 各雙重重播；events／requests／misses 分別維持
  183／1／182 與 184／1／183，raw framebuffer 等於各自 control。
- 2× 差異 941 pixels、3× 差異 2,038 pixels，全部位於核准矩形內；矩形外及 x≥136 的
  玩家輸入欄均為 0 pixels。
- 16×16 GOLEMFNT SHA-256 為
  `aa53cc31dd2a17fc5554792d3767c1cef757ecda69326c4d17643d211f4d0ea1`；原始解析度實圖已確認
  兩倍率皆完整顯示提示，且 `A` 保持可見。

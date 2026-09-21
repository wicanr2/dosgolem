# 210 — Buck Rogers 職業技能配置執行期繁中覆繪

狀態：CONFORMED

## 幾何證據

14 個 exact identities 均使用原文起點與長度作單列矩形，不做任何延伸。技能列由 x=8 開始，
最長右界 x=136；動態 points／bonus／total 分別從 x=184／232／280 開始。兩個標題數值位於
x=184，固定標題右界分別為 x=168／152；所有矩形與動態欄均有明確間隔。

## 契約

1. rectangle catalog 必須與 14 個 event keys 雙向一對一；x／y／width／height 精確等於原版
   event 的 column×8、row×8、length×8、8。
2. 16×16 字型、單列、`single-line-reject`；缺字、容量不足、非 2／3 倍率都失敗即關閉。
3. normal／selected variants 各有自己的 event key 與同位置矩形；原版清除 hook 必須先失效
   舊 stamp，再由新 request 建立新 stamp，終態同位置只能有一個 active key。
4. presenter 只改輸出 RGBA，不改原版 framebuffer、palette、events、輸入或技能數值。

## 驗收

- base／Down 各有 control；2×／3× 各雙重重播，presentation 欄位外等於 control。
- 矩形外及 x≥184 的動態數值區差異為 0；所有 action 均有完整字模且 contained。
- base 終態保留 headers、七個非選取 normal skills 與 notice selected；Down 終態改為 notice
  normal、maneuver selected，其餘不變，沒有同位置 normal／selected 並存。
- 正式資料測試、Go 正反例、全套 test、vet、race detector 與原始解析度目視通過後升 CONFORMED。

## READY 審查

request、選取列重畫、通用清除 hook、動態欄起點與雙倍率 renderer 都已有 CONFORMED 證據；
本規格不擴張矩形、不改技能規則或選定產品倍率，可以進入實作。

## CONFORMED 收據

- base／Down × 2×／3× 各雙重重播，JSON、RGBA、baseline 與 raw framebuffer 決定性一致。
- 2×／3× 矩形內差異為 11,280／22,791 px；矩形外與動態數值欄均為 0 px。
- base 與 Down 的 active keys 符合 normal／selected 取代契約；姓名提示已在轉場失效。
- 127 項專案 Python 測試通過；dosgolem 正式套件 test／vet 與相關 race 驗證通過。

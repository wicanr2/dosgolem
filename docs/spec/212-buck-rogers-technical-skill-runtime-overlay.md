# 212 — Buck Rogers 技術技能配置執行期繁中覆繪

狀態：CONFORMED

## 幾何證據

第 211 規格已證實技術技能頁新增 17 個非重複 exact identities：兩個專屬標題、13 個一般技能列
及前兩列 selected variants。每筆事件都有原版 row、column 與 original length；points／bonus／total
動態欄從文字 column 23／29／35（x=184／232／280）開始。另兩個標題和職業技能頁共用完全
相同 identity，直接沿用第 210 規格已 CONFORMED 的矩形，不重複定義。

## 契約

1. technical rectangle catalog 必須與 17 個 technical event keys 雙向一對一；x／y／width／height
   精確等於 column×8、row×8、original length×8、8。
2. 啟用 technical overlay 時必須同時提供 career catalog 與 career rectangles；合併後每個 identity
   只有一個 catalog entry 與一個 rectangle。
3. 16×16 字型、單列、`single-line-reject`；缺字、容量不足、孤兒／缺漏矩形或非 2／3 倍率都
   失敗即關閉。
4. normal／selected variants 各有自己的 event key，但同一位置由原版清除 hook 先失效舊 stamp，
   終態不可並存。
5. presenter 只改輸出 RGBA，不改原版 framebuffer、palette、events、輸入或技能數值。

## 驗收

- base／Down 各有 control；2×／3× 各雙重重播，presentation 欄位外等於 control。
- 矩形外與 x≥184 動態數值區差異為 0；所有 action 都必須 contained 且無缺字。
- base 終態第一列 selected、第二列 normal；Down 終態第一列 normal、第二列 selected，且其餘
  11 列、四個標題都各保留唯一 active key。
- 正式資料正反例、Go 旗標與 coverage 正反例、完整 test／vet／race 及原始解析度目視通過後，
  才可升為 CONFORMED。

## READY 審查

request、技術頁入場與 Down 重畫、共享 identity、動態欄邊界、通用清除 hook及雙倍率 renderer
都有 CONFORMED 證據。本規格不擴張矩形、不改技能規則、不新增 selected variant，也不選定產品
預設倍率，可以進入實作。

## 實作回退發現

第一輪以第 72 階段 request 收據的停止步數取終態時，active keys 已正確取代，但技術技能
selected stamp 仍是 `Pending`，輸出圖露出原版英文。規格因此退回 DRAFT。延後 100,000 steps
跨過下一次垂直回掃後，沒有新增文字事件，selected stamp 進入 `Shown` 並正確顯示繁中；這是
收據停止點過早，不是 renderer 或清除 hook 缺陷。規格現重新審查為 READY；正式覆繪收據必須
停在下一個穩定 frame，且人工圖像驗收不得由 active key 或 containment 數字取代。

## 權利與停止線

正式資料只保存 content-safe 雜湊、幾何與繁中譯文，不保存英文全文、原版 framebuffer 或掃描
手冊。若譯文超出 exact 矩形、共享幾何不一致或清除生命週期不成立，規格退回 DRAFT，不以截斷、
縮寫或技能專屬清除特例補洞。

## CONFORMED 收據

- base／Down 使用 103,500,000／103,800,000 的穩定 frame 停止點；各自 control、2×、3×
  雙重重播的 JSON、RGBA、baseline 與 raw framebuffer 逐位元一致。
- base 為 289 events／32 requests／257 misses；Down 為 297／34／263。兩倍率的 presentation
  欄位外 projection 與 control 相同，原版 framebuffer 雜湊維持 Phase 72 基準。
- 2×／3× 矩形內差異都是 16,611／34,423 px；矩形外與動態數值欄均為 0 px。
- base 終態第一列為繁中 selected，Down 終態第二列為繁中 selected；四張正式 PNG 已目視確認
  無英文殘字、裁切、列碰撞或數值污染。
- 專案 136 項 Python 測試、dosgolem 全正式套件 test、`go vet` 及相關 race detector 全數通過。

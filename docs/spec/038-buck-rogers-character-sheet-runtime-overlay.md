# Buck Rogers 角色資料頁明示倍率執行期繁中覆繪

狀態：CONFORMED

## 範圍

本規格核准把 spec 037 的 35 個角色資料靜態 `DisplayRequest` 與專案正式
`character-sheet-text-safe-rects.tsv` 接到既有 `RuntimeMenuOverlay`。呼叫端仍須明示 2 或 3；
不設定產品預設倍率，不翻譯或清除動態姓名、身分、摘要、能力值、技能值與重擲回答，也不改
原版 indexed VRAM、CPU、亂數、輸入、檔案或存檔。

## 幾何契約

1. 33 個事件的清除矩形等於原文字串矩形；`AC` 與 `THAC0` 兩筆可由原文起點向右延伸，但右界
   必須恰為同列動態值 col 35 的左界。這兩筆延伸是具名、受限例外，不得泛化為任意加寬。
2. 所有矩形高 8 logical pixels、8-pixel 對齊、單列、`single-line-reject`；譯文超過容量、缺字、
   同列重疊、越過 320×200 或碰到動態值均失敗即關閉。
3. `LoadCharacterSheetOverlayRects` 是既有 `MenuOverlayRects` 的角色資料頁受限 loader；
   `RuntimeMenuOverlay.Apply` 只有由此 loader 標記的 `AC`／`THAC0` event key 可接受大於
   `OriginalLength*8` 的寬度，其他 catalog 仍要求完全相等。

## 命令列與生命週期

- `buckrogers-text-receipt` 新增 `-character-sheet-rects`。啟用 character-sheet catalog 的 overlay
  模式必須同時提供此表；孤兒表、缺表、缺字型、缺 output 或非 2／3 倍率均拒絕。
- guarded post-call 產生 request 後立即 Apply；`026F:029C` typed clear 與真實 frame clock 契約
  完全沿用 spec 035／036。
- 同一技能 label 可被原版重畫多次；每次 request 必須有一筆 action，但 layer 使用同 event key
  replace，不得累積重複 stamp。

## 驗收

- 由權威 state 正常排入四次 Enter；另一分支再排入 `Y`，2×／3× 各 fresh 重跑兩次。
- base 預期 118 events／43 requests；`Y` 預期 149 events／52 requests。requests 與 actions
  一一對應，只有第 64 階段既有非 catalog 動態事件形成 miss。
- 同倍率同分支 JSON／RGBA 逐 byte 相同；raw framebuffer 與無覆繪 baseline 相同。
- `-baseline-rgba-out` 可在同一終態與相同 palette 寫出未套 stamp 的縮放 RGBA；它必須與
  `-overlay-rgba-out` 同時使用，只供 containment 對照，不改 machine 或 receipt 語意。
- 所有 action `Contained=true`，安全矩形外 RGBA 差異為零；原始解析度 PNG 人工確認動態值、
  重擲回答與金框未被覆蓋，沒有半字、殘字或跨列。
- 正式 Go packages test、相關 `vet`／race detector 與專案完整 Python 回歸通過後，才可升為
  CONFORMED。

## 停止線

若真實 RGBA 顯示 `AC`／`THAC0` 延伸矩形碰到動態值，或 `Y` 重畫後 stamp 生命週期不一致，
本規格退回 DRAFT 並補事件證據；禁止縮寫正式譯文、遮掉動態值或以終態 key 硬刪。

## CONFORMED 收據（2026-09-21）

- base 四次 Enter：118 events／43 requests／75 misses；`Y` 分支：149 events／52 requests／
  97 misses。兩分支的 2×／3× 各 fresh 重跑兩次，request 與 action 一一對應，終態皆為
  35 個唯一 active keys。
- base 2×／3× JSON：`bd22f043…7e8f5`／`01775430…a7c67`；`Y`：
  `9e91f700…0422e`／`ed735717…46717`。同組兩次逐 byte 相同。
- 2×／3× overlay RGBA：`e140f249…3def`／`5706ef26…aa4b`；同 frame/palette baseline：
  `3403cae2…44b4`／`5a357214…d980`。差異像素分別 18,379／37,048，安全矩形外皆為 0。
- raw framebuffer 仍為 base `1f3b8194…d4dd`、`Y` `03d9bf1f…0f97`，逐 byte 等於既有原版
  收據。74-glyph GOLEMFNT 為 `5949e5b2…b2dc`，零缺字。
- PNG 人工確認「命中指數」、35 個靜態欄位、金框與 `ES` 回答沒有裁切、跨列或重疊。此終態
  dosgolem palette 將前景色 15 顯示為黑色，因此原版本來的動態值在 baseline 亦不可見；
  raw framebuffer 的 base／`Y` 差異仍存在，且矩形外零差異證實中文層沒有清除它們。

因此本規格在上述固定輸入與正常路徑範圍升為 CONFORMED；不選定產品預設倍率，也不把
palette 限制誤記為翻譯完成。

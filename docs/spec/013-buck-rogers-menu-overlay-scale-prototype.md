# Buck Rogers 功能選單覆繪倍率診斷工具

狀態：**CONFORMED**

本規格只核准一個離線、可重生的 A/B 診斷命令。命令把既有 indexed framebuffer、RGB
palette、GOLEMFNT、正式 menu event／譯文／text-safe rectangle TSV 交給現有 `xlate.Draw`，
分別產生 2× 或 3× PNG 與 content-safe JSON 收據。它不掛 runtime hook、不送輸入、不改
原版狀態，也不是 production renderer。

## 證據與輸入

- 原版畫面來自 spec 011 的 `-screen-out`，恰為 64,000-byte 320×200 indexed buffer。
- `.pal` 必須是 dosgolem `cmd/probe.writeShot` 寫出的 768-byte `machine.Palette()` 結果，已是
  8-bit RGB；禁止再次套用 VGA DAC 6→8 位元轉換。
- `menu-events.tsv`、`menu.zh-TW.tsv` 與 `menu-text-safe-rects.tsv` 必須使用完整固定 header；
  event key 精確連到 text key、色號、清除矩形、draw anchor 與容量。
- GOLEMFNT 由現有 `xlate.LoadFont` 讀取；缺字必須失敗，不得靜默留白或換字。

輸入與來源檔 SHA-256 都必須寫入收據；原版 framebuffer、palette 與字型二進位不進版控。

## 支援畫面

命令只接受下列兩個已證實的固定畫面集合：

- `steady`：提示、selected Terran、其餘五筆 normal 選項；
- `down`：提示、normal Terran、selected Martian、其餘四筆 normal 選項。

集合內 event key 由正式 catalog 精確解析。普通選項的清除矩形由 col 1 起、中文 anchor 在
col 3；工具須以兩個全形空白格保留 draw offset，使 `xlate.Stamp` 清除完整原文、只從正式
anchor 畫字。不得用座標或 text key 模糊挑選 variant。

## 倍率與色彩

- 只接受 scale 2 或 3，皆以最近鄰整數放大原版 RGB framebuffer。
- 2×：16×16 字模填滿每個 16×16 輸出格，`GlyphX=GlyphY=0`。
- 3×：16×16 字模置中 24×24 輸出格，`GlyphX=GlyphY=4`。
- 背景／前景 RGB 由 event 的原版色號索引同一份 8-bit palette；不得自訂主題色。
- 原版固定終點中 selected row 可因原版色彩狀態成為黑底黑字；診斷工具必須忠實保留，
  不得為了展示效果改成自選高亮色。

## 輸出與失敗模式

每次執行必須同時輸出未覆繪 PNG、繁中 PNG 與 JSON。JSON 不保存英文或中文全文，只保存：

- 工具識別、畫面、倍率、canvas 尺寸與各輸入 SHA-256；
- event／text key、譯文字數、色號、清除矩形、draw anchor、ink rectangle 與 containment；
- 缺字、矩形重疊、安全矩形外差異與兩張 PNG SHA-256。

遇到尺寸、header、數值、容量、overflow policy、缺 event／rect／translation、缺字、ink 越界、
矩形重疊或安全矩形外差異時一律非零退出，不得留下可誤認成功的 JSON。

## 驗收

- `go test` 覆蓋畫面 event 集、8-bit palette 不二次轉換及幾何失敗即關閉 helper。
- 同一組輸入完整執行兩次，所有 PNG／JSON 逐 byte 相同。
- steady／down 的 2×／3× 共四份收據皆為零 missing glyph、零 overlapping rectangle、零
  safe-rectangle 外差異，且每筆 ink contained。
- 由人實際檢視原生像素 PNG；Golden Box PC-98 只提供 640×400、約 16×16 CJK cell 的
  比較證據，不是本作色彩、框線或行為 oracle。

符合後可把本規格標為 CONFORMED。正式倍率仍須由使用者決定；本工具通過不得被解讀為
任一倍率已獲採用，亦不得宣稱玩家可見中文化完成。

## 符合性紀錄（2026-09-21）

- `cmd/buckrogers-overlay-prototype/main.go` SHA-256：
  `ad65d369057225848709b94a0c3b3d667087bbc42626ea636257f98aed076a0c`；測試檔 SHA-256：
  `c63c8388866597e025e1464e354e0e4850fc4f83b4e05b1dd1959c413dfac99a`。
- steady／down × 2×／3× 各完整重生兩次，兩批所有 PNG／JSON 逐 byte 相同。四份 JSON
  SHA-256 依序為 steady 2× `ae796fc1…ed13`、steady 3× `06c342ab…97140`、down 2×
  `0e9ee862…6d3`、down 3× `2c0dd1d0…758`。
- 每份收據恰有七個 visible event，零 missing glyph、零 overlapping rectangle、零
  safe-rectangle 外差異，且所有 ink rectangle contained。
- 2× PNG 為 640×400、3× PNG 為 960×600；已在原生像素實際檢視。無插值並列圖
  SHA-256 為 `c03f865e0d9b2f122a99f6d9007d7f12e2e9adaf83e0fb906b501b13e17babc1`。
- dosgolem 排除既有非正式 `workplace/` 草稿後，全部正式 packages test／vet 通過；
  `apps/buckrogers` 與本命令的 race detector 通過。
- 目視結果：2× 以 16×16 CJK 字填滿放大格，與 PC-98 Golden Box 常見 640×400／約
  16×16 cell 相符；3× 保持 24×24 advance、16×16 ink，留白較多。這是比較證據，不是
  使用者倍率決策。selected row 的黑底黑字沿用原版固定終點色彩，未人工美化。

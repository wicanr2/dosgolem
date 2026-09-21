# 215 — Buck Rogers 技能操作列執行期繁中覆繪

狀態：DRAFT
日期：2026-09-21

## 目標與邊界

將 spec 214 已 CONFORMED 的技能操作列 `DisplayRequest` 接入既有 `xlate.Layer`，在 2×／3×
明示倍率輸出中清除原版英文墨跡並繪製繁中。本規格只改 presentation RGBA；不得寫回 indexed
framebuffer、VRAM、DOS 記憶體、鍵盤、技能點、焦點或存檔。

## 已證實輸入與幾何

- 原版 `START.EXE` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`。
- `text/skill-action-bar-events.tsv` SHA-256：
  `99550186d923590e37289955baf8d867c6f02da16f61e05b3f7a7ea2a508f4a0`。
- 八個配置的原文矩形均為 `y=192, height=8`；career 為 `[0,24)`、`[32,96)`、`[104,136)`，
  technical 另有 `[144,176)` 與 `[184,216)`。矩形互不重疊，五筆譯文皆為兩個 Unicode rune。
- 既有 16×16 GOLEMFNT 在 2× 輸出採 glyph scale 1、offset 0；在 3× 採 glyph scale 1、offset 4。
  因此 ink 可被 8 logical-pixel 高矩形容納。擴張成 `y=184..200` 會侵入原版底框，不允許。
- normal 原版是黑底、第一字元 palette 15、其餘字元 palette 10；focus 是 palette 15 底、
  palette 0 字。normal 繁中要延續混色或改為單一綠色仍是待使用者確認的產品決策。

## 擬定 typed 契約

- 新增 action-bar 專用安全矩形 TSV；event key 必須與 spec 214 的 16 個 normal／focus identities
  雙向覆蓋，normal／focus 可有相同幾何但不得重複 event key。
- 每筆固定單列、`height=8`、`draw_y=192`、`single-line-reject`；x／width 必須等於 event 的
  exact x0／x1，capacity 必須等於 width／8，兩字譯文必須完整容納。
- action overlay 的 `Apply` 必須以完整 `ActionBarEvent` 與 request 同時驗證 screen、variant、
  hash、長度、row／column、x0／y0／x1／y1 及 guarded post-call，再建立 `Pending` stamp。
- 同一原點的新 normal／focus stamp 必須取代舊 stamp；`026F:029C` clear 與離開技能頁的 exact
  anchor lifecycle 必須清掉相交 stamp。不得用 wall-clock timeout 清除。
- loader 拒絕 schema、缺少／孤兒／重複 key、越界、重疊、容量不足與字型缺字。

## 尚未 READY 的決策

normal 繁中文字色尚未確認。兩個可丟棄 prototype 已以真實 technical／career framebuffer
產生：A 延續首字白、次字綠；B 使用全綠。此決策只影響 presentation style，但會改變玩家
看到的命令提示，因此在使用者確認前不得寫入 production 規則。

## READY 後驗收

1. 純核心覆蓋 16 identities、兩倍率 containment、same-origin replacement、clear／transition
   invalidation，以及所有 loader 負向案例。
2. career base／Subtract／Done 與 technical base／Subtract／Prev／Next／Done 至少抽取能覆蓋
   normal、focus、焦點移動及轉場的正常玩家路徑；每條 2×／3× 各雙重播。
3. 原版 indexed framebuffer 與 control 逐 byte 相同；RGBA 差異只在核准矩形內；穩定 frame
   人工檢查無英文殘字、裁字、框線侵入或繁中 stamp 堆疊。
4. 未選定產品預設倍率；disabled 維持 unknown；手冊版面不受本規格影響。

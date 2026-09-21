# 215 — Buck Rogers 技能操作列執行期繁中覆繪

狀態：READY
日期：2026-09-21

## 目標與邊界

將 spec 214 已 CONFORMED 的技能操作列 `DisplayRequest` 接入既有 `xlate.Layer`，在 2×／3×
明示倍率輸出中清除原版英文墨跡並繪製繁中。本規格只改 presentation RGBA；不得寫回 indexed
framebuffer、VRAM、DOS 記憶體、鍵盤、技能點、焦點或存檔。

## 已證實輸入與幾何

- 原版 `START.EXE` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`。
- `text/skill-action-bar-events.tsv` 的八筆 content-safe identity 仍是原版輸出權威；正式實作
  必須由專案測試核對其版本，不能以本規格內的舊快照雜湊放寬漂移。
- 八個配置的原文矩形均為 `y=192, height=8`；career 為 `[0,24)`、`[32,96)`、`[104,136)`，
  technical 另有 `[144,176)` 與 `[184,216)`。矩形互不重疊，五筆譯文皆為兩個 Unicode rune。
- 既有 16×16 GOLEMFNT 在 2× 輸出採 glyph scale 1、offset 0；在 3× 採 glyph scale 1、offset 4。
  因此 ink 可被 8 logical-pixel 高矩形容納。擴張成 `y=184..200` 會侵入原版底框，不允許。
- normal 原版是黑底、第一字元 palette 15、其餘字元 palette 10；focus 是 palette 15 底、
  palette 0 字。使用者已決定保留括號內拉丁助記字母：正式顯示為 `(A)加點`、`(S)減點`、
  `(P)上頁`、`(N)下頁`、`(D)完成`。normal 只有字母為 palette 15，括號與中文為 palette 10；
  focus 全組為 palette 0 字／palette 15 底。

## 擬定 typed 契約

- 新增 action-bar 專用安全矩形 TSV；event key 必須與 spec 214 的 16 個 normal／focus identities
  雙向覆蓋，normal／focus 可有相同幾何但不得重複 event key。
- 每筆固定單列、`height=8`、`draw_y=192`、`single-line-reject`。ASCII 前進 4 logical pixels，
  CJK 前進 8，因此五字顯示寬 28。除 `action.add` 外 x／width 等於 event exact x0／x1；Add
  核准使用原版 `[24,32)` 空白，presentation clear rect 為 `[0,32)`。其餘標籤起點不得移動。
- action overlay 的 `Apply` 必須以完整 `ActionBarEvent` 與 request 同時驗證 screen、variant、
  hash、長度、row／column、x0／y0／x1／y1 及 guarded post-call，再建立 `Pending` stamp。
- 同一原點的新 normal／focus stamp 必須取代舊 stamp；`026F:029C` clear 與離開技能頁的 exact
  anchor lifecycle 必須清掉相交 stamp。不得用 wall-clock timeout 清除。
- loader 拒絕 schema、缺少／孤兒／重複 key、越界、重疊、容量不足與字型缺字。

## READY 決策與證據審查

Phase 77 的「首中文字白／次字綠」及「兩字全綠」prototype 已被使用者決定取代。Phase 80
以真實 career／technical framebuffer 及正式 Unifont 來源重生兩倍率 prototype；28-pixel
墨跡均在核准矩形內，Add 擴張不碰 x=32 的下一標籤，也不侵入 y<192 金框。可見 A／S／P／N／D
身分已證實，但直接按字母是否執行命令仍未知；renderer 只保留顯示，不改輸入語意。

## READY 後驗收

1. 純核心覆蓋 16 identities、兩倍率 containment、same-origin replacement、clear／transition
   invalidation，以及所有 loader 負向案例。
2. career base／Subtract／Done 與 technical base／Subtract／Prev／Next／Done 至少抽取能覆蓋
   normal、focus、焦點移動及轉場的正常玩家路徑；每條 2×／3× 各雙重播。
3. 原版 indexed framebuffer 與 control 逐 byte 相同；RGBA 差異只在核准矩形內；穩定 frame
   人工檢查無英文殘字、裁字、框線侵入或繁中 stamp 堆疊。
4. 未選定產品預設倍率；disabled 維持 unknown；手冊版面不受本規格影響。

## 歷史 DRAFT 配色中立核心收據（2026-09-21）

- 正式 `skill-action-bar-text-safe-rects.tsv` 已展開 16 個 normal／focus event keys；loader 與
  coverage validator 固定 exact band、容量及雙向 coverage。
- `BuildActionBarOverlay` 不提供 normal 預設配色；caller 必須為每個譯文 rune 明示 palette
  10 或 15。缺少、長度不符或未證實色號皆拒絕。focus 固定使用 palette 15 底／0 字。
- 首字 15／次字 10 與兩字 10 兩個候選均以同一核心在 2×／3× 通過 ink containment；
  這只證明兩方案技術可行，不構成 production 選擇。
- dosgolem 全套 test、vet 與 Buck Rogers race detector 通過。CLI／runtime presenter 尚未接線，
  因此本 spec 仍為 DRAFT。

## 歷史 DRAFT runtime lifecycle 收據（2026-09-21）

- `RuntimeActionBarOverlay` constructor 強制 caller 注入 normal 配色，並在每次 `Frame` 完成
  `xlate` 指紋／錨點初始化後重新套用該明示 palette；因此 `[10,10]` 不會被原版首字 15 覆寫。
- 同 action normal／focus 以 exact rectangle 整組取代，其他 action 保留；partial clear 會將
  不完整 multi-stamp group 全組移除，不留下半個繁中文字。
- career↔technical exact anchor 切換及 unrelated event 清空全部 action groups；technical 的兩個
  已證實共享 career heading 仍保留 group，與 spec 213 watcher 契約一致。
- 兩候選、2×／3×、frame／draw、非法 clear、unanchored 與 identity drift 純核心測試通過。
  正式 package test、排除未版控 `workplace/` probes 的 vet，以及 Buck Rogers race 通過。
- 當時正式 CLI 尚未接線且 normal 配色待確認，所以該輪維持 DRAFT；此歷史狀態已由下節取代。

## READY 混合寬度核心收據（2026-09-21）

- 正式 catalog 已包含括號、助記字母與兩字繁中；矩形 verifier 以 ASCII 4／CJK 8 logical pixels
  計算容量，只允許 Add 多用一個已證實空白格。
- `BuildActionBarOverlay` 與 `RuntimeActionBarOverlay` 只接受
  `[10,15,10,10,10]`；遺失白色字母或改染中文字均失敗即關閉。五個 rune 使用混合前進，仍以
  action group 原子取代、清除及失效。
- 2×／3× 核心與 presentation lifecycle 測試已通過；正式 CLI 與完整正常玩家路徑 A/B 尚未完成，
  因此本規格是 READY，不是 CONFORMED。

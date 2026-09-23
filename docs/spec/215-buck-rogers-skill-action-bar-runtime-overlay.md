# 215 — Buck Rogers 技能操作列執行期繁中覆繪

狀態：**限縮 CONFORMED；僅已量的八條操作列焦點路徑與技術技能頁 Escape→Y 離頁，2×／3×。**
日期：2026-09-21

2026-09-24 勘誤與驗收：較早的 READY 原型只清掉 28 logical-pixel 譯文字格，
`Subtract` 等較長原版標籤右側仍留下英文；矩形外零差並不能證明英文
完整清除。正式 `RuntimeActionBarOverlay.Draw` 現在先以已核准安全矩形
的原版背景色清底，再畫括號、白色字母與繁中；只改 RGBA。下文
「READY 後驗收」保留准入契約，實際限縮結論見末節。

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
  technical 另有 `[144,176)` 與 `[184,216)`。矩形互不重疊，五筆譯文各為三個 ASCII
  與兩個中文字，共五個 Unicode rune。
- 2× 仍用 16×16 GOLEMFNT 字模，offset 0；使用者於 2026-09-22
  明確要求 3× 中文更大、間距更緊，故正式 3× 將**中文字**由本機
  16×16 倚天字模在記憶體最近鄰放大為 22×22，置於 24×24 字格
  offset `(1,1)`；ASCII 字母與括號維持 16×16、offset `(4,4)`。
  這訂正舊版「所有 3× 字皆 16×16、offset 4」的過時敘述；
  來源是 Buck Rogers 專案的[第九十九階段收據](../../../../docs/re/phase-99-action-bar-3x-density.md)。
  ink 仍在 8 logical-pixel 高矩形內；擴張到 `y=184..200` 會侵入底框，不允許。
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
- 每個已顯示的 action group 應先以其**完整核准清除矩形**及 exact normal／focus
  背景色清除原版英文，再繪製 28 logical-pixel 的混合寬度譯文。未全數達
  `Shown` 的 group 不得部分清底或部分顯示；清底不可碰相鄰 action、
  金框、DOS indexed framebuffer 或 VRAM。
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

## 2026-09-24 已量路徑的限縮 CONFORMED

本機 Buck Rogers 專案的[第二百零四階段](../../../../docs/re/phase-204-action-bar-runtime-conformance.md)
從同一合法角色建立 state，沿
Phase 75 已固定的鍵序重生 career base／Subtract／Done、technical
base／Subtract／Prev／Next／Done 八條路徑，另沿已證實 Escape→Y
從技術頁離開。每條 control、2×、3× 各雙重播；修正前後的 control
JSON SHA-256 相同。修正後每路徑原版 JSON（排除新增的 presentation
欄位）與 indexed 位元組相同，RGBA 差異只在核准操作列矩形；正常態
白色字母、原色中文字與 focus 原版白底黑字的 style receipt 均符合
本規格。各較長原版標籤的譯文後尾部現在是單一背景色，修正前
收據會被相同 verifier 拒絕；修正前後唯一新增的 RGBA 像素變更
都在該尾部。雙倍率 PNG 目視已確認沒有殘英文、裁字或底框侵入。
技術 Escape→Y 離頁後 active action group 為零、覆繪 RGBA
逐位元組等於 baseline。正式 Go 定向 test、vet、race 與獨立審查通過。

此結論**不**證實 disabled variant、其他未量技能操作／重新進入、
原版存讀檔、冷開機全流程或 Linux 視窗。早期 READY 與 phase99
像素隔離只證當時範圍，不再作為「英文已完整清除」的證據。

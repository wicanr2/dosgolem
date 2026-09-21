# 213 — Buck Rogers 技能配置底部操作列字元事件

狀態：CONFORMED
日期：2026-09-21
證據審查：2026-09-21
實作中訂正：2026-09-21 首條 runtime 收據曝露局部清除誤失效，規格曾退回 DRAFT；
第二次重跑又證實 row 24 本身會在同畫面重畫前清除。因此分開 screen anchor 與
candidate 生命週期：清除只丟棄未完成候選，exact 畫面事件才建立／取代／撤銷
screen anchor。第三次重跑依 Phase 74 原始 JSON 訂正 `[BP+06]` 低位 mode
為 1，不是 0。technical 首條收據又重現 Phase 72 已證實的兩個 career 共享標題
identity；將它們列為 technical anchor 的精確保留 key，而非泛化前綴。契約與
負向測試再次審查為 READY。

## 目標

為 `apps/buckrogers` 定義可將底部逐字 glyph 呼叫收旂成 content-safe 操作標籤事件的
watcher，供後續繁體中文輸出端覆繪使用。不修改原版記憶體、鍵盤輸入、技能點或
選單控制流。

## 已證實契約

- 原版逐字進入 runtime `0763:026B`，最終由 `0763:1809` 輸出 8×8 glyph。
- 參數可取得 glyph、repeat、background、foreground、row 與 column；只使用低位 byte。
- 標籤皆在 row 24；職業頁為 17 字元，技術頁為 27 字元。
- 焦點 caller 是 `37F1:0337`；一般快捷鍵首字是 `37F1:0391`，其餘字是
  `37F1:03CE`。caller 只是 guarded identity，不是可重命名的語意事實。
- 八個畫面配置的長度、SHA-256、字格座標與色彩來自主專案
  `text/skill-action-bar-events.tsv`。

## 輸入版本與位址空間

- 原版 `START.EXE` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`。
- runtime `0763:0000` 8 KiB 快照 SHA-256：
  `436711fefc7071fcaf0811ef0b4243a5f2fd42840ca0f3b11aaaf121454deac6`。
- 正式 action catalog SHA-256：
  `99550186d923590e37289955baf8d867c6f02da16f61e05b3f7a7ea2a508f4a0`。
- 執行位址一律是 dosgolem real-mode `segment:offset`；IDA 匯出的 database EA
  等於 runtime segment `0763` 的 offset。不將 IDA linear address 與 runtime 地址混用。
- 證據工具為 IDA Pro 9.4 與 dosgolem branch `buck-rogers-cht-output-overlay`；
  主專案證據見 `docs/re/phase-74-skill-action-bar-output-path-inventory.md`。

## typed 輸入、狀態與輸出

`GlyphCall` 只接受已完成 far return guard 的單字呼叫，欄位為：

- `entry_step`、`post_call_step`、`caller`、`mode`、`glyph`、`repeat`；
- `background`、`foreground`、`row`、`column`。

watcher 狀態只有：

1. `unanchored`：不收集任何 glyph。
2. `anchored(screen)`：由已通過 exact catalog 的技能畫面標題設定 `career`
   或 `technical`。
3. `candidate(identity, index, bytes)`：只有已錨定畫面的已知起始 column
   才能建立；下一字必須在同 row 且 column 連續。

候選長度達正式 identity 後計算 SHA-256。只有雜湊、每字 caller，每字色彩、
`mode=1`、`repeat=1`、row、column 全部 exact match 才輸出 `ActionBarEvent`：
`screen`、`event_key`、`variant=normal|focus`、步數範圍、長度、SHA-256 及矩形。
輸出不含原文 bytes。

## 錨定、失效與失敗模式

- career 錨定只接受 exact request `career.screen.remaining_points.heading`；technical
  錨定只接受 `technical.screen.general_points.heading`。純看 glyph 內容不能啟用。
- `026F:029C` 同時服務畫面內局部清除，包含同一技能畫面重畫操作列前的
  row 24 清除。它會丟棄未完成 candidate，但不會單獨撤銷 screen anchor。
- 已錨定時，同一 `career.screen.*` 或 `technical.screen.*` exact request 維持錨定；
  technical 另精確保留 Phase 72 已證實的共享 key
  `career.screen.maximum_per_skill.heading` 與 `career.screen.columns.heading`。另一技能畫面的
  標題取代錨定；任一其他 exact request 撤銷錨定。只靠原文 glyph 不能建立或延長
  screen anchor。
- 重疊 glyph entry、far-return SS/SP 不符、錨誤 mode/repeat/caller/色彩/座標、
  跨列、跳號或錨定失效：丟棄候選，不輸出事件。
- 在已錨定畫面中，來自已知起始 column 但不能 exact resolve 的完整候選計一次
  miss；其他畫面文字與間隔空白不計 miss。
- watcher 只讀 CPU 與堆疊 snapshot，沒有鍵盤、原版記憶體、VRAM 或覆繪寫入能力。

## READY watcher

1. 只在 Buck Rogers adapter 與已證實技能畫面錨定啟用。
2. 以同列、連續 column 收集字元；遇到座標斷裂、呼叫端不合法、repeat 不為 1
   或畫面錨定失效時，丟棄候選事件。
3. 完成候選後只輸出長度、SHA-256、座標、色彩與 variant，不把原文加入日誌或收據。
4. 只接受 exact catalog hit；任一字元、順序、座標、色彩或 caller 漂移都維持 miss。
5. 原版照常繪製；watcher 本身不清畫面、不覆繪、不改操作結果。

## 驗收

- 主專案清冊 verifier 與正反向單元測試必須通過。
- watcher 純核心必須重建職業三標籤與技術五標籤，並拒絕部分序列、串接跨列、
  未知 caller 與非 exact 輸入。
- base 及所有 Right 焦點路徑必須各重跑兩次，產生與基準數量相符的事件，
  JSON 收據決定性一致，而 framebuffer 逐 byte
  與無 watcher 對照組一致。
- disabled 畫法目前是 `unknown`；不將一般狀態自動命名為「可用」，也不擴張
  catalog。若未來有正常輸入證據，另以新 identity 審核。

## 垂直鏈、已知差異與權利邊界

- 垂直鏈是原版 glyph 呼叫 → guarded call snapshot → collector → exact catalog
  → content-safe event → receipt JSON。此階段不會影響存檔或玩家輸入。
- 已知差異：disabled variant 未知，維持 miss／無事件；本 spec 不提供譯文或覆繪。
- 原版遊戲與收據留在使用者本機，不加入 Git。正式 catalog 只保存雜湊與
  metadata，無法還原原作文字。

## CONFORMED 條件

READY 實作、負向測試、決定性 runtime 收據與原版 framebuffer 零差異全數通過後，
才可升為 CONFORMED。繁中 overlay 是後續獨立規格，不屬本檔完成聲明。

## CONFORMED 收據

- `ActionBarWatcher` 已實作 exact heading anchor、共享 key allowlist、候選失效、
  `0763:026B` far-return SS/SP guard、每字 caller／色彩／座標檢查及 SHA-256 resolver。
- 純核心測試覆蓋 career 3 標籤、technical 5 標籤、未錨定內容、部分序列、
  mode/repeat/row/column/caller/SS/SP 漂移、未知雜湊、清除、共享 key 與非相關 key。
- career base／Subtract／Done 及 technical base／Subtract／Prev／Next／Done
  八條正常路徑的 watcher A/B JSON 逐 byte 一致，事件數依次為
  3、6、9、8、13、18、23、28，全部 0 miss、0 drop。
- 八條 watcher A/B/control 的 64,000-byte indexed framebuffer 各自逐 byte 一致；移除
  `action_bar_*` metadata 後，watcher 與 control 的其餘 JSON 語意收據一致。
- 主專案 Python 142 項回歸通過；dosgolem 全部正式套件 test／vet 通過，
  相關套件 race detector 通過。原始收據留在主專案被忽略的 `workplace/phase75/`。

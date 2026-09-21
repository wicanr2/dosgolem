# 214 — Buck Rogers 技能操作列繁中顯示請求

狀態：CONFORMED
日期：2026-09-21
證據審查：2026-09-21

## 目標與範圍

將 spec 213 已 CONFORMED 的 `ActionBarEvent` 以 exact catalog 解析為繁體中文
`DisplayRequest`。本 spec 只涵蓋 catalog loader、resolver、watcher request 佇列與 content-safe
receipt metadata；不清除原文、不繪製中文、不改鍵盤、技能點、焦點、存檔或
原版文字記憶體。

## 輸入版本與譯文來源

- `START.EXE` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`。
- `text/skill-action-bar-events.tsv` SHA-256：
  `99550186d923590e37289955baf8d867c6f02da16f61e05b3f7a7ea2a508f4a0`。
- spec 213：CONFORMED；正式 event 只包含 career 3 配置、technical 5 配置，
  每配置各有 normal／focus variant。disabled 仍是 `unknown`。
- 中文手冊與現有 catalog 沒有這五個英文介面按鈕的逐字對照。譯文因此分級為
  `runtime-interface`：`action.add=加點`、`action.subtract=減點`、`action.prev=上頁`、
  `action.next=下頁`、`action.done=完成`。
- 「加點／減點」沿用專案 Phase 45–47 玩家可見操作的既有用詞；「上頁／下頁／完成」
  是精簡的繁中介面詞。它們不冒稱中文手冊逐字譯名。

## typed catalog 與 resolver

譯文 TSV schema 固定為 `key<TAB>translation<TAB>source`，且：

- key 必須恰為上述五個，唯一、不可空白；source 必須為 `runtime-interface`。
- 譯文必須是 UTF-8、NFC，不得含 BOM、控制或格式字元。
- event catalog 的五個 key 與譯文必須雙向覆蓋；career／technical 與 normal／focus
  共用同一 text key，不複製譯文。

`Resolve(ActionBarEvent)` 只在下列欄位同時 exact match 時回傳一筆 `DisplayRequest`：

- screen、action key、variant、original length／SHA-256、row／column 與 x0／y0／x1／y1；
- `post_call_step > entry_step`；
- normal 或 focus 必須是 spec 213 已證實 variant。

request 包含穩定 event key、text key 與譯文，但譯文只流向輸出端佇列。resolver
不持有 machine／DOS／keyboard／VRAM 參考，不能影響原版語意。

## 失敗模式

- TSV schema、UTF-8、NFC、來源、key coverage、identity 唯一性或幾何不符：loader 拒絕啟動。
- 事件未完成、任一 identity 欄位漂移、未知 variant：resolver miss，不產生空白請求，
  不沿用前一筆譯文。
- nil catalog 保持純 event watcher 模式：事件繼續收集，request 與 catalog miss 都不增加。
- disabled 樣式未知，不以 normal 事件模糊代用。

## 驗收

1. loader 正負向測試覆蓋八個畫面配置、16 個 expanded identities、五個 text keys、
   重複、漏譯、孤兒、非 NFC／控制字元、證據與幾何漂移。
2. resolver 正向測試覆蓋 career／technical 所有 normal／focus；負向測試逐欄拒絕
   不完成、雜湊、畫面、key、variant 與幾何漂移。
3. Phase 75 八條路徑每條 A/B 重播；request 數必須等於 action event 數，且 request
   順序與 event 一對一、0 miss／drop。
4. 移除 request metadata 後，catalog 收據必須與無 request control 收據相同；三組 indexed
   framebuffer 逐 byte 一致。

## 垂直鏈與邊界

`0763:026B` → spec 213 guarded event → spec 214 exact resolver → display-only request
→ content-safe receipt。本 spec 到 request 為止；安全矩形、字型與 runtime overlay 必須以後續
獨立 READY spec 驗收。原版遊戲、字串與收據留在使用者本機，不加入 Git。

## CONFORMED 條件

上述內部測試、八路決定性收據、request／event 一對一、語意隔離與 framebuffer
非干擾全數通過後，才可升為 CONFORMED。

## CONFORMED 收據（2026-09-21）

- 八條路徑的 event／request 數為 3／6／9／8／13／18／23／28，全部 0 miss／drop。
- 每條 catalog 路徑 A/B JSON 相同；移除 request metadata 後與純 event control 相同。
- 每條 A/B/control 的 320×200 indexed framebuffer 逐 byte 相同。
- 正式 catalog、完整 `go test ./...`、`go vet ./...` 與相關 race detector 均通過。
  本規格只宣告 request 層 CONFORMED，不宣告已覆繪中文。

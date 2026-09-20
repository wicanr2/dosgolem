# Buck Rogers 保存、名冊與加入隊伍執行期繁中顯示請求

狀態：**CONFORMED**

## 範圍

本規格只核准把《Buck Rogers: Countdown to Doomsday》保存→功能選單→角色名冊→加入隊伍
正常玩家路徑的已證實靜態文字 identity，接到既有 `MenuCatalog` 與
`MenuRequestWatcher`。輸出只包含 typed `DisplayRequest`；不載入字型、不清除或繪製像素、
不送輸入、不修改遊戲狀態、保存檔或 FileOps，也不決定 2×／3×。

## 固定輸入與證據

- 專案完整 18-event 清冊 `text/save-roster-join-events.tsv`：
  `f178fae862f42eb6cc901251b97f2deea8dfa943ae17456f28208fee81adbcec`。
- 擴充後功能選單事件／譯文：
  `fddbd09ae363e986a2879013e384cdfef717f46fa47786bbad0cb96ae67bfcfc`／
  `16db36301675ef3c528ce6b37525463ab9e222305356fca9404d8242b91fa366`。
- 名冊新增靜態事件／譯文：
  `0f798b3246d1ba4860be6ed9b648bde8f3bdc8089f0f6b5533484ad23f8a7f89`／
  `d0adf666a27ed70f1d1dfeb9814255452d8dce1ba006fdbafef1bb7033e949a7`。
- 固定 `a-add.state`：
  `c89e94c0968b56206147d4542413e8eb97759e64ffd50f760d540c211016caf1`。
- 第五十四階段完整收據樣本 `b-joined.json`：
  `b6e9956d5dcbea8723673890bfef1a084112185ea27009ca45fe4502c74139ae`。
- 位址皆為 dosgolem DOS runtime `segment:offset`；不是 IDA 線性位址或檔案偏移。

## 證據結論

18 筆 guarded post-call 事件中有 14 筆靜態介面輸出與四筆動態姓名輸出。動態 bytes 已由原版
重播及 SHA-256 證實為 `A` 加 14 空白、反白／正常 `A`、以及 `* A`。它們只能形成 catalog
miss，不得以 fixture 姓名、caller 特例或前綴規則翻譯。

既有功能選單 catalog 負責前 11 筆事件；其中重複 redraw 仍以同一完整 identity 命中。
名冊 catalog 只新增兩個唯一 identity：`roster.add_prompt` 與 `roster.loading`；前者在正常
路徑出現兩次，應產生兩筆相同 key 的獨立請求。

## Typed 契約

1. 新增 `LoadRosterCatalog`，只提供檔名錯誤語境，解析、identity、唯一性、UTF-8 TSV 與
   `Resolve` 全部沿用 `loadExactCatalog`，不得複製解析器。
2. `buckrogers-text-receipt` 新增成對 `-roster-events`／`-roster-translations`；只給其中之一
   必須失敗。它與 menu／gender／class catalog 以 `MergeMenuCatalogs` 合併後交給唯一 watcher。
3. catalog 精確命中時，每個已完成事件至多產生一筆請求；未知 identity 只增加 miss。
4. 收據 request 仍只含 `event_key`、`text_key`、`translation_runes`，不得保存全文或姓名。
5. catalog 不得參與 DOS 記憶體、條件比較、路徑查找、保存檔名、序列化或玩家輸入。

## 正常路徑驗收

以第五十四階段相同 `save-before.state`、fresh scratch 與四筆既定 BIOS 輸入，跑至
#122,400,000：

- 只載入 menu＋roster catalog；完成事件恰為 18 筆，request 恰為 14 筆。
- 四筆動態姓名各形成一次 miss；總 miss 恰為 4，零 pending／drop。
- `roster.add_prompt` 產生兩筆請求，`roster.loading` 產生一筆。
- 兩次 fresh scratch 重播的正規化 JSON、framebuffer、FileOps、writes、保存檔結果一致。
- 與不載 catalog 的相同輸入相比，framebuffer、FileOps、writes 與保存檔逐 byte 相同。

## 測試與失敗模式

- `LoadRosterCatalog` 正常載入與 malformed TSV；完整 identity 任一欄漂移即不命中。
- roster 旗標成對驗證；沒有 roster flags 時保持既有命令相容性。
- catalog 合併 identity 衝突、錯誤 request／miss 數、pending／drop 均失敗即關閉。
- 專案 verifier 必須證明 runtime projection 來自完整 18-event 清冊，而非獨立手寫真相。
- 全部正式 packages test／vet、`apps/buckrogers` 與 receipt command race detector 必須通過。

## 權利與停止線

正式程式與收據不含原版素材、角色檔內容或譯文全文。原版執行檔、checkpoint、scratch 與
重播收據只留使用者本機、被版控忽略的工作目錄。完成本規格不代表玩家已看到中文；renderer、
字型、英文像素清除、生命週期與倍率選擇仍需獨立 READY 規格。

## READY 審查紀錄（2026-09-21）

- 由完整清冊重新計數為 18 events＝14 static＋4 dynamic；四筆 dynamic 的
  `translation_key` 均為空。
- 兩筆 runtime projection identity 均可逐欄回查完整清冊；重複出現的 add prompt 以同一
  identity 命中兩次，不製造重複 catalog identity。
- 共用解析器已具備 strict TSV、完整 identity、合併衝突與 miss 失敗即關閉契約；實作只需
  新增具名 loader 與成對旗標，不需要猜測原版行為或新增 watcher。
- 正常路徑 checkpoint、鍵盤步數、事件邊界、檔案收據與驗收數量皆已有第五十四階段原版
  oracle；沒有未決的行為、資料格式或玩家體驗取捨。

## 符合性紀錄（2026-09-21）

- `LoadRosterCatalog` 沿用 `loadExactCatalog`；收據命令只新增成對 roster 旗標，並以既有
  `MergeMenuCatalogs` 交給唯一 `MenuRequestWatcher`。
- 兩個 fresh scratch 從 #119,800,000 跑至 #122,400,000，皆為 18 events、14 requests、
  4 misses、3 writes、2,611 FileOps、零 pending／drop／未實作服務。
- 正規化兩份 JSON 逐 byte 相同，SHA-256 均為
  `898963d5fc705518705b887e849fda87bc42bd07743274b13c7ff8fe57b19299`。
- 事件、BIOS 排程、writes、FileOps、起訖步數逐項等於第五十四階段無 catalog baseline。
  framebuffer SHA-256 均為
  `0c43f315a772f9f10281eb1fe38ebcb43a9d0cdc0b106f3cb06f49f707e7d003`；`A.who`／`A.stf`
  仍為 `43b7dc3b…aa53`／`90bd8380…a1ab`。
- `menu.go`／測試 SHA-256：`3867508b…670e`／`ce48854e…9ec5`；receipt command／測試：
  `c4bcc4a5…46d5`／`2ebdb048…16d2`。
- 專案 100 項測試與正式收據 verifier 通過；dosgolem 全部正式 packages test／vet，以及
  `apps/buckrogers`／`cmd/buckrogers-text-receipt` race detector 全數通過。

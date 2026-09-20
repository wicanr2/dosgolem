# 008 — Buck Rogers 手冊題目 runtime watcher

狀態：**READY**  
適用程式：DOS《Buck Rogers: Countdown to Doomsday》固定版本  
位址空間：原版執行期 `segment:offset`

## 1. 目的與邊界

本規格把 `0763:0424` 顯示分派器上的真實呼叫，轉交給
`007-buck-rogers-manual-event-adapter.md` 的純狀態核心。watcher 只讀 CPU、堆疊與
長度前綴字串；它不得送鍵、回答手冊題、寫原版記憶體、改返回值或繪製中文。

固定 state 只用來縮短已由正常玩家路徑取得的可重播收據，不成為 `oracle` 公開持久化
API，也不得取代正常玩家路徑驗收。

## 2. 已證實輸入

- dispatcher entry：`0763:0424`。
- entry 堆疊的 `Arg(0)` 是字串 offset，`Arg(1)` 是 segment；來源是最多 255 bytes 的
  單 byte 長度前綴字串。
- `Caller()` 是 far return address，也是 007 規格中的呼叫端事件位址。
- dispatcher 以 `RETF 0Ch` 返回，因此有效 post-call 必須同時符合：目前 `CS:IP` 等於
  entry 保存的 return address、`SS` 未變、`SP == entrySP + 0x10`。
- `37F1:15BD` 可自然落入曾出現過的 return address；只看 `CS:IP` 不足以證明返回。
- clear entry 是 `026F:029C`。

證據：專案 `docs/re/phase-7-text-post-call-generation-event.md`、
`docs/re/phase-18-manual-event-adapter-contract.md` 與其列出的原始收據。

## 3. 事件契約

1. 每次 dispatcher entry 都保存不可變 frame：caller、return address、SS、entry SP、
   字串及當下 generation。
2. 精確的 `2A33:01ED` 加 `In the Log Book on page` 在 entry 立即呼叫
   `Collector.BeginEntry`，使舊題目立即失效；此 frame 返回時不當成六段內容之一。
3. 其他 frame 只有通過第 2 節的三重 return guard 才可呼叫 `Collector.PostCall`。
4. 完成題目後，以同一個 generation 呼叫 immutable `Catalog.Resolve`；只有精確命中才
   產生 `DisplayRequest`。
5. `026F:029C` entry 只呼叫 `Collector.ClearEntry`，不清除仍在列印中的 pending 題目。
6. 觀測紀錄只含 step、事件種類、caller 與命中後的 event/text key、翻譯字元數；不得
   把答案或完整手冊段落寫進收據。

## 4. 失敗即關閉

- 同時出現第二個 dispatcher frame、錯誤 return address／SS／SP、stale frame、無效長度
  或無法解析的事件序列：丟棄 frame，不產生顯示請求。
- 已進入目前 generation 的未知 caller 或錯誤固定文字仍交由 007 collector poison；下一個
  精確 begin 才能復原。
- 自然 fall-through 及沒有 pending frame 的 return hook 一律無效果。
- watcher 不嘗試猜測未知題目、大小寫、序數或 catalog fallback。

## 5. 驗收矩陣

- 單元測試：有效返回、自然 fall-through、錯誤 caller、錯誤 SS、錯誤 SP、巢狀 frame、
  stale return、begin／clear 生命週期與 catalog miss。
- 真實重播：由既有固定 state 重生至少一個已收錄題目的 metadata-only 收據；另以既有
  未收錄事件或 catalog miss 證明不會產生請求。
- `go test ./...` 通過，且 watcher API 不提供輸入、記憶體寫入或 renderer 能力。

## 6. 證據審查

第 2 節每個位址、參數順序及 stack delta 均已有第 7／18 階段動態收據；007 的題目序列、
generation 與 catalog 契約已是 READY。本規格沒有新增未證實的原版語意，故核准為
**READY**，可進入 implementation。

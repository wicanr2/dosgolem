# Buck Rogers 性別／職業明示倍率 runtime overlay

狀態：**CONFORMED**

## 範圍

本規格核准把既有 gender／class typed `DisplayRequest` 與正式 text-safe rectangle 加入
`RuntimeMenuOverlay` 的同一長存 `xlate.Layer`。呼叫端仍須明示 2 或 3；不設定產品預設倍率，
不修改原版 indexed VRAM、CPU、輸入、記憶體、檔案或存檔。

## 固定輸入與證據

- gender events：`a8c96c8edc393c26717a5d05bc46fc031d25a8d0c269f3fe1c6b617ee0dda297`。
- gender translations：`8fd64b9a15fc94aff73a8f7d06ff100b406763b09ac562989a82344eeeac6857`。
- gender rects：`1f0f9060284aa06fc560b44cfe8d0c6adb4b13c6ab749d2a9798fbd804cfe5ae`。
- class events：`1eca7113244d1d8bc7e667c2beeab703e98d172d7806b25eda54f293843bb4de`。
- class translations：`4d4c83dc2e95620109af27c9bd1a8b09d9fc1820adf00a89b60cbb4388673794`。
- class rects：`d099a81f4e66b7540b92c61c0fb523deca475ef3f4aa39b88d69704362baef91`。
- spec 018／021 已 CONFORMED：正常路徑 exact request、selected／normal variants、Escape miss。
- spec 022 已 CONFORMED：四個真實 framebuffer 的 2×／3× 零缺字、零 overlap、零矩形外差異。
- spec 035 已 CONFORMED：frame clock、typed clear、同原點 replace 與輸出端 RGBA 語意隔離。

上述位址與 event identity 均屬 DOS 原版執行期 `segment:offset`；本規格不新增位址推論。

## 輸入契約

1. 每個啟用的 menu／gender／class／roster catalog，在 overlay 模式下必須提供同類 rect TSV；
   不得為缺表的 catalog 猜座標。
2. overlay 模式至少須有一份 catalog＋rect，並同時提供 16×16 GOLEMFNT、RGBA output 與明示
   scale 2 或 3；部分旗標、孤兒 rect、缺字、衝突 event key 全部拒絕。
3. 所有 rect 仍由 `LoadMenuOverlayRects` 嚴格解析並由 `MergeMenuOverlayRects` 原子合併；
   `Apply` 必須反查 runtime event 的 row、column、length 與矩形完全相符。
4. font 必須覆蓋合併 catalog 的全部譯文；receipt 可使用本機決定性合併的 GOLEMFNT，正式
   儲存庫不得內嵌字模或翻譯 literal。

## 生命週期

- guarded post-call 每產生一筆 request，立即以其原版色號、identity 與安全矩形 `Apply`。
- selected／normal variant 在相同原點使用 `Layer.Replace`，舊 variant 不得殘留。
- `026F:029C` 的已證實文字格清除參數傳給 `ClearTextCells`；轉場後舊畫面 stamps 不得存活。
- 真實垂直回掃呼叫 `Frame`；最終只由原版 indexed framebuffer 的複本建立 RGBA，再呼叫
  `Draw`。presentation 不得回寫 machine。

## 驗收

- 正常 BIOS 路徑由功能選單依序進入性別、移動選取、確認進入職業、移動選取；2×／3×
  各 fresh 重播兩次。
- requests 與 overlay actions 一一對應；零 pending、drop、catalog miss、缺字與幾何錯誤。
- 同倍率兩次 receipt／RGBA 逐 byte 相同；所有 action `Contained=true`。
- selected／normal 同原點只保留目前 variant；轉場後上一畫面 keys 不得留在 active keys。
- 原版 events、BIOS keys、raw framebuffer 與無 overlay baseline 相同；本路徑不應新增 DOS write。
- 全部正式 Go packages test／vet、`apps/buckrogers` 與 receipt race detector 通過後，才可升
  `CONFORMED`。

## 權利與停止線

原版 state、framebuffer、RGBA、字型與完整 receipt 只放被忽略的 `workplace/`。產品倍率仍
由使用者決定，本規格只驗證兩個明示候選。若真實路徑顯示新殘字，退回 DRAFT，以 typed
clear／replace 修正；禁止依終態 key 硬刪。

## READY 審查

事件、矩形、譯文、倍率、時鐘、失效與 same-state 驗收都有既有 CONFORMED 規格支撐；本次
只擴充完整輸入配對與正常路徑組合，不新增玩家規則或未證實座標。故可進入 implementation。

## CONFORMED 收據（2026-09-21）

- 由權威 `after-bios-space-100m.state` 正常排入三次 Enter 與一次 Down；baseline、2××2、
  3××2 均為 24 events、24 requests、24 overlay actions、0 catalog miss／pending／drop。
- 2× JSON／RGBA SHA-256：`0978effc…844b2`／`cd62698f…cc8c2`；3×：
  `74d3cac8…d0208`／`b769435f…e6725`。同倍率兩次逐 byte 相同。
- 終態 active keys 只有職業畫面的提示、五個選項、normal Rocket Jock 與 selected Warrior；
  性別與舊選取 variant 均已由 typed clear／replace 移除。
- 原版 indexed framebuffer 五份皆為 `49d8b036…cccc81c`，與既有 Phase 41 class Down 收據相同；
  overlay JSON 移除 presentation 欄位後逐欄等於 baseline。
- 2×／3× 差異像素分別為 3,288／6,225，核准安全矩形外均為 0；合併 77-glyph GOLEMFNT
  SHA-256 `3714d47c…7545ae`，零缺字。
- 原始解析度 PNG 已目視確認無跨列、裁切或殘字；selected Warrior 維持原版黑底黑字，沒有
  自行改色。
- 全部正式 Go packages test／vet、相關 race detector，以及專案 105 項 Python 測試通過。

因此本規格在上述正常路徑與固定輸入範圍升為 CONFORMED；不外推其他角色建立畫面，也不
選定產品預設倍率。

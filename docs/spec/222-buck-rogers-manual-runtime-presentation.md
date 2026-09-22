# 222 — Buck Rogers 手冊正常重播繁中 presenter

狀態：**CONFORMED（固定 state 的第一題；重抽 clear）**  
日期：2026-09-22  
前置：[`216-buck-rogers-manual-presentation-lifecycle.md`](216-buck-rogers-manual-presentation-lifecycle.md)、[`217-buck-rogers-manual-multiline-presenter-core.md`](217-buck-rogers-manual-multiline-presenter-core.md)、[`220-buck-rogers-manual-presentation-queue-consumer.md`](220-buck-rogers-manual-presentation-queue-consumer.md)、[`221-buck-rogers-manual-watcher-snapshot-bridge.md`](221-buck-rogers-manual-watcher-snapshot-bridge.md)。

## 契約

`cmd/buckrogers-text-receipt` 在同一個正常 DOS 指令 loop 中，以原版 `0763:0424` dispatcher 的六個既有參數讀取精確手冊題首的 `background`、`foreground`、`row`、`column`。只有精確 `2A33:01ED`／`In the Log Book on page` begin 可保留這份 `ManualTextStyle`；相容的無 style API 不得宣稱已觀測顏色。`Watcher.Install` 與 command 都走 `ObserveDispatchEntryWithStyle`。

command 將 `Watcher → ManualPresentationBridge → ManualPresentationConsumer → RuntimeManualOverlay` 同步於每一指令，並只在原版 frame callback 與終態 snapshot 呼叫 `Frame`。`Draw` 前缺少觀測 style 時 command 必須失敗。字色不得由空白正文的 framebuffer 合成樣本推斷；presenter 使用題首原始 palette index。原版 VRAM、palette、記憶體、DOS 輸入與答案邏輯均不寫入。

覆繪仍只可變動 logical `[7,312)×[72,184)`；原版頁碼、標題與序數不畫入手冊 layer。2×與3×僅改 RGBA output 尺寸。

## 命令與收據

完整的手冊 mode 需要：

```text
--manual-events --manual-ordinals --manual-translations --manual-layout
--manual-overlay-font --manual-overlay-scale=2|3
--manual-overlay-rgba-out --manual-baseline-rgba-out
--manual-overlay-png-out --manual-baseline-png-out
```

收據必須含原版終態 `memory_sha256`、`indexed_sha256`、`palette_sha256`，以及 `manual_presentation_events`、`manual_observations`、原版 `manual_style` 和 `manual_overlay` 的 baseline／output SHA-256、內外正文差異、added pixel count、action、active keys、missing glyphs。可用相同 state、input 與 scratch 的 no-overlay control 比對這些原版欄位。

## 固定 state 收據

`phase12-before-question.state` 重播到 `266557247`：原版 begin style 是 background 0、foreground 10、row 2、column 3；presentation queue 為 begin `266486493`、clear `266524821`、request `266557246`，request 為 `manual.page34.deimos_prison.word10`。2×與3×各自通過正文外差異 0、無缺字、14 行 active stamps 與新增中文像素。這只證實此固定 state 的第一題。

錯答排程 `300100000:2d:78`、`301100000:1c:0d` 重播至 `301240000` 時，第一代在第二代 begin `301127835` 移除；第二代 clear 為 `301166163`。其後完整題目在 `301202688` 是正式 catalog 的 exact miss，因此 fail-closed、active keys 為空且 baseline／overlay RGBA 相同。這證實重抽不殘字，不宣稱缺 catalog 的第二題已中文化。

## 已知固定 state 限制

此舊 state 的 palette index 15 為黑色，故原版使用 index 15 的頁碼、標題與序數在該收據中不可見。這是原版 snapshot 的 palette 資料，不是 presenter 可以補白的區域；不得藉由硬寫白色或改 palette 修飾。應以新鮮正常玩家 state 另行驗證原始英文周邊文字。

## 2026-09-22：手冊 3× 中文字距調整

依使用者對實際 3× 畫面的回饋，只有手冊 presenter 的 3× 中文字模從輸入的 16×16
在記憶體以最近鄰取樣成 22×22，置於既有 24×24 輸出字格中。ASCII 仍保持原 16×16
墨跡並置中；2× 使用原字模與原位置，輸出逐位元不變。正式 GOLEMFNT 原檔、
36×14 文字格、輸入與原版 framebuffer 均不改。這個放大僅作用於輸出 RGBA。
同狀態第一題 3× 正文內變更 6591 像素、外部 0；2× RGBA SHA-256 維持
`da3007bc54ecd0e52dd6a7f8979619808e54521ca6e176686403374dcafed5fb`。
第二題與 restore 收據見專案 `docs/re/phase-97-manual-cjk-density.md`。

## frame 與 restore 邊界

正常 replay 在 `machine.SetOnFrame` callback 呼叫 `RuntimeManualOverlay.Frame`，receipt 的
`manual_overlay.frame_callbacks` 量化實際 callback 次數；終態另以同一個原始 indexed frame／palette
做一次 `Frame`／`Draw` 收據。沒有 begin/request 的 savestate restore 不包含任何 derived overlay
state：新 presenter 不得復原舊段落、不得猜測 style，必須輸出原始 RGBA baseline、0 active keys、
0 changed pixels。這個 no-residual 行為有 synthetic test；待字型產物與現行 catalog 再次對齊後，
以真實 restore replay 補充收據。

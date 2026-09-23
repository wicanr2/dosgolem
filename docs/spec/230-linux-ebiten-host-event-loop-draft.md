# 230 — Linux Ebitengine host 事件迴圈（限縮 DRAFT）

狀態：**DRAFT（等待獨立審查；未授權 production）**  
日期：2026-09-23  
前置：spec 004、219、225、226、227、228。

## 目的與界線

本切片提出 `frontend/ebiten` 的 Linux Ebitengine 2.9.9 視窗事件迴圈候選。它在同一個
`Update` goroutine 內：接收 Ebitengine keyboard／left-pointer edge、依 immutable host layout
分類事件、透過既有 `KeyboardBridge`／`MouseBridge` 送入 DOS，並以 caller supplied 的
active-layer presentation snapshot 繪製完整 320×200 canvas。

它**不授權** production frontend 或任何 Buck Rogers player command。候選 router 沒有任何既有
command import；未來若另有 raw-frame developer launcher，也必須標為開發／原型入口，絕不可叫作
可玩中文版。它沒有接通 Buck Rogers watcher、正式繁中 catalog、完整開機路徑、存讀檔或原文／繁中
same-state A/B，因此不得被稱為完成中文化或完整正常玩家路徑。

## 已確認輸入與畫面契約

* 初始 active scale 是 2，僅記憶體保存；設定面板選 2×／3×只改 selected，Apply 提交並自動收合，
  Cancel 捨棄暫選。這些轉移由 `host.PanelController`（219／225）既有 CONFORMED 契約定義。
* 只有 active scale、panel open state 或外部 layout 實際改變時，frontend 才由 current `PanelState`
  衍生新的 `host.MouseLayout` epoch；一般 Update 的 Down→Up 不得無故換 epoch。canvas 永遠是
  `[0,320*s)×[chrome,chrome+200*s)`，完整畫面下推，且 snapshot 以同一 active scale 重建。
* closed panel 的 canvas left Down／Up 交給 CONFORMED `host.MouseBridge`；只有已接受的 Down 可在
  後續 canvas 外、host chrome、panel 或 focus loss 的 Up 作唯一 `ReleaseMouse(0)` cleanup。
* panel open 的所有新 pointer（包括空白 miss）一律 host consume，絕不建立新 DOS press；host target
  Down 建立 host capture，避免其 Up 因重新 layout 而送 DOS。
* 所有 keyboard 先走 `presentation.KeyboardBridge.DeliverBIOSKey`。此切片只映射 dosgolem 已有
  `KeyNamed`／`KeyForRune` 所列 Enter、Esc、Backspace、Tab、Space、方向與 ASCII 字母／數字；其他鍵
  fail-closed。面板開時 bridge 必須不改 BIOS queue。
* `Snapshot` 與 `Advance` 都只由 Ebitengine game goroutine 呼叫；frontend 不保留 machine reference、
  不直接 Step、也不寫 VRAM、DOS memory、存檔或 IRQ。

## 尚待 READY 的證據審查

host chrome 的 2×字型固定為本機 16×16，3×字型固定為本機 22×22；兩份皆要含
「設定／套用／取消／2×／3×」，各 label 必須在對應安全矩形內量測 containment。3×不得稀疏
放大 2×字型。

所有 player-visible state transition 已由既有 219／225／226／228 定案；本切片沒有新增產品選擇，
但候選程式尚未經獨立審查，故維持 DRAFT。
Ebitengine 的 Linux/Xvfb 可開窗能力，以及 2×／3×實體 host input／MouseBridge receipt 已由 project
phase 106、125、154、179 建立。這裡只把其已決事件路由移入可版控通用實作，且把未能證實的 game
observer／translation lifecycle 排除在外。

可測的 fail-closed 邊界為：nil dependency、snapshot 尺寸或 RGBA 長度不符、未知 key、非 left mouse、
無效 layout、沒有 paired Down 的 Up，均不授權 DOS write。單元測試須覆蓋 panel open miss、Apply／Cancel
layout epoch、keyboard focus、closed canvas forwarding 及 accepted press cleanup。Linux Xvfb smoke 必須確認
視窗可建立、raw frame 有畫素且正常結束；它不是原版行為同狀態收據。

目前候選只以可注入座標的 pure router 測試證實上述 route；Ebitengine public pointer callback 的
實體 X11→logical coordinate mapping、callback 與 caller `Advance`／`Snapshot` 的同 goroutine 實驗，
仍是 READY 前缺口。程式沒有自行開 goroutine 只是可查 code 事實，不是該 callback 契約的充分證據。

## 原型測試與停止線

候選通過後最多只能聲稱「Linux Ebitengine frontend DRAFT router 已被測試」。獲獨立審查升 READY
並接進 production 後，才可稱「限縮事件迴圈已接通」。要把 spec 004 升 READY／CONFORMED，
仍需要已接實際 Buck Rogers active translation layer 的正常開機、切換後繼續遊玩與存讀檔 same-state
receipt；本規格不偷渡這些 gate。

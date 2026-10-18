# 451：session 的滑鼠移動事件

狀態：**READY**（第 4 版，依三輪審查修訂）
日期：2026-10-18
前置：[`199-live-session-api`](199-live-session-api.md)、[`449-session-irq1-scan-channel`](449-session-irq1-scan-channel.md)、
[`450-session-boot-from-state`](450-session-boot-from-state.md)

dosgolem 的程式位置以 commit `a59dc5c` 為準。推論等級：已證實、強推論、假說、未知。

用語：

| 用語 | 意義 |
|---|---|
| 移動事件 | `Kind` 為新增的 `host.MouseEventMove` 的 `host.MouseEvent` |
| DOS 座標 | host 換算給 `dos.MoveMouse` 的邏輯像素座標，等於 `(X / Scale, (Y − ChromeHeight) / Scale)`，整數除法 |
| 回合 | 一次 `Deliver` 加上隨後的一次 `Advance`（同 449） |
| 整批拒絕 | `prepare` 回錯誤；照既有 `frontendFaultInputReceipt`（`session/owner.go` 894 到 899 行），owner 進入 `PhaseFailed` |

---

## 1. 為什麼要做

前端要讓玩家移動滑鼠時，原版看得到游標移動。session 目前只接按下、放開與失焦：

- `host.MouseEventKind` 只有 `MouseEventDown`、`MouseEventUp`、`MouseEventFocusLost`（`host/mouse_bridge.go` 22 到 26 行）。
- `PlanMouseRoute` 與 `MouseBridge.Handle` 對其他種類回 `unknown-event-rejected`（`host/mouse_route_plan.go` 72 到 75 行、`host/mouse_bridge.go` 189 到 191 行）。
- `CapturedUpdate` 只有 `PointerDown`、`PointerUp`、`FocusLost`（`session/owner.go` 87 到 101 行）。按下與放開會先送一次 `MoveMouse` 再送按鍵，但沒有不按鍵的移動。

《Strike Commander》OPTTEST 的熱點標籤只在游標停在熱點上時出現（strike_commander RE 012 §4 樣本 B、C：probe 以 `-mouse-x -mouse-y -mouse-at` 移動游標後，標籤才換成 "Transport to Strike Base"、"Fly Mission"）。沒有移動事件，session 前端看不到這些標籤，也無法驗收它們的中文覆繪（已證實：程式碼）。

DOS 層已有移動：`dos.MoveMouse`（`internal/dos/bios.go` 710 到 723 行）先依遊戲以 `int 33h` 設定的範圍夾限座標；夾限後座標沒變是 no-op，有變就更新座標、累加 mickey，再經 `fireMouseEventMickeys`（同檔 752 行起）排事件。只有遊戲裝了回呼（`Handler.Set`）且遮罩含移動位元時，事件才進回呼佇列與 `Mouse.Events`；座標與 mickey 的更新不受回呼影響。本規格只補 host 路由與 session 交付，不改 DOS 層。

## 2. 規則

### 2.1 `host.MouseEventMove`

在 `MouseEventFocusLost` 之後新增 `MouseEventMove`（既有三個值不變）。`PlanMouseRoute` 與 `MouseBridge.Handle` 對移動事件依序判定，兩者必須逐項一致：

| 列 | 條件 | Route | 動作 |
|---|---|---|---|
| 1 | `!HasCurrent` 或 `layout != Current` | 既有 `stale-layout-epoch-rejected` | 無 |
| 2 | `HostCaptured` | `ConsumedByHost`，`host-captured-move-consumed` | 無 |
| 3 | `layout.PanelOpen` | `ConsumedByHost`，`panel-open-move-consumed` | 無 |
| 4 | `Target != MouseTargetCanvas`，或座標不在 `canvasContains` 內 | `outside-canvas-move-rejected` | 無 |
| 5 | 其他 | `ForwardedToDOS`，`canvas-move-forwarded` | 一個 `MouseActionMove`，DOS 座標 |

- 移動事件不看 `Button`。按鍵語意由 Down／Up 負責；前端在拖曳右鍵時帶非零 `Button` 的移動照常處理，不會因此整批拒絕。
- 移動事件不改 `Pressed`、`PressedEpoch`、`HostCaptured`，任何一列都一樣。
- 第 2 列與 Down／Up 的 `host-captured-*-consumed` 一致；第 3 列與 `panel-open-consumed` 一致。
- 第 4 列在 host 層不夾限：游標停在最後一次畫布內的位置。DOS 層的夾限照舊。
- 按住左鍵時的移動照上表判定：
  - `Pressed` 且 `PressedEpoch == layout.Epoch`：第 5 列送出（拖曳）。
  - `Pressed` 且 `PressedEpoch != layout.Epoch`（按住期間換過倍率）：仍照第 5 列送出，座標用目前的 layout 換算；按鍵的處置留給之後的 Up（`epoch-changed-release`）。
  - `Pressed` 且 `PanelOpen`：第 3 列，DOS 游標凍結到 Up 或 FocusLost 放開為止。
- `Pressed` 且 `HostCaptured` 是 `validateMouseRouteSnapshot` 拒收的快照（`host/mouse_route_plan.go` 152 到 154 行），不列入。

### 2.2 `CapturedUpdate.PointerMoves`

`CapturedUpdate` 新增 `PointerMoves []host.MouseEvent`。

- 下列情形整批拒絕。它們是前端程式錯誤，不是可恢復的輸入：
  - 有元素的 `Kind` 不是 `MouseEventMove`。
  - 超過 64 個（64 個接受）。前端要自行合併，通常每畫格只送最後的位置。
  - `Target` 不是已分類的三種之一。這條沿用既有 `planMouseEvent` 的檢查（`session/owner.go` 546 行），不另寫。
  - `PlanMouseRoute` 回第 1 列（`stale-layout-epoch-rejected` 已在 `session/owner.go` 554 行的清單內）。
- 在 session 內，第 1 列不可達：`update.Layout`、`o.layout`、`baseMouse.Current` 在 `prepare` 前段已被要求相等（`session/owner.go` 433 到 456 行）。過期 layout 由那裡整批拒絕。
- 第 2、3、4 列照常收下，不產生動作。
- `hasPointer`（`session/owner.go` 462 行）加上 `len(update.PointerMoves) != 0`。
- 規劃順序：`PointerMoves` 依序，再 `PointerDown`、`PointerUp`、`FocusLost`。`PointerMoves` 的規劃迴圈放在 `if hasPointer` 區塊內、緊接在 `PointerDown` 之前（§3.3 N14 以此為前提）。commit 依同一順序呼叫 `MouseBridge.Handle`（`session/owner.go` 582 到 587 行的迴圈不變）。所以同一批的「移動到 A、在 B 按下」會先送 `MoveMouse(A)`，再送 `MoveMouse(B)`、`PressMouse(0)`。
- 與 `PanelEvents` 同批時允許，照 `PointerUp` 的先例：commit 先跑滑鼠迴圈，再跑面板事件。所以批次開頭面板關著、同批才打開時，第 5 列的移動照樣寫進 DOS，即使這一批最後是 paused。BIOS 鍵與掃描碼在 paused 批次不 commit 的規則不變。
- 同一批內只能表達「先移動，再按下或放開」。按下之後的游標位移要放到下一批。

### 2.3 計數

`InputReceipt.DOSCallsCommitted` 照既有定義（`session/owner.go` 116 到 118 行）計入移動動作：每個第 5 列的移動算一次 `MoveMouse` 呼叫，即使 DOS 座標沒變、DOS 層沒排事件。不新增收據欄位。

### 2.4 唯讀的 DOS 滑鼠狀態

新增 `func (o *Owner) DOSMouse() (DOSMouseState, error)`，回傳值複本 `DOSMouseState{X, Y uint16; MickeyX, MickeyY int16; Buttons uint16}`，取自 `o.dos.Mouse`（型別照 `internal/dos/dos.go` 65 到 84 行）。

- `PhaseRunning` 時可呼叫，`turnPending` 期間也可以。與 `Digest` 不同（`session/digest.go` 42 行拒絕 `turnPending`）：`Deliver` 對 DOS 滑鼠的寫入在 commit 內一次完成，回合中沒有半寫入狀態。
- 其他 phase 回錯誤，不改 phase。
- 不給任何寫入能力。供 §3.2 比對，也供前端顯示除錯資訊。

### 2.5 與 probe 的對應

probe 的 `-mouse-x X -mouse-y Y -mouse-at S` 在主迴圈 `m.Steps == S` 時、下一次 `Step` 之前呼叫 `d.MoveMouse(X, Y)`（`cmd/probe/main.go` 797 到 799 行；817 到 823 行的第二次呼叫座標相同，是 no-op）。

session 的對應做法：

1. `Advance` 的預算總和要讓 `m.Steps` 正好停在 S。
2. 交付一個 DOS 座標為 (X, Y) 的移動。
3. 下一次 `Advance` 才往下執行。

兩者在同一個指令邊界呼叫同一個 `MoveMouse`。之後的 `Step` 都從開頭的 `tick` 處理回呼佇列（`internal/machine/machine.go` 1492 到 1496 行）。沒有中斷點時，`RunUntil` 與逐次 `Step` 等價（`internal/machine/probe.go` 168 到 190 行）。觀察器是唯讀的步前掛鉤，不碰滑鼠。`Advance` 先呼叫的 `ScheduleHeldRepeats` 在沒有按住的鍵時不改狀態（449 §2.5）。所以同一起點、同一 S，兩者之後的狀態在 `Digest` 涵蓋的範圍內應相同（強推論；§3.2 驗證）。`Digest` 不含 DOS 滑鼠座標、mickey 與回呼佇列（`session/digest.go` 49 到 73 行），§3.2 另以 §2.4 的 `DOSMouse` 比座標與 mickey。

### 2.6 範圍外

- 右鍵、中鍵、滾輪。
- 相對移動與 mickey 比例（`dos.MoveMouse` 一個邏輯像素算一個 mickey，維持現狀）。
- 狀態檔在滑鼠移動後不久存檔，接著跑會跳到 `0000:0000`（strike_commander 規格 001 §4，假說：待送的滑鼠回呼不在狀態檔裡）。另立規格。
- 前端何時送移動（每畫格、只在座標改變時）由前端決定；本規格只定 session 怎麼收。

## 3. 驗收

### 3.1 單元測試

1. `host`：分兩段。
   - **Plan 單測**：直接把合法快照餵給 `PlanMouseRoute`，§2.1 每一列至少一例。斷言 Route、動作數與內容、`Next` 不變。
     - 第 1 列：`!HasCurrent`，以及 `layout != Current` 各一例。其餘各列 `layout == snapshot.Current` 且有效。
     - `HostCaptured` 且 `PanelOpen`，得第 2 列。
     - 第 4 列：`Target` 為 `MouseTargetHost` 且座標在畫布內（`HostCaptured` 仍為 false）；`Target` 為 `MouseTargetOutside`；`Target` 為 Canvas 而座標在邊界外各一例：x = −1、x = 寬 × 倍率、y = ChromeHeight − 1、y = ChromeHeight + 高 × 倍率。另以 x = 寬 × 倍率 − 1、y = ChromeHeight 得第 5 列。
     - `Button` 為 1、其他條件同第 5 列，得第 5 列。
     - `Pressed` 且 `PressedEpoch == layout.Epoch`，得第 5 列，`Pressed` 仍為 true，不是 `duplicate-down-rejected`。
     - `Pressed` 且 `PressedEpoch != layout.Epoch`，得第 5 列，`Pressed` 仍為 true（不是放開）。
     - `Pressed` 且 `PanelOpen`，得第 3 列。
   - **Plan 與 Handle 一致**：照 `host/mouse_route_plan_test.go` 20 到 87 行的矩陣做法，以事件序列與 `ApplyLayout` 把 `MouseBridge` 帶到各狀態，`Handle` 以記錄呼叫的 `MouseOutput` 得到的動作清單與 Route 要與同狀態的 `PlanMouseRoute` 相同。涵蓋可達的狀態：無按鍵、`Pressed`（同 epoch）、`Pressed` 後換 layout（不同 epoch，含 `PanelOpen`）、`HostCaptured`（含之後開面板）。`Pressed` 且 `PressedEpoch == Epoch` 且 `PanelOpen` 對 `Handle` 不可達（開面板必換 epoch），只做 Plan 單測。
2. `host`：DOS 座標換算，`PlanMouseRoute` 與 `Handle` 都驗。自訂 layout 要滿足 `MouseLayout.valid()`（`FrameWidth ≥ 320 × 倍率`、`FrameHeight ≥ ChromeHeight + 200 × 倍率`，`host/mouse_bridge.go` 48 到 62 行）；既有 helper 的 chrome 固定為 18 × 倍率，不能直接用。
   - 2 倍、`ChromeHeight` 24：host (321, 175) 得 DOS (160, 75)。
   - 3 倍、`ChromeHeight` 25：host (2, 27) 得 DOS (0, 0)。「先除再減」會得 (0, 1)，「漏減 chrome」會得 (0, 9)。
3. `session`：下列各情形整批拒絕。斷言 `Deliver` 回錯誤、`Phase == PhaseFailed`，且 epoch、generation、mouse snapshot、DOS 滑鼠座標都沒變：
   - 只含一個 `Kind` 為 `MouseEventDown` 的 `PointerMoves`。
   - 只含一個 `Target` 為零值的移動。
   - 65 個移動。另以 64 個移動驗證接受。64 與 65 個都用合法的移動（`Kind` 為移動、`Target` 為 Canvas、座標在畫布內），避免被其他檢查混淆。
4. `session`：以 `startSyntheticOwnerWithLayout` 與 `sessionLayout(1, OutputScale2, false)` 建 owner（DOS 座標 (x, y) 對 host (2x, 36 + 2y)；初始游標 (0, 0)；沒設範圍時 DOS 層不夾限）。設 `o.dos.Mouse.Handler.Set = true` 與 `Handler.Mask = dos.EvMove | dos.EvLeftDown | dos.EvLeftUp`（`session/owner_test.go` 541、542 行的做法），A、B 都與初始座標不同。同批「移動到 A、在 B 按下、在 B 放開」：
   - `Mouse.Events` 依序是：移動到 A、移動到 B、左鍵按下、左鍵放開。以各事件的 `Buttons`（事件遮罩）與 X、Y 斷言。
   - `DOSCallsCommitted` 是 5。
5. `session`：同第 4 項建 owner、設 Handler 與 Mask。兩種批次各用獨立的 owner，不在中間 `Advance`。
   - 只含一個移動到目前座標的批次：`DOSCallsCommitted` 是 1，`Mouse.Events` 不增加。
   - 只含一個移動到不同座標的批次：`Mouse.Events` 加 1（正對照）。
6. `session`：面板開著時只含移動的批次，不送 DOS，`DOSCallsCommitted` 是 0，`HostChanged` 為 false，`Paused` 為 true。
7. `session`：批次開頭面板關著，同批有移動與打開面板的 `PanelEvents`。移動寫進 DOS（座標變了），批次 paused。
8. `session`：起始座標 X ≠ Y。只沿 X 軸移動 dx 後，`DOSMouse` 回傳值與 `o.dos.Mouse` 逐欄相同，`MickeyX` 增加 dx、`MickeyY` 不變（mickey 是像素差，`internal/dos/bios.go` 716 到 721 行）。`Deliver` 之後、`Advance` 之前可呼叫；非 `PhaseRunning` 時回錯誤，phase 不變。這項不需要 Handler。
9. `session`：拖曳。Handler 與 Mask 在第一批之後才設（避免 `Advance` 時回呼跳到未設定的位址），或把 `Handler` 的 Seg:Off 指向一個 `IRET` 樁；兩種做法擇一並在 §5 記錄。
   - 第一批只含 `PointerDown`，用 `consumeRunningTurn` 結束回合。
   - 第二批只含一個移動到不同座標。第二批後 `Mouse.Events` 多一筆移動，`DOSMouse().Buttons` 仍含左鍵。
   - 第三批只含 `PointerUp`，route 是 `up-release`（證明移動沒有清掉 `Pressed`）。

### 3.2 觸發案例（主代理執行，結果寫進 §5）

遊戲專屬，放在 `apps/strikecommander` 的長測試（通用層不依賴遊戲 app）。環境變數同 strike_commander 規格 001 的長測試：`DOSGOLEM_SC_ROOT`、`DOSGOLEM_SC_STATE_DIR`。甲、乙都用 `apps/strikecommander.SessionConfig`（`apps/strikecommander/receipt.go` 44 行起：`Mode13Phase`、`SoundBlaster`、`AdLib`、`SBDMA8`、`KbdIntercept` 等與 probe 旗標對應的選項，448 §3.2），SaveRoot 複製、`SC.CFG` 雜湊檢查與 `RebaseFrom` 照 001 長測試既有的做法。

乙的狀態檔由 probe 產生（不要重跑：會覆寫 `p530/s4495b.state`，要重跑就改用新檔名），指令（strike_commander `tools/probe/psc.sh`，內含 `-mode13-phase`、`-sound-blaster -adlib -sb-dma8`、`-pm-profile` 等固定旗標）：

```text
psc.sh p530 probe546 -kbd-int15 -load-state /scratch/s449.state -steps 4495600001 \
  -mouse-x 160 -mouse-y 75 -mouse-at 4495000000 -save-state 4495600000:/scratch/s4495b.state
```

`RunReceipt` 只做一次空 `Deliver` 加一次 `Advance`，不支援中途移動。甲在長測試裡手寫回合：每次 `Advance` 前都要先 `Deliver`（`session/owner.go` 699 到 701 行）。

- **甲（移動）**：
  1. `BootFromState` 讀 `p485/s449.state`（SHA-256 `7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0`，第 4490000000 步）。
  2. 空 `Deliver`，`Advance(4495000000 − 起始步數)`，確認停在第 4495000000 步。
  3. 以 `View().Layout` 把 DOS (160, 75) 換成 host 座標，`Deliver` 一個只含這個移動的批次。
  4. `Advance` 到第 4496200000 步。

  觀察器在第 1 步之前就要建好：`Config.Observer` 只能在 `session.New` 時給、之後不能換（`apps/strikecommander/receipt.go` 146 行的做法）。掛 `LabelObserver`，呼叫端檢查用預設值（開），`CursorY` 設 75（與 001 案例 B 相同，`apps/strikecommander/trigger_long_test.go` 56 行）。
- **乙（對照）**：
  1. `BootFromState` 讀 `p530/s4495b.state`（SHA-256 `34c5b2e5a50131cda4537428ad0c1e4d999a3302a6bc6e60c34ba177fc3b48cd`）。它是上列指令從同一個 `s449` 在第 4495000000 步移到 (160, 75) 後、第 4495600000 步存的。
  2. `Advance` 到第 4496200000 步。
- **主斷言**（獨立 oracle）：
  - 甲在第 4496200000 步的 Indexed，SHA-256 等於 `133eaa9464a191e0f71cedf63acce990da37a19eadaa332e425b510f541ad779`。這是 probe 連續執行以 `-shots` 寫出的 `h.png` 的雜湊；該檔是原始 Indexed 位元組，不是 PNG 編碼（`cmd/probe/main.go` 1979 到 1981 行）。
  - 甲的移動回合 `DOSCallsCommitted` 是 1。
  - 甲的觀察器在第 4496160311 步的 blit 提交 `"Transport to Strike Base"`，X0 68、X1 251、Y 175，與 001 §5.3 案例 B 相同。甲從第 4490000000 步起跑，之前會先有 "Talk to Gwen" 的提交；斷言只針對這一筆，不比整個提交序列。
- **預期**（失敗時記錄、不判甲錯）：甲、乙的 `Digest`（Memory、CPU、Indexed、Palette、Ext）與 `DOSMouse()` 全同。乙經過一次 probe 存檔與 session 讀檔；001 §3.3 只驗過乙的 Indexed 與 probe 連續執行相同，Memory、CPU、Ext 與滑鼠欄位的往返無損沒有證據。不同時列出差異欄位，寫進 §5。
- **負對照**：甲不交付移動，Indexed 必須不同（畫面停在 "Talk to Gwen"）。

乙依賴一個假說：移動後 600000 步存的狀態檔接著跑是正常的（§2.6 的狀態檔問題只在移動後約 1600 步內出現）。001 §3.3 的 `h2.png` 已實測乙正常。若重跑時乙跳到 `0000:0000`，問題在乙，不影響主斷言。

### 3.3 負對照（實作子代理逐列執行，結果寫進 §5）

| 編號 | 突變 | 必須失敗的測試 |
|---|---|---|
| N1 | `PlanMouseRoute` 對移動回 `unknown-event-rejected` | 3.1 第 1 項、3.2 甲 |
| N2 | `Handle` 的第 5 列不呼叫 `MoveMouse` | 3.1 第 1 項（動作清單比對）、第 4 項、3.2 甲 |
| N3 | 第 2、3 列順序對調 | 3.1 第 1 項（`HostCaptured` 且 `PanelOpen`） |
| N4 | 第 4 列改為夾到畫布邊界再送出 | 3.1 第 1 項 |
| N5 | 移動事件把 `Pressed` 清掉（Plan 與 Handle 同改） | 3.1 第 1 項、第 9 項（第三批不是 `up-release`） |
| N6 | 按住時的移動回 `duplicate-down-rejected` | 3.1 第 1 項、第 9 項 |
| N7 | 拿掉第 1 列 | 3.1 第 1 項 |
| N8 | 拿掉第 2 列 | 3.1 第 1 項 |
| N9 | 拿掉第 3 列 | 3.1 第 1 項、第 6 項 |
| N10 | 第 4 列拿掉 `Target != MouseTargetCanvas` | 3.1 第 1 項（Target 為 Host） |
| N11 | 移動事件 `Button != 0` 時拒絕 | 3.1 第 1 項（`Button` 為 1） |
| N12 | session 規劃時把 `PointerMoves` 放在 `PointerDown` 之後 | 3.1 第 4 項 |
| N13 | 上限改成 65 | 3.1 第 3 項 |
| N14 | `hasPointer` 不計 `PointerMoves` | 3.1 第 3 項（零值 Target）、第 5 項 |
| N15 | `mouseActionCount` 不計移動動作 | 3.1 第 4、5 項 |
| N16 | DOS 座標換算漏減 `ChromeHeight` | 3.1 第 2 項 |
| N17 | DOS 座標換算改為先除再減 | 3.1 第 2 項 |
| N18 | `DOSMouse` 回傳 `MickeyX`、`MickeyY` 對調 | 3.1 第 8 項 |
| N19 | commit 時滑鼠迴圈移到 panel 事件之後 | 3.1 第 7 項（commit 比對在 `session/owner.go` 584 到 586 行 panic，以 panic 視為失敗） |
| N20 | 拿掉 `PointerMoves` 的 `Kind` 檢查 | 3.1 第 3 項（`Kind` 為 Down） |
| N21 | 對移動事件略過 `Target` 未分類檢查 | 3.1 第 3 項（零值 Target） |
| N22 | 按住且 `PressedEpoch != Epoch` 的移動改為放開 | 3.1 第 1 項 |
| N23 | `canvasContains` 的邊界差一 | 3.1 第 1 項（四個邊界與兩個內緣） |
| N24 | `DOSMouse` 在 `turnPending` 時回錯誤 | 3.1 第 8 項 |
| N25 | `DOSMouse` 在非 Running 時不回錯誤 | 3.1 第 8 項 |
| N26 | 上限改成 63 | 3.1 第 3 項（64 個接受） |

實作子代理證明為等價突變的列，在 §5 記錄處置，不退回 DRAFT。

### 3.4 回歸

改動前後 `go test -count=1 ./...` 的套件結果相同（3 個 ebiten 套件在無顯示器的容器裡改動前就失敗，照 449、450 的記法）。449、450 的長測試與 strike_commander 規格 001 的長測試重跑通過。

## 4. 已知限制

- 第 5 列不夾限，玩家把游標快速移出畫布時，原版游標停在最後一個畫布內的事件位置，不貼邊。
- 移動事件不帶時間戳。同一回合內的多個移動在同一個指令邊界送出，原版只看到回呼佇列裡排著的多個事件，看不到其間的時間差。
- 一批內只能先移動再按鍵；按下之後同一畫格的位移延到下一批（§2.2）。
- 整批拒絕會讓 owner 進入 `PhaseFailed`。前端要保證移動事件的 `Kind` 與 `Target` 合法，並自行合併超過 64 個的移動。

## 5. 實作收據（2026-10-18）

實作由子代理完成，主代理在 Docker 內重跑測試。改動：`host/mouse_bridge.go`（`MouseEventMove`、`handleMove`）、`host/mouse_route_plan.go`、`session/owner.go`（`PointerMoves`、`maxPointerMoves`）、`session/digest.go`（`DOSMouseState`、`DOSMouse`）；測試 `host/mouse_move_test.go`、`session/pointer_move_test.go`、`apps/strikecommander/pointer_move_long_test.go`（`TestPointerMoveTrigger`）。

### 5.1 單元測試與回歸

- gofmt 無輸出，`go vet ./host/ ./session/ ./apps/strikecommander/` 通過，三個套件的 `go test -count=1` 全過（主代理重跑）。
- `DOSMouseState` 實際型別與 §2.4 相同。
- 回歸：改動前（`15c769e` 的乾淨匯出）與改動後，`go test -count=1 ./...` 的套件結果相同：3 個 ebiten 套件無顯示器失敗，`internal/repolint` 因 451 未登索引失敗。補上 `000-index.md` 後 repolint 通過。
- 長測試：
  - 450 `TestBootFromStateTrigger` 通過。
  - strike_commander 規格 001 `TestLabelOverlayTrigger` 的 A、B、C、ANoCaller、AOverlayOff 通過（字型用含測試字的 `font-test`）。
  - 449 `TestIRQ1ScanTrigger` 的 Base、NoRepeats 在 `--memory 6g` 通過。在 `--memory 3g` 被 OOM 砍掉，改動前的乾淨樹在同樣設定下也被砍，與本規格無關。449 §5 當時以 3g 通過，差異原因未查。

### 5.2 負對照

N1 到 N26 由子代理逐列執行（`strike_commander/workplace/i451/neg/results.txt`），全部使對應測試失敗，沒有等價突變。

- N2 只改 `Handle`，Plan 單測照預期通過，由 Plan 對 Handle 一致的測試與 §3.1 第 4 項抓到。
- N23c 到 f 的 Plan 對 Handle 測試通過：兩邊共用 `canvasContains`，由 Plan 單測抓到。
- 子代理另加兩列補充 N23g、N23h（左內緣、下內緣），並為此補一個案例 `row5-other-inner-edges`。
- 主代理在工作樹複本上抽驗 N12、N14，對應測試失敗。

### 5.3 觸發案例

`TestPointerMoveTrigger` 通過（約 12 秒）：

| 項目 | 結果 |
|---|---|
| 甲的 Indexed | `133eaa94…d779`，等於 probe `h.png` |
| 甲的移動回合 | `DOSCallsCommitted` 1 |
| 甲的熱點標籤 | 第 4496160311 步的 blit 提交 `"Transport to Strike Base"`，X0 68、X1 251、Y 175；之前五筆是 `"Talk to Gwen"` |
| 預期（甲、乙全同） | 成立。Memory `b13b3ddc…`、CPU `e270e559…`、Indexed、Palette、Ext 全同；`DOSMouse` 兩邊都是 X 160、Y 75、MickeyX 0、MickeyY −80、Buttons 0 |
| 負對照（不移動） | Indexed `813fc5c5…`，與甲不同；第 4496151737 步仍提交 `"Talk to Gwen"` |

乙經 probe 存檔、session 讀檔後與甲逐位元組相同，所以 §2.5 的推論在這個案例成立。乙的狀態檔沒有跳到 `0000:0000`。

### 5.4 實作期間的處置

- §3.1 第 9 項的 Handler：回呼以 `FarCallWithReturn` 進入，只推返回位址，所以樁用 `RETF`，不是規格寫的 `IRET`。合成 COM 是 `90 90 90 90 CB`，Handler 指向 `CS:0104`，一開始就設好。「第一批之後才設 Handler」不夠：第二批的移動也會排回呼，下一次 `Advance` 仍會跳到 Handler。
- 第三批的 `up-release` 由測試先呼叫 `o.prepare`、讀出 route 再 `commit` 取得。
- `CapturedUpdate.PointerMoves` 放在 `BIOSKeys` 之後，避免 gofmt 重排既有欄位。
- 規格行號以 `a59dc5c` 為準，與實作時的 `15c769e` 有偏移，內容一致。

# 228：host MouseBridge READY 候選

狀態：DRAFT，待 READY 審查；不得接 production。

輸入基準：私有 `phase12-before-question.state` SHA-256=`8cbc27f568057fbf3ce2f91d407953ec94836f2b723f50b7b73e56100e859269`、
`GAME.OVR` SHA-256=`3a4ad4856c08fe5973179f1d907feed1d870af99d08abd1cb884b316324f3cc0`、dosgolem branch
`buck-rogers-cht-output-overlay`（commit 必須在 READY 審查時填入）。工具為 Docker
`eob-remake-go:1.26.7-ebiten2.9.9`。位址／座標空間是 Ebitengine logical pixels、host canvas
pixels、DOS mouse virtual coordinates；不宣稱遊戲 EXE callsite。原版、state、字型及畫面不得入 Git。

## 邊界

來源為視窗後端已換算的 typed `Pointer{Kind:Down|Up,Button:Left,X,Y}`，目的端為同一 machine
的 DOS `MoveMouse`、`PressMouse(0)`、`ReleaseMouse(0)`。結果必須回傳
`{ConsumedByHost,ForwardedToDOS,Reason}`；bridge 不 Step machine、不寫 VRAM、不排 keyboard IRQ，
不持有 Ebitengine 物件。既有 `internal/dos/bios.go` 的三個 API 是能力證據；phase127 已量到
Move+Press+Release 改 mouse position、而未 Step 時 indexed／BIOS／IRQ 不變。

只有 panel closed、指標在 canvas `[0,320*s)×[chromeH,chromeH+200*s)` 時可轉送；
轉換為 `(x/s,(y-chromeH)/s)`，其中 `s` 僅可為 2 或 3。chrome、panel、負座標、右／下邊界
與未知 button 均拒絕且不改 DOS。panel open 的所有**新** pointer 一律消費且不改 DOS。scale 非 2/3、
負 chrome、resize 後 rect 不一致、out-of-range 與 Up 無匹配 Down 均 fail-closed。重複 Down、
重複 Up 也不得重複呼叫 DOS；bridge 以單一 left-button pressed state 拒絕。

使用者已定案：只有 closed panel、canvas 內的 Left Down 可建立新的 DOS 按鍵狀態。若該 Down 已被轉送，後續 Left Up
即使位於 canvas 外、chrome、已開面板或視窗失焦，仍必須只呼叫 `ReleaseMouse(0)`；不得 `MoveMouse`，
使用最後有效 DOS 座標。這是唯一允許在 open panel 觸及 DOS 的既有 pressed-button cleanup，不表示新的
pointer forwarding。

Down 的序列是 Move→Press；若 Up 仍在 closed panel 的 canvas 內，則 Move→Release。若 Up
已在 canvas 外、chrome、panel 或失焦，則只 Release、不 Move，以最後有效 DOS 座標清鍵。
**原型勘誤與修正（2026-09-22）**：ignored 主專案
`workplace/phase128-mousebridge-prototype/bridge.go` 原先對所有配對 Up 都只呼叫
`ReleaseLeft()`，與 closed-canvas 內 `Move→Release` 契約不符；舊收據不能證明此格。
可丟棄原型現已在 closed canvas Up 先 Move 再 Release，畫布外／panel／失焦仍只
Release。`golang:1.26.7-bookworm` Docker 的 `go vet ./...`、`go test -race -count=1 ./...`
通過雙倍率四角與邊界負例；`bridge.go` SHA-256
`b984f77aec9edd2c774c8507a58662bdf3a054825ccca946a3852ae570d11977`，
`bridge_test.go` SHA-256
`f226695e1617a771184814e2868223606f7f07b1d85be4690f39c7b816bdf7ff`。
這只補純 fake 核心，實體 Ebitengine／dosgolem 矩陣與玩家效果仍未驗；維持 DRAFT。
現有 `MoveMouse`／`PressMouse`／`ReleaseMouse` 均為 void，沒有可檢查的 error 或 rollback；bridge
因此只能在每個 API 呼叫後更新自身 pressed state，並以單元測試釘住呼叫順序。host hit 由 backend 分類為 host consume，
不可誤進 bridge。

## READY 前測試

- 2×／3×各測 canvas 四角內點、chrome、右下邊界與負座標。
- left down 後再 up，確認 mouse 座標與 press/release 計數順序；不 Step machine。
- accepted Down 後分別測 canvas 外 Up、chrome Up、panel-open Up、window-focus-lost cleanup：皆只 Release、
  不 Move、清 button；沒有 Down 的同類 Up 零 DOS 呼叫。2×／3×各跑一例。
- closed canvas 唯一可轉送；open 的 host hit 與 miss 都零 DOS 差異。
- 每例比較 BIOS queue、KeyIRQs、indexed VRAM，均不得變更。

已完成的 prototype evidence：phase127 在同一 restored state、同一 closed-panel Ebitengine logical
click (200,200) 比較 no-forward／直接 DOS API；DOS mouse 由 (160,100)→(100,82)，而 indexed、BIOS、
IRQ、buttons/events/polls 不變。canvas `320×200` 來自 Mode 13h frame；2× closed chrome offset `36`、open
chrome offset `184` 來自 phase118 host layout prototype。這些是 DRAFT 幾何來源，仍需純核心 2×／3×
完整矩陣與有界 Step receipt。

## 驗證 oracle 與玩家效果

純核心 oracle 是 DOS mouse state／press-release 統計與完整 frame hash；它只驗 bridge 邊界。另一條
ignored Ebitengine/Xvfb normal-player receipt 必須以同一原版 state、同一實體 click、accepted Down/Up
後有界 Step，分開記錄 mouse state 與可見差異。若該 state 無可見反應，必須明載，不得推論遊戲可操作。

phase127 僅證實實驗 API state mutation、未 Step 的 indexed 相同；不能證明玩家可見效果。

## 2026-09-22 phase134 補充證據（仍為 DRAFT）

ignored 的真實 Ebitengine/Xvfb 2× receipt 已新增兩個狹窄案例：accepted canvas Down 後，harness
以既有 `PanelController` 設為 open，再由真實 chrome Up 做 cleanup；及 panel-open harness precondition
下的真實新 Down/Up。前者只出現 `MoveMouse(100,82)`、`PressMouse(0)`、`ReleaseMouse(0)`，Up 不移動
DOS 座標且清 button；後者零 DOS API、mouse／BIOS／IRQ／indexed hash 均不變，Up 後 50,000 Step
仍無 indexed 差異。完整 private receipt 位於主專案 `phase134-panel-open-up-2x-v2` 與
`phase134-panel-open-new-2x-v5`。

兩例的 X11 pointer edge 都是真實 Ebitengine 事件，但 panel state 是 harness precondition，非正式
backend host hit routing。因此它們不滿足 READY 矩陣中「panel open 的 host hit 與 miss」或 focus-loss、
orphan/repeated Up、3×對應格；本規格維持 DRAFT。

## 2026-09-22 phase135／137 補充證據（仍為 DRAFT）

主專案 `docs/re/phase-135-real-ebiten-mouse-focus-loss-and-release-edges.md` 的真實 X11
focus-loss 收據證明：2×已接受 Down 後，`ebiten.IsFocused()` 真→假時只
`ReleaseMouse(0)`、不 `MoveMouse`，DOS button 由 1 清為 0；孤兒與重複的實體
`mouseup` 沒有形成新的 Ebitengine public input release edge，不能冒稱 bridge
收到該 callback。3× panel-open 前提下的新 Down/Up 零 DOS API。

主專案 `docs/re/phase-137-real-host-panel-route-and-3x-cleanup.md` 又記錄 2×／3×真實
host Open hit，及 open-panel 空白 miss：hit 由 `PanelController` 消費，miss 核心
仍回 `ConsumedByHost=false,ForwardToDOS=false`，但 open-panel bridge 不送新事件進 DOS。
3×已接受 Down 後再由 harness 開面板、於 chrome Up 只 Release；這不是實體 host
hit 在 pressed 時開面板。上述案例均以 `cmd/state-compare` 核對起點相等，且沒有
可歸因的遊戲滑鼠可見效果。四角／邊界、雙倍率每種 cleanup、正式 miss route
與正常玩家因果 A/B 仍缺，故本規格不升 READY。

## 2026-09-22 READY 證據審查

結論：**維持 DRAFT，不能升 READY。** 更正先前搜尋深度不足的誤述：phase127 的可回讀 A/B 收據
位於主專案 `workplace/phase118-game-ebiten-active-story/out/phase127-clean-{no-forward,dos-mouse}/`
`pointer-ab-receipt.json`，其文件是 `docs/re/phase-127-pointer-miss-ab-prototype.md`，主專案登錄
commit 為 `5f20e0c`。兩份收據確實證實同一私有 restored state、同一 closed-panel Ebitengine
logical `(200,200)` 實體 click、且 machine 未 Step 時：no-forward 不變更 DOS state；實驗性直接
`MoveMouse/PressMouse/ReleaseMouse` 改 `(160,100)` 為 `(100,82)`，而 indexed、BIOS、IRQ、buttons、
events、polls 不變。

但 phase127 明示是 pointer-miss 的 direct API UX 實驗，`PanelEventPointerMiss` 回兩個 false，且一次
click 內直接 Move→Press→Release；它沒有 MouseBridge、沒有分離 Down／Up、沒有 3×、沒有 canvas
accepted Down、沒有 panel-open 新事件、沒有已 pressed 的外部 Up／focus-lost cleanup，也沒有有界 Step
玩家效果收據。因此不能替代下列矩陣或證明重複 Down／Up、pressed cleanup 與呼叫順序。

使用者已確認的語意本身完整，無需新增產品決策：closed canvas Left Down 才轉送；open panel 的新
pointer 全由 host 消費；已轉送 Down 後的 Left Up 不論落在 canvas 外、chrome、panel 或失焦，皆只
`ReleaseMouse(0)`，且不 `MoveMouse`。但下列每格都要以同一 private state 的可重跑 content-safe
receipt 釘住，才可升 READY：

| 倍率 | 事件序列 | 必要 DOS 觀測 |
| --- | --- | --- |
| 2×、3× | canvas 四角內 Left Down→Up | Move→Press；同點 Move→Release；座標換算正確 |
| 2×、3× | chrome／負座標／右下邊界 Down | 零 Move／Press／Release，VRAM、BIOS、IRQ 不變 |
| 2×、3× | accepted Down→canvas 外／chrome／panel-open／focus-lost Up | 僅 Release；最後 DOS 座標不變；pressed 清除 |
| 2×、3× | 無 Down 的各類 Up、重複 Down、重複 Up | 零額外 DOS 呼叫與零 state 改動 |
| 2×、3× | panel open 的 host hit 與 miss | 零 DOS 差異；不得形成新的 pressed state |

每例需同時記錄前後 mouse position、button／press-release counters、BIOS queue、KeyIRQs、indexed
SHA-256、machine memory SHA-256、所用 state／原版輸入 SHA-256、dosgolem commit 與完整 command。
另需一條有界 Step 的真實 Ebitengine click receipt；若無玩家可見反應，只能記為「未觀察到」，
不可據此宣稱 mouse 可操作。

## 2026-09-22 修正後畫布內 Up 的實體補證

主專案 `docs/re/phase-146-real-ebiten-inside-up-corrected.md` 以修正後的
ignored bridge 在真實 Ebitengine/Xvfb 重跑 2×與3×同畫布 Down→Up，兩例均觀測
`Move→Press→Move→Release`，DOS button 清除，API 邊界的 BIOS／IRQ／indexed／memory
保持不變。兩份私有 receipt 與 SHA-256 見該文件。這只補上原型勘誤中的畫布內 Up
呼叫順序；其餘四角、邊界、cleanup、正式 panel route 及正常玩家因果 A/B 仍缺，
本規格維持 DRAFT。

## 2026-09-23：DRAFT 純核心 router／result 補證

此節只將使用者已定案的 host 所有權語意寫成可丟棄的純核心契約，並補其單元測試；
**不**接 dosgolem production、**不**決定正式 resize／Ebitengine hit-test API、也**不**將本規格升
READY。實體 receipt 仍以各 phase 所記錄內容為準，不能由本節的 fake mouse 取代。

輸入分成 `RoutedPointer{Pointer, Target}`：`TargetCanvas`、`TargetHost`、
`TargetOutside` 是 host 在 Left Down 當刻完成的分類。`Router` 對 `TargetHost` 的 Down 建立
host capture；其後的 Left Up 必須依該 capture `ConsumedByHost=true`，即使面板開啟或新的版面座標
看似落在 canvas，也不可重新 hit-test 後轉送 DOS。closed panel 的 canvas Down 則由 `Bridge`
擁有；其後 Up／focus-loss 以既定的 DOS cleanup 優先，故可在 panel open 時唯一地觸及 DOS 並只
Release。panel open 的任何新 pointer（含 blank miss）均為 host consume，不建立 DOS pressed state。

`Result` 固定為 `{ConsumedByHost, ForwardedToDOS, Cleanup, Reason}`：`ForwardedToDOS` 只在該次事件
實際呼叫 DOS mouse API 時為真；`Cleanup` 只表示釋放既有的 DOS left press，host capture、orphan Up、
repeat Up 皆為假。已測的純核心格包括 2×／3×四角 Down→同畫布 Up、canvas-out／chrome／panel／
focus 的 release-only cleanup、每個位置的 orphan／repeat Up、duplicate Down、non-left、unknown kind、
invalid scale、negative chrome，以及 panel blank miss／host target 跨 panel-open 的 Up capture。它們均
檢查三個 result bit、DOS 呼叫數與順序。

可重現身分（僅此純核心補證）：dosgolem 本機分支
`buck-rogers-cht-output-overlay` 在 `c0f6d76b0eb60caa72e74a619c1b981c91b340a5`，當時 worktree clean；
ignored `workplace/phase128-mousebridge-prototype/bridge.go` SHA-256
`e3bdf4597895cd86f9e19c1eb1abfaf045e8f3ee522ea81ddb9bcbbfc89223c8`，
`bridge_test.go` SHA-256
`7b24568fbeea447f785de755cadc1e407625e2234c1843dfad42b5c03799d596`。執行命令如下：

```sh
docker run --rm --network none --memory 1g --cpus 2 --pids-limit 256 \
  -u "$(id -u):$(id -g)" -e GOCACHE=/tmp/phase128-go-build \
  -v /home/anr2/cht/golden_box/拯救地球:/repo:rw \
  -w /repo/workplace/phase128-mousebridge-prototype golang:1.26.7-bookworm \
  sh -c 'gofmt -w bridge.go bridge_test.go && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./...'
```

結果：`go vet ./...`、`go test -count=1 ./...`、`go test -race -count=1 ./...` 全數通過。
先前段落所列舊 bridge hash 保留作其所屬 phase 的歷史定位；不得誤當成本節的 source 身分。

## 2026-09-23：DRAFT immutable frame snapshot／epoch 補證與無 guard 停止線

本節採用既定的 Ebitengine、2×預設／Apply 切 3×、以及 Down target capture；它不改 UX、
不接 production、也不升 READY。每個 `Update` 必須建立一份不可變的
`LayoutSnapshot{Epoch, Scale, Chrome, CanvasW, CanvasH, FrameW, FrameH, PanelOpen}`；
同一輪的 hit-test、canvas 邊界與 DOS 座標換算只能使用該份 snapshot。它必須滿足
`CanvasW=320*Scale`、`CanvasH=200*Scale`、`FrameW>=CanvasW`、
`FrameH>=Chrome+CanvasH`，且 epoch 嚴格遞增。無效／過期／非單調 snapshot 一律不替換目前
snapshot 並 fail-closed。

已接受的 DOS canvas Down 記住其 Down epoch：同 epoch、closed canvas 的 Up 才可依該 snapshot
`Move→Release`；若 Apply／resize 已換 epoch，第一個 current-epoch Up 只 `Release` 最後有效 DOS
座標，不能用新幾何重分類或重算 Move。host Down capture 則綁定至其 Up，跨 epoch 仍 host consume；
新 Down 一律依新 epoch 判定。純核心 `epoch_test.go` 已覆蓋 2×／3×同 epoch、2× Down→Apply3×→
release-only、host capture 跨 epoch、以及 stale／inconsistent snapshot 的零 DOS 呼叫。

純核心命令（Docker、無網路、`go vet`、一般及 race 測試皆通過）：

```sh
docker run --rm --network none --memory 1g --cpus 2 --pids-limit 256 \
  -u "$(id -u):$(id -g)" -e GOCACHE=/tmp/phase128-go-build \
  -v /home/anr2/cht/golden_box/拯救地球:/repo:rw \
  -w /repo/workplace/phase128-mousebridge-prototype golang:1.26.7-bookworm \
  sh -c 'gofmt -w bridge.go bridge_test.go epoch.go epoch_test.go && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./...'
```

無 guard 的真實 Ebitengine/Xvfb 右／下邊界停止線：以既有
`physical_mouse_receipt.sh geometry <2|3> <right|bottom>` 移除原先 `+1 logical pixel`
視窗／driver guard 後，四個輸出 `phase158-no-guard-{2x,3x}-{right,bottom}` 都只寫出
`mouse-ready.json`／before state，沒有 `mouse-receipt.json`。X11 實測映射為 2× right
`(640,36)`、bottom `(0,436)` 於 640×436 視窗外緣；3× right `(960,54)`、bottom `(0,654)`
同樣映到固定 640×436 X11 視窗外緣。因此既有 Ebitengine public input surface 無法在**不改變
正式 logical canvas**下觀測 exclusive right／bottom；不得以這些無 receipt 的 run 宣稱邊界通過，
也不得恢復 guard 作正式幾何。本項在選擇另一個已批准的觀測機制前維持停止。

既有真實 host 2×→Apply3× runner 仍可重跑，輸出
`workplace/phase118-game-ebiten-active-story/out/phase158-real-apply-2x-to-3x/receipt.json`
SHA-256 `de9419ea0494505a9fc7081765526f2705d1afd2d80d5881c4ea6070b0fefcb4`：
`initial_scale=2`、`apply_3x_closes=true`、`physical_host_events_no_dos_writes=true`。這只證實真實
host Apply transition 與 DOS 隔離；因不能以同一 Left press 完成 Apply，它不取代上列跨 epoch
captured Up 的純核心契約或正常玩家路徑驗收。

本節 source 基準為 dosgolem `95f2c456368e316107517c28ca188392d1357e13`；ignored
`epoch.go` SHA-256 `fb052a423e890a1a2b79d8549006acf522c015e24c9d4cbd6f70e5d4aac14870`、
`epoch_test.go` SHA-256 `f3d8e11ad0637136807bcb1ced266d2c620d13ca19780fc30b73e101b17fc9eb`。

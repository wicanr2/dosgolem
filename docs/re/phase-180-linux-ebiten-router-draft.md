# 第一百八十階段：Linux Ebitengine host router DRAFT

日期：2026-09-23  
對應規格：[spec 230](../spec/230-linux-ebiten-host-event-loop-draft.md)（DRAFT；等待獨立審查）。

## 範圍

本階段只建立未被 production command import 的 `frontend/ebiten` 候選 router，以及其單元測試。
它依現有 `PanelController`、`KeyboardBridge`、限縮 CONFORMED 的 `MouseBridge` 操作；沒有新增
Buck Rogers adapter、原版輸入注入、翻譯文字、字型 bytes、原版檔案或任何玩家路徑聲明。

候選每次只在 active scale 或 panel open state 改變時換 `MouseLayout.Epoch`。測試固定一般
closed-canvas Down→idle refresh→Up 仍是同 epoch 並完成 `Move, Press, Move, Release`；open panel
才換 epoch。因此不會以每幀重建 layout 意外降格成 release-only。

候選要求 caller 給予本機 `xlate.Font`，並 fail-closed 驗證「設定／套用／取消／2×／3×」的每個
glyph；chrome 確實以該字型繪出這些標籤。snapshot 邊界固定拒絕非 320×200、非 active scale、
與 current layout 不一致或 RGBA 精確長度不符者，避免錯誤來源造成超大配置或畫面裁切。

## 離線驗證收據

工具：`eob-remake-go:1.26.7-ebiten2.9.9`，容器內 Go 為 `/usr/local/go/bin/go`；相依採 phase118
已驗證的 Ebitengine `v2.9.9` 雜湊，環境固定 `GOPROXY=off`、`GOSUMDB=off`。Ebitengine 初始化需要
X11，故 Docker container 內啟動 bounded `Xvfb :99 -screen 0 1280x800x24`，以 shell `trap` 在結束時
停止它；無 DISPLAY 的 GLFW failure 是環境預期，不是產品結果。

```sh
docker run --rm --network none --memory 2g --cpus 2 --pids-limit 256 \
  -u "$(id -u):$(id -g)" -e GOCACHE=/tmp/dosgolem-ebiten-cache \
  -e GOPROXY=off -e GOSUMDB=off \
  -v /home/anr2/cht/golden_box/拯救地球/workplace/dosgolem:/src:rw \
  -w /src eob-remake-go:1.26.7-ebiten2.9.9 \
  bash -lc 'Xvfb :99 -screen 0 1280x800x24 >/tmp/dosgolem-ebiten-xvfb.log 2>&1 & xvfb_pid=$!; trap "kill $xvfb_pid 2>/dev/null || true" EXIT; export DISPLAY=:99; /usr/local/go/bin/go test ./host ./presentation ./frontend/ebiten && /usr/local/go/bin/go vet ./host ./presentation ./frontend/ebiten && git diff --check'
```

結果：通過。`frontend/ebiten` 的 pure router test 覆蓋 idle epoch、closed canvas Down→Up、host
capture 跨 layout Up、open-panel blank miss、Cancel、Apply 3×、accepted Down 的 focus-loss
release-only、DOS key map、host font 和 snapshot fail-closed；`host`、`presentation` 既有測試與
`go vet` 同時通過。這是 unit／static verification；未執行完整實體玩家視窗 smoke，因 DRAFT router
尚無 production launcher。Ebitengine public input callback 至 `routePointer` 的實體 X11 event mapping
亦維持未驗，不得由純 router 綠燈外推。它也不是原版 same-state 收據。

## 停止線

等待獨立審查 spec 230、候選 source 與本收據；在升 READY 前不得由任何正式 command import，
不得新增 `buckrogers-player` 或聲稱 raw framebuffer 是可玩中文版。

## 2026-09-23 實體視窗與譯文注入補充（仍為 DRAFT）

前端現改由呼叫端提供 `HostLabels`；畫面程式不硬編繁中，並按 2×／3× 各自驗證
五個標籤的字形存在、非零墨跡及安全矩形。選取 Apply 後會同步更新實際視窗大小。
可選實體測試直接讀取專案 `text/host-ui.zh-TW.tsv` 的五個 key；該 catalog SHA-256 為
`c430f4424da2f090c4031a4079c1043fbd47dd6fa779c6eb208abcc5abf36b76`。

主代理以同一離線 Docker image 與容器內有界 Xvfb 執行
`go test -count=1 ./frontend/ebiten ./apps/buckrogers` 和 `go vet`，皆通過；實體 X11
按鍵順序為開面板、選 3×、Apply、畫布點擊。收據為
`workplace/phase181-ebiten-callback/out/receipt.txt`（專案本機 ignored），SHA-256
`8d1968ad3f48c1c1e60789e4dc49e2348f7c7fac70bdac1bc53b492c9b2a1c03`；
記錄最後倍率 3×、DOS mouse 呼叫僅 `move,press,move,release`，Update／Draw／Advance
同 goroutine。2× 截圖 640×436，SHA-256
`870780069e41b5f42c29eb07c252974f987f3a476504cb36fc7808d0ad313795`；
3× 截圖 960×654，SHA-256
`113d77d946f8bccb70d2e8d458f23c4f76cd8eb0e87be763c9d2705be7275e72`。
截圖中只含 host 面板與測試色塊，沒有原版遊戲畫面；因此這證明實體事件及視窗倍率，
不證明 Buck Rogers 正常玩家路徑、翻譯覆繪或正式前端已可玩。

## 2026-09-23 真實 Buck Rogers 畫布 smoke（仍為 DRAFT）

同一個 opt-in `frontend/ebiten.TestPhysicalDraftOpenApplyCanvas` 已改為載入使用者本機
`workplace/phase54/a-joined.state`，並以既有
`MachineFrameSource → LayerSnapshotProvider → Config.Snapshot` 讀取 machine 當下的原版
320×200 indexed framebuffer 與 palette。測試的 layer 是空 layer：這刻意排除 watcher、譯文、
手冊與 overlay lifecycle；它只量「Ebiten 畫的是原版畫布，不是前一版的合成色塊」。原始 state
記錄的 DOS 根目錄為 `/orig`，所以容器必須把合法原版目錄唯讀掛到該精確位置。

本機倚天 GOLEMFNT 仍只從 ignored 工作區讀取；正式 `text/host-ui.zh-TW.tsv` 的五個 host key
仍是唯一 label source（SHA-256：`c430f4424da2f090c4031a4079c1043fbd47dd6fa779c6eb208abcc5abf36b76`）。
3× 的 22×22 host raster 僅在測試程序記憶體中產生，不加入 repository 或交付包。

離線 Docker/Xvfb 收據輸出在 ignored
`workplace/phase230-real-canvas-smoke/`：`ui-2x.png` SHA-256 為
`c0749d033bd6970141831f6190982beac10bfcec1b34efd23602b0755ef387d8`，`ui-3x.png` 為
`416c437bd73a607a6527744584219347d98137f37265422ac8a0fa11fafc7c69`。receipt 記錄 3×
RGBA SHA-256 `960c4145e9cf59056b0a485a6f5a0cb08aa0df353667496e572fd7285a566a38`、8 個 distinct RGB
及 host 操作前後相同的 raw indexed SHA-256
`0c43f315a772f9f10281eb1fe38ebcb43a9d0cdc0b106f3cb06f49f707e7d003`。真實 X11 序列是
設定 →（面板開啟 Enter）→ 選 3× → 套用 →（面板收合 Enter）→ 畫布 left click；
receipt 的 BIOS pending 為 `0 → 1`，所以第一個 Enter 被 host 消費，收合後才有一個 Enter
可交給 DOS。host 操作沒有 Step 或寫 VRAM。

最小重跑命令（先確認所有掛載來源存在）為：

```sh
docker run --rm --network none --memory 3g --cpus 2 --pids-limit 384 \
  -u "$(id -u):$(id -g)" --entrypoint /bin/sh \
  -v "$PROJECT:/project:rw" -v "$PROJECT/workplace/original/BRcdoom:/orig:ro" \
  -v "$PROJECT/workplace/dosgolem:/src:rw" -w /src \
  eob-remake-go:1.26.7-ebiten2.9.9 -lc '
    export PATH=/usr/local/go/bin:$PATH
    Xvfb :99 -screen 0 1280x1024x24 & xvfb=$!
    trap "kill $xvfb 2>/dev/null || true" EXIT
    DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 EBITEN_PHYSICAL_DRAFT=1 \
      EBITEN_RECEIPT_OUT=/project/workplace/phase230-real-canvas-smoke \
      GOCACHE=/project/workplace/gocache \
      go test -run "^TestPhysicalDraftOpenApplyCanvas$" -count=1 -v ./frontend/ebiten
  '
```

這不是 production、不是可玩版，也不是中文化／same-state A/B 收據。升 READY 前仍缺完整冷開機與
正常玩家路徑、真實 watcher/active translation layer 的 lifecycle、存讀檔後續行為，以及 3× host
字型的獨立量測與玩家可見審查。

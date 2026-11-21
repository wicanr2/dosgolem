# 441 — 通用的即時前端 `cmd/play`（畫面、鍵盤、滑鼠、聲音）

狀態：**READY**
日期：2026-10-18
前置：[`240-opl-audio-output-draft`](240-opl-audio-output-draft.md)、[`440-mixer-cd-audio`](440-mixer-cd-audio.md)
案例：三國志V（san5）

---

## 1. 為什麼要做

能跑任意程式的前端都不能出聲：`webplay`（`200-webplay`）沒有音訊也沒有滑鼠，`buckrogers-play` 只認 Buck Rogers
（`session.Owner` 的開機要求 SHA-256、存檔目錄與 Buck 的畫面配置）。三國志V 要用滑鼠操作，音樂是 CD 音軌、音效是 OPL，
要邊玩邊聽，需要一個不認識特定程式、但接上混音器的前端。

## 2. 語意

### 2.1 啟動

- `play -prog <.COM|.EXE> -root <目錄> [-dir] [-scratch] [-args] [-cpu 386|186|8086] [-program-path] [-adlib]
  [-cdrom D:<曲目表>] [-cdrom-image <映像>] [-input bios|hw|both] [-clock 百分比] [-scale 倍率]
  [-frames N] [-wav <輸出>] [-shot <輸出.png>]`。
- 機器與 DOS 的建法同 `webplay`：`machine.New`、`dos.New`、`Install`，不經 `session.Owner`。`-program-path` 沒給時用
  `C:\` 加執行檔名的大寫（同 `probe`）。`-cdrom` 的曲目表格式同 `434`／`438`。
- 混音器（`audio/mixer`）掛在 `ObserveOPLWrites`、`ObserveSpeaker`、`ObservePITChannel2`；給了 `-cdrom` 與
  `-cdrom-image` 再 `AttachCD`。

### 2.2 時間

- 每秒步數 `S = machine.StepsPerSecond() × clock ÷ 100`（取整數），混音器用同一個 `S`。
- 每個主機畫格（`ebiten.SetTPS(60)`）執行到 `目標(f) = (f + 1) × S ÷ 60` 步（整數除法，不累積誤差），程式結束或出錯就停。
- 每格結束時 `Render(本格起點步數, 本格終點步數)`。互動模式把樣本推進 `mixer.Ring`（上限 200 毫秒），自動模式不開播放裝置。
- 不做 `webplay` 的即時校正（不改 `IRQ0Base`）：改了每秒步數會讓混音器的時間軸失準。主機跟不上時聲音斷流
  （`Ring` 淡出淡入並計次），結束時印出畫格率與斷流次數。

### 2.3 畫面

- 每格把目前畫面轉成 RGB：平面模式用 `PlanarRGB`，mode 13h 用調色盤，其餘當 80×25 文字畫（同 `webplay` §3）。
- 視窗起始大小為 640×480 乘 `-scale`；`Layout` 回目前畫面的寬高，由 ebiten 縮放。

### 2.4 輸入

- 滑鼠：`Layout` 回畫面的寬高，所以 `ebiten.CursorPosition()` 已經是畫面像素座標；變化時 `MoveMouse`；左鍵、右鍵的按下與放開送
  `PressMouse`／`ReleaseMouse`（0 左、1 右）。游標在畫面外不送移動。
- 鍵盤：按下的鍵查表得到 set 1 掃描碼與 ASCII（字母依 Shift 與 Caps Lock 分大小寫，數字列與符號依 Shift）。
  `-input bios` 推 BIOS 鍵盤佇列（`PushKey`；單獨的 Ctrl、Shift、Alt 不推，實機 BIOS 不放進 int 16h 佇列），`hw` 送 IRQ1 掃描碼（按下送碼、放開送碼加 80h），`both` 兩者都送。預設 `bios`。

### 2.5 自動模式

- `-frames N`（N > 0）：不開播放裝置，跑 N 格後結束，不讀輸入。`-wav` 把混音樣本寫成 48 kHz 16 位元立體聲 WAV
  （轉換同 `440` §2.2：×32768 四捨五入再夾），`-shot` 把最後一格存成 PNG。
- 自動模式的混音與 `probe -dump-mix-wav` 跑同樣的步數時逐樣本相同（同一個混音器、同一條時間軸）。

## 3. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 主機跟不上 | 聲音斷流，遊戲照固定步數前進 | DOSBox 降低模擬速度 | 刻意不照做：混音器的時間軸以步數為準 |
| `-clock` 不是 100 時的 CD | CD 照真實時間的 44.1 kHz 播，遊戲以為的播放位置（MSCDEX 以 `StepsPerSecondNow` 算）不同步 | 實機 CD 與 CPU 速度無關 | remake-owned。到期條件：有程式依賴兩者同步時再處理 |
| 狀態存讀、截圖鍵、倍率切換 | 沒有 | `buckrogers-play` 有 | remake-owned。到期條件：有需求時加 |
| 鍵盤 | 只涵蓋常用鍵（字母、數字、符號、方向、功能鍵、Enter、Esc、Tab、Backspace、Shift、Ctrl、Alt、Space、Home 等） | 實機 101 鍵 | remake-owned |

## 4. 驗收

1. `cmd/play/play_test.go`：
   - `TestFrameTargetsNoDrift`：`S = 11,580,882` 時 600 格的目標步數總和等於 `600 × S ÷ 60`，每格步數只差 0 或 1。
   - `TestKeyTable`：字母依 Shift 分大小寫、`Enter` 給 0Dh、方向鍵 ASCII 為 0；掃描碼抽查與 `webplay` 的表相同；單獨的 Shift、Ctrl、Alt 判為修飾鍵。
   - `TestMouseInside`：640×480 的畫面，(639, 479) 在畫面內，(640, 0)、(0, 480)、(−1, 0) 在畫面外。
2. 負對照：目標步數改成每格固定 `S ÷ 60`，第 1 條的第一個測試必須失敗（2026-10-18 手動確認後還原）。
3. 上層收據（三國志V，在容器內以 xvfb 跑）：`play -frames 600 -wav` 與 `probe -steps 115808820（= 600 格的目標 600 × S ÷ 60）-dump-mix-wav`
   的 WAV 逐位元組相同，且其中有開場第 2 軌的聲音。

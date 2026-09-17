# 201 — 逐步操作：以機器時間送鍵、一次一步

狀態：**READY**
日期：2026-09-17
前置：[`199-live-session-api.md`](199-live-session-api.md)（`RunCycles`、`KeyDown`／`KeyUp`、狀態檔）、[`198-dosbox-cycles.md`](198-dosbox-cycles.md)

---

## 1. 問題

要讓代理（或測試腳本）**像玩家一樣**操作：看一眼畫面，決定按什麼、按多久、等多久，再看下一眼。
現有的兩條路都不合適：

- probe 的 `-press`／`-hold` 以「第幾道指令」排程，要先知道步數；看畫面決定下一步時，步數是未知的。
- 前端（Ebiten）要常駐行程與視窗，代理的每次工具呼叫之間無法保留它。

需要的是：**每一步是一次獨立的指令**，從狀態檔開始、做幾個以毫秒計的動作、存回狀態檔與畫面。

## 2. 規格

### 2.1 動作腳本

`oracle.ParseActions(s string) ([]Action, error)`，逗號分隔，每項：

| 寫法 | 意義 |
|---|---|
| `tap:<鍵>[:<ms>]` | `KeyDown`，跑 ms（預設 150），`KeyUp` |
| `hold:<鍵>:<ms>` | 同 `tap`，ms 必填 |
| `down:<鍵>`、`up:<鍵>` | 只按下／只放開（按著的鍵跨步保留在狀態檔之外，§2.4） |
| `wait:<ms>` | 不動，跑 ms |
| `type:<文字>` | 每個字元 `tap:<字元>:150` 後 `wait:150` |

- 鍵名：`internal/dos` 的具名鍵（`Return`、`Space`、`Esc`、方向鍵…，不分大小寫；`internal/dos` 目前沒有功能鍵，`F1` 會被拒絕），或單一字母、數字（`KeyForRune`）。
  不分大小寫是在 `oracle` 鏡射一份鍵名清單比對；`internal/dos` 加鍵名時要同步。
- 認不得的鍵、ms ≤ 0、未知動作：整串拒絕並回錯，不執行任何一項。

### 2.2 執行

`(*Oracle).RunActions(acts []Action, chunkMs float64, onChunk func()) error`：

- 需要 DOSBox cycles（`SetDOSBoxCycles`）；沒開時任何動作都不執行，直接回錯誤。
- 每個「跑 ms」先算總 cycles（`round(ms × DOSBoxCycles())`）再切段，避免逐段四捨五入累積誤差。
- 每個「跑 ms」切成每段 `chunkMs`（≤ 0 用 1000/60）機器毫秒的 `RunCycles`，每段之後呼叫 `onChunk`（可為 nil）。前端每幀做的事（例如疊字定色）放這裡，與前端的時間粒度一致。
- 程式結束（`ExitError`）照樣回傳，呼叫端決定怎麼處理。

### 2.3 `cmd/step`

```
step -exe <PW.EXE> -root <目錄> [-load-state <in>] [-cycles 750] -do "<動作>" \
     [-save-state <out>] [-shot <out.png>] [-scale 3] [-scratch <目錄>]
```

- `-exe`、`-root` 必填（`oracle.Load` 需要）；給了 `-load-state` 就在載入後整個蓋掉。沒給時從程式進入點開始。
- 結束印一行摘要：累計步數與 cycles（串下一步用）、這一步跑掉的機器毫秒、視訊模式、按著的鍵。
- `-shot`：`ScreenRGB()` 以 nearest 放大 scale 倍存 PNG。

### 2.4 限制

- 狀態檔不保存「按著的鍵」與 typematic 排程：`down:` 之後存檔、下一步再載入時，那個鍵視為已放開。需要跨步按住就在同一步裡用 `hold:`。
- 動作之間沒有牆上時間；ms 全部是機器時間（`DOSBoxCycles()` × ms 個 cycle）。

## 3. 驗收

1. `ParseActions`：`tap:Up,wait:500,hold:Space:2000,type:ab` 解出 1＋1＋1＋4 項，時間與鍵正確；`tap:Nope`、`wait:0`、`jump:1` 整串回錯。
2. 合成程式（`jmp $`）開 750 cycles：`wait:1000` 跑了 750,000 個 cycle（±最後一道指令）；`chunkMs` 16.67 時 `onChunk` 呼叫 60 次。
3. `tap:Up:150` 之後機器排過一個 48h 按下碼與一個 C8h 放開碼，兩者相隔 150 ms 份的步數（±1 段）。
4. 決定性：同一狀態、同一腳本跑兩次，存出的狀態檔逐位元組相同。
5. `cmd/step`：`-do "wait:100" -save-state a` 再 `-load-state a -do "wait:100" -save-state b`，與一次 `-do "wait:100,wait:100"` 的狀態檔 cycles 相同。

## 4. 不做

- 滑鼠動作（沿用 `MoveMouse`／`Click`）。
- 跨步保留按著的鍵。
- 疊字、字型等遊戲專屬的畫面合成（由呼叫端在 `onChunk` 與存圖時處理；通用部分見 `202-translation-overlay`）。

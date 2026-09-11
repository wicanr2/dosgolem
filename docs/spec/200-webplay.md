# 200 — `cmd/webplay`：在瀏覽器裡即時玩一支 DOS 程式

狀態：**READY**
日期：2026-09-11
前置：[`005-oracle-api`](005-oracle-api.md)、[`006-layering`](006-layering.md)、[`190-irq0-calibration`](190-irq0-calibration.md)

---

## 1. 定位

dosgolem 本體是無頭、決定性的執行器。`cmd/webplay` 是它上面的一個**互動外殼**：
載入一支程式、以接近真實時間的速度執行，畫面透過 HTTP 送給瀏覽器，瀏覽器的按鍵送回來。
用途是「人自己玩玩看」——例如確認自己用 BCC 編的程式在 dosgolem 上的樣子。

它**不是**對拍工具：按鍵時機來自人，執行速度跟著主機負載走，重跑不會得到相同結果。
要決定性的觀測照舊用 `cmd/run`、`cmd/shots` 與 `oracle`。這一條寫在工具的說明裡。

分層（`006-layering`）：只用 `internal/machine`、`internal/dos` 的既有公開介面；
不認識任何特定程式，位址一律不寫死。

## 2. 介面

```
webplay -prog /orig/TETRIS.EXE -root /orig [-scratch DIR] [-dir SUB] [-args " ..."]
        [-cpu 8086|186|386] [-listen 0.0.0.0:8086] [-input bios|hw|both] [-realtime=true]
```

| 路徑 | 方法 | 內容 |
|---|---|---|
| `/` | GET | 內嵌的單頁：canvas、按鍵轉送、狀態列 |
| `/frame?have=H` | GET | 目前畫面的 PNG；回應標頭 `X-Frame` 是畫面內容的雜湊。畫面雜湊等於 `H` 時回 `204` |
| `/key` | POST | JSON `{"code": KeyboardEvent.code, "key": KeyboardEvent.key, "down": bool}` |
| `/status` | GET | JSON：指令數、計時器中斷數、每秒指令數、目前模式、是否結束與回傳碼、主控台輸出 |

## 3. 畫面

| 模式 | 輸出 |
|---|---|
| 平面模式（0Dh、0Eh、10h、12h…） | `Machine.PlanarRGB` |
| 13h | `VideoRaw` 的色號經 `Palette` |
| 其他（文字模式） | B800h 的 80×25 字元，以 `F000:FA6E` 的 8×8 字型畫成 640×200，屬性用固定的 16 色 CGA 色表 |

文字模式的顏色不走屬性調色盤與 DAC，是簡化；會在頁面上標明。

## 4. 速度：讓 BIOS 時鐘跟上真實時間

dosgolem 以指令數當時鐘：分頻 65536 時一個計時器中斷大約是 63 萬條指令（`190-irq0-calibration`），
要每秒 18.2 次就得每秒執行約 1,160 萬條指令。主機跟不上時，程式裡「每 N 個 tick 做一件事」的
節奏會整體變慢。

`-realtime`（預設開）的做法：

1. 每 1/60 秒執行一段；執行得比真實時間快就休息，不空轉。
2. 每 0.5 秒量一次實際的每秒指令數 `ips`，若跟不上，把 `IRQ0Base` 設成
   `ips × 17000 ／ 1193182 × 0.95`（`IRQ0Base` 是分頻 17,000 時一個 tick 的指令數，
   程式改過分頻時照比例換算，機制不變）；跟得上就不動。
   **只往下調、不調回**：主機負載恢復後 BIOS 時間仍然正確（跑得比真實時間快就休息），
   只是模擬 CPU 維持在較慢的速度；要恢復就重開 webplay。

效果是「模擬出來的 CPU 變慢、時間照常走」。用的是既有的公開介面：設 `Machine.IRQ0Base`，
再呼叫 `Machine.RecalcIRQ0()` 重算目前分頻下的間隔（與 `pitWrite` 走同一條路），不新增機器 API。
`-realtime=false` 則完全照指令數走，與其他工具相同。

## 5. 按鍵

`KeyboardEvent.code` 對應 IBM PC/AT set 1 掃描碼（方向鍵、字母、數字、Enter、Esc、空白、Backspace、
Tab、F1–F10、常用標點）；ASCII 取 `KeyboardEvent.key` 的單一可列印字元，Enter／Esc／Backspace／Tab
給對應控制碼，方向鍵與功能鍵給 0。

| `-input` | 按下 | 放開 |
|---|---|---|
| `bios`（預設） | 排進 BIOS 鍵盤佇列（`int 16h`） | 不送 |
| `hw` | 硬體佇列送 make code（IRQ1） | 送 break code |
| `both` | 兩者都送 | 硬體佇列送 break code |

預設 `bios` 的理由：程式沒自己掛 `int 09h` 時硬體佇列送不出去，會一直累積（`keyboard.go` 的 `KeyStalls`）。

## 6. 驗收

1. 單元測試：按鍵對照表（方向鍵、字母、Enter）；`/frame` 對 mode 12h 的機器回 640×480 PNG，
   同一畫面帶 `have` 回 204；`/key` 之後 BIOS 佇列多一個鍵；調降 `IRQ0Base` 之後 `IRQ0Every` 照比例變小。
2. `tools/go.sh test ./cmd/webplay` 全綠。
3. 在容器裡跑 Borland C++ 2.0 編的俄羅斯方塊，瀏覽器頁面看得到畫面、按鍵有反應（以 headless Chrome
   截圖佐證）。

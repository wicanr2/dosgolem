# 240 — OPL（AdLib）音訊輸出

狀態：**DRAFT**（2026-09-27）
日期：2026-09-27
前置：[`238-session-step-observer`](238-session-step-observer-draft.md)（觀測介面）、
[`190-opl-vgm-dump`](190-opl-vgm-dump.md)（logh3 分支的 OPL 寫入擷取，僅參考）、
`rich2/docs/spec/049`（波形逐樣本一致屬既定停止線）。

## 1. 為什麼要做

dosgolem 已模擬 OPL2／OPL3 的暫存器與計時器（`internal/machine` 的 `oplWrite`、`OPLRegs`、
0x388 狀態埠），程式能偵測到 AdLib 並寫入音樂資料，但沒有合成與播放。Buck Rogers 的
`BUCK.CFG` 設定為 AdLib（該遊戲的音效選項為 AdLib、Tandy 1000、PC 喇叭），發行版需要聽得到音樂與音效。

## 2. 證據與來源

- `Machine.oplWrite` 每次寫入都在更新暫存器狀態後追加 `OPLWrite{Reg, Val, Step, Bank}` 到 `m.OPL`（已證實，程式）。
- 合成核心來源：DOSBox-X `src/hardware/nukedopl.cpp`（1,596 行，SHA-256 `ddc4255e…cdabd8`）與
  `nukedopl.h`（SHA-256 `50ed8560…947d1c`），Nuked OPL3（Nuke.YKT，含 Nuked-OPL3-fast 修改），
  **LGPL-2.1-or-later**。本機路徑 `~/cht/DOSBox-X-MCP-Debugger/dosbox-src/`（使用者定案：dosgolem 擴充以
  DOSBox-X 原始碼為依據）。DOSBox 的 DBOPL 為 GPL，與本專案授權不相容，不採用。

## 3. 契約

### 3.1 machine：OPL 寫入觀測

- 新增 `func (m *Machine) ObserveOPLWrites(fn func(OPLWrite))`；`nil` 關閉。在 `oplWrite` 更新暫存器、
  追加 `m.OPL` 之後呼叫，參數為同一筆 `OPLWrite`。callback 不得改變機器狀態。
- `m.OPL` 的既有行為不變（`ClearOPL` 仍由呼叫端負責）。

### 3.2 session：選用觀測介面

- 新增選用介面 `OPLWriteObserver { OPLWrite(w machine.OPLWrite) }`。`StepObserver` 不變（規格 238 的
  必要方法不變）；Owner 掛上觀測器時，若它也實作 `OPLWriteObserver`，就以 `ObserveOPLWrites` 轉交。
- `StepView` 不新增方法。

### 3.3 合成核心：`audio/nukedopl`（LGPL-2.1-or-later）

- 獨立套件，檔頭保留原著作權與 LGPL 標示，套件目錄附 `COPYING.LGPL` 與來源說明（檔名、SHA-256、
  移植日期）。本套件以外的程式維持 dosgolem 原授權；二進位發行附 LGPL 全文與取得原始碼的方式。
- 忠實移植 Nuked OPL3 的暫存器處理、包絡、相位、打擊樂模式與 OPL2 相容模式；介面：
  `New(sampleRate int) *Chip`、`WriteReg(bank, reg, val uint8)`、`Generate(dst []int16)`（交錯立體聲）。
- 一致性：以同一份 C 原始碼在 Docker 內編譯的參考程式，對同一組合成暫存器序列輸出樣本，
  Go 移植逐樣本相同（可用固定取樣率與固定序列；序列為人工合成，不含原版音樂）。

### 3.4 前端：時間對應與播放

- 時間基準是模擬步數，不是主機時鐘：一個主機畫格推進 `stepsPerHostFrame` 步，對應 1/60 秒音訊。
- 每個主機畫格：把該格收到的 OPL 寫入依 `Step` 排序（本就遞增），換算成畫格內的樣本位置
  `(Step − 畫格起始步) × 每格樣本數 / stepsPerHostFrame`，依序「先合成到該位置、再寫暫存器」，
  最後補齊整格樣本。
- 輸出：ebiten 音訊（48,000 Hz，16-bit 立體聲）以環形緩衝供應；緩衝不足輸出靜音，過多丟棄最舊的部分，
  上限約 200 毫秒。暫停或視窗失焦時照原版速度的規則處理（不另調速）。
- 音量與靜音由前端設定（預設開啟），不影響機器。
- 無頭工具（receipt runner、oracle）不掛音訊，收據不變。

### 3.5 不做什麼

- 不做 Sound Blaster 數位音效（DSP／DMA 取樣播放）、Tandy、PC 喇叭；遊戲以 AdLib 設定執行。
- 不追求與實機逐樣本一致（`rich2/docs/spec/049` 停止線）；一致性只對 Nuked 參考實作。

## 4. 驗收

1. 單元測試：`ObserveOPLWrites` 依序收到每筆寫入且不改狀態；未掛觀測器時行為不變；session 只在觀測器
   實作 `OPLWriteObserver` 時轉交；時間對應（畫格邊界、同一步多筆寫入、空畫格）；環形緩衝不足／過多。
2. 一致性：Go 移植與 Docker 內 C 參考對三組合成序列（單音、打擊樂模式、OPL3 雙組）逐樣本相同。
3. 決定性：掛與不掛音訊觀測器，同一輸入的機器狀態雜湊相同。
4. Buck Rogers：無頭工具把標題與站內的 OPL 寫入合成為本機 WAV（只在 ignored workplace），確認非靜音、
   有節奏起伏；前端實機由使用者試聽確認。

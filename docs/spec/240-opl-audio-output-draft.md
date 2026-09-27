# 240 — AdLib（OPL）與 PC 喇叭音訊輸出

狀態：**READY**（2026-09-27，兩輪獨立審查後）
日期：2026-09-27
前置：[`238-session-step-observer`](238-session-step-observer-draft.md)（觀測介面、回合與 panic 模型）、
[`016-pit-and-speaker`](016-pit-and-speaker.md)（PIT 與喇叭模型）、
`rich2/docs/spec/049`（波形逐樣本一致屬既定停止線）。logh3 分支另有 OPL 寫入擷取成 VGM 的規格（該分支的 190 號），僅參考。

## 1. 為什麼要做

dosgolem 已模擬 OPL 暫存器與計時器、PIT 與喇叭資料線，但沒有合成與播放。Buck Rogers 發行版要聽得到
音樂與音效。

## 2. 證據

- `START.EXE` 內的設定畫面字串列出音效選項 `[1] AdLib`、`[2] Tandy 1000`、`[3] PC speaker`（已證實，字串）。
  使用者本機 `BUCK.CFG` 第二欄為 `A`（強推論：AdLib）。
- `Machine.SetAdLib` 預設關閉；關閉時狀態埠 0x388 回 0，程式偵測不到 AdLib（已證實，程式）。sealed session、
  `buckrogers-play`、receipt runner 都沒有打開它；目前所有 CONFORMED 收據都是 AdLib 關閉下取得。
- 以 `cmd/probe -adlib` 冷開機跑 150,000,000 步（已證實，本機量測）：
  - OPL 寫入 564 筆，約從第 59,769,987 步起持續（每千萬步約 50 筆），其中 key-on 79 次：標題畫面在播音樂。
  - PIT 通道 0 分頻值 16384（約 73 Hz），寫入 3 次；通道 2（埠 42h）寫入 424 筆、埠 61h 寫入 523 筆：
    **PC 喇叭同時在發聲**（強推論：部分音效走喇叭）。
  - 音樂節拍來源：IRQ0（73 Hz）為強推論；是否另以 OPL 計時器輪詢定速，未量（見 §3.6）。
- 合成核心來源：DOSBox-X `src/hardware/` 的 Nuked OPL3（上游 1.8，commit `cfedb09`；fork `1.8-fast.1`，
  commit `fb9afa9`；著作權人 Nuke.YKT 與 Tony Gies），LGPL-2.1-or-later：
  - `nukedopl.cpp` SHA-256 `ddc4255e433042afbc97ce0bbd376415db42faa2415c9741c582d1e9b4cdabd8`
  - `nukedopl.h` SHA-256 `50ed8560b776b90acf663f38e175f84820a61163793ee7774bd9ca6cea947d1c`
  - `nukedopl_wf_rom.h` SHA-256 `2d92fb305bc8ecf9d95c3bc26e340681f4433062d5db7cbaf57471a65d09bb99`
  - 呼叫方式依 DOSBox-X `adlib.cpp`：`OPL3_WriteRegBuffered` 加 `OPL3_GenerateStream`，49,716 Hz 以 `rateratio`
    重新取樣到輸出率。DOSBox 的 DBOPL 與 MAME fmopl 為 GPL，不相容，不採用。

## 3. 契約

### 3.1 機器設定：AdLib 開關

- `session.Config` 新增 `AdLib bool`，在第一個指令前呼叫 `SetAdLib`；預設 false。
- `session.StateDigest` 新增 `AdLib bool` 欄位（取自 machine 的 `oplPresent`），使開關兩邊的收據可以分辨。
- AdLib 開啟時，session 一律掛一個音訊收集器（即使 `Config.Observer` 沒有實作 `AudioObserver`，
  也以空收集器接住），確保 `m.OPL`／`m.Speaker` 不會在長時間執行時增長。
- 開啟 AdLib 會改變原版的偵測與執行路徑，屬於不同的輸入條件：**開與關各自是獨立基準**，既有 AdLib 關閉的
  CONFORMED 收據不因本規格改變；開啟下的收據另行建立。
- `buckrogers-play` 預設開啟；無頭工具預設關閉，另以旗標開啟。

### 3.2 machine：寫入觀測與序列上限

- 新增 `ObserveOPLWrites(fn func(OPLWrite))` 與 `ObserveSpeaker(fn func(SpeakerSample))`；`nil` 關閉。
  在 `oplWrite` 更新暫存器之後、`outSpeaker` 記錄之後呼叫。callback 不得改變機器狀態。
- 觀測器掛上時不再追加 `m.OPL`／`m.Speaker`（避免長時間執行無限增長）；未掛時行為不變。
- `outSpeaker` 的「值有沒有變」改以獨立欄位記錄上一次的 level／gate，不再依賴 `m.Speaker` 最後一筆；
  該欄位與喇叭狀態一樣不進快照（見下）。
- PIT 通道 2（新的通用能力）：解碼 43h 對通道 2 的命令（存取方式、低／高位元組狀態機、模式）與 42h 寫入，
  保存目前分頻值與模式；新增 `ObservePITChannel2(fn func(PIT2Change))`，事件帶步數、分頻值與模式，和
  `SpeakerSample` 同一條時間軸。通道 2 狀態不進快照，屬與 OPL 相同的已知缺口。
- `m.OPL`、`m.Speaker` 不在任何狀態雜湊內（現況如此，本規格明文固定）。
- 快照：`Snapshot`／`SaveState` 目前不含 OPL 狀態（`oplReg`、`oplRegs`、`oplPresent`、計時器），屬**已知缺口**：
  還原後偵測結果可能不同。本規格不補；合成器在任何不連續（Restore、觀測錯誤）後以 `OPLRegs` 兩組
  256 bytes 依序重放重建。session 目前沒有機器層 Restore；遊戲自己的 DOS 存讀檔不影響合成器。

### 3.3 session：選用觀測介面

- 新增選用介面 `AudioObserver { OPLWrite(machine.OPLWrite); SpeakerSample(machine.SpeakerSample) }`。
  `StepObserver`、`StepView` 不變。
- 依規格 238 §2.3／§2.5：只在 `runObserved` 內安裝、回合結束一律解除；callback 內 panic 走 `videoPanic`
  同一個鎖存成 `ObserverFault`；`Config.Observer` 為 nil 或未實作 `AudioObserver` 時不轉交。
- callback 只收集寫入（附步數），不在 Step 內合成。

### 3.4 合成：`audio/nukedopl`（LGPL-2.1-or-later）與喇叭

- 獨立套件；逐檔 SPDX `LGPL-2.1-or-later`、原著作權人、改作聲明與日期；目錄附 `COPYING.LGPL`（LGPL-2.1 全文）
  與來源說明（檔名、SHA-256、版本、commit）。
- 忠實移植 Nuked OPL3：暫存器處理、包絡、相位、噪音、打擊樂模式、OPL2 相容（`newm=0`）與 OPL3 第二組；
  固定巨集 `OPL_ENABLE_STEREOEXT=0`、`OPL_QUIRK_CHANNELSAMPLEDELAY=1`。介面依 DOSBox-X 用法：
  `New(rate)`、`WriteRegBuffered(addr uint16, v uint8)`、`GenerateStream(dst []int16)`（交錯立體聲）。
- 位址：machine 的 `Bank 0` → `addr = reg`；`Bank 1` → 依 DOSBox-X OPL3 模式規則，只有 `reg == 0x05` 或 `newm`
  已設時 `addr = 0x100 | reg`，否則對到第一組（`adlib.cpp:386`）。Sound Blaster 別名埠（0x220–0x223、0x228/0x229）
  不在範圍內；以埠統計確認 Buck 只寫 0x388/0x389。
- 已知差異（寫明，不修）：Nuked 在 `newm=0` 時忽略 WSE（`nukedopl.cpp:554`），沒有 OPL2 的 DAC 特性；
  狀態埠低位元與 DOSBox-X OPL2 模式（`|0x06`）不同，屬 machine 既有行為，本規格不改。
- PC 喇叭：輸出 =（gate 開時）通道 2 方波 AND data 位元；（gate 關時）直接跟 data 位元。方波頻率 =
  1,193,182 ÷ 分頻值；不做抗混疊，頻率高於 24 kHz 時輸出靜音（已知差異）。與 OPL 混音。
- 計時器暫存器：位址對映後等於 0x02、0x03、0x04 的寫入**不送合成器**（DOSBox-X `Chip::Write`，
  `adlib.cpp:1050–1081`）；第二組在非 `newm` 下寫 04h，對映後也是 0x04，同樣不送。
- 第二組定址用前端送出當下追蹤的 `newm`（照 `adlib.cpp:379–380`），不讀 Nuked 內部值。
- 已知差異另一項：machine 只在第一組處理 04h 計時器（`machine.go:801`），DOSBox-X 在非 `newm` 下第二組
  寫 04h 也算計時器；本規格不改 machine。

### 3.5 前端：時間對應、播放與執行緒

- 時間基準是模擬步數：音訊時刻 = 絕對步數 ÷ 每秒步數。每秒步數取前端推進速率 `stepsPerHostFrame × 60`
  （11,550,000）；與 `StepsPerSecond()`（約 11,580,851）相差約 0.27%，音樂與畫面一起比 PIT 名目頻率慢，接受。
- 樣本位置以「絕對步數 → 絕對樣本序號」換算，不累積誤差；每格實際產生的樣本數依 `Advance` 實際執行的步數
  （`receipt.Steps`）決定。暫停回合（0 步）不前進、輸出靜音；程式結束後停止。
- 寫入依步數排序後以 `WriteRegBuffered` 送進合成器，由 Nuked 的寫入緩衝處理同一樣本內的先後
  （例如同一 tick 先 key-off 再 key-on）。
- 播放：ebiten 音訊 48,000 Hz，建議 `NewPlayerF32`。合成只在 Update goroutine；環形緩衝是唯一跨 goroutine 物件，
  以 mutex 保護；播放端 `Read` 一律回滿請求長度（不足補 0）並對齊立體聲框。目標預填 50–100 毫秒，
  上限 200 毫秒（含 Player 自身緩衝），過多丟最舊；記錄緩衝不足次數。
- F2 只切換倍率，不影響音訊。ebiten 預設失焦時照常執行（`RunnableOnUnfocused` 為 true），音訊照常。
- 音量與靜音為前端設定，預設開啟，不影響機器。

### 3.6 未量項（實作時補）

- 音樂節拍是否只靠 IRQ0：以 `-adlib` 量測 OPL 計時器（04h）的寫入與狀態埠輪詢。若驅動以輪詢 OPL 計時器定速，
  machine 的計時器模型（啟動且未遮罩即視為逾時）會讓節拍錯誤，需另立規格修正。

### 3.7 授權

- dosgolem `LICENSE` 第 2 條 (d) 明列 `audio/nukedopl` 為 LGPL-2.1-or-later 第三方元件，並加一條例外：
  對含此元件的可執行檔，允許接受者為自用修改本作品及為除錯此類修改而逆向（LGPL-2.1 §6 的要求），
  不受第 6 條 (a) 及第 3 條的非商業前提拘束。
- 每份副本顯著告知使用 Nuked OPL3 且受 LGPL-2.1 規範：`buckrogers-play --version` 與發行包說明文件列出。
- 發行的二進位對應的確切 commit 必須公開可取得；否則附原始碼包或三年有效書面提供。
- 發行二進位附 LGPL-2.1 全文、元件來源說明，以及整個連結程式的完整對應原始碼取得方式（本儲存庫公開原始碼；
  另附三年有效的書面提供或直接附原始碼包）。
- 法律細節建議日後請專業人士確認。

### 3.8 發行前檢查

- 任何二進位發行前，`LICENSE` 例外條文、`COPYING.LGPL`、各檔 SPDX 與來源說明都已到位。

### 3.9 不做什麼

- 不做 Sound Blaster 數位音效（DSP／DMA）、Tandy；不追求與實機逐樣本一致（`rich2/docs/spec/049`）。

## 4. 驗收

1. 單元測試：兩個觀測器依序收到每筆寫入且不改狀態；喇叭去重在停止追加後仍正確；PIT 通道 2 解碼與事件；
   計時器暫存器不送合成器；`StateDigest.AdLib` 反映開關；掛上時不追加序列；session 只在回合內、只對
   `AudioObserver` 轉交，回合外不轉交，panic 鎖存為 `ObserverFault`；Bank 1 定址規則；時間對應
   （畫格邊界、同一步多筆、空畫格、暫停、預算未用完）；環形緩衝不足／過多。
2. 一致性：Docker 內以 g++ 編譯 DOSBox-X 的三個原始檔作參考程式；C 參考與 Go 讀同一份「樣本位置 → 寫入」
   排程檔，對固定 seed 的隨機暫存器序列
   （涵蓋 0x104/0x105、打擊樂模式、4-op，各數秒）與人工序列，Go 移植以同樣的帶延遲寫入與重新取樣，逐樣本相同。
3. 決定性：掛與不掛音訊觀測器（同為 AdLib 開啟），同一輸入的機器狀態雜湊相同。AdLib 關閉的既有回歸不變。
4. 前置閘門：先完成 §3.6 的節拍來源量測；若驅動以輪詢 OPL 計時器定速，本規格暫停，另立規格修正計時器。
   Buck Rogers：無頭工具（時間基準與前端相同，每秒 11,550,000 步）以 `-adlib` 把標題與站內的 OPL／喇叭寫入合成為本機 WAV（只在 ignored workplace），
   確認非靜音；key-on 間隔換成秒，與 DOSBox-X 的 OPL 擷取對照節拍（DOSBox-X 只作診斷）；
   前端實機由使用者試聽確認。AdLib 開啟後，前端端到端路徑（phase-278）重跑無殘字。

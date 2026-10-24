# 461 讀入狀態檔後的計時器頻率：通道 0 讀數一律取 IRQ0 實際使用的分頻

狀態：**READY**（第 3 版，2026-10-23；初審與複審各兩位獨立審查者，審查紀錄在陸空戰將 repo `workplace/perf100/review460-461*.md`）。2026-10-24 已實作，現有素材範圍已複驗，未標 CONFORMED。既有單元、負對照、perf100 與 15 支陸空戰將長測試紀錄見陸空戰將 repo `workplace/impl461/receipt.md`；本輪見 §7。MOO2 缺素材，不能列為已驗證。
基準：dosgolem-sc `395b679`。來源：strike_commander `workplace/perf100/report.md`「時間基準」節；446 §4.1 登記的既有缺口。

## 0. 處置紀錄

### 0.1 第 3 版

複審報告：`workplace/perf100/review460-461v2-A.md`（甲）、`review460-461v2-B.md`（乙），兩位皆無阻擋。主代理裁決：應改全部採納；
`PITProgrammed()` 採甲建議 1。

| 意見 | 處置 | 節 |
|---|---|---|
| 甲建議 1（主代理採納） | `PITProgrammed()` 改讀 `setPITDiv` 的寫入計數，`LoadState` 清零；刪除 `Machine` 的 `m.pit` 欄位與 `pit.out` 呼叫及其讀者；寫明對分歧情形的影響 | §2、§3.3 |
| 甲建議：新負對照 | `sb8Delta` 時基改用常數 65,536 時 `TestSB8DeltaAfterLoadState` 必失敗 | §5 |
| 甲應改 1、乙 M2：rich2 | `rich2/state_test` 的 `load(t)` 已查證為冷開機（`rich2_test.go:11-23`、`rich2.go:35-37`，乙核對），不覆蓋讀檔路徑；§5 抽樣涵蓋 rich2 | §3.2、§5 |
| 甲應改 2、乙 M4：容差 | §5 第 4 條並列兩算式，要求相對差 < 1e-9 且取整後相等 | §5 |
| 乙 M1：抽樣機制 | 刪除 `m.pit` 後改為 machine 套件內測試，把 `PortLog` 的 40h、43h 寫入重播給獨立的 `pit` 解讀器（型別仍供 LE 使用）；0 映射 65,536；只在完整寫完後比 | §5 |
| 乙 M3：初值 | 寫明 `New()` 的 `PITDiv` 初值為 65,536（`machine.go:812`），補單元測試 | §2、§5 |

### 0.2 第 2 版

| 意見 | 處置 |
|---|---|
| 甲阻擋 1：改公式會改變浮點次序 | 維持 `StepsPerSecondNow() = IRQ0Every × PITHz()`，只改 `PITDivisor()` 的來源 |
| 乙 B2：零值 `Machine` 與 `pit_test.go:181-201` | `PITDiv` 為 0 回預設；該測試改走 `Out8` |
| 甲應改 2、乙 C1：消費者清單 | 寫進本體（§3.2） |
| 甲應改 4、乙 C3：`PITProgrammed()` | 第 2 版：`LoadState` 清零 `m.pit`；第 3 版改為寫入計數（§2） |
| 乙 C2：分歧實際場景 | 記入 §1 |
| 甲應改 5：pit2、ToneDecoder | 登記於 §4 |
| 主代理裁決：主案 A | 次案 A′、備案 B 移到 §6 |

## 1. 問題

機器裡有兩份 PIT 通道 0 的分頻，各自解讀同一組埠寫入：

| 欄位 | 解讀者 | 用途 | 狀態檔 | 記憶體快照 |
|---|---|---|---|---|
| `m.PITDiv` | `pitWrite`（`machine.go:1344-1346`、`1406-1430`）→ `setPITDiv` → `recalcIRQ0` | IRQ0 間隔 `IRQ0Every`、`cycPerIRQ0`；SB16 時基（444 §2.4、446） | 存（`state.go:84`、讀回 `364-371`） | 存（`snapshot.go:45`、`253`） |
| `m.pit.divisor`、`m.pit.writes` | `pit.out`（`machine.go:1311`、`pit.go:35-73`） | `PITDivisor()`、`PITProgrammed()`、`PITHz()`（`pit.go:76-90`）；經 `PITHz()` 進 `StepsPerSecondNow()`（`opl_vgm.go:28-33`） | **不存** | 存（`snapshot.go:115`、`294`） |

`LoadState` 只還原 `PITDiv` 並以它重算 `IRQ0Every`（`state.go:364-382`），`m.pit` 不動。讀檔之後若程式不再寫 PIT，`PITDivisor()` 回
65,536、`PITHz()` 回 18.2065 Hz，而 IRQ0 實際照 `PITDiv` 走。

陸空戰將：遊戲在存檔前已把通道 0 設成約 120 Hz，讀檔後不再寫。perf100 的 `TIMER` 行（`out/r1-flq-full-c2/test.log`）：
`pitdiv=9943 pitdivisor=65536 programmed=false every=96506 now=1757037 spsdiv=11580932`。`StepsPerSecondNow()` 比實際少 6.59 倍
（120 ÷ 18.2）。3 億步以 `now` 換算 170.742 秒，以 `IRQ0Every × PITBaseHz ÷ PITDiv` 換算 25.905 秒。已證實。

不釘死間隔時 `IRQ0Every = stepsPerTick(IRQ0Base, PITDiv)`（`machine.go:1482`），所以 `IRQ0Every × PITBaseHz ÷ PITDiv ≈ StepsPerSecond()`，
分頻約掉，差在 `stepsPerTick` 的四捨五入（190 §3.2 已知限制）。

兩個解讀者不完全等價（讀碼；乙指出實際場景）：

- `pit.out` 收 40h 之前要先看到選通道 0 的 43h 命令（`selected` 初值為假，`pit.go:38-48`）；`pitWrite` 不要求。
- 43h 選了通道 1、2 之後（例如喇叭常用的 43h=B6h，再寫 42h），`pit.out` 的 `selected` 變假，之後的 40h 寫入被忽略；`pitWrite` 照舊用
  通道 0 的存取方式解讀 40h。實機 40h 永遠是通道 0，`pitWrite` 較接近實機。

## 2. 決定

1. `PITDivisor()` 回 `m.PITDiv`；`m.PITDiv == 0`（零值 `Machine`，例如 `&Machine{}`）時回 `PITDefaultDivisor`。`New()` 的 `PITDiv` 初值
   已是 `PITDefaultDivisor`（`machine.go:812`），不改。
2. `PITHz()`、`StepsPerSecondNow()`、`PITStepsPerTick()` 的公式不變（`PITBaseHz ÷ PITDivisor()`、`IRQ0Every × PITHz()`、
   `PITStepsPerTick(m.PITDivisor())`，`pit.go:90`、`opl_vgm.go:32`、`pit.go:144`）。新開機且兩份分頻相同時，讀數與浮點次序逐位元相同。
3. `PITProgrammed()` 改為 `m.pitWrites > 0`。`pitWrites` 是新的 `Machine` 欄位，`setPITDiv`（`machine.go:1433`）每次呼叫加 1。
   - 語意：「本次開機或本次載入後，程式完整寫過通道 0 的分頻沒有」。
   - 狀態檔不存；`LoadState` 設為 0（`LoadState` 直接指定 `m.PITDiv`，`state.go:364`，不經 `setPITDiv`，所以不會自己加 1）。
   - 記憶體快照存：`snapshot.go` 以 `pitWrites` 取代 `pit` 欄位（`snapshot.go:115`、`202`、`294`），`Restore` 照舊回到快照時的值，
     `speaker_test.go:105-128` 的「還原之後回到未設定」不變。
   - 理由：只剩一個解讀者，分頻與「寫過沒有」出自同一組寫入，不再有兩份紀錄互相矛盾。
4. 刪除 `Machine` 的 `pit` 欄位（`machine.go:588`）與 `Out8` 內的 `m.pit.out(p, v)`（`machine.go:1311`）；`pit.go:76-87` 的讀者改讀 §2 第 1、3 條。
   `pit` 型別與 `pit.out` 保留，因為 `LEMachine` 仍使用（`le_machine.go:26`、`149`、`216-227`），`pit_test.go` 直接測 `pit` 型別的條目
   （`pit_test.go:126-179`）保留。
5. 不加狀態檔欄位、不改 `SaveState`。

### 2.1 對分歧情形的影響

改後通道 0 只有 `pitWrite` 一個解讀者，§1 末的兩種情形讀數改變：

| 情形 | 改前 | 改後 |
|---|---|---|
| 開機後未送 43h=36h 就寫 40h | `PITDivisor()` 仍 65,536、`PITProgrammed()` 假；IRQ0 已照新分頻 | 兩者都反映新分頻（與 IRQ0 一致） |
| 43h=B6h（通道 2）之後未重送 43h=36h 就寫 40h | 40h 寫入被 `pit.out` 忽略，讀數停在舊分頻 | 讀數照 `pitWrite` 解讀的分頻（與 IRQ0 一致） |

`PITProgrammed()` 的變化：`pit.out` 要先看到選通道 0 的 43h，而存取方式非 00 的 43h 會同時重設兩個解讀者的存取方式與相位；之後只要
`pit.out` 仍選中，兩者看到同一串 40h，完整寫入的時點相同。所以在一般序列下，改後的 `pitWrites` 大於等於改前的 `pit.writes`，
`PITProgrammed()` 只可能由假變真。例外（讀碼推得，強推論）：43h 選通道 2 之後、`pitWrite` 已吃了單數個 40h，再以通道 0 的鎖存命令
（存取方式 00，兩者都不重設相位）重新選中，兩者相位錯開。這種組合由 §5 第 6 條抽樣檢出。這兩種情形是否在既有遊戲出現，由 §5 第 6 條抽樣回答；
出現時 VGM 傾印的曲速與診斷行會改變，改後的值與機器實際的 IRQ0 一致。

## 3. 影響

### 3.1 機器與收據

- 狀態檔位元組、454 `MachineDigest`（`session/digest.go:105-117` 對 `machine.SaveState` 的 gob 取雜湊）：不變。
- 機器執行（IRQ0、指令序列、記憶體）：不變（IRQ0 本來就用 `PITDiv`；被刪的 `pit.out` 只寫 `m.pit`）。
- 陸空戰將收據（001 到 014、459 長測試常數）：不變。`apps/strikecommander` 非測試碼不呼叫 `StepsPerSecondNow`、`PITHz`、`PITDivisor`、
  `PITProgrammed`（`git grep` 確認）。
- 沒有 golden VGM 檔進版控（乙核對）；VGM 只在執行期產生。

### 3.2 消費者清單

非測試碼：

| 位置 | 讀的函式 | 改後 |
|---|---|---|
| `internal/machine/opl_vgm.go:28-33`（`StepsPerSecondNow`）、`45`（`OPLVGM` 預設基準） | `PITHz()` | 讀檔後與 §2.1 情形改變 |
| `internal/machine/pit.go:144`（`PITStepsPerTick()`） | `PITDivisor()` | 同上 |
| `cmd/probe/observe.go:353-357`（`-dump-vgm` 未給 `-vgm-steps-per-second`） | `StepsPerSecondNow()` | 讀檔後的傾印改成正確曲速 |
| `cmd/probe/main.go:1169`（報告行） | `PITDivisor()`、`PITHz()` | 讀檔後回實際值 |
| `oracle/vgm.go:24`（`StepsPerSecond`） | `StepsPerSecondNow()` | 同 `-dump-vgm` |
| `oracle/oracle.go:627`、`630`、`636`（`TimerHz`、`TimerDivisor`、`TimerProgrammed`） | `PITHz`、`PITDivisor`、`PITProgrammed` | 讀檔後回實際值；`TimerProgrammed` 照 §2 第 3 條 |
| `oracle/speaker.go:29-32`（`PITDivisor`、`PITHz`） | 同名 | 讀檔後回實際值 |
| `apps/rich2/cmd/movetrace/main.go:62` | `TimerDivisor`、`TimerHz`、`TimerProgrammed`、`StepsPerTick` | 診斷輸出 |

前端與即時音訊（`oracle/live.go:74-87`）用 `ToneDecoder` 與 `opl2`，不讀上列函式（`git grep` 確認）；`cmd/probe/main.go:3309`、`3375` 用常數
`StepsPerSecond()`，不受影響。

測試：

| 測試 | 改後 |
|---|---|
| `internal/machine/pit_test.go:181-201`（`TestPITHzDefaultsToBIOSRate`） | 現以 `&Machine{}` 直接呼叫 `m.pit.out`，`m.pit` 欄位刪除後無法編譯。改為：零值 `Machine` 斷言預設（驗 §2 第 1 條的 0 防禦）；`New()` 後走 `Out8(0x43, 0x36)`、`Out8(0x40, …)` 兩次，斷言 `PITProgrammed()` 與 70.187 Hz |
| `internal/machine/pit_test.go:126-179` | 直接測 `pit` 型別，保留 |
| `internal/machine/speaker_test.go:29-32` | 經 `Out8`，不變 |
| `internal/machine/speaker_test.go:105-128` | `Restore` 還原 `pitWrites`，不變 |
| `internal/machine/opl_vgm_test.go:36-104` | 經 `Out8` 或直接設定；需逐條確認，以 `PITHz()` 為期望值的條目不變 |
| `internal/machine/sb_dma8_test.go:2104`、`2411` | 經 `Out8`，不變 |
| `internal/machine/sb_dma8_test.go:2817-2819`（`TestSB8DeltaAfterLoadState`） | 測試仍過；註解「時基若讀 `PITDivisor()` 會在這裡差出來」失效。446 驗收的兩條突變（忙碌時基與 `sb8Delta` 改回 `m.PITDivisor()`）改後成為等價突變，446 突變表改記為「461 之後等價，移除」，並以 §5 第 7 條的新負對照替代 |
| `apps/rich2/state_test.go:96-112` | `load(t)` 是冷開機（`rich2_test.go:11-23`、`rich2.go:35-37`，乙核對）；遊戲經 `Out8` 寫 PIT，兩個解讀者一致，不變。不覆蓋讀檔路徑 |
| `internal/machine/le_machine_test.go:1732-1738` | LE 路徑，不在範圍，不變 |

### 3.3 要修訂的文件

- 188：`PITDivisor()` 改為 IRQ0 使用的分頻；`PITProgrammed()` 改為 `setPITDiv` 的寫入計數與讀檔後的語意；`Machine` 不再有第二個解讀者
  （188:37、55-57）。
- 190 §3.2：公式文字不變，補「`PITHz()` 讀 `PITDiv`，讀檔後亦正確」。
- 444 §2.4、446 §4.1：缺口登記改為已收尾；446 突變表兩條改記等價。
- 122（`docs/spec/122-cpu386-port-output.md:26-27`，「`PITDivisor()` 為 65,536」）：READY 前重驗該路徑。
- dosgolem-sc `CLAUDE.md:35` 的時間基準說明與 `pit.go`、`opl_vgm.go`、`machine.go:1309-1311` 的註解。

## 4. 同類缺口（登記，不在本規格修）

- pit2（通道 2，`machine.go:345`、`audio_observe.go:14-78`）：只解讀不存檔，讀檔後喇叭方波頻率從零值開始，直到程式再寫 42h。
- `ToneDecoder`（`tone.go:17-35`，`oracle/live.go:74-87` 的即時音訊）：逐筆吃埠寫入，讀檔後沒有歷史，同樣要等下一次寫入。
- LE 路徑（`le_machine.go:26`、`149`、`216-227`）：自有 `pit`，仍是兩個解讀者並存，不在範圍。

## 5. 驗收

1. 單元（存讀檔）：`New()` 經 `Out8` 寫 43h=36h、40h 低、40h 高（分頻 9943），`SaveState` → 新機器 `LoadState`：`PITDivisor()` 為 9943；
   `StepsPerSecondNow()` 與存檔前逐位元相同；`PITHz()` 與常數 `315e6/264/9943` 相同；`PITProgrammed()` 為假。
2. 載入到舊機器：先寫過 PIT 的機器 `LoadState` 一份未寫過 PIT 的狀態，`PITProgrammed()` 為假、`PITDivisor()` 等於狀態檔的 `PITDiv`。
3. 初值：`New()` 未寫 PIT 時 `PITDivisor()` 為 65,536、`PITProgrammed()` 為假；零值 `(&Machine{}).PITDivisor()` 為 65,536。
4. 陸空戰將：perf100 `TestPerf100` 讀檔後比較兩算式
   - (a) `StepsPerSecondNow()`，即 `IRQ0Every × (PITBaseHz ÷ PITDivisor())`；
   - (b) `IRQ0Every × PITBaseHz ÷ PITDiv`（perf100 的 `spsdiv`）。
   要求相對差 < 1e-9，且四捨五入到整數後相等（現況差 6.59 倍）。全部陸空戰將長測試收據與 `MachineDigest` 與改動前逐位元組相同。
5. 新開機不變：dosgolem-sc 既有 `go test ./...` 除 §3.2 列明要改的測試外全過。
6. 分歧抽樣（machine 套件內測試，因 `m.pit` 刪除後沒有公開 API 可讀舊解讀者）：跑既有遊戲的開機路徑（至少 rich2，另取有 PIT 寫入的
   FD2、MOO2 oracle 測試），之後把 `m.PortLog` 中埠 40h、43h 的寫入依序重播給一個獨立的 `pit` 值（型別仍在）。每次重播後，若
   `pit.haveLo` 為假（完整寫完），比較 `pit.divisor`（0 映射 65,536）與同一步之後的 `m.PITDiv`；後者以 `setPITDiv` 呼叫時記錄的
   （步數、分頻）序列取得。列出所有不同之處（程式、步數、兩個值），寫入測試輸出；有不同就在 §2.1 補上實例。
7. 負對照（每條必須讓指定條目失敗）：
   - `PITDivisor()` 改回固定 65,536（等同舊行為讀不到的情形）：第 1、2、4 條失敗。
   - `LoadState` 不清 `pitWrites`：第 2 條失敗。
   - `setPITDiv` 不加 `pitWrites`：第 3 條改寫後的 `TestPITHzDefaultsToBIOSRate` 失敗。
   - SB16 `sb8Delta` 與忙碌時基改用常數 65,536 取代 `m.PITDiv`：`TestSB8DeltaAfterLoadState`（`sb_dma8_test.go:2819`）失敗；取代 446 中
     已成等價的兩條突變。

## 6. 未採用方案（紀錄）

- 次案 A′：只在 `LoadState` 末尾 `m.pit.divisor = m.PITDiv`。分歧情形不處理；兩個解讀者並存。
- 第 2 版做法：保留 `m.pit` 只供 `PITProgrammed()`，`LoadState` 清零。可行，但留下兩份「寫過沒有」的紀錄來源；第 3 版改為單一解讀者。
- 備案 B：狀態檔補存 `m.pit`。gob 型別描述含欄位名，新存狀態檔位元組與所有收據的 454 `MachineDigest` 都會變；舊狀態檔仍需退路；與
  strike_commander AGENTS §2（2026-10-15）「gob 位元組錨點只再變這一次」衝突。

## 7. 2026-10-24 接手複驗

工具為 `dosgolem-go-ebiten:1.24-r1`、Go 1.24.13、Linux amd64。
證據在陸空戰將 repo `workplace/impl460/out/codex-validation-r1/short/`；原版 rich2、FD2 輸入 SHA-256 在 `pit-original-inputs.sha256`。

- `go vet ./...` 與全庫 47 個套件測試通過；本輪沒有改動 461 的正式 Go 來源。
- `PIT461` 的存讀檔、初值、快照與重播器測試通過。rich2 原版冷開機至 2 億步，165 筆埠寫入、163 次比較、分歧 0；FD2 原版 LE 路徑 3 筆寫入、分歧 0。
- 七個 461 核心來源與測試檔對先前 `workplace/impl461/src461/` 的逐檔 diff 為空。既有 15 支陸空戰將長測試皆以 exit=0 結束，P1–P4 負對照在指定斷言失敗；原收據仍保留在 `workplace/impl461/`。
- MOO2 原版素材仍缺，§5 第 6 條的 MOO2 抽樣未完成。維持 READY，不把缺檔列為通過。

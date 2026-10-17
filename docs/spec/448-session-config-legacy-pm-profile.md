# 448：session.Config 支援 JEMM／SB16／鍵盤攔截的執行選項

狀態：**READY**（第 3 版，依兩輪審查修訂）
日期：2026-10-18
前置：[`238-session-step-observer-draft`](238-session-step-observer-draft.md)、[`240-opl-audio-output-draft`](240-opl-audio-output-draft.md)、
[`242-machine-clock-scale-draft`](242-machine-clock-scale-draft.md)、[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)、[`439-exec-child-environment`](439-exec-child-environment.md)、
[`441-exec-reclaim-nil-path-child-block`](441-exec-reclaim-nil-path-child-block.md)、[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md)、
[`447-bios-int09-int15-keyboard-intercept`](447-bios-int09-int15-keyboard-intercept.md)

dosgolem 的程式位置以 commit `7e2b8cb` 為準。推論等級：已證實、強推論、假說、未知。

---

## 1. 為什麼要做

《Strike Commander》以 `SCCD.EXE` 冷開機，經 JEMM 進 V86，依序 EXEC `MENU.EXE`、`OPTTEST.EXE`、`STRIKE.EXE`。`cmd/probe` 要帶下列選項才能從冷開機走到飛行（`strike_commander` 的 `docs/re/007` §2、`010` §2）：

```text
-dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile -exec-env -exec-reclaim -mode13-phase -sound-blaster -adlib -sb-dma8 -kbd-int15
```

probe 另外會把 `m.ProgramPath` 設成 `C:\` 加執行檔名的大寫（`cmd/probe/main.go` 367 行），`LoadEXE` 把它寫進環境區塊（`internal/machine/loader.go` 299 行起；空字串時用 `C:\PROG.EXE`）。`dos.New` 建立時也讀它：路徑帶目錄時把目前目錄設成該目錄（`internal/dos/dos.go` 666–676 行）。`C:\SCCD.EXE` 不帶目錄，`programDir` 回空字串，目前目錄維持預設，所以對本遊戲只影響環境區塊。

中文化前端要走 `session.Owner`，但 `session.Config` 只有 `AdLib` 與 `ClockPercent`（`session/owner.go` 107–118 行），`New` 也只套這兩項（171–231 行）。上列選項目前只有 `cmd/probe` 會設（`main.go` 339–353、367、375、479–483、532 行）。session 因此跑不起這個遊戲，是已證實的缺口（程式碼）。

鍵盤是另一個缺口，不在本規格：session 的 `Deliver` 只把 `CapturedUpdate.BIOSKeys` 放進 BIOS 緩衝（`owner.go` 489 行 `o.dos.PushKey`），而且拒絕面板轉送給 DOS 的鍵盤事件（387 行）。`presentation.KeyboardBridge.DeliverDOSScan`（`presentation/keyboard.go` 66 行）沒有非測試的呼叫者。OPTTEST 與 STRIKE 只從 `int 15h AH=4Fh` 收鍵（447 §1），需要 IRQ1 掃描碼通道，另立規格 449。

## 2. 規則

### 2.1 新欄位

`session.Config` 加下列值欄位，零值等於現狀：

| 欄位 | 型別 | 對應 probe | 套用 | 失敗來源 |
|---|---|---|---|---|
| `ProgramPath` | `string` | `m.ProgramPath`（main.go 367） | `m.ProgramPath`；空字串不設，維持 machine 預設 | 無 |
| `DOSFiles` | `int` | `-dos-files` | `m.SetDOSFiles` | 非 0 且不在 6..255（`sft.go` 30–32） |
| `ExtendedMemoryKB` | `int` | `-ext-mem-kb` | `m.SetExtendedMemoryKB` | setter 的範圍檢查（`extmem.go`） |
| `NoEMS` | `bool` | `-no-ems` | `m.SetEMSPresent(!NoEMS)` | setter 回錯 |
| `ProtectedModeProfile` | `bool` | `-pm-profile` | `m.SetProtectedModeProfile` | 沒有延伸記憶體等（433 §6） |
| `SoundBlaster` | `bool` | `-sound-blaster` | 為真時才呼叫 `m.SetSoundBlaster(true)` | 無 |
| `SBDMA8` | `bool` | `-sb-dma8` | `m.SetSBDMA8` | `SoundBlaster` 為假時由 `New` 回錯（444 §2.1） |
| `Mode13Phase` | `bool` | `-mode13-phase` | `m.Mode13Phase` | 無 |
| `ExecEnvCopy` | `bool` | `-exec-env` | `d.ExecEnvCopy` | 無 |
| `ExecReclaim` | `bool` | `-exec-reclaim` | `d.ExecReclaim` | 無 |
| `KbdIntercept` | `bool` | `-kbd-int15` | `m.EnableKbdIntercept()` | setter 回錯 |

既有的 `AdLib` 對應 `-adlib`，維持「為真時才呼叫 `SetAdLib(true)`」。`SetSoundBlaster(false)` 在 SB16 DMA 開啟後會 panic（`machine.go` 1008–1011 行），無條件的 `SetAdLib(false)` 會在 SB 之後把 OPL 關掉，所以兩者都不得以假值呼叫。`EmptyEnv` 不加，維持 machine 預設（probe 的預設也是關）。

### 2.2 套用時點

全部在 `New` 內、`machine.New()` 之後依序套用，與 probe 冷開機路徑的先後相同：

1. `ExtendedMemoryKB`、`NoEMS`、`ProtectedModeProfile`（probe `configureMem`、`configurePM`）。
2. `ProgramPath`，必須在 `dos.New` 之前，`dos.New` 才會依它設目前目錄（probe 367 行在 `dos.New` 之前）。
3. `AdLib`（既有）、`SoundBlaster`、`SBDMA8`。
4. `DOSFiles`。它要求程式尚未載入（`sft.go` 28 行），`New` 階段一定成立。
5. `Mode13Phase`、`KbdIntercept`。
6. `dos.New` 之後設 `ExecEnvCopy`、`ExecReclaim`。

`KbdIntercept` 寫的是 dosgolem 自己的 stub 區（`StubOff(9)` 與 `KbdInterceptOff`，447 §2.2）。這兩處由 `machine.New()` 建好（`machine.go` 779、1863–1870 行），`LoadEXE` 與 `dos.Install` 都不碰（`loader.go` 149–164 行，`dos.go` 726–732 行只裝字型）。所以在 `New` 套用即可，不改 `startLoadedMachine` 與 `BootOriginal`。

為了上述先後，`dos.New(m, "")` 要移到 machine setter 之後；`dos.New` 之前不得有依賴 DOS 的步驟。

### 2.3 錯誤處理

- `SBDMA8` 為真而 `SoundBlaster` 為假：`New` 在呼叫任何 setter 前回錯。
- 其餘不合法值以 machine setter 的回錯為準，setter 回錯時 `New` 回錯、不回 owner。
- 搬動後 `dos.New` 在 setter 之後，早期失敗時 DOS 還沒建立。關閉 DOS 的 `keepDOS` defer（`owner.go` 184–190 行）要放在 `dos.New` 之後註冊，`dos.New` 之前的錯誤直接回傳。所有錯誤分支仍在 `keepDOS = true` 之前。

### 2.4 OPL 記錄

`SetSoundBlaster(true)` 會讓 OPL 存在（`machine.go` 1008–1014 行），沒有掛觀測時 OPL 寫入會一直累積在 `m.OPL`（1070–1073 行）。既有的 `muteAudio` 只在 `AdLib` 為真時呼叫（`owner.go` 221–224 行）。改成在 `newOwner` 之後（`muteAudio` 是 `Owner` 方法），只要 `m.AdLibPresent()` 為真就呼叫一次。這與 `runObserved` 的 defer 已經以 `AdLibPresent()` 決定是否 mute 一致（`session/observer.go` 171–178 行）；只開 AdLib 時行為與現狀相同，`muteAudio` 只換回呼，不改 digest。

### 2.5 摘要

`StateDigest` 不加欄位。選項由呼叫端記錄（§5 收據寫完整的 `Config`）。

### 2.6 範圍外

- IRQ1 掃描碼送鍵通道（規格 449）。
- SB16 的聲音輸出。本規格只讓 SB16 的偵測、DMA 與 IRQ7 時序與 probe 一致，前端聽不到數位音效。
- 存讀檔、`-sb-dma-log` 等觀測旗標、前端 `cmd/<app>-play` 的參數。

## 3. 驗收

### 3.1 單元測試

`session` 套件，不需原版素材，期望值一律寫字面值：

1. 零值 `Config` 與改動前相同：既有 session 測試全過，`TestObserverSeesEveryStepAndChangesNothing` 等 digest 測試不變。另加字面錨點：改動前在 HEAD 以零值 `Config` 加 `audioLoopCOM` 跑固定步數，記下 `Digest` 的 Memory 與 CPU SHA-256 寫成字面期望值，改動後必須相同（抓 `dos.New` 搬動的副作用）。
2. 每個新欄位開啟後，對應的狀態被設定，逐欄一個測試：`m.ProgramPath`、`DOSFiles()`、`ExtendedMemoryKB()`、`EMSPresent()`、`ProtectedModeProfile()`、`SoundBlaster()`、`SBDMA8()`、`Mode13Phase`、`d.ExecEnvCopy`、`d.ExecReclaim`。`ProgramPath` 用帶目錄的路徑（例如 `C:\GAME\X.EXE`），另驗 `d.Dir == "GAME"`。
3. `KbdIntercept`：`New` 之後與 `BootOriginal`（`minimalBootMZ`，只載入不執行）之後各讀一次。以 `ReadSup8` 讀 `StubSeg:StubOff(9)` 起三個 byte 等於 `machine.KbdInterceptStub9()`（`E9 91 04`），`KbdInterceptOff` 起 14 個 byte 等於 447 §2.2 的客體碼，`Machine.KbdIntercept` 為真；未開時三個 byte 為 `CD 09 CF`，`KbdIntercept` 為假。`BootOriginal` 之後仍成立，證明 `LoadEXE` 與 `dos.Install` 不碰 stub 區。
4. 錯誤路徑：`SBDMA8` 無 `SoundBlaster`、`DOSFiles: 5`、`ProtectedModeProfile` 無延伸記憶體，各回錯且 owner 為 nil。
5. 只開 `SoundBlaster`（不開 `AdLib`）時，跑 `audioLoopCOM` 後 `len(m.OPL)` 為 0。現有 helper 不接受 `Config` 的其他欄位，新增一個：`New(cfg)` 之後 `o.machine.LoadCOM(audioLoopCOM)`、`startLoadedMachine`、設 layout，再以 `runTurn` 推進（`owner_test.go` 735–752 行的做法）。
6. 負對照（實作者回報結果，不進版控）：把 `KbdIntercept` 的套用條件反向，3 必須失敗；拿掉 §2.4 的 `muteAudio` 呼叫，5 必須失敗；把 `ProgramPath` 移到 `dos.New` 之後，2 的 `d.Dir` 斷言（帶目錄的路徑）必須失敗。

### 3.2 觸發案例（主代理執行，結果寫進 §5）

**輸入**：

- `SaveRoot`：把 `strike_commander` 的 `workplace/root/`（SB16 設定，`SC.CFG` 含 `-ma -ds`，SHA-256 `317b0abd…34e36f`）以 `cp -r` 實體複製出一棵獨立樹。不得用硬連結樹，Root 與 Scratch 是同一目錄（`boot_original.go` 75–78 行），寫入會改到共享 inode。
- `BootInput.EXE`：`SCCD.EXE`，`ExpectedEXESHA256` `5dbfe18c528f51b9b1b0c989c1578cb297ba56c64038be7ca9d903dd88e93d01`。
- `Config`：`ProgramPath: "C:\\SCCD.EXE"`、`DOSFiles: 30`、`ExtendedMemoryKB: 8192`、`NoEMS`、`ProtectedModeProfile`、`ExecEnvCopy`、`ExecReclaim`、`Mode13Phase`、`SoundBlaster`、`AdLib`、`SBDMA8`、`KbdIntercept`，`ClockPercent` 零值。`InitialScale` 與 `InitialLayout` 給合法值（`InitialLayout.Epoch` 非 0，與面板一致，`owner.go` 191–209 行），`View` 與 `Deliver` 需要 `layoutSet`（328、657 行）。
- 一個只記錄的觀察器：在 `BeforeStep` 看到 `Steps == 1000000000`（已執行 10 億道、第 10 億零 1 道之前）時記下 CS、IP、AX、BX、CX、DX、DS、ES、SS、SP。

**步驟**：`BootOriginal` → `View()` → `Deliver(u)` → `Advance(1000000000)`；斷言 `MachineStepsAfter == 1000000000`，取 `Digest()`。再 `View()` → `Deliver(u)` → `Advance(1)`，讓觀察器記到 `Steps == 1000000000` 的暫存器。`u` 是沒有任何輸入的 `CapturedUpdate`，`StartedPanel`、`Layout`、`SourceGeneration` 取自同一次 `View()` 的 `Panel`、`Layout`、`SourceGeneration`（`prepare` 的檢查在 `owner.go` 307–342 行；現有測試的寫法見 `session/observer_test.go` 61 行）。這樣的 update 只動 mouse、panel、biosKeys，不碰機器（489–497 行）。

**期望值**（`cmd/probe` 以同一組選項冷開機，獨立於被測程式）：

| 斷言 | 期望 | 出處 |
|---|---|---|
| `IndexedSHA256` | `4d752df6353106aced440d235109abee038776ca2096953ce86c208e405e2148` | `workplace/scratch/p502/c100.png`（第 1000000000 步的色號檔，與 `p472/s100.png` 位元組相同） |
| `PaletteSHA256` | `ea48da3ee0628b0cf7609482ed5946e7ae78939b8c45d2fb0850a8bb566a5b60` | `workplace/scratch/p502/c100.pal`（768 bytes，排列與 `digest.go` 61–66 行相同） |
| `Steps == 1000000000` 的暫存器 | CS:IP `060F:034A`，AX `0000`，BX `0000`，CX `2042`，DX `FB54`，DS `250B`，ES `250B`，SS:SP `250B:FAD6` | `probe502b.log`（`-steps 1000000000` 的終態）；以 `probe-sc` 載入 `p472/s100.state`（另一個 build `probe-sc.446` 存的第 10 億步狀態）印出的終態相同（`probe503.log`） |

probe 指令（`strike_commander` 的 `tools/probe/psc.sh`）。`workplace/bin/probe-sc` 的 SHA-256 是 `a62b5c79d0b7a1b5055e32c843284102623b8841802113254ea95033d9e9afeb`，內嵌 `vcs.revision=0ca3fff`、`vcs.modified=true`、`go1.24.13`（447 實作期間的工作樹建置）。Indexed 與 Palette 另有 `probe-sc.446` 產生的 `p472` 佐證：

```text
probe-sc -exe /orig/SCCD.EXE -root /orig -scratch /scratch -ext-mem-kb 8192 -no-ems -pm-profile -exec-env -exec-reclaim \
  -mode13-phase -sound-blaster -adlib -sb-dma8 -sb-dma-log /scratch/sbdma.tsv -dos-files 30 -steps 1000000000|1000000001 \
  [-shots 1000000000:/scratch/c100.png]
```

probe502、502b 沒帶 `-kbd-int15`，`KbdIntercept` 開啟時 stub 區記憶體不同（`StubOff(9)` 3 bytes、`KbdInterceptOff` 14 bytes），`MemorySHA256` 必然不同，所以 §3.2 不比對記憶體雜湊。客體碼只在 IRQ1、軟體 `int 09h` 或 `int 15h AH=4Fh` 落到預設服務時才有作用；這段冷開機 `KeyIRQs=0`，MENU 在第 1616799 步已接管 `int 15h`（`probe502b.log` 的中斷向量紀錄）。另以 `-kbd-int15` 冷開機到第 10 億步取第二份參考（`probe504.log` 截圖與存檔、`probe504b.log` 載入該檔印終態）：色號檔、色盤與暫存器都與上表相同，已證實這段不受 `KbdIntercept` 影響。

判定：`KbdIntercept` 為真與為假的兩種 `Config` 都要符合上表。只有為真時不符而為假時符合，記為 447 的新發現並回報，不算本規格失敗。

**負對照**：逐一關掉 `Config` 的每個新欄位各跑一次，記錄三項斷言哪些改變。`Advance` 回 fault 也記為「有改變」。關掉 `SoundBlaster` 時同時關 `SBDMA8`，否則 `New` 失敗。不敏感的欄位在 §5 明寫「觸發案例無法證明，由 §3.1 負責」。

**執行環境**：觸發案例是長測試（probe 約 4 分鐘），以環境變數 `DOSGOLEM_SC_ROOT` 指向複製樹時才跑，否則 `t.Skip`。在 Docker 內執行（`--rm`、`--memory`、`--cpus`、`--network none`）。每次約 5 到 10 分鐘，基準與負對照拆成各自的子測試，每個以 `go test -run <子測試> -timeout 30m` 單獨執行。

### 3.3 回歸

`go vet ./...`、`go test ./...` 在 Docker 內全過；`apps/` 與 `cmd/` 的既有測試結果與改動前相同。

## 4. 已知限制

- probe 與 session 的主迴圈不同：probe 有點擊與送鍵排程、`pcGuard` 每步檢查（`main.go` 694 行一帶）、`CallTrace` 等觀測，session 沒有。§3.2 只在不送輸入的冷開機上比對；PC 跑進 `A0000` 以上時兩者的停止行為不同。
- `ClockPercent` 與 `Mode13Phase` 同時使用的交互沒有量，§3.2 以 `ClockPercent` 零值為準。
- §3.2 的暫存器比對是局部的：probe 的報表只有 CS:IP、AX、BX、CX、DX、DS、ES、SS、SP，SI、DI、BP、FLAGS 與 `CPUSHA256` 無法對拍。
- `BootInput` 只驗 `SCCD.EXE`。MENU、OPTTEST、STRIKE、`JEMM.OVL` 由 DOS 從 `SaveRoot` 讀取，雜湊由呼叫端記錄（§5 收據寫 `SaveRoot` 內各檔的 SHA-256）。

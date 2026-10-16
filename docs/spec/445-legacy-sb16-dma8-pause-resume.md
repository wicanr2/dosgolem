# 445：一般機器 SB16 8 位元 DMA 的暫停與恢復（DSP D0h／D4h）

狀態：**READY**（第 2 版，依審查修訂）
日期：2026-10-17
前置：[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md)、[`309-sb16-pause-resume-dma8`](309-sb16-pause-resume-dma8.md)
修訂：444 §2.2、§2.3、§2.4、§2.8、§2.10、§3.4、§3.11、§4.1、§4.2 的對應條文已依本規格改寫（各註「由 445 修訂」）。444 §5 收據不動。
本規格 §1.2、§1.3、§2.3、§2.11、§3、§4.1、§4.2 已依 [`446-legacy-sb16-idle-control-and-busy-cycle`](446-legacy-sb16-idle-control-and-busy-cycle.md) 改寫（各註「由 446 修訂」）；§5 收據不動。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/` 下的檔案，`sblaster.cpp` 省略路徑 `src/hardware/`。程式位置以 commit `021ee31` 為準（到 `1f7e533` 為止
`internal/`、`cmd/` 沒有變動）。驅動程式位移為 `B5C5:0000` 起的映像位移（444 §1.2）。推論等級：已證實、強推論、假說、未知。

---

## 1. 為什麼要做

### 1.1 觸發案例

《Strike Commander》SB16 設定（`SC.CFG` 含 `-ma -ds`）冷開機。MENU 的音效偵測通過（DMA 完成 3 次、IRQ7 遞送 3 次），播放 59750 bytes 的 8 位元取樣
（第 16823080 步開始、第 79099541 步完成並遞送 IRQ7），第 125256614 步開始播 18430 bytes（到期 144465906），第 131427746 步驅動在 `B5C5:03F9`
寫 `22Ch←D0h`，444 失敗即關閉：「DSP 寫入 22Ch←D0h 未被接受」（`strike_commander` 的 `workplace/logs/probe470.log`、`workplace/scratch/p470/sbdma.tsv`，已證實）。

probe470 的收據沒有記錄命令列。旗標以主代理的執行指令為準：
`-dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile -exec-env -exec-reclaim -mode13-phase -sound-blaster -adlib -sb-dma8 -sb-dma-log`。

`D0h` 是單一位元組命令，暫停 8 位元 DMA 請求，適用單次與自動初始化；`D4h` 恢復被 `D0h` 暫停的傳輸（Creative「Sound Blaster Series Hardware Programming Guide」
印刷頁 6-24、6-26，見規格 309 的引用）。`internal/machine/sb_dsp.go:212-216` 已在無待收參數時把 `D0h`／`D4h` 交給 `ControlDMA8(paused bool)`；
444 的一般機器沒有設這個回呼（`sb_dma8.go:108-115`），所以回 false。LE 路徑由 309 接上，不在本規格範圍。

### 1.2 驅動的停止、暫停與恢復

靜態證據：主代理在容器內以 `objdump -D -b binary -m i8086` 反組譯 444 §1.2 的同一份映像（`LILTRE.TRE` 位移 793944 起 4579 bytes），本機 scratchpad，不進版控。
整體等級為強推論。

| 位移 | 內容 |
|---|---|
| `03BC` | 送 `D0h` 的內部程序。`pushf`；`DX=cs:[FE]`（`22Ch`）；`CX:BX=7:0000h` 為逾時計數。迴圈 `03CF`–`03EA`：計數減一（`js` 時跳 `03F7`），`sti`，`cs:[106]==0`（未播放）時跳 `03FA` 不送；否則 `cli`、`in 22Ch`，bit7 為 0 時跳回 `03CF`。bit7 為 1 後，`03EC`–`03F5` 再等 bit7 回 0（最多 `8000h` 次）。`03F7`、`03F9` 寫 `22Ch←D0h`。`03FA` `popf`，`03FB` 清 `cs:[106]=0`，`lret`。參數未被讀取 |
| `03A4` | 經 `0228` 寫 `D4h`，設 `cs:[106]=1`，`lret` |
| `0372` | `cs:[1CC]≠0` 時清掉它，呼叫 `03BC`，再呼叫 `0228(D3h)`（喇叭關）、`0BAD(10h)`（等 16 次回掃），`lret`。只由 `0BEF` 呼叫 |
| `0BEF` | 關閉流程：`cs:[1D7]≠0` 時依序呼叫 `0372`、`1141`（此時 `cs:[106]` 已被 `03FB` 清為 0，不送第二個 `D0h`）、`0283`（DSP 重設），再清 `cs:[1D7]` |
| `1141` | `cli`，`cs:[1BC]=0`，`04B6`（還原 IMR 與向量），呼叫 `03BC`，`lret` |
| `1187` | `cli`，`cs:[1BC]==2` 時設為 1 並呼叫 `03BC`（暫停） |
| `11AE` | `cli`，`cs:[1BC]==1` 時設為 2 並呼叫 `03A4`（恢復） |
| `10E5`–`113B` | 開始播放前的路徑之一：`cs:[1BC]==0` 時 `cli`、呼叫 `03BC`、`cs:[1BC]=2`、呼叫 `08F2` |

`03BC` 有四個呼叫點（`0387`、`112D`、`116D`、`11A2`），`03A4` 只有一個（`11C7`）。走 `0BEF` 關閉流程時，暫停中的傳輸由 `0283` 的 DSP 重設取消（§2.8），不經隱式取消。`cs:[1BC]` 是播放狀態（0、1、2），`1187`／`11AE` 是成對的暫停與恢復 API。
觸發案例的 `D0h` 是從哪個呼叫點來的未知（§3 第 7 項記錄）。

兩點推論：

1. `cs:[106]` 為 0 時 `03BC` 不送 `D0h`，所以驅動在 idle 不送 `D0h`（強推論，靜態）。由 446 修訂：傳輸完成後只有 ISR 清 `cs:[106]`，ISR 沒跑時驅動會對 idle 送 `D0h`（446 §1.2、§1.3）。
2. 停止流程（`0372`、`1141`）只送 `D0h`，之後不送 `D4h`。不走 `0BEF` 關閉流程時，下一段取樣由 `0562` 重設 DMA 通道 1（`0Ah←05h`、頁、`0Ch`、位址、計數、`0Bh←49h`、`0Ah←01h`），
   再送新的 `14h`（或 ADPCM、`80h`）。dosgolem 此時處於「已暫停、未結束」的狀態，444 的現行規則下會失敗即關閉：DMA 寫入在 `sb_dma8.go:464`
   （`xfer` 條件）之後的各分支，新啟動在 `sb_dma8.go:164`、`:193`（`case s.active`）。

時間上的觀察：dosgolem 的 `22Ch` 在非重設時恆回 0（`sb_dsp.go:83-87`），`03BC` 的迴圈等不到 bit7 立起，只能跑到逾時。每圈 13 道指令，`70000h` 圈約 596 萬步；
`D0h` 距離 `start` 617 萬步（131427746 − 125256614），量級吻合，所以停止 API 大約在 `start` 之後 20 萬步被呼叫（強推論）。逾時迴圈期間 IF=1，
傳輸若在這段時間到期、處理常式清掉 `cs:[106]`，`03BC` 就不送 `D0h`。

### 1.3 DOSBox-X 對照

| 項目 | DOSBox-X | 本規格 |
|---|---|---|
| `D0h` | `sblaster.cpp:2298-2307`：`FillUp`，`mode=MODE_DMA_PAUSE`，移除 `END_DMA_Event`、`DMA_DAC_Event`。不檢查目前模式，idle 也進入暫停（`2302-2304` 的註解） | DMA 傳輸中暫停；idle 與靜音中接受、無狀態變化（446 §2.2，由 446 修訂） |
| `D4h` | `2343-2348`：只在 `MODE_DMA_PAUSE` 時改 `MODE_DMA_MASKED` 並 `Register_Callback`；其他模式不動作 | 已暫停時恢復；未暫停的 DMA 傳輸中不動作；idle 與靜音中接受、無狀態變化（446 §2.2，由 446 修訂） |
| 恢復的接續 | `include/dma.h:125-131` 的 `Register_Callback` 呼叫 `SetMask`，觸發 `DMA_UNMASKED`；`sblaster.cpp:3665-3670` 在 `MODE_DMA_MASKED` 時改回 `MODE_DMA` 並 `CheckDMAEnd`，從剩餘的 `dma.left` 續傳 | 平移開始與到期時刻（§2.5） |
| 暫停中寫 DMA 暫存器 | `3643-3644` 的 `DMA_MASKED` 只在 `MODE_DMA` 時動作，`3665-3666` 的 `DMA_UNMASKED` 只在 `MODE_DMA_MASKED` 時動作；`MODE_DMA_PAUSE` 下兩者都不動作，寫入無害 | 允許，記「暫停期間 DMA 已改寫」（§2.6） |
| 暫停中的新 DMA 命令 | `DSP_DoDMATransfer`（`1111` 起）：`1114` 無條件 `mode=MODE_DMA_MASKED`，`1177-1178` `dma.left=dma.total`，`1181` `irq.pending_8bit=false`，`1226-1227` 移除舊事件，`1233` 重新註冊回呼。舊傳輸被取代，不發 IRQ | 隱式取消舊傳輸，不發 IRQ7，再照 444 開始（§2.7）。不動 `IRQPending`（§4.1） |
| 暫停中的 `80h` | `2219-2222` 只排 `DSP_RaiseIRQEvent`，不改 `mode`；舊 DMA 維持暫停 | 隱式取消舊傳輸再開始靜音（§4.1） |
| 靜音中的 `D0h` | `2306-2307` 不移除 `DSP_RaiseIRQEvent`，靜音的 IRQ 照發 | 接受、無狀態變化，靜音照常發 IRQ7（446 §2.2，由 446 修訂） |
| 改寫 DMA 後的 `D4h` | 同 `2343-2348`，以新暫存器與舊 `dma.left` 續傳 | 拒絕（§2.3） |
| `22Ch` 讀值 | `3192-3214`：`DSP_busy_cycle` 或 `write_busy` 時回 `FFh`，否則 `7Fh` | 選項開啟時為忙碌週期（446 §2.4，由 446 修訂） |

---

## 2. 規則

### 2.1 接線

`wireSB8`（`sb_dma8.go:109-115`）另設 `d.ControlDMA8 = m.sb8Control`。`SetSBDMA8`、`Restore`、`LoadState` 都經 `wireSB8`，所以三處一併生效。
選項關閉時 `ControlDMA8` 維持 nil，`D0h`／`D4h` 照舊回 false、不報錯。`sb_dsp.go` 不改。

### 2.2 新狀態

`sbDMA8` 加下列欄位：

| 欄位 | 意義 | 進狀態檔 |
|---|---|---|
| `paused bool` | 進行中的 DMA 傳輸已被 `D0h` 暫停 | 是（§2.10） |
| `pausedAt uint64` | 暫停那一刻的時鐘值，單位同 `s.cycles`（§2.4）；未暫停時為 0 | 是 |
| `dmaRewritten bool` | 暫停期間通道 1 的 DMA 已改寫（§2.6）；未暫停時為假 | 是 |
| `pauses`、`resumes uint64` | 實際進入暫停、實際恢復的次數，冪等的重複命令不計 | 否（§2.9） |

不變式：`paused` ⇒ `active` 且 `cmd≠80h`；`!paused` ⇒ `pausedAt==0` 且 `!dmaRewritten`。下列動作把三個欄位一起清除：恢復（§2.5）、
取消（§2.8，含隱式取消）、DSP 重設（§2.8）、新傳輸開始（`sb8Begin`）、傳輸完成（暫停中不會發生，§2.4）。

### 2.3 `D0h`、`D4h` 的處理

`sb8Control(paused bool) bool`。`paused` 為真代表 `D0h`。

| 狀態 | `D0h` | `D4h` |
|---|---|---|
| 無傳輸（`!active`） | 接受，無狀態變化，記 `control-idle`（446 §2.2，由 446 修訂） | 同左 |
| `80h` 靜音進行中 | 接受，無狀態變化，記 `control-idle`，靜音照常完成（446 §2.2，由 446 修訂） | 同左 |
| DMA 傳輸中、未暫停 | 暫停：`pausedAt` 取原始時鐘（§2.5 的 `now`），`paused=true`，`pauses++`，記 `pause` | 接受，不動作 |
| 已暫停、DMA 未改寫 | 接受，不動作（`pausedAt` 不覆寫） | 恢復（§2.5） |
| 已暫停、DMA 已改寫 | 接受，不動作 | 拒絕 |

「DMA 傳輸」指 `cmd` 屬 `14h`、`74h`–`77h`、`16h`、`17h`。

拒絕的做法：`sb8Control` 先以 `sb8Fail` 寫入下列訊息，再回 false。`sb_dsp.go:216` 把 false 傳回 `Machine.Out8`，`Out8` 接著呼叫 `sb8DSPFail`；
`sb8Fail` 只留第一個錯誤（`sb_dma8.go:119-121`），所以保留的是這裡的訊息。訊息含「未被接受」，不含「未實作」。
「無傳輸」「靜音中」兩列的拒絕訊息已刪除（由 446 修訂）：

| 情形 | 訊息主體 |
|---|---|
| 改寫後 `D4h` | `DSP 寫入 22Ch←D4h 未被接受：暫停期間通道 1 的 DMA 已改寫，恢復的語意沒有證據（docs/spec/445 §2.3）` |

idle 與靜音中改為接受的理由見 446 §2.2（由 446 修訂）。

### 2.4 暫停期間

- `sb8Now()`：`paused` 為真時回 `pausedAt`。這一處同時涵蓋進度（`sb8Progress`）、DMA 位址與計數的即時讀值（`sb8ReadDMAReg`）、取消時寫回的進度與事件的 `Transferred`。
- `sb8Poll()`：`paused` 為真時第一行就返回，不完成傳輸。這是必要條件：步數時鐘下 `pausedAt==due` 可能發生（§2.5），週期時鐘下 `pausedAt≥due` 可能發生，
  只靠 `sb8Now()` 回 `pausedAt` 會在暫停中完成。`Step`（`machine.go:1466`）在 `active` 時呼叫 `sb8Poll`，暫停中 `active` 仍為真，這行不改。
- IRR7、ISR7、PIC、`IRQPending` 不動。已待處理的 IRQ7 照 444 §2.7 遞送，`22Eh` 讀取照 444 §2.3 清 IRR7。
- 暫停不影響 `eventThisStep` 與 444 §2.7 的遞送條件。
- 其他 DSP 命令（`40h`、`41h`、`48h`、`D1h`、`D3h`、`D8h`、`E1h`）照 444 處理。

### 2.5 恢復的平移

原始時鐘 `now`：傳輸的時鐘單位（`s.cycles`）為步數時，取 `m.Steps`；為週期時，取 `m.CPU.Cycles`。不經 `sb8Now()`（暫停中它回 `pausedAt`，差值會是 0）。

- `pausedAt`：`D0h` 那一道 `out` 執行時的 `now`。步數時鐘下是該指令的序號（`Step` 在 `CPU.Step` 之前已遞增）；週期時鐘下已含這道 `out` 的
  `charge(cycIO)`（`internal/cpu/cycles.go:78-79`，在埠處理之前扣）。
- `D4h` 那一道 `out` 執行時取 `now`，`d = now − pausedAt`。步數時鐘下就是「`D4h` 那一道的序號 − `D0h` 那一道的序號」；週期時鐘下是兩道 `out` 各自扣過 `cycIO` 之後的
  `CPU.Cycles` 之差。
- `due + d` 溢位 `uint64` 時失敗即關閉，欄位都不改。否則 `start += d`、`due += d`（`start ≤ due`，`start + d` 不會溢位），清 `paused`、`pausedAt`、`dmaRewritten`，
  `resumes++`，最後記 `resume`（`Due` 為平移後的值）。
- 連續性：Δ＝`due − start` 不變。恢復時 `now − start' = pausedAt − start`，進度與暫停那一刻相同；之後照 444 §2.5 的公式前進。不補暫停期間的取樣。
- 多次暫停：每次恢復各自平移，總平移量為各次 `d` 之和。
- 步數時鐘下 `pausedAt ≤ due`：`D0h` 那一步開頭的 `sb8Poll` 以 `m.Steps = pausedAt − 1` 比較，沒有完成表示 `pausedAt − 1 < due`。等號可能成立。
- 週期時鐘下 `pausedAt ≥ due` 可能成立：`D0h` 那一步開頭 `Cycles < due`，這道 `out` 扣 `cycIO` 後越過 `due`。此時進度夾在 `n`（`sb8Progress` 的 `el ≥ delta`），
  恢復後 `due' = due + d ≤ now`，下一個 `Step` 開頭的 `sb8Poll` 即完成。

### 2.6 暫停期間的 DMA 寫入

444 §2.2 的「傳輸進行中，下列寫入失敗即關閉」只適用於未暫停的傳輸：`sb8Out` 的 `xfer` 條件改為 `active && !paused && cmd≠80h`。未暫停時的規則不變。

暫停期間，下列寫入視為改寫通道 1：

| 埠 | 條件 |
|---|---|
| `02h`、`03h`、`83h` | 任何值 |
| `0Ah`、`0Bh` | `v&3==1` |
| `0Dh`、`0Eh`、`0Fh` | 任何值（都會寫到通道 1 的遮罩位元） |

`0Ch`（清 flip-flop）、`08h`、`09h`、讀取、其他通道的暫存器不算。

暫停期間第一次改寫之前，先把凍結的進度 `k` 寫進 `Current[2] = (addr+k)&FFFFh`、`Current[3] = (count−k)&FFFFh`（式子同 `sb8Cancel`，`Base` 不動），
接著照 444 §2.2 執行寫入，設 `dmaRewritten=true`。理由：真機的 8237 現值暫存器在暫停時已前進到 `k`，只寫一個位元組時另一個位元組保留前進後的值。
之後的同類寫入直接照 444 §2.2 執行。

`dmaRewritten` 為真之後：

- `sb8ReadDMAReg` 讀 `02h`、`03h` 回暫存器本身，不再以進度計算（即時讀值的條件改為 `active && !dmaRewritten && cmd≠80h`）。
- 任何取消（隱式或重設）都不寫回進度。
- `D4h` 拒絕（§2.3）。

### 2.7 暫停期間的新啟動

`sb8StartDMA`（`14h`、`74h`–`77h`、`16h`、`17h`）與 `sb8StartSilence`（`80h`）在長度高位元組那一道被呼叫。這時：

1. `active && !paused`：照 444 失敗即關閉（`sb_dma8.go:164`、`:193` 不變）。
2. `active && paused`：先隱式取消舊傳輸（§2.8），再照 444 §2.4 第 2–8 項檢查並開始新傳輸。通道 1 的暫存器是驅動改寫後的值，或隱式取消寫回的進度。
3. 新傳輸的檢查不成立時照 444 失敗即關閉；舊傳輸已取消，次序只影響觀測紀錄。

隱式取消不設 TC，不拉也不清 IRR7，不動 ISR7 與 `IRQPending`。觀測紀錄的 `cancel` 與緊接的 `start` 有相同的 `Step`、`CS`、`IP`，
消費端以此與 DSP 重設的 `cancel`（在寫 `226h` 的那一道）區分。

### 2.8 取消與重設

`sb8Cancel` 拆成共用的取消主體與 DSP 重設專屬的部分：

- 取消主體（隱式取消與重設共用）：`active` 時，先以 `sb8Now()` 算進度 `k`（暫停中即凍結的進度），`cmd≠80h` 且 `!dmaRewritten` 時把進度寫回 `Current[2]`、`Current[3]`，
  記 `cancel`（`Transferred=k`），最後清 `active`、`paused`、`pausedAt`、`dmaRewritten`。清旗標必須在算進度之後。
- DSP 重設（`CancelDMA`，`226h←1`）：取消主體之後清 IRR7，與 444 §2.3 相同。
- `sb8Begin` 開始新傳輸時同樣清 `paused`、`pausedAt`、`dmaRewritten`。

### 2.9 觀測

`SBDMAEvent` 的 `Kind` 加 `pause`、`resume`，欄位與欄數不變（`SBDMAEventFields` 不改，444 §3.2 的「欄數固定」照舊成立）。

| 欄位 | `pause` | `resume` |
|---|---|---|
| `CS`、`IP` | 本道指令起點（`inInsn`） | 同左 |
| 傳輸欄位 | 照帶（`xfer`） | 同左 |
| `Due` | 暫停前的到期時刻 | 平移後的到期時刻（平移之後才記） |
| `Transferred` | 凍結的進度 `k` | 同一個 `k` |
| `SHA256` | 空 | 空 |

`sb8Event` 的 SHA-256 條件（`sb_dma8.go:695`）改為只在 `start`、`complete` 計算。暫停過的傳輸 `start.Due ≠ complete.Due`；`complete.Due` 等於最後一個 `resume.Due`。
`cancel.Due` 為取消當下的 `due`（恢復過時為平移後的值）。

`SBDMA8Stats` 加 `Paused bool`、`Pauses`、`Resumes uint64`。兩個計數是診斷，與 `SBDMAUnknownReads` 同類：不進狀態檔，`LoadState`、`Restore` 不改它們。
`DMA8Completions` 進狀態檔，所以讀檔後兩類計數的起點不同，報告分開列出。

`cmd/probe/sbdma8.go`：`-sb-dma-log` 的說明文字（`:22`）列出的 `Kind` 加 `pause`、`resume`；`reportSBDMA8` 印出 `暫停中`、暫停次數、恢復次數。

### 2.10 狀態檔

- `sbDMA8State` 加三個匯出欄位：`Paused bool`、`PausedAt uint64`、`DMARewrittenWhilePaused bool`，三者都帶 `json:",omitempty"`。`sbDMA8StateV` 維持 1，
  `machineState.Version` 維持 4。`sb8Save`、`sb8Apply` 同步讀寫三欄。
- gob 位元組：第二個值的型別描述多了三個欄位，所以每個版本 4 檔案的位元組都與 444 版不同（未暫停也一樣）。`machineState` 的型別、選項關閉時的版本 3 位元組與
  `internal/state/baseline_anchor_test.go` 的錨點常數不變。
- 444 版程式寫的版本 4 檔：三欄讀成零值，即未暫停。
- 444 版程式讀本規格的暫停中檔案：gob 略過不認得的欄位，`Paused` 被忽略，傳輸在原到期時刻完成。登記在 §4.1；目前沒有外部存檔。
- 驗證：新增獨立函式 `sb8Validate(st *sbDMA8State) error`，在 `LoadState` 中 `sbDMA8State.V` 的檢查之後呼叫（`state.go:273-275` 之後、`:277` 之前），
  早於任何寫入。不成立時回錯誤，機器不變。條件：
  1. `Paused` ⇒ `Active` 且 `Cmd≠80h`。
  2. `Paused` ⇒ `Start ≤ PausedAt`。
  3. `Paused` 且步數時鐘（`!CycleUnit`）⇒ `PausedAt ≤ Due`。週期時鐘不檢查這條（§2.5）。
  4. `!Paused` ⇒ `PausedAt==0` 且 `!DMARewrittenWhilePaused`。
- `Snapshot`／`Restore`：快照由 `sb8Save` 產生，`Restore` 不驗證。`snapshot.go` 不改。
- `StateDigest`：`state_digest.go` 不改。`sbDMA8State` 的 JSON 因 `omitempty`，未暫停時與 444 版相同，摘要不變；暫停時三欄進入 JSON，摘要隨之改變。

### 2.11 不在範圍

16 位元控制（`D5h`、`D6h`）；自動初始化的暫停（444 沒有自動初始化）；idle 與靜音中的 `D0h`／`D4h`（由 446 處理，由 446 修訂）；改寫 DMA 後的 `D4h`（拒絕）；
未暫停時遮罩通道 1 造成的暫停（444 §2.2 照舊失敗即關閉）；`22Ch` 的忙碌位元模型（由 446 處理，由 446 修訂）；聲音輸出與暫停期間的取樣信用。

### 2.12 改動點

| 位置 | 改動 |
|---|---|
| `internal/machine/sb_dma8.go` | §2.2 的欄位；`wireSB8` 接 `ControlDMA8`；`sb8Control`；`sb8Now`、`sb8Poll`；`sb8StartDMA`、`sb8StartSilence` 的隱式取消；`sb8Cancel` 拆分；`sb8Begin` 清欄位；`sb8Out` 的 `xfer` 條件、改寫判定與進度寫入；`sb8ReadDMAReg` 的即時讀值條件；`sb8Event` 的 SHA-256 條件；`SBDMA8Stats`；`sbDMA8State` 三欄與 `sb8Save`、`sb8Apply`；`sb8Validate` |
| `internal/machine/state.go` | `LoadState` 呼叫 `sb8Validate` |
| `cmd/probe/sbdma8.go` | `-sb-dma-log` 說明文字、`reportSBDMA8` |
| `internal/machine/sb_dma8_test.go`、`cmd/probe` 的測試 | §3 |
| 不改 | `sb_dsp.go`（`:212-216` 已把 `D0h`／`D4h` 交給回呼）；`machine.go`（`Step`、`Out8`、`In8`）；`snapshot.go`；`state_digest.go`；`dma8237.go`；`le_*.go`；`internal/cpu`；`internal/cpu386`；`internal/state` |

`machine.go` 不改，選項關閉的熱路徑不變，不另量效能。

---

## 3. 驗收

1. 選項關閉：`go test ./internal/... ./cmd/probe ./session ./oracle` 與 `go test ./apps/...` 與改動前相同；錨點常數不變。`D0h`、`D4h` 在回呼為 nil 時回 false，
   由既有的 `TestDSPDMA8ControlCommandParsingAndRefusal`（`sb16_dma8_control_test.go:9` 起，`available=false` 的列）涵蓋；選項關閉時 `wireSB8` 不執行，回呼維持 nil。
   LE 的 309 測試（`go test ./internal/machine -run 'LE|DMA|DSP|SB16|IRQ7'`）不改而通過。
2. `internal/machine` 新測試（選項開啟，未註明時為步數時鐘）。步號在測試內寫死；期望的進度以 `⌊經過·n/Δ⌋` 在測試內獨立計算，不呼叫 `sb8Progress`。
   1. `14h` 暫停與恢復：`D0h` 在序號 p；之後 N 步不完成，讀 `02h`、`03h` 固定等於 `k(p)`；`D4h` 在序號 r；恢復後立刻讀與中途讀，等於獨立算出的進度，
      且在 r 那一刻等於 `k(p)`（與暫停前連續）；完成發生在 `m.Steps`（遞增前）首次 ≥ `due + (r − p)` 的那一步，IRQ7 遞送。紀錄依序為 `start`、`pause`、`resume`、
      `complete`、`irq7`、`eoi`；`start.Due` 與 `pause.Due` 為原到期，`resume.Due` 與 `complete.Due` 為 `due + (r − p)`；`pause`、`resume` 的 `SHA256` 為空、
      `Transferred` 為 `k(p)`。
   2. `74h` ADPCM 一組（Δ 以 `2n` 個取樣計，進度以 DMA byte 計）。
   3. 兩次暫停：兩組 `D0h`／`D4h`，完成時刻為 `due + d1 + d2`。
   4. 第二個 `D0h` 隔幾步再送：`pausedAt` 不被覆寫，恢復的平移以第一個 `D0h` 為準。
   5. 完整暫停與恢復後再送 `D4h`：接受，`start`、`due` 不動；未暫停的傳輸中送 `D4h` 同樣接受、不動作。
   6. 步數時鐘下 `D0h` 落在序號等於 `due` 的那一道：暫停中不完成；`D4h` 後在 `due + d` 完成。
   7. 週期時鐘：`d` 以兩道 `out` 之後的 `CPU.Cycles` 差計算，完成時刻寫死。另一例構造 `pausedAt ≥ due`（`D0h` 那一步開頭 `Cycles < due`，扣 `cycIO` 後越過）：
      暫停中讀值為 `n` 對應的位址與計數、不完成；`D4h` 之後下一個 `Step` 開頭完成。
   8. 暫停中的 IRQ7：前一個傳輸在 IMR bit7 設起時完成，IRR7 待處理；開始新傳輸並暫停，清 IMR bit7 後 IRQ7 照常遞送；另一例在暫停中讀 `22Eh`，IRR7 清除、不遞送。
      暫停那一步之後第一個允許的步可以遞送（暫停不設 `eventThisStep`）。
   9. 暫停中重設 DMA 並送新 `14h`：照 §1.2 `0562` 的次序寫 `0Ah←05h`、`83h`、`0Ch`、`02h`×2、`03h`×2、`0Bh←49h`、`0Ah←01h`，不報錯；改寫後讀 `02h`、`03h`
      得驅動寫入值；送 `14h` 後舊傳輸有 `cancel`（`Transferred=k(p)`，與新 `start` 同 `Step`、`CS`、`IP`），舊傳輸沒有 IRQ7、沒有 TC；新傳輸照 444 在寫死的步號完成並遞送一次 IRQ7。
   10. 部分改寫：暫停後只寫 `0Ch`、`02h` 的低位元組，`Current[2]` 的高位元組等於 `addr+k(p)` 的高位元組。
   11. 暫停中不改 DMA 直接送新 `14h`：隱式取消把進度寫回 `Current[2]`、`Current[3]`，新傳輸以寫回值檢查 444 §2.4 第 5 項並開始。
   12. 暫停中送 `80h`：舊傳輸隱式取消，靜音照 444 開始與完成。
   13. 隱式取消前 IRR7、ISR7、`IRQPending` 各自為真的情形：取消後三者不變。
   14. 暫停中改寫 DMA 後送 `D4h`：失敗即關閉，訊息含「未被接受」與「已改寫」。
   15. idle 的 `D0h`、`D4h` 與 `80h` 靜音中的 `D0h`、`D4h`：接受，見 446 §3 第 2、3 項（由 446 修訂）。
   16. 暫停中 `226h` 重設：`cancel` 的 `Transferred=k(p)`，未改寫時暫存器寫回 `k(p)`、改寫過時不寫回；`paused`、`pausedAt`、`dmaRewritten` 清除，IRR7 清除；
       之後的 `D4h` 依 446 §2.2 接受，不恢復（`Resumes==0`、多一筆 `control-idle`，由 446 修訂）。
   17. 狀態檔：暫停中 `SaveState`／`LoadState` 與 `Snapshot`／`Restore`，步數時鐘與週期時鐘各一例；讀回後仍暫停，接著 `D4h`、完成、處理常式再多 100 步，
       最終 `SaveState` 位元組與不中斷的執行相同。`sb8Validate` 的四個條件各以一份竄改的狀態檔驗證被拒且機器不變。不含三欄的 444 版 `sbDMA8State`
       （測試內以同名欄位子集的型別編碼）讀成未暫停。`StateDigest` 能偵測 `Paused` 的改變；未暫停時 `sbDMA8State` 的 JSON 不含三個欄名。
   18. `SBDMA8Stats` 的 `Paused`、`Pauses`、`Resumes` 正確，冪等的重複命令不計；`reportSBDMA8` 的輸出含這三項；`-sb-dma-log` 的說明文字含 `pause`、`resume`。
3. 突變自檢（各至少一個測試失敗）：暫停時 `sb8Poll` 照常完成（拿掉早退）；`sb8Now` 暫停時不回 `pausedAt`；`d` 用 `sb8Now()` 計算；`D4h` 不平移；只平移 `due`；
   先清 `paused` 再算取消進度；第二個 `D0h` 覆寫 `pausedAt`；暫停中新啟動不取消舊傳輸；暫停中 DMA 寫入仍失敗即關閉；改寫前不寫入凍結進度；
   改寫後讀值仍以進度計算；取消時仍寫回已改寫的暫存器；隱式取消清 IRR7；改寫後 `D4h` 被接受；重設不清 `paused`（「idle `D0h` 被接受」一條刪除，由 446 修訂）；
   `sb8Validate` 不檢查；`pause` 計算 SHA-256；`resume` 在平移前記 `Due`。
4. `gofmt`、`go build ./...`、`go vet ./internal/... ./cmd/probe` 無輸出；SingleStepTests 以掛載的語料實際執行、0 skip；§2.12「不改」列的 `git diff` 為空；
   `go test ./apps/...` 改動前後排序後逐行相同。
5. 觸發案例（需原版，主代理另附收據）：同 §1.1 的旗標冷開機，`-steps` 給足越過第 131427746 步。記錄：
   - `pause` 事件的步數、`CS:IP`、`Transferred` 與 `Due`。預測：第 131427746 步、`B5C5:03F9`、`Transferred` 為 `⌊6171132·18430/19209292⌋＝5920`、`Due`＝144465906（收據另行獨立驗算）。
   - `D0h` 的呼叫點：`03BC` 執行 `lret` 後的 `CS:IP`，判斷是 `0372`、`1141`、`1187` 或 `10E5` 路徑（§1.2）。
   - `D0h` 之後是否緊接 `D3h`（`0372` 路徑），以及 `0BAD(10h)` 的步數（帶 `-mode13-phase` 時預期約 16×165000 步）。
   - 是否出現 `D4h`（`11AE` 路徑）。
   - `D0h` 之後是否出現 `04B6` 的 IMR 寫入與 `226h←1`（`0BEF` 關閉流程）；出現時記錄重設造成的 `cancel`（`Transferred` 預測 5920）與 IRR7 的值。
   - 下一段取樣的 DMA 重設（各埠寫入的步數與值）、隱式取消的 `cancel`（`Transferred`）、新 `start` 的命令、長度與 `Due`，以及完成與 IRQ7。
   - MENU 的去向、下一個停點、原因與總步數。停在失敗即關閉時記下命令位元組與 `CS:IP`。停在別處時照實記錄，不改程式湊結果。

---

## 4. 未知與已決定事項

### 4.1 未知與風險

| 項目 | 現況 |
|---|---|
| `D0h` 的呼叫點 | `03BC` 有四個呼叫點（§1.2），觸發案例走哪一個未知，§3 第 5 項記錄。若是 `1187`（暫停 API），之後可能出現 `D4h`；若是 `0BEF` 關閉流程，暫停以 DSP 重設結束，444 的重設取消已涵蓋 |
| `22Ch` 的忙碌位元 | dosgolem 恆回 0（`sb_dsp.go:83-87`），`03BC` 只能跑到逾時才送 `D0h`，約 596 萬步。DOSBox-X 有忙碌週期（`sblaster.cpp:3192-3214`）。影響停止的時刻與「逾時期間傳輸到期而跳過 `D0h`」的機率，不影響本規格的規則。由 446 處理（由 446 修訂） |
| idle 的 `D0h`、`D4h` | DOSBox-X 接受且無害（`2298-2307`、`2343-2348`）。由 446 處理：接受、無狀態變化（由 446 修訂） |
| 靜音中的 `D0h`、`D4h` | DOSBox-X 接受，靜音的 IRQ 照發（§1.3）。由 446 處理：接受、無狀態變化（由 446 修訂） |
| 暫停中的 `80h` | DOSBox-X 保留暫停中的舊 DMA，之後的 `D4h` 仍可恢復它；本規格隱式取消。兩者的 IRQ 結果相同（只有靜音的 IRQ） |
| 改寫 DMA 後的 `D4h` | DOSBox-X 以新暫存器、舊剩餘長度續傳；本規格拒絕，沒有證據不猜 |
| 新啟動與 `IRQPending` | DOSBox-X 的 `DSP_DoDMATransfer` 清 `irq.pending_8bit`（`1181`），不降 PIC 線。444 與本規格都不動 `IRQPending`。前一次完成的 `22Eh` 尚未讀時，新傳輸完成後不會再拉 IRR7；本驅動的處理常式先讀 `22Eh`，不受影響 |
| 444 版程式讀暫停中的新檔 | gob 略過 `Paused`，傳輸在原到期完成。目前沒有外部存檔 |
| 暫停與恢復的計數 | 不進狀態檔，讀檔後從本機的累計繼續（§2.9） |
| 暫停的粒度 | 進度以整數 byte 計（`⌊經過·n/Δ⌋`），真機停在 byte 之間的時序不建模 |

### 4.2 已決定（2026-10-17）

1. **暫停是「已停止的傳輸」**（主代理決定）。暫停期間允許改寫 DMA，收到新的啟動命令時隱式取消舊傳輸。依據：停止流程只送 `D0h`，下一段取樣直接重設 DMA 並啟動（§1.2）；
   DOSBox-X 的新命令同樣取代暫停中的傳輸（§1.3）。
2. **idle 與靜音中的 `D0h`、`D4h` 失敗即關閉**（主代理決定）。驅動在 idle 不送 `D0h`（§1.2）；DOSBox-X 的差異登記在 §4.1。
   由 446 推翻：ISR 沒跑時驅動會對 idle 送 `D0h`（446 §1.3），改為接受、無狀態變化（446 §2.2）（由 446 修訂）。
3. **改寫 DMA 後的 `D4h` 失敗即關閉**（主代理決定）。沒有證據的組合不猜。
4. **改寫前先寫入凍結進度**（依審查）。單一位元組的寫入保留前進後的另一個位元組，與 8237 現值暫存器的行為一致；之後讀值與取消都以暫存器為準。
5. **隱式取消不動 IRR7、ISR7、`IRQPending`**（依審查）。清 IRR7 是 DSP 重設的語意，新命令在 DOSBox-X 也不降 PIC 線。
6. **不加觀測欄位**（依審查）。隱式取消以「與新 `start` 同一步、同 `CS:IP`」辨識，`SBDMAEventFields` 與 probe 表頭不變。
7. **`sbDMA8StateV` 維持 1，新欄位用 `omitempty`**（主代理決定）。未暫停時摘要不變；gob 位元組改變，照實登記（§2.10）。

## 5. 實作收據（2026-10-17）

commit：`8ef399c`。

- §3 單元測試：容器內 `go test -count=1 ./internal/... ./cmd/probe ./session ./oracle` 13 個套件 ok；SingleStepTests 316 PASS、0 SKIP；`gofmt`、`go vet` 無輸出。
  突變 19 條各至少一個測試失敗。§2.12 列為不改的檔案無 diff；LE 測試不變；`go test -v ./apps/...` 與改動前相同。
- 與 §2.8 字面的差異：靜音（`80h`）被取消時 `Transferred` 沿用 444 記 0（靜音沒有 DMA 位元組）；445 路徑上觀測不到（靜音不能暫停）。
- 觸發案例（`strike_commander` 的 `workplace/logs/probe471.log`、`workplace/scratch/p471/sbdma.tsv`）：SB16 設定冷開機，旗標同 §1。越過第 131427746 步：
  `D0h` 暫停（進度 5920，與 §3 預測相同），第 131431883 步同一緩衝區隱式取消並重新開始，第 150641175 步完成、遞送 IRQ7、EOI；下一段 12726 bytes 同樣暫停、重啟、
  完成。第 172112189 步完成時 IVT[0Fh] 已指回 StubSeg，IRQ7 由 BIOS 預設處理（`defaulted`）；第 176621909 步驅動對 idle 的 DSP 送 `D0h`，§2.3 拒絕而停。
  idle `D0h` 在真機與 DOSBox-X 無害，由規格 446 改為接受。每次 `start` 到 `D0h` 約 617 萬步，原因是 `22Ch` 恆回 0，驅動等忙碌位元脈衝到逾時；同由 446 處理。

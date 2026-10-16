# 446：一般機器 SB16 的 idle 控制命令與 `22Ch` 忙碌週期

狀態：**DRAFT**（第 1 版）
日期：2026-10-17
前置：[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md)、[`445-legacy-sb16-dma8-pause-resume`](445-legacy-sb16-dma8-pause-resume.md)
修訂（READY 後由主代理執行，本版不動其他檔）：445 §1.3、§2.3、§2.11、§3 第 2.15 項與突變清單、§4.1、§4.2 決定 2；444 §2.3（`22Ch` 讀列）、§2.10、§3.11、§4.1。
逐條內容見 §2.7。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/` 下的檔案，`sblaster.cpp` 省略路徑 `src/hardware/`。程式位置以 commit `8ef399c` 為準
（`bb0467a` 只改文件）。驅動程式位移為 `B5C5:0000` 起的映像位移（444 §1.2）。推論等級：已證實、強推論、假說、未知。

---

## 1. 為什麼要做

### 1.1 觸發案例

《Strike Commander》SB16 設定冷開機，旗標 `-dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile -exec-env -exec-reclaim -mode13-phase -sound-blaster -adlib -sb-dma8 -sb-dma-log`
（`strike_commander` 的 `workplace/logs/probe471.log`、`workplace/scratch/p471/sbdma.tsv`，已證實）。時鐘為步數時鐘，`IRQ0Every=96506`、`PITDiv=9943`。

| 步 | 事件 | `CS:IP` | 備註 |
|---|---|---|---|
| 125256614 | `start` 18430 bytes | `B5C5:0255` | `Due`＝144465906 |
| 131427746 | `pause` | `B5C5:03F9` | 距 `start` 6171132 步，進度 5920 |
| 131431883 | `cancel`、`start` | `B5C5:0255` | 同一緩衝區隱式取消並重新開始（445 §2.7） |
| 150641175 | `complete`、`irq7`、`eoi` | | |
| 152656981 | `start` 12726 bytes | `B5C5:0255` | |
| 158843947 | `pause` | `B5C5:03F9` | 距 `start` 6186966 步，進度 5935 |
| 158848084 | `cancel`、`start` | `B5C5:0255` | `Due`＝172112189 |
| 172112189 | `complete`、`defaulted` | `B5C5:03E6` | IVT[0Fh] 已指回 StubSeg，驅動的 ISR 沒跑 |
| 176621909 | 寫 `22Ch←D0h` | `B5C5:03F9` | 445 §2.3 拒絕：「DSP 寫入 22Ch←D0h 未被接受：沒有進行中的傳輸」 |

### 1.2 驅動的 `03BC` 與 `0228`

靜態證據：主代理以 `objdump -D -b binary -m i8086` 反組譯 `LILTRE.TRE` 位移 793944 起 4579 bytes（同 444 §1.2、445 §1.2；本機 scratchpad，不進版控）。
等級為強推論。`cs:[FE]` 存 `22Ch`。

| 位移 | 內容 | 每圈指令數 |
|---|---|---|
| `03BC`–`03CC` | `enter`、`push ds/si/di`、`pushf`、`mov dx,cs:[FE]`、`mov cx,7`、`mov bx,0` | 進入 8 道 |
| `03CF`–`03EA` | 計數減一（`js` 時跳 `03F7`）、`sti`、三個 `jmp $+2`、`cs:[106]==0` 時跳 `03FA`、`cli`、`in 22Ch`、`or al,al`、`jns 03CF`（等 bit7 立起） | 13，`in` 是第 11 道 |
| `03EC` | `mov cx,8000h` | 1 |
| `03EF`–`03F5` | `dec cx`、`je 03F7`、`in 22Ch`、`or al,al`、`js 03EF`（等 bit7 落下） | 5 |
| `03F7`、`03F9` | `mov al,D0h`、`out 22Ch` | 2 |
| `03FA`、`03FB` | `popf`；`cs:[106]=0` | |
| `0228` | DSP 寫入程序：`234` 讀 `cs:[100]` 的埠一次；`245`–`250` 等 `22Ch` bit7 落下（5 道一圈，逾時 `3:0000`）後 `255` 寫入；`25C`–`267` 寫入後再等 bit7 落下 | 5 |

逾時：`CX:BX=7:0000h` 遞減到負數才 `js`，共 `70000h` 個完整圈加一個 3 道的殘圈。從 `03BC` 進入到 `03F9` 的 `out`，不計中斷是
`8 + 70000h·13 + 3 + 2 = 5963789` 步。

### 1.3 現況為何失敗

dosgolem 的 `22Ch` 讀值在非重設時恆為 0（`internal/machine/sb_dsp.go:83-87`），`03BC` 的第一個迴圈等不到 bit7，每次都跑到逾時（已證實：
`5963789` 步與 §1.1 的 `start`→`pause` 間隔 6171132、6186966 同量級，差額是停止 API 被呼叫前的時間與迴圈中的中斷處理）。由此推回 `03BC` 的進入點：

| `D0h` 步 | `03BC` 進入點上限（`D0h` − 5963789） | 距前一個 `start` |
|---|---|---|
| 131427746 | 125463957 | 207343 |
| 158843947 | 152880158 | 223177 |
| 176621909 | 170658120 | 11810036（`start`＝158848084） |

最後一段的競態（強推論）：

1. 第 170658120 步前後，驅動經某個停止路徑進入 `03BC`，此時傳輸進行中、`cs:[106]≠0`。
2. `03BC` 的迴圈每圈有 `sti`（`03D7`）到 `cli`（`03E6`）的開中斷窗，逾時要 596 萬步。傳輸在第 172112189 步到期，`complete` 的 `CS:IP` 是 `B5C5:03E6`，
   正是這個迴圈內（已證實，`sbdma.tsv`）。
3. 到期時 IVT[0Fh] 已是 StubSeg，IRQ7 走 `defaulted`（444 §2.7），驅動的處理常式沒有執行，`cs:[106]` 沒被清掉。
   `1141` 先呼叫 `04B6` 還原向量再呼叫 `03BC`（445 §1.2），與「迴圈內到期卻是預設處理」相符，所以呼叫點是 `1141` 的可能性最高（強推論）。
4. 逾時後 `03F9` 對已經 idle 的 DSP 寫 `D0h`，445 §2.3 拒絕而停。

兩個成因：

- **`22Ch` 恆 0**：真卡與 DOSBox-X 的 SB16 有持續的忙碌週期，`03BC` 的兩個迴圈在數十步內結束。dosgolem 把停止延後 596 萬步，開了一個本來不存在的時間窗。
- **idle 的 `D0h` 失敗即關閉**：即使忙碌週期正確，處理常式沒跑的情形下，驅動仍可能對 idle 的 DSP 送 `D0h`。DOSBox-X 接受（§1.4）。

### 1.4 DOSBox-X 對照

| 項目 | DOSBox-X | 本規格 |
|---|---|---|
| `D0h` | `sblaster.cpp:2298-2307`：`FillUp`，`mode=MODE_DMA_PAUSE`，移除 `END_DMA_Event`、`DMA_DAC_Event`。不檢查目前模式（`2302-2304` 的註解） | idle 與靜音中接受、無狀態變化（§2.2） |
| `D4h` | `2343-2348`：只在 `MODE_DMA_PAUSE` 時改 `MODE_DMA_MASKED` 並 `Register_Callback` | 同上 |
| idle `D0h` 之後的 `D4h` | 進 `MODE_DMA_MASKED`；之後 `DMA_UNMASKED`（`3665-3670`）呼叫 `CheckDMAEnd`，`dma.left==0` 時直接返回（`CheckDMAEnd` 第一行）。下一個 DMA 命令無條件重設 `mode`（`1114`） | 不建模殘留的 `mode`。可觀測結果相同：不發 IRQ、不傳資料（強推論） |
| 靜音中的 `D0h` | `80h` 以 `PIC_AddEvent(&DSP_RaiseIRQEvent, …)` 排程（`2219-2222`）；`D0h` 只移除 `END_DMA_Event`、`DMA_DAC_Event`（`2306-2307`），靜音的 IRQ 照發 | 靜音照常到期發 IRQ7 |
| `22Ch` 讀值 | `3192-3219`：`DSP_S_NORMAL` 時先 `busy_cycle_io_hack++`（`3204`），再呼叫 `DSP_busy_cycle()`（`3205`）；忙碌或 `write_busy` 時回 `FFh`，否則 `7Fh`（`3213`）。`DSP_S_RESET`、`DSP_S_RESET_WAIT` 回 `FFh`（`3216-3218`），不碰計數 | §2.4 |
| 忙碌週期的條件 | `DSP_busy_cycle_active()`（`1717-1730`）：`busy_cycle_always` 為真即成立。`busy_cycle_always` 在 `dsp busy cycle always=-1`（預設，`src/dosbox.cpp:4046`）且 `SBT_16` 時為真（`4433-4436`） | SB16 固定組態（444 §2.1），恆成立 |
| 忙碌週期的相位 | `DSP_busy_cycle()`（`1732-1762`）：`now=PIC_FullIndex()`（毫秒，`include/pic.h:72-74`）；`now ≥ last_check + 0.02`（20 µs）時 `io_hack = ⌊fmod(now/1000·hz, 1)·16⌋`（`1756-1757`）；每次呼叫都 `last_check=now`（`1759`）；`t=(io_hack%16)·100/16`，`t < duty` 為忙碌（`1760-1761`） | §2.4 |
| 參數預設 | `dsp busy cycle rate=-1`、`duty=-1`（`src/dosbox.cpp:4039`、`4050`）；`busy_cycle_hz = 8333333/6/16`（`4342-4346`，C 整數除法得 86805）；duty 不在 0–100 時取 50（`4439-4440`）；`busy_cycle_last_check`、`busy_cycle_io_hack` 初值 0（`4126-4127`） | 同值 |
| 寫入後忙碌 | `DSP_DoWrite`（`1765-1800`）在每次寫入後設 `write_busy=1`，延遲 `dsp_write_busy_time`（預設 1000 ns，`4321-4323`）後由 `DSP_BusyComplete`（`3825` 起）清除；`write_busy` 期間的寫入被忽略（`1766-1769`） | 不建模（§2.8） |

`busy_cycle_io_hack` 的用意（`1738-1754` 的註解）：DOSBox-X 的 I/O 不依 ISA 匯流排計時，CPU 週期設高時連續讀取之間的模擬時間趨近於零，
只看時鐘的話要讀很多次才轉態。所以連續讀取改以計數前進，保證 8 次讀取轉態一次。dosgolem 同樣有這個問題的變形：只看時鐘時，
迴圈每圈的時鐘數若是忙碌週期的整數倍，相位會凍結。本規格照抄這個計數。

---

## 2. 規則

### 2.1 範圍與接線

- 只在 `-sb-dma8` 開啟（`m.sb8 != nil`）時生效。選項關閉時 `22Ch` 讀值、`D0h`／`D4h` 的處理與現況逐位元組相同。
- `22Ch` 讀取：`sb8In`（`sb_dma8.go:501` 起）在 `case 0x22E`（`:534`）旁加 `case 0x22C`。`Machine.In8`（`machine.go:1089-1092`）先交給 `sb8In`，
  所以 `SoundBlasterDSP.In8`（`sb_dsp.go:83-87`）不改，LE 的 `LEOPLPorts.dsp` 與選項關閉的路徑照舊。`TestDSPResetHandshake`（`sb_dsp_test.go`）的 `80h` 不變。
- `D0h`／`D4h`：只改 `sb8Control`（`sb_dma8.go:445` 起）的前兩個分支。`sb_dsp.go:212-216` 不改。

### 2.2 idle 與靜音中的 `D0h`、`D4h`

445 §2.3 的表改為：

| 狀態 | `D0h` | `D4h` |
|---|---|---|
| 無傳輸（`!active`） | 接受，無狀態變化，記 `control-idle` | 接受，無狀態變化，記 `control-idle` |
| `80h` 靜音進行中 | 接受，無狀態變化，記 `control-idle`；靜音照原到期時刻完成並拉 IRQ7 | 同左 |
| DMA 傳輸中、未暫停 | 暫停（445 不變） | 接受，不動作（445 不變） |
| 已暫停、DMA 未改寫 | 接受，不動作（445 不變） | 恢復（445 不變） |
| 已暫停、DMA 已改寫 | 接受，不動作（445 不變） | 失敗即關閉（445 不變） |

「無狀態變化」：`paused`、`pausedAt`、`dmaRewritten`、`start`、`due`、`active`、`cmd`、DMA 暫存器、TC、IRR7、ISR7、`IRQPending`、`pauses`、`resumes` 都不動，
`sb8Err` 不設。只有 §2.3 的計數與紀錄。

理由：真卡與 DOSBox-X 都接受（§1.4），驅動在處理常式沒跑時會送出這個命令（§1.3）。445 §4.2 決定 2 的前提「驅動在 idle 不送 `D0h`」被 §1.3 的事件推翻。

### 2.3 觀測與計數

- `SBDMAEvent.Kind` 加 `control-idle`，欄位與欄數不變（`SBDMAEventFields` 不改，445 §4.2 決定 6）。
- 無傳輸：`sb8Event("control-idle", true, false)`，再把 `Command` 設為 `D0h` 或 `D4h`。其餘傳輸欄位為零值，`Format` 為空。
- 靜音中：`sb8Event("control-idle", true, true)`，帶入靜音的 `Length`、`Samples`、`Due`、`Format=silence`，再把 `Command` 覆寫為 `D0h` 或 `D4h`。
  `Transferred` 為 0，`SHA256` 為空（`sb8Event` 的 SHA-256 只在 `start`、`complete` 計算）。
- `CS`、`IP` 為本道指令起點（`inInsn`）。
- `sbDMA8` 加 `idleControls uint64`，每次 §2.2 前兩列的接受加 1。`SBDMA8Stats` 加 `IdleControls uint64`。與 `pauses` 同類，是診斷，不進狀態檔，`LoadState`、`Restore` 不改它。
- `cmd/probe/sbdma8.go`：`-sb-dma-log` 的說明文字列出 `control-idle`；`reportSBDMA8` 在 445 那一行之後印 idle 控制次數。

### 2.4 `22Ch` 讀值：忙碌週期

#### 時基

與 444 §2.4 同一個時鐘。時鐘值 `t` 與每秒時鐘數 `A/B`：

| 時鐘 | `t` | `A` | `B` |
|---|---|---|---|
| 週期時鐘（`m.CycleClock`） | `m.CPU.Cycles` | `hz`（`CPUHz`，為 0 時 `DefaultCPUHz`） | 1 |
| 指令數時鐘，`IRQ0Every>0` | `m.Steps` | `IRQ0Every·315000000` | `264·PITDiv` |
| 指令數時鐘，`IRQ0Every==0` | `m.Steps` | `165000·315000000·pct` | `264·17000·100` |

- 時鐘種類依讀取當下的 `m.CycleClock`，不用傳輸的 `s.cycles`（忙碌週期與傳輸無關）。`PITDiv`、`IRQ0Every`、`pct` 也取讀取當下的值。
- `t` 是這道 `in` 執行時的值：步數時鐘下等於本道指令的序號（`Step` 在 `CPU.Step` 之前已遞增）；週期時鐘下已含這道 `in` 的 `charge(cycIO)`（445 §2.5 同理）。
- 程式改 PIT 分頻時 `A/B` 隨之改變，下一次重新取相位會跳。這是決定性的，收據記錄分頻即可。

#### 常數

`H = 86805`（`8333333/6/16`，C 整數除法，`sblaster.cpp:4344`）。duty 固定 50：`t=(io·100)/16 < 50` 只在 `io` 為 0–7 時成立，所以「忙碌」⇔ `io < 8`。

#### 狀態

`sbDMA8` 加三個欄位：

| 欄位 | 意義 | 初值 |
|---|---|---|
| `busyIO uint8` | `busy_cycle_io_hack` 對 16 取餘（只有餘數影響結果，`1760`） | 0 |
| `busyLast uint64` | 上一次一般狀態下讀 `22Ch` 的 `t`（`busy_cycle_last_check`） | 0 |
| `busyLastCycles bool` | `busyLast` 的單位：真為 `CPU.Cycles` | 假 |

#### 每次讀取

`dsp.reset`（`226h` 寫 1 之後、寫 0 之前）為真：回 `FFh`，三個欄位都不動（`3216-3218`）。dosgolem 沒有 DOSBox-X 的 `DSP_S_RESET_WAIT`（20 µs），
`FFh` 只持續到 `226h←0`；444 §2.10 已把重設延遲列為不在範圍。選項關閉時重設中照舊回 `80h`。

否則依序：

1. `busyIO = (busyIO + 1) & 15`（`3204`，在判斷之前）。
2. 判斷是否重新取相位：`busyLastCycles ≠ m.CycleClock`，或 `t < busyLast`，或 `(t − busyLast)·50000·B ≥ A`（即 `t − busyLast` 不少於 20 µs，`1756`）。
   成立時 `busyIO = p(t)`，第 1 步的加一被覆寫。
3. `busyLast = t`，`busyLastCycles = m.CycleClock`（`1759`，每次都更新，不只在重新取相位時）。
4. `busyIO < 8` 回 `FFh`，否則回 `7Fh`。

相位：`p(t) = ⌊16·H·B·t / A⌋ mod 16`，即 `⌊fmod(t·H·B/A, 1)·16⌋`（`1757`，`now/1000` 是秒）。

#### 整數計算

不用浮點。

- `K = 16·H·B`、`G = 50000·B`，以 `mulU64` 計算。`A` 依上表以 `mulU64` 計算。任一溢位 `uint64` 或 `A==0` 時，以 `sb8Fail` 失敗即關閉，訊息含「22Ch 忙碌週期的時基溢位」與 `docs/spec/446 §2.4`，回 `FFh`。
- `p(t)`：`(hi, lo) = Mul64(K, t)`，`q = Div64(hi mod A, lo, A)` 的商，`p = q & 15`。理由：`hi = qₕ·A + r` 時 `⌊(hi·2⁶⁴+lo)/A⌋ = qₕ·2⁶⁴ + ⌊(r·2⁶⁴+lo)/A⌋`，
  `2⁶⁴` 是 16 的倍數，所以只需低 64 位元，`Div64` 也不會因 `hi ≥ A` 而 panic。
- 20 µs 判斷：`(gh, gl) = Mul64(t − busyLast, G)`，`gh ≠ 0 || gl ≥ A` 為成立。等號算成立，與 `1756` 的 `>=` 相同。
- DOSBox-X 以 `double` 計算，在相位邊界可能差一格。驗收以本節的整數式為準，另在遠離邊界的點與 DOSBox-X 的浮點式比對（§3 第 4 項）。

#### 數值

- 觸發案例的時鐘（`IRQ0Every=96506`、`PITDiv=9943`）：`A/B ≈ 11580931.8` 步／秒。20 µs ≈ 231.6 步，間隔 232 步以上才重新取相位。
  週期 `A/(B·H) ≈ 133.4` 步，每格 ≈ 8.34 步。
- 週期時鐘、`hz=33000000`：20 µs 為 660 週期，週期 ≈ 380.2 週期。

### 2.5 本案效果推算

`03BC` 的讀取每 13 步（第一個迴圈）或 5 步（第二個迴圈）一次，都小於 232，連續讀取以計數前進。第一次讀取距前一次 `22Ch` 讀取遠大於 232 步
（前一次在 `0228` 送上一個命令時），重新取相位，設得 `p₀`。第一個迴圈讀 `j` 次、第二個迴圈讀 `k` 次：

- `p₀` 為 0–7：第一次讀即忙碌，`j=1`；第二個迴圈讀到 `io=8` 為止，`k = 8 − p₀`。
- `p₀` 為 8–15：第一個迴圈讀到 `io` 繞回 0，`j = 17 − p₀`；`k = 8`。

從進入 `03BC` 到 `out` 的步數（不計中斷）：`8 + 13j + 1 + 5k + 2`。

| `p₀` | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 步數 | 64 | 59 | 54 | 49 | 44 | 39 | 34 | 29 | 168 | 155 | 142 | 129 | 116 | 103 | 90 | 77 |

上限 168 步，現況 5963789 步。只要連續兩次讀取的間隔小於 20 µs（步數時鐘下這裡是 232 步），上限與時鐘設定無關。中斷在 `sti` 窗內插入且處理超過 232 步時，下一次讀取重新取相位，上限另加處理常式的步數與至多
一次重新開始的 168 步（假說：機率約為 168／96506）。

預測（假說，收據檢驗）：

- `start`→`D0h` 的間隔由 617 萬步縮為約 20.7 萬到 22.3 萬步（§1.3 表的「距前一個 `start`」加 168 以內）。絕對步數不能沿用，見下一點。
- `0228` 每次寫 DSP 前後各多讀至多 8 次（每次 5 步），偵測與播放的絕對步數都會移動；444 §5、445 §5、probe470、probe471 的步號不再適用。
- 只預測間隔，不預測各段的絕對位置。每段的 `start` 與停止呼叫是跟著前一段走，還是跟著遊戲時間（IRQ0）走，目前無法判斷，最後一段因此有兩個分支：
  - 跟著該段走：`03BC` 進入點約在該段 `start` 之後 1181 萬步，傳輸 Δ＝13264105 步，`D0h` 落在傳輸進行中（約 89%），記 `pause`，之後看 `0BEF` 的動作。
  - 跟著遊戲時間走：第二段的 `D0h` 提早約 596 萬步，重新開始的 `start` 約在第 152885000 步，`Due` 約 166150000；停止呼叫仍在約第 170658000 步。
    傳輸在驅動的向量還在時完成，IRQ7 正常遞送，處理常式清 `cs:[106]`，`03BC` 不送 `D0h`。這一段既沒有 `pause` 也沒有 `control-idle`。
  - 收據以「最後一段是否出現 `pause`、`control-idle`，或兩者皆無」與「停止呼叫的進入步數和該段 `Due` 的先後」判定是哪一個分支（§3 第 11 項）。

### 2.6 狀態檔

計數影響之後的讀值，所以三個忙碌欄位進狀態檔；`idleControls` 不進。比照 445 §2.10：

- `sbDMA8State` 加 `DSPBusyIO uint8`、`DSPBusyLast uint64`、`DSPBusyLastCycles bool`，都帶 `json:",omitempty"`。`sbDMA8StateV` 維持 1，`machineState.Version` 維持 4。
  `sb8Save`、`sb8Apply` 同步讀寫。`machineState` 本身有 `CycleClock`（`state.go:80`），但 `busyLast` 的單位要跟著它自己的值走：讀檔後的機器若換了時鐘種類，
  第一次讀取必須重新取相位，所以另存 `DSPBusyLastCycles`，不借用傳輸的 `CycleUnit`（傳輸結束後 `CycleUnit` 是舊值）。
- gob 位元組：`sbDMA8State` 的型別描述多三個欄位，每個版本 4 檔的位元組都與 445 版不同。`machineState` 的型別、選項關閉時的版本 3 位元組、
  `internal/state/baseline_anchor_test.go` 的錨點常數不變。
- `StateDigest`：`state_digest.go` 不改。三欄為零值時 JSON 不含它們。SC 的 `0228` 在每次寫 DSP 之前讀 `22Ch`，第一次 DSP 命令之後三欄通常不為零，
  所以實務上 SC 開 `-sb-dma8` 的版本 4 檔的摘要都會與 445 版不同。照實登記。
- 445 版程式寫的版本 4 檔：三欄讀成零值。第一次讀取時 `t ≥ 0`、`busyLastCycles` 與目前時鐘相符與否決定是否重新取相位；`t` 超過 20 µs 時一定重新取相位，與存檔前的連續性只差在
  存檔當下正處於連續讀取中的情形。
- 445 版程式讀本規格的檔：gob 略過三欄，忙碌模型不存在，`22Ch` 回 0。目前沒有外部存檔。
- `sb8Validate` 加一條：`DSPBusyIO < 16`。`DSPBusyLast` 不驗證（`t < busyLast` 由 §2.4 第 2 步處理）。
- `Snapshot`／`Restore` 經 `sb8Save`、`sb8Apply`，一併涵蓋。`snapshot.go` 不改。

### 2.7 對 444、445 的修訂

READY 之後由主代理改寫，各註「由 446 修訂」：

| 位置 | 改動 |
|---|---|
| 445 §2.3 | 表的「無傳輸」「`80h` 靜音進行中」兩列改為 §2.2；拒絕訊息表刪「無傳輸」「靜音中」兩列，只留「改寫後 `D4h`」；「idle 與靜音中拒絕的理由」一段改為指向 446 §2.2 |
| 445 §1.3 | 表的「靜音中的 `D0h`」列（本規格：拒絕）與「`22Ch` 讀值」列（本規格：既有 0）改為指向 446 §2.2、§2.4 |
| 445 §2.11 | 「idle 與靜音中的 `D0h`／`D4h`（拒絕）」「`22Ch` 的忙碌位元模型」兩項刪除，改註「由 446 處理」 |
| 445 §3 第 2.15 項 | 改為「idle 與靜音中的 `D0h`、`D4h` 接受，見 446 §3」；突變清單的「idle `D0h` 被接受」刪除 |
| 445 §4.1 | 「`22Ch` 的忙碌位元」「idle 的 `D0h`、`D4h`」「靜音中的 `D0h`、`D4h`」三列改為「由 446 處理」 |
| 445 §4.2 決定 2 | 註「由 446 推翻：§1.3 的事件顯示處理常式沒跑時驅動會對 idle 送 `D0h`」 |
| 444 §2.3 | `22Ch` 讀列改為「選項開啟時見 446 §2.4；關閉時既有：重設中 `80h`，否則 0」；`D0h`／`D4h` 的註記加「idle 與靜音中接受（446）」 |
| 444 §2.10 | 「DSP 命令處理時間與忙碌週期（DOSBox-X `DSP_busy_cycle`）」改為「DSP 命令處理時間（`write_busy`）；忙碌週期由 446 處理」 |
| 444 §3.11 | 「無傳輸或靜音中的 `D0h`」從失敗即關閉的清單刪除 |
| 444 §4.1 | 「DSP 的延遲」列加註忙碌週期由 446 處理 |

### 2.8 不在範圍

- `write_busy`（`1765-1800`、`3825` 起，預設 1 µs）。它在每次寫入後讓 `22Ch` 回 `FFh` 約 11.6 步（觸發案例的時鐘），並忽略這段時間內的寫入。
  驅動的 `0228` 在寫入前後都等 bit7 落下，忙碌週期本身已讓這兩個等待有限步內結束；忽略寫入的行為若照抄，沒等待就寫入的程式會掉命令，
  需要另外的證據。另立規格。
- `DSP_S_RESET_WAIT` 的 20 µs、`DSP_S_HIGHSPEED`、SB 2.0 的 `AAh`／`2Ah` 回值（`3210-3211`）。
- 可設定的 `busy_cycle_hz`、duty、`busy_cycle_always`（固定 SB16 預設）。
- LE 的 `22Ch`（`LEOPLPorts`）。
- 16 位元控制（`D5h`、`D6h`），同 445。

### 2.9 改動點

| 位置 | 改動 |
|---|---|
| `internal/machine/sb_dma8.go` | `sbDMA8` 的 `busyIO`、`busyLast`、`busyLastCycles`、`idleControls`；`sb8In` 的 `case 0x22C`（§2.4）；`sb8Control` 前兩個分支（§2.2）；`control-idle` 事件；`SBDMA8Stats.IdleControls`；`sbDMA8State` 三欄與 `sb8Save`、`sb8Apply`；`sb8Validate` 一條 |
| `cmd/probe/sbdma8.go` | `-sb-dma-log` 說明文字、`reportSBDMA8` |
| `internal/machine/sb_dma8_test.go`、`cmd/probe/sbdma8_test.go` | §3 |
| 不改 | `sb_dsp.go`；`machine.go`；`state.go`（`sb8Validate` 的呼叫點已存在）；`snapshot.go`；`state_digest.go`；`dma8237.go`；`le_*.go`；`internal/cpu`；`internal/cpu386`；`internal/state` |

`machine.go` 不改，選項關閉的熱路徑不變，不另量效能。

---

## 3. 驗收

1. 選項關閉：`SetSoundBlaster(true)` 而未開 `-sb-dma8` 時，`In8(0x22C)` 在重設中回 `80h`、其他時候回 0（新增一例；`TestDSPResetHandshake` 不改而通過）；
   `D0h`、`D4h` 回 false 不報錯（既有 `TestDSPDMA8ControlCommandParsingAndRefusal` 的 `available=false` 列）。錨點常數不變。
2. idle 與靜音的控制命令（取代 `TestSB8ControlRefusedIdleAndSilence`，改名 `TestSB8ControlAcceptedIdleAndSilence`）：
   1. idle 的 `D0h`、`D4h`，各自單獨與 `D0h` 後接 `D4h`：`sb8Err` 為 nil，`*m.sb8` 除 `idleControls` 與忙碌欄位外與前一步相同（測試以結構拷貝比較），
      `IdleControls` 依次數增加，`Pauses`、`Resumes` 為 0；紀錄各有一筆 `control-idle`，`Command` 為 `D0`／`D4`、`Format` 為空、`Due` 為 0。
   2. 靜音 `80h` 進行中的 `D0h`、`D4h`：接受、無狀態變化；靜音在原 `Due`（測試內以 444 §2.4 公式寫死）完成並遞送 IRQ7；`control-idle` 的 `Format=silence`、
      `Command` 為 `D0`／`D4`、`Due` 等於靜音的到期時刻。
   3. 445 的行為不變：DMA 傳輸中的 `D0h` 照常暫停，已暫停且改寫後的 `D4h` 照常失敗即關閉（既有 `TestSB8Pause*` 與第 2.14 項的測試不改而通過）。
3. 被取代的既有斷言：
   - `TestSB8FailClosed`（`sb_dma8_test.go:900`）的清單刪除 `{nil, 0xD0}`、`{nil, 0xD4}` 兩列，其餘 DSP 命令列不變。
   - `TestSB8PauseReset`（`sb_dma8_test.go:1805`）最後的「重設後的 `D4h` 依 idle 拒絕」改為「接受、無狀態變化、記 `control-idle`」。
   - `cmd/probe/sbdma8_test.go:138-151` 的 `D0h`、`D4h` 都在傳輸中，不受影響。
   - 實作前以 `grep -n '沒有進行中的傳輸\|靜音\|未被接受' internal/machine/*_test.go cmd/probe/*_test.go` 重新盤點，收據列出改了哪些斷言。
4. `22Ch` 讀值（machine 層測試，直接設定 `m.Steps` 或 `m.CPU.Cycles` 後呼叫 `m.In8(0x22C)`；期望值在測試內以 `math/big` 依 §2.4 的有理數式獨立計算，不呼叫被測函式）：
   1. 步數時鐘、`IRQ0Every=96506`、`PITDiv=9943`：從 `t₀`（遠離相位邊界，且 `t₀ ≥ 232`，使第一次讀取由初值 `busyLast=0` 重新取相位）起每 13 步讀一次共 17 次，第一次等於 `p(t₀)` 對應的值，之後的 16 次嚴格依
      `io` 加一、`<8` 回 `FFh`、`≥8` 回 `7Fh` 輪替；接著間隔 232 步讀一次，等於 `p(t)`；間隔 231 步讀一次，等於前一個 `io` 加一。
   2. 第 1 例的序列在測試內寫死成位元組陣列（例如 17 個 `FFh`／`7Fh`），讓日後改公式時可見。
   3. 週期時鐘（`CPUHz=33000000` 與 `CPUHz=0`）各一例，門檻 660 與 659 週期。
   4. 與 DOSBox-X 浮點式比對：測試內另寫 `int(fmod(t/rate·H, 1)·16)`（`float64`），在 1000 個遠離邊界（與最近邊界相距超過 1/1000 格）的 `t` 上與 `p(t)` 相同。
   5. 重設中：`226h←1` 後讀 `22Ch` 回 `FFh`，`busyIO`、`busyLast`、`busyLastCycles` 不變；`226h←0` 後恢復 §2.4。
   6. 時鐘種類改變：`busyLastCycles` 與 `m.CycleClock` 不同時，下一次讀取重新取相位。`t < busyLast`（以 `LoadState` 構造）時同樣重新取相位。
   7. 溢位：以極端 `IRQ0Pinned`（`IRQ0Every` 使 `A` 溢位）讀 `22Ch`，失敗即關閉，訊息含「時基溢位」。
   8. 與傳輸無關：idle、DMA 傳輸中、暫停中、靜音中各讀一段，序列只由時鐘與計數決定。
5. `03BC` 同形迴圈：以 `sbAsm` 組出與 §1.2 同形的程式（進入 8 道、13 道一圈、`mov cx`、5 道一圈、`mov al,D0h`、`out`；旗標字放在 PSP 段），不複製原版位元組。
   測試遮罩 IRQ0（IMR bit0）並確認沒有其他事件在 200 步內遞送，否則 `sti` 窗內的處理常式會打亂步數。在 DMA 傳輸中呼叫：`D0h` 的步號減進入步號等於 §2.5 表中 `p₀` 對應的值（`p₀` 在測試內獨立計算），且 ≤ 168；寫 `D0h` 後傳輸暫停。另以 16 個 `t₀` 涵蓋 `p₀` 的 16 個值。
6. 狀態檔：在第 4 項第 1 例讀到一半時 `SaveState`／`LoadState` 與 `Snapshot`／`Restore`，之後的讀值序列與不中斷的執行相同，最終 `SaveState` 位元組相同；
   `DSPBusyIO=16` 的竄改檔被 `sb8Validate` 拒絕且機器不變；不含三欄的 445 版 `sbDMA8State`（測試內以欄位子集的型別編碼）讀成零值；
   三欄為零時 `sbDMA8State` 的 JSON 不含三個欄名，非零時 `StateDigest` 改變。
7. 觀測與報告：`SBDMA8Stats.IdleControls` 正確；`reportSBDMA8` 的輸出含 idle 控制次數；`-sb-dma-log` 的說明文字含 `control-idle`。
8. LE 不變：`go test ./internal/machine -run 'LE|DMA|DSP|SB16|IRQ7'` 不改而通過；`LEOPLPorts` 的 `22Ch` 讀值照舊。
9. 突變自檢（各至少一個測試失敗）：idle `D0h` 失敗即關閉；靜音中 `D0h` 移除靜音或延後到期；idle 控制改動 `paused` 或計入 `pauses`；`control-idle` 的 `Command` 未覆寫；
   加一放在判斷之後；每次讀取都重新取相位；只在重新取相位時更新 `busyLast`；忙碌條件寫成 `io ≤ 8`；20 µs 比較寫成 `>`；`p(t)` 少乘 `B` 或少乘 16；
   時鐘種類用 `s.cycles`；重設中的讀取加一或更新 `busyLast`；重設中回 `80h`；狀態檔漏存 `busyIO` 或 `busyLast`；`sb8Validate` 不檢查 `DSPBusyIO`；
   選項關閉時也套用忙碌模型。
10. `gofmt`、`go build ./...`、`go vet ./internal/... ./cmd/probe` 無輸出；`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠；SingleStepTests 以掛載的語料實際執行、
    0 skip（掛載方式同 433 §8.2）；§2.9「不改」列的 `git diff` 為空；`go test ./apps/...` 改動前後排序後逐行相同。
11. 觸發案例（需原版，主代理另附收據）：同 §1.1 的旗標冷開機，`-steps` 給足越過第 176621909 步（只當步數預算，不是預期事件位置；絕對步號會移動，§2.5）。記錄：
    - 偵測兩次是否仍通過；`start`→`complete` 的步數與 444 §5 比較。
    - 每一組 `start`→`pause`（`D0h`）的間隔，預測約 20.7 萬到 22.3 萬步；`pause`→`cancel`／`start` 的間隔（probe471 為 4137）。
    - `D0h` 的 `CS:IP` 仍為 `B5C5:03F9`；可行時記錄 `03BC` 的進入步數，與 `D0h` 的差值是否 ≤ 168（另加中斷處理）。
    - 是否出現 `control-idle`，出現時記錄步數、`Command`、是否緊接在 `defaulted` 之後。
    - 最後一段是否出現 `pause`、`control-idle`，或兩者皆無；停止呼叫（`1141` 或 `0BEF`）的進入步數與該段 `Due` 的先後，據此判定 §2.5 的分支。
      之後的 `226h` 重設、`cancel` 的 `Transferred`、下一段 `start`。
    - `22Ch` 的讀取次數（`PortsIn[0x22C]`）與改動前比較。
    - MENU 的去向、下一個停點、原因與總步數。停在失敗即關閉時記下命令位元組與 `CS:IP`。停在別處時照實記錄，不改程式湊結果。

---

## 4. 未知與待決定

### 4.1 未知與風險

| 項目 | 現況 |
|---|---|
| `03BC` 的呼叫點 | 前兩段（`start` 後約 20 萬步，之後 4137 步重新開始）像是 `10E5` 路徑；最後一段像是 `1141`（§1.3 第 3 點）。都是強推論，§3 第 11 項記錄 |
| `defaulted` 之後的 `IRQPending` | 驅動的處理常式沒跑，`22Eh` 沒被讀，`IRQPending` 留為真。444 §3.8：`IRQPending` 已為真時，之後的完成不再拉 IRR7，下一段傳輸可能永遠收不到 IRQ7。`0228` 在 `234` 讀 `cs:[100]` 的埠一次，若那是 `22Eh` 就會清掉。主代理在執行期讀 `B5C5:0100` 的字確認 |
| 絕對步號移動 | `0228` 每次寫入多讀至多 16 次，444 §5、445 §5、probe470、probe471 的步號與檢查點不再適用（版本 3 檢查點本來就不能載入開 `-sb-dma8` 的機器） |
| 其他程式讀 `22Ch` | MENU 以外的程式（OPTTEST、MKTERR、STRIKE）若有不設逾時、等 `22Ch` 為 0 的迴圈，現在最多等 8 次讀取。若有程式等 bit7 恆為 0 才繼續，行為會變。§3 第 11 項的讀取次數可看出影響 |
| 浮點與整數的差異 | 相位邊界上可能與 DOSBox-X 差一格（§2.4）。只影響重新取相位那一次讀取 |
| idle `D0h` 後的 `D4h` | DOSBox-X 會留下 `MODE_DMA_MASKED` 並重註冊回呼（§1.4），本規格不建模。可觀測結果相同是強推論，沒有實測 |
| `write_busy` | 不建模（§2.8）。寫入後 1 µs 內讀 `22Ch` 時，DOSBox-X 回 `FFh`，本規格依計數回值。差異至多一次讀取 |
| 445 版程式讀新檔 | 忙碌欄位被略過，`22Ch` 回 0。目前沒有外部存檔 |

### 4.2 待決定（附建議）

1. **連續讀取用計數前進，照抄 `busy_cycle_io_hack`**。建議採用。替代方案是只看時鐘：`22Ch` 回值只由 `p(t)` 決定，不加狀態欄位，`sbDMA8State` 與摘要不變；
   代價是迴圈每圈的時鐘數接近忙碌週期的整數倍時相位幾乎不動（例如週期時鐘 `hz` 為 86805 的倍數時可能完全凍結），等待的長度隨時鐘設定大幅變動，
   也與 DOSBox-X 的交叉對照不同。本案的時鐘下兩者都在 200 步內結束，差別在其他設定與其他程式。
2. **忙碌模型只在 `-sb-dma8` 開啟時生效**。建議採用。全面開啟會改變其他 app、LE 以外的 SB 使用者與既有收據的 `22Ch` 讀值；選項開啟的機器本來就要從冷開機重錄。
3. **重設中回 `FFh`**（DOSBox-X 值）。建議採用，只在選項開啟時。bit7 與既有的 `80h` 相同，其他位元與 DOSBox-X 一致；選項關閉與 LE 照舊 `80h`。
4. **idle 與靜音中的 `D0h`、`D4h` 接受且無狀態變化**。建議採用（§2.2）。替代方案是照 DOSBox-X 在 idle `D0h` 後進入一個「空暫停」狀態，等 `D4h`；沒有任何可觀測差異的證據，不建議。
5. **`control-idle` 一個種類涵蓋 idle 與靜音**，以 `Format` 區分。建議採用，欄數不變。
6. **時鐘種類另存 `DSPBusyLastCycles`**。建議採用（§2.6）。替代方案是不存、讀檔後一律重新取相位；那會讓存讀檔在連續讀取中途時與不中斷的執行不同，違反 §3 第 6 項。

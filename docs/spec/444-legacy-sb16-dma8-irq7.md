# 444：一般機器的 SB16 8 位元單次 DMA 與 IRQ7

狀態：**DRAFT**（第 1 版）
日期：2026-10-17
前置：[`190-opl-vgm-dump`](190-opl-vgm-dump.md) §8、[`190-irq0-calibration`](190-irq0-calibration.md)、[`198-dosbox-cycles`](198-dosbox-cycles.md)、
[`009-keyboard-irq1`](009-keyboard-irq1.md)、[`012-input-keyboard-irq-and-mouse-scale`](012-input-keyboard-irq-and-mouse-scale.md)、
[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、
[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)、[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)、
[`443-segmented-32bit-code-and-stack`](443-segmented-32bit-code-and-stack.md)。LE 路徑的對照：`242`–`245`、`247`、`296`、`304`、`305`、`309`。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/hardware/` 下的檔案。驅動程式位移為 `B5C5:0000` 起的映像位移（§1.2）。
程式位置以 commit `d22e4ea` 為準。推論等級：已證實、強推論、假說、未知。

---

## 1. 為什麼要做

### 1.1 觸發案例

《Strike Commander》的 SB16 設定（`SC.CFG` 含 `-ma -ds`）在 dosgolem 冷開機，probe 帶 `-sound-blaster -adlib -pm-profile`。MENU 印
「Strike Commander cannot locate the sound card(s) you have selected.」並以 1 結束（`strike_commander` 的 `workplace/logs/probe460.log`、
`probe461.log`，總步數 4431748）。

| 項目 | 內容 | 等級 |
|---|---|---|
| 驅動程式載入 | MENU 從 `LILTRE.TRE`（檔案位移 793944）讀 4579 bytes 到 `B5C5:0000`，第 2418666 步 | 已證實（probe460 讀檔紀錄） |
| IRQ7 向量 | 第 2421158 步 `AH=35h AL=0Fh` 讀到舊向量 `0080:003C`（StubSeg），第 2421278 步 `AH=25h` 把 `int 0Fh` 設為 `B5C5:05C1` | 已證實（probe460 第 71、717–718 行） |
| 埠寫入序列 | `workplace/scratch/p460/sb.tsv`，見下表 | 已證實 |
| CPU 模式 | 此時在 JEMM 監控下的 V86；IRQ 經 JEMM 的 IDT 遞送，JEMM 把 `08h`–`0Fh` 反射到 V86 的 IVT（435 §1.1） | 已證實（V86）；反射為 435 的靜態證據 |
| 視訊模式 | 第 137937 步切到 13h，第 4407299 步切回 03h；偵測期間是 13h | 已證實（probe460 模式切換紀錄） |
| 時鐘 | 指令數時鐘（未帶 `-cpuhz`），計時器送出 8 次；結束時 PIT 分頻 65536 | 已證實（結束時）；偵測當下的分頻為強推論 |

埠寫入（step、port、value）：

| 步數 | 寫入 | 意義 |
|---|---|---|
| 2419353 | `22C←D3` | DSP 關喇叭 |
| 2420855、2420899 | `226←01`、`226←00` | DSP 重設 |
| 2420972、2421007 | `22C←40`、`22C←A6` | 時間常數 `A6h`：取樣週期 `256−166=90` µs，約 11111 Hz |
| 2421287、2421291 | `0A1←FF`、`021←00` | 讀改寫 PIC 遮罩，放行 IRQ7（§1.2 的 `0449`） |
| 2421354–2421380 | `00A←05`、`083←00`、`00C←00`、`002←00`×2、`003←03`、`003←00`、`00B←49`、`00A←01` | DMA 通道 1：遮罩、頁 0、清 flip-flop、位址 0、計數 3（4 bytes）、模式 `49h`（單次、記憶體到裝置、遞增、不自動初始化）、解除遮罩 |
| 2421414–2421489 | `22C←14`、`22C←03`、`22C←00` | DSP `14h` 8 位元單次輸出，長度 4 |
| 2425604、2425608 | `0A1←FF`、`021←00` | 還原遮罩（§1.2 的 `04B6`） |

### 1.2 驅動程式的偵測流程

以 probe460 的讀檔紀錄從 `workplace/root/LILTRE.TRE` 位移 793944 取 4579 bytes（SHA-256
`68b0e62af80a317949501bfc446b1962f2353a41bfe2c488ded7634dab8ef949`），在容器內以 `objdump -D -b binary -m i8086` 反組譯（本機，未存檔）。
映像開頭是「Copyright (C) 1991,1992 Miles Design, Inc.」。下列為靜態證據，執行期次序與 §1.1 的埠寫入吻合，整體等級為強推論。

| 位移 | 內容 |
|---|---|
| `0C7B`–`0D83` | 偵測主程式。`cs:FA`–`cs:100` 依基底算出 `226h`、`22Ah`、`22Ch`、`22Eh`；`cs:102`＝IRQ（7）、`cs:104`＝DMA 通道（1）。依序：`1D9`（等 `22Ch` bit7 清除後寫 `D3h`）、`0BAD(16)`、`0283`（重設，回 0 即失敗）、`sti`、`cs:[19C]=0`、`0407(A6h)`、`0449(B5C5:05C1)`、`07EB(0000:0000, 長度 4)`，接著 `DI=22h` 次迴圈：`0BAD(1)` 後比較 `cs:[19C]==1`，成立就跳出；最後 `04B6` 還原並以 `SI`（1 或 0）回傳 |
| `05C1` | IRQ 處理常式：`in al,[cs:100]`（讀 `22Eh`）、`cs:[19C]=1`、`cs:[106]=0`、`out 20h,20h`（IRQ ≥ 8 時另寫 `A0h`）、`iret`。不執行 `sti` |
| `0BAD` | 等 n 次垂直回掃：`DX=[0040:0063]+6`（`3DAh`），每次先等 bit3 立起、再等 bit3 落下 |
| `0283` | `226←01`、讀 `226h` 20 次、`226←00`，最多 16 次「等 `22Eh` bit7、讀 `22Ah`」直到讀到 `AAh` |
| `0407` | `1000000/(256−TC)` 存區域變數；經 `0228` 寫 `40h`、TC。`0228` 以 `22Ch` bit7 輪詢，逾時則重設後重試一次 |
| `0449` | `int 21h AH=35h/25h` 存舊向量、設新向量（向量 `08h+IRQ`）；`in A1h`／`in 21h` 存起來，`AND` 掉 IRQ 位元後寫回 |
| `04B6` | `21h`、`A1h` 以 `OR` IRQ 位元再 `AND` 舊值寫回；`int 15h AX=1209h BX=4753h`（JEMM 私有 API），回 0 時直接寫 IVT 還原向量，否則 `int 21h AH=25h` |
| `07EB` | 以 `段×16+位移` 算線性位址，`≥ A0000h` 時以 `int 67h AX=DE06h`（VCPI 取實體位址）換算；之後呼叫 `0562` 程式 DMA，再送 DSP 命令與長度 |
| `0562` | `out 0Ah, ch|4`、頁暫存器（`80h+[F2h+ch]`，表為 `87h/83h/81h/82h`）、`out 0Ch`、位址兩次、計數兩次、`out 0Bh, 48h|ch`、`out 0Ah, ch` |
| `00EA` | DSP 播放命令表：`14h`、`75h`、`77h`、`17h`、`14h`、`74h`、`76h`、`16h`。另有 `80h`（`09B6`，靜音區塊）、`D0h`（`03F7`）、`D4h`（`03A7`）、`D1h`、`D3h`。沒有 `1Ch`、`48h`、`C6h` 等自動初始化命令 |

所以偵測成立的條件是：DSP `14h` 傳完 4 bytes 後 IRQ7 送達 `B5C5:05C1`，而且在 34 次回掃等待結束之前。
`07EB` 的 VCPI 換算說明這支驅動把 DMA 位址當實體位址處理；本例緩衝區在 `0000:0000`，低於 `A0000h`，不經換算（強推論）。
JEMM 的 TSS I/O 點陣圖靜態全 0（436 §1 的表），V86 下這些 `in`／`out` 直接到 dosgolem 的埠，不經 JEMM 模擬。

### 1.3 現況為何失敗

| 位置 | 現況 |
|---|---|
| `internal/machine/machine.go` 342–345 行 | 一般機器的 `soundBlaster SoundBlasterDSP` 沒有接 `StartDMA` 等回呼。`22C←14`、`03` 被接受，`22C←00` 時 `StartDMA==nil`，`Out8` 回 false，`Machine.Out8` 不報錯，DSP 停在「等長度高位元組」的半狀態 |
| 同檔 `In8`／`Out8` | `00h`–`0Fh`、`81h`–`87h` 沒有處理：寫入只記 `PortLog`，讀回 `FFh`。`A1h` 不保存，讀回 `FFh`。`20h` 讀回 `FFh`，寫入不處理 |
| 同檔 561–566、1616 行 | `picMask` 保存整個位元組，只有 bit0（IRQ0，`tick`）與 bit1（IRQ1，`keyboard.go` 的 `keyTick`）被參考；沒有 IRR／ISR，EOI 不處理 |
| `sb_dsp.go`、`dma8237.go`、`le_opl_ports.go`、`le_dma_clock.go` | 8237、DSP 單次傳輸、IRQ7、PIC 主片完整遮罩與 EOI 只接在 `LEOPLPorts`（DOS4GW 路徑），時鐘是 1 微秒／指令的近似（304） |

結果是 IRQ7 從不發生，`cs:[19C]` 維持 0，偵測回 0。

### 1.4 時間窗

| 項目 | 數值 | 等級 |
|---|---|---|
| 真機的偵測時間窗 | 34 次垂直回掃，mode 13h 約 70 Hz，約 486 ms | 強推論（§1.2 的 `0BAD`） |
| dosgolem 現況的時間窗 | 從 `22C←00`（第 2421489 步）到還原（第 2425604 步）共 4115 步 | 已證實（sb.tsv） |
| 時間窗變短的原因 | `0x3DA` 走行為模型：`(portTicks>>4)&1` 決定 bit3（`machine.go` 的 `In8`），回掃週期由輪詢次數決定，一次 `0BAD(1)` 約百餘道指令。`Mode13Phase`（`-mode13-phase`）與 `CRTCTiming` 預設關閉 | 強推論（4115 與 34 次輪詢的量級吻合；收據待補） |
| 4 bytes 的傳輸時間 | `4×90 µs＝360 µs` | 已證實（TC＝`A6h`） |
| 360 µs 換成指令數時鐘 | §2.4 公式，`IRQ0Every＝636085`、分頻 65536：4170 步 | 計算；分頻為強推論 |
| 開 `-mode13-phase` 的時間窗 | 每次回掃一個 `VGAFrameEvery`（預設 165000 步），34 次約 561 萬步 | 強推論（`mode13_time.go` 的 `mode13InputStatus`） |

與 IRQ0 同一個時基時，IRQ7 在第 2421489＋4170 步才到，晚於時間窗的 4115 步，偵測仍然失敗；之後 `04B6` 把 IRQ7 遮罩，IRQ7 留在 IRR 不送。
真因是回掃的行為模型把 486 ms 壓成 4115 步，而 DSP 的時間是依機器時鐘換算的真時間。本規格不改 `0x3DA` 的模型；SC 要過偵測，觸發案例要同時帶
`-mode13-phase`（§4 待決定第 3 項）。這個預測可以檢驗：不帶 `-mode13-phase` 時，§3 第 12 項的收據應看到 IRQ7 在預測的步數成為待處理、被遮罩、沒有遞送。

---

## 2. 規則

### 2.1 選項

- `Machine.SetSBDMA8(on bool) error`，預設關閉。開啟的條件：`SoundBlasterPresent()` 為真（`SetSoundBlaster(true)` 之後）、`programLoaded` 為假、
  `len(m.Mem)==MemSize`。違反時回錯誤，狀態不變。開啟後不可關閉（再呼叫 `SetSBDMA8(false)` 回錯誤）。
- 新增 `Machine.SBDMA8() bool`。新狀態放在 `sb8 *sbDMA8`（nil 表示關閉）；新程式碼都在 `m.sb8 != nil` 之內。
- `cmd/probe` 加 `flag.Bool` 旗標 `-sb-dma8`（放在 `-sound-blaster` 旁，說明文字引用本規格），要求同時給 `-sound-blaster`，否則 `die`。
  另加 `-sb-dma-log <檔>`，把 §2.8 的觀測紀錄寫成 TSV。
- 硬體組態固定為基底 `220h`、8 位元 DMA 通道 1、IRQ 7，與觸發案例的 `SC.CFG` 相同。其他組態屬後續規格。
- 預設關閉的理由同 439、441：這是共用層，`20h`、`A1h`、`00h`–`0Fh`、`81h`–`87h` 的讀值與 IRQ 的遞送在開啟後會改變；`apps/` 下有程式碰這些埠時
  既有收據可能改變。要不要預設開啟由使用者決定。

### 2.2 8237 主片（通道 0–3）

沿用 `dma8237.go` 的 `DMA8237` 型別當暫存器檔（位址、計數、頁、模式、單一遮罩、全遮罩、flip-flop），在一般機器新建一個實例。
傳輸引擎另寫，不抽出 `LEOPLPorts` 的 `advanceDMA`。

取捨：LE 的 `advanceDMA` 綁在 `LEOPLPorts` 的欄位、1 微秒／指令的時鐘、逐 byte 的取樣信用、PCM 擷取與 16 位元通道上，有 242–309 共十餘份 CONFORMED
收據。抽成共用要改這些檔案，風險高於重寫一個只做單次傳輸的引擎。保證：`dma8237.go`、`sb_dsp.go`、`le_*.go` 的 `git diff` 為空，LE 的測試不改。

| 埠 | 開啟時的行為 |
|---|---|
| `00h`–`07h` 寫 | `DMA8237.Out8`：依 flip-flop 寫低／高位元組，`Base` 與 `Current` 同時更新 |
| `00h`–`07h` 讀 | 傳輸中的通道 1 位址、計數依 §2.5 的進度即時算出（不寫回暫存器）；其餘走 `DMA8237.In8`。從未寫過的暫存器（`Known` 位元缺）回 `FFh` 並累計 `SBDMAUnknownReads`（診斷，不進狀態） |
| `08h` 讀 | 狀態暫存器：bit0–3 為各通道的 TC，讀後清除；bit4–7 為 0（DOSBox-X `dma.cpp:388-397`）。`DMA8237` 沒有 TC 欄位，TC 位元存在 `sbDMA8` |
| `08h` 寫（命令）、`09h` 寫（請求） | 值為 0 時接受不動作；非 0 失敗即關閉（§2.7） |
| `0Ah` | 單一遮罩（`DMA8237.Out8`） |
| `0Bh` | 模式：`Mode[ch]=v&FCh`（`DMA8237.Out8`） |
| `0Ch` | 清 flip-flop |
| `0Dh` | 主清除：flip-flop 清除、四通道遮罩；另清 TC 位元（`dma.cpp:336-344`） |
| `0Eh`、`0Fh` | 清全遮罩、寫全遮罩 |
| `87h`、`83h`、`81h`、`82h` | 通道 0–3 的頁暫存器，讀寫（`DMA8237` 的 `dmaPageChannel`）。`80h`、`84h`–`86h`、`88h` 等其他埠維持現況 |

傳輸中改變通道 1 的位址、計數、頁、模式，或遮罩通道 1，失敗即關閉。真機會暫停或改道，屬後續規格（DOSBox-X 以 `DMA_MASKED`、`DMA_UNMASKED` 事件暫停，
`sblaster.cpp:3636-3667`）。

### 2.3 DSP 命令

一般機器沿用 `SoundBlasterDSP`（`sb_dsp.go`）的命令解析，不改該檔。開啟時接上兩個回呼：`StartDMA`（§2.4）與 `CancelDMA`（§2.5 的取消）。
`StartAutoDMA`、`StartSB16AutoDMA`、`Start16DMA`、`ControlDMA8` 維持 nil。

| 命令或埠 | 行為 | DOSBox-X |
|---|---|---|
| `226h←1`、`226h←0` | 既有重設；`22Ah` 回 `AAh`。`CancelDMA`：進行中的傳輸取消（依 §2.5 的進度更新 DMA 暫存器，不設 TC），IRR7 若尚未被 CPU 接受則清除 | `DSP_DoReset` `sblaster.cpp:1489-1500`；`DSP_Reset` 清 `pending_8bit`（約 1480 行），但不降 PIC 線。本規格降線，與 LE 的 `CancelDMA`（`le_opl_ports.go:61-69`）一致 |
| `D1h`、`D3h` | 只改 `SpeakerOn`，無輸出 | `2309`、`2313` |
| `40h` TC | `RateNumerator=1000000`、`RateDenominator=256−TC` | `2129-2134`：`256000000/(65536−TC<<8)`，與 `1e6/(256−TC)` 相同 |
| `41h` 取樣率 | 既有：5000–45000 Hz，`RateDenominator=1` | |
| `14h` lo hi | 長度 `n=hi×256+lo+1`，呼叫 `StartDMA(n)` | `2113-2120`、`1325`（`dma.total=1+len`） |
| `E1h` | 既有：回 `04h 05h` | `2368` |
| `22Ah` 讀 | 既有：取回覆位元組 | |
| `22Ch` 讀 | 既有：重設中回 `80h`，否則 0（不忙） | |
| `22Eh` 讀 | 既有：清 `IRQPending`，有回覆時 `80h`。開啟時另外：若 IRR7 為 1 且尚未被 CPU 接受，清 IRR7 | `3158-3163`：清 `pending_8bit` 並 `PIC_DeActivateIRQ`；`pic.cpp:95-106` 的 `lower_irq` 清 IRR |
| `SoundBlasterDSP.Out8` 對 `224h`–`22Fh` 回 false 的寫入 | 失敗即關閉，錯誤含命令位元組與 DSP 半狀態。涵蓋 `1Ch`、`C6h`、`B0h`、`D0h`、`D4h`、`74h`–`77h`、`16h`、`17h`、`80h` 及其他未實作命令 | |

`48h`（設定區塊大小、不啟動傳輸）沿用既有行為。選項關閉時 `SoundBlasterDSP` 回 false 的寫入維持現況（不報錯）。

### 2.4 傳輸啟動與時間

`StartDMA(n)` 在下列條件全部成立時回 true；任一不成立回 false，由 §2.3 的規則失敗即關閉：

1. 沒有進行中的傳輸。
2. 通道 1 的位址、計數兩個暫存器的高低位元組都寫過（`Known[2]==3`、`Known[3]==3`），頁暫存器寫過（`PageKnown[1]`）。
3. `Mode[1]==48h`（單次、讀、遞增、不自動初始化）。其他模式（含 `58h` 自動初始化、`68h` 遞減、需求或區塊模式）屬後續規格。
4. 通道 1 未遮罩（`Mask&2==0`）。
5. `n ≤ Current[3]+1`。`n` 較大時真機的 DMA 先到 TC、DSP 等不到資料而不發 IRQ，屬後續規格。
6. `RateNumerator>0`、`RateDenominator>0`。

成立時記下開始時刻與到期時刻。時間單位與 IRQ0 相同，依機器的時鐘模式：

| 時鐘 | 單位 | 間隔 Δ |
|---|---|---|
| 週期時鐘（`CycleClock`） | `CPU.Cycles` | `⌈n·D·CPUHz / R⌉` |
| 指令數時鐘，`IRQ0Every>0` | `Steps` | `⌈n·D·IRQ0Every·315000000 / (R·264·PITDiv)⌉` |
| 指令數時鐘，`IRQ0Every==0` | `Steps` | `⌈n·D·165000·315000000·pct / (R·264·17000·100)⌉` |

- `R`、`D` 為 `SoundBlasterDSP` 的 `RateNumerator`、`RateDenominator`；`PITDiv` 為 `m.PITDivisor()`（1–65536）；`pct` 為 `ClockPercent()`。
  第二列等於 `n·D/R` 秒乘 `StepsPerSecondNow()`（`opl_vgm.go:28`，`IRQ0Every×PITHz`，`PITBaseHz=315e6/264`）；第三列等於乘 `StepsPerSecondScaled()`。
  所以 DSP 與 IRQ0、`-dump-vgm` 用同一個時基；`IRQ0Pinned` 時第二列照樣成立（`StepsPerSecondNow` 的註解）。
- 以 128 位元整數計算（`math/bits.Mul64`、`Div64`），不用浮點；Δ 至少為 1。
- 開始時刻取 `Out8` 當下的 `m.Steps`（本道指令的序號）或 `m.CPU.Cycles`。到期時刻＝開始時刻＋Δ，存成絕對值。
- 傳輸期間程式改 PIT 分頻、改時鐘比例，不重算到期時刻。
- 決定性：Δ 只由暫存器值與時鐘欄位決定，與主機時間、執行速度無關；同一狀態、同一輸入得到同一個到期時刻。

觸發案例：`n=4`、`D=90`、`R=1000000`、`IRQ0Every=636085`、`PITDiv=65536`，Δ＝`⌈72132039000000000 / 17301504000000⌉`＝4170 步。

### 2.5 傳輸完成與進度

`Step` 每一步在 `tick` 之前判斷：有進行中的傳輸，且 `m.Steps ≥ 到期`（指令數時鐘）或 `m.CPU.Cycles ≥ 到期`（週期時鐘），就完成這次傳輸。
分段模式（ring 0）下照樣完成（與幀、IRQ0 時鐘相同，裝置時間不因 CPU 模式停下）。完成依序：

1. 通道 1：`Current[2] = (Current[2]+n) & FFFFh`（頁不進位，8237 位址計數器 16 位元）；`Current[3] = (Current[3]−n) & FFFFh`。
   `n == 舊 Current[3]+1` 時到 TC：計數成 `FFFFh`、設通道 1 的 TC 位元、設遮罩（`dma.cpp:513-549`）。
2. 觀測：以 `peekPhysRaw`（`physical.go`，無 VGA latch 與讀取監看的副作用）讀 `n` 個實體位址 `Page[1]<<16 | ((Addr+i)&FFFFh)`，算 SHA-256，記進 §2.8。
   不寫任何記憶體，不改模擬狀態。DMA 不經 CPU 的分頁與 A20 閘（實體位址，強推論；JEMM 下也一樣，驅動以 VCPI `DE06h` 自行換算，§1.2）。
3. DSP：`IRQPending` 原本為假時拉 IRQ7 線（IRR7＝1）；接著 `IRQPending=true`。原本為真時不再拉（邊緣只發生一次，`sblaster.cpp:792-819`）。
4. 傳輸結束，`DMA8Completions++`。

進度（讀 DMA 暫存器、`CancelDMA` 用）：已傳 `k = min(n, ⌊經過·n/Δ⌋)`，經過＝現在減開始（同一單位）。讀到的位址＝`(Base位址起點+k)&FFFFh`，
計數＝`(起點計數−k)&FFFFh`。進度只影響讀值與取消時寫回的暫存器，不提早觸發 TC。

### 2.6 PIC 主片

選項開啟時以下列最小模型取代現況的「只看 bit0、bit1」對 IRQ7 的部分；IRQ0、IRQ1 的閘門（`machine.go` 的 `tick`、`keyboard.go` 的 `keyTick`、
`keyhold.go` 的 `timedKeyTick`）不改。

| 項目 | 規則 |
|---|---|
| IMR | 沿用 `picMask`（`21h` 寫入已保存整個位元組）；IRQ7 參考 bit7。初值不變（現況為 0） |
| IRR7、ISR7 | 新增兩個位元。IRR7 由 §2.5 第 3 步拉起，由 `22Eh` 讀取（§2.3）、`CancelDMA` 或 CPU 接受時清除。ISR7 由 CPU 接受時設起，由 EOI 清除 |
| `20h` 寫，OCW2 | `20h`（非特定 EOI）：ISR7 為 1 時清除，否則不動作（IRQ0、IRQ1 沒有 ISR 模型，它們的 EOI 落在這裡）。`60h`–`67h`（特定 EOI）：`67h` 清 ISR7，其餘不動作。`40h`（NOP）接受。旋轉（bit7 為 1）與設定優先序（`C0h`–`C7h`）失敗即關閉 |
| `20h` 寫，OCW3（`v&18h==08h`） | bit1–0 為 `10b`／`11b` 時選讀 IRR／ISR；bit2（poll）或 bit6（特殊遮罩）為 1 時失敗即關閉 |
| `20h` 寫，ICW1（`v&10h`） | 失敗即關閉。JEMM 不重新程式化 PIC（435 §1.1），向量基底固定 `08h` |
| `20h` 讀 | 依 OCW3 選擇，初值選 IRR。IRR：bit0＝`irq0Pending`、bit7＝IRR7。ISR：bit7＝ISR7。其餘位元 0（`pic.cpp:337-353`） |
| `A0h` 寫 | OCW2 的 EOI（`20h`、`60h`–`67h`）接受不動作；OCW3 保存讀取選擇；ICW1 失敗即關閉 |
| `A0h` 讀 | 回 0（從片 IRQ 不建模） |
| `A1h` | 保存從片 IMR，讀回保存值。初值 `FFh`（等於現況的讀值）。不影響任何遞送 |

優先序：IRQ0 > IRQ1 > IRQ7，由 `Step` 內的判斷次序實現（§2.7）。

已知差異（刻意，見 §4.1）：IRQ0、IRQ1 沒有 ISR。真機上 IRQ0 處理常式在 EOI 之前執行 `sti` 時 IRQ7 被 ISR0 擋住，這裡會被遞送；
ISR7 與 IRQ0 同時在服務中時，IRQ0 處理常式的非特定 EOI 在真機清 ISR0，這裡清 ISR7。IRQ0、IRQ1 不加 ISR 的理由：BIOS 的 `int 08h` stub
（`machine.go` 的 `initVectors`，`biosTimerOff`）沒有 EOI，加了之後 IRQ0 會在第一次之後永遠被擋。

### 2.7 IRQ7 遞送

`Step` 在鍵盤分支之後、`m.Steps++` 之前判斷 IRQ7。條件依序：

1. `m.sb8 != nil`、IRR7＝1、ISR7＝0、`picMask` bit7＝0。
2. 本步沒有發生其他事件：`tick` 沒有啟動回呼、週期遠呼叫或 IRQ0 遞送，鍵盤分支沒有遞送。IRQ0、IRQ1 的遞送會清 IF，所以通常由第 3 步擋下；
   回呼與週期遠呼叫不清 IF，要另外判斷（197 §3.1 的「同一道指令只送一個事件」）。
3. `IF=1`。
4. 分段模式（`CPU.Segmented()`，ring 0）：不遞送、不消耗，照 434 §5.3 延後；`segWait` 的「有待處理 IRQ」條件加上第 1 步成立的 IRQ7。
5. `ReadSup16(0x0F*4+2) == StubSeg`：不遞送、不消耗，`IRQ7Stalls++`（同 `keyTick` 的 `keyStalls`；V86 下 JEMM 也會反射到這個 IVT 項）。
6. `ivtGuardIRQ(0x0F)`：實模式查 433 的 IDTR；V86 呼叫 `prepareV86Delivery`（435 §2.4），失敗時 `irqErr` 由 `Step` 經 `takeIRQErr` 回傳。
7. 消耗：IRR7＝0、ISR7＝1、`IRQ7Deliveries++`、記觀測（§2.8 的 `irq7`）、`raiseIRQ(0x0F)`：V86 時 `commitV86Delivery`，否則 `CPU.Interrupt(0x0F)`。

435 §2.1 的事件表新增一列：外部中斷 IRQ7，向量 `0Fh`，無錯誤碼，無 IOPL 與閘 DPL 檢查，框架 `EIP` 為遞送當下的 `IP`。

觸發案例的路徑（強推論）：V86 → JEMM IDT 閘 `0Fh` → `4BDE` → `4C03`–`4C3B` 反射到 IVT[0Fh]＝`B5C5:05C1` → 處理常式在 V86 讀 `22Eh`、寫 `20h`、`iret`。

時間窗與決定性：IRQ7 的到期時刻在 `22C←00` 那一道指令就固定（§2.4）。之後能不能在時間窗內被接受，只取決於 IF、遮罩、ISR7 與分段模式，
這些都是模擬狀態，不受主機影響。時間窗本身由 `0BAD` 的回掃輪詢決定（§1.4）。

### 2.8 觀測紀錄

給 M3 語音字幕用的唯讀紀錄。不進狀態檔，不影響執行；`Snapshot`／`Restore` 不還原紀錄。

`Machine.SBDMALog []SBDMAEvent`（上限 4096 筆，超過時丟最舊的並累計 `SBDMALogDropped`），以及 `Machine.OnSBDMA func(SBDMAEvent)`（非 nil 時每筆呼叫，
與 `onOPL` 同類）。欄位：

| 欄位 | 內容 |
|---|---|
| `Kind` | `start`、`complete`、`cancel`、`irq7`（CPU 接受）、`eoi`（ISR7 清除） |
| `Step`、`Cycles` | 事件當下的 `m.Steps`、`m.CPU.Cycles` |
| `CPUMode` | `real`、`v86`、`seg`（分段模式） |
| `CS`、`IP` | 事件當下的段值與位移（`start` 為寫 `22Ch` 的那一道） |
| `Command` | DSP 命令位元組（本規格只有 `14h`） |
| `Length` | `n` |
| `RateNum`、`RateDen`、`TimeConstant`、`TCKnown` | DSP 的取樣率設定 |
| `Channel`、`DMAMode`、`Page`、`Addr`、`Count` | 開始時的通道 1 設定（`Count` 為程式寫入的計數） |
| `Phys` | 開始實體位址 `Page<<16|Addr` |
| `Due` | 到期時刻與單位（steps 或 cycles） |
| `Transferred` | `complete` 為 `n`；`cancel` 為進度 `k` |
| `SHA256` | `complete` 時 §2.5 第 2 步算出的雜湊 |

語音字幕的鍵可由 `Phys`、`Length`、`SHA256` 組成，並與 dosgolem 既有的讀檔紀錄（來源檔名與位移）對照；鍵格式屬 M3 的規格。

### 2.9 狀態檔

- gob 型別不變，錨點測試的常數不變（`internal/state/baseline_anchor_test.go`）。434 的 `PM` 欄位只放保護模式狀態，本規格不用它。
- 選項關閉：`SaveState` 照舊寫 `Version: 3`，與現況逐位元組相同。
- 選項開啟：`machineState.Version` 寫 4，並以同一個 `gob.Encoder` 緊接著編碼第二個值 `sbDMA8State`（新型別，欄位匯出）。
  `internal/state` 的機器段有 8 bytes 長度前綴，第二個值落在同一段內，不影響 DOS 段。
- `LoadState`：接受版本 2–4。版本 4 以同一個 `gob.Decoder` 再解 `sbDMA8State`；選項開關與「版本是否為 4」不一致時回錯誤，放在任何寫入之前
  （與 432 的 PM／profile 檢查相同）。`LegacyState` 改為 `Version < 3`。舊版程式讀到版本 4 的檔案時因版本超出範圍而拒絕，失敗即關閉。
- `StateDigest`：接受版本 3 與 4。版本 4 解出第二個值，把它的 JSON 的 SHA-256 接在既有摘要輸入之後。版本 3 的摘要不變。
- `sbDMA8State` 內容：自帶 `V int`（1）；`SoundBlasterDSP` 除回呼以外的全部欄位（含未匯出的命令半狀態、混音器、回覆佇列）；主片 `DMA8237` 全部欄位
  （含 flip-flop）與 TC 位元；`picMask`；從片 IMR；IRR7、ISR7；OCW3 讀取選擇（主、從）；傳輸的進行旗標、`n`、開始與到期時刻、時鐘單位、
  開始時的通道 1 位址與計數；`DMA8Completions`、`IRQ7Deliveries`、`IRQ7Stalls`。
- `picMask` 在選項關閉時不在 `SaveState` 內（現況缺口，`state.go:159-226` 沒有它；`Snapshot` 有）。本規格只在選項開啟時補存，關閉時不動，以守錨點。
- `Snapshot`／`Restore`：加入同一組欄位的深拷貝；`SoundBlasterDSP` 的回呼在 `Restore` 後重新接到同一台機器。

### 2.10 不在範圍

聲音輸出與混音（另立規格）；自動初始化 DMA（`1Ch`、`C6h`、`58h` 模式）；ADPCM（`74h`–`77h`、`16h`、`17h`）；靜音區塊 `80h`；`D0h`／`D4h` 暫停與恢復；
16 位元 DMA 與第二組控制器；其他基底、DMA 通道、IRQ；從片 PIC 的 IRQ；PIC 重新程式化；IRQ0、IRQ1 的 ISR；DSP 命令處理時間與忙碌週期
（DOSBox-X `DSP_busy_cycle`）、重設的 20 µs 延遲、PIC 遞送延遲；8259 的假 IRQ7；dosgolem 內建 EMM（非 JEMM）下以 page frame 為 DMA 緩衝區時的對應
（DOSBox-X `dma.cpp:68-74` 的 `UpdateEMSMapping`）；`0x3DA` 的時間模型（§1.4）。

### 2.11 改動點

| 位置 | 改動 |
|---|---|
| 新檔 `internal/machine/sb_dma8.go` | `sbDMA8` 型別、`SetSBDMA8`、`SBDMA8`、`StartDMA`／`CancelDMA` 回呼、§2.4 的時間計算、§2.5 的完成與進度、§2.6 的 PIC 埠、§2.7 的遞送、§2.8 的紀錄 |
| `internal/machine/machine.go` 的 `In8`、`Out8` | `m.sb8 != nil` 時，`00h`–`0Fh`、`20h`、`A0h`、`A1h`、`81h`–`83h`、`87h` 先交給 `sbDMA8`；`22Eh` 讀取後降 IRR7；DSP 寫入回 false 時記錯誤 |
| `internal/machine/machine.go` 的 `Step` | 進入條件加 `m.sb8 != nil` 的完成與 IRQ7 判斷；鍵盤分支之後呼叫 IRQ7 遞送；`CPU.Step` 之後的錯誤條件加上裝置錯誤，於同一步回傳 |
| `internal/machine/machine.go` 的 `segWait` | §2.7 第 4 步 |
| `internal/machine/state.go`、`state_digest.go`、`snapshot.go` | §2.9 |
| `cmd/probe/main.go` | `-sb-dma8`、`-sb-dma-log`；報告列出完成次數、IRQ7 遞送與擱置次數、IRR7／ISR7 終值 |
| 不改 | `sb_dsp.go`、`dma8237.go`、`le_*.go`、`internal/cpu`、`internal/cpu386`、`internal/state` |

選項關閉時熱路徑只多一個指標判斷（`Step` 的進入條件與錯誤條件），依 431 §7 量測。

---

## 3. 驗收

1. 選項關閉：錨點測試常數、`StateDigest` 不變；新增測試以 §1.1 的完整埠序列跑一段合成程式，`20h`、`A1h`、`08h`、`02h` 讀回 `FFh`，DMA 埠寫入只進
   `PortLog`，沒有 IRQ7，`SoundBlasterDSP` 的半狀態與現況相同。`go test ./internal/...` 中既有測試全部不改而通過。
2. 實模式合成程式（machine 層測試，不讀原版檔），位元組在測試內組出：IVT[0Fh] 指向一段處理常式（讀 `22Eh`、寫旗標、`out 20h,20h`、`iret`），
   依 §1.1 的次序執行重設（含讀 `22Ah` 得 `AAh`）、`40h A6h`、讀改寫 `21h`／`A1h`、DMA 程式設定、`14h 03h 00h`、`sti` 後輪詢旗標。期望：
   - IRQ7 在「`22C←00` 那一步＋Δ」之後第一個允許遞送的步被接受；Δ 由測試內以 `math/big` 獨立計算，不呼叫被測函式；
   - 處理常式執行後旗標為 1、IRR7＝0、ISR7＝0、`IRQPending`＝false；
   - 通道 1 `Current[2]=4`、`Current[3]=FFFFh`、遮罩位元設起；讀 `08h` 得 bit1，再讀得 0；
   - 觀測紀錄依序為 `start`、`complete`、`irq7`、`eoi`，`complete` 的 `SHA256` 等於測試以 `Mem[0:4]` 獨立算出的值。
   時鐘變體各一例：指令數時鐘（預設）、`IRQ0Pinned`、`ClockPercent=50`、`IRQ0Every=0`、週期時鐘（`CycleClock`、`CPUHz` 任一值）。
3. V86：沿用 `v86deliver_test.go` 的 `TestV86DeliverIRQ` 夾具，IRQ7 經 IDT 閘 `0Fh` 進 ring 0，框架 9 個 dword、`EIP` 為遞送當下的 `IP`、清 IF、
   進分段模式；ring 0 的 stub 寫 `out 20h,20h` 後 `iretd` 回 V86，ISR7 清除、`CS:IP` 回原處。另一例 IVT[0Fh] 為 StubSeg 時不遞送、`IRQ7Stalls` 增加、IRR7 保留。
4. 分段模式：IRR7 成立、IF=1 時不遞送、不消耗，`segWait` 計數；回 V86 後遞送；連續一百萬步失敗即關閉（434 §5.3），錯誤訊息列出 IRQ7。
   傳輸在分段模式期間到期時照樣完成。
5. 遮罩：`21h` bit7 設起時到期後不遞送、IRR7 保留；清除後下一個允許的步遞送。ISR7 為 1 時不遞送，EOI 後遞送。
   `22Eh` 讀取在遞送前清 IRR7 時不遞送；`IRQPending` 已為真時再次完成不重拉 IRR7。
6. 優先序與單一事件：同一步 IRQ0 與 IRQ7 都到期時先送 IRQ0；同一步有回呼啟動時不送 IRQ7。
7. PIC 埠：`20h` 讀依 OCW3 回 IRR／ISR；`67h` 清 ISR7；`60h`–`66h` 不動作；ICW1、旋轉、poll、特殊遮罩各一例失敗即關閉；`A1h` 讀回寫入值。
8. DMA 與 DSP 的失敗即關閉（各一例）：模式 `58h`、`68h`；通道遮罩中啟動；`n > 計數+1`；位址或頁未寫；傳輸中改寫通道 1 或遮罩；`08h`／`09h` 寫非 0；
   DSP `1Ch`、`C6h` 序列、`74h`、`80h`、`D0h`。錯誤在寫入的那一步由 `Step` 回傳。
9. 取消與進度：傳輸一半時 `226←1`，暫存器依進度 k 更新、不設 TC、不拉 IRR7，紀錄有 `cancel`。傳輸中讀通道 1 的位址與計數得進度值。
10. 狀態檔：在 `start` 與到期之間 `SaveState`／`LoadState`、`Snapshot`／`Restore`，接著跑過完成與處理常式再多 100 步，最終 `SaveState` 位元組與不中斷的執行相同；
    選項開啟寫版本 4、關閉寫版本 3；選項與版本不一致時兩個方向都拒絕且機器不變；`StateDigest` 讀版本 4 並能偵測 IRR7 的改變。
11. 突變自檢（各至少一個測試失敗）：Δ 不乘 `D`；Δ 改用 1 微秒／步；忽略 `PITDiv` 或 `ClockPercent`；遮罩 bit7 不擋；ISR7 不擋；非特定 EOI 不清 ISR7；`67h` 不清；
    `22Eh` 不降 IRR7；`IRQPending` 已真仍重拉；TC 不設遮罩；計數不成 `FFFFh`；實體位址不含頁；分段模式下遞送；`segWait` 不計 IRQ7；同一步送兩個事件；
    略過 StubSeg 判斷；V86 下改走 IVT；重設不取消傳輸；存讀檔漏掉到期時刻；選項關閉仍寫版本 4；`LoadState` 接受不一致。
12. 觸發案例（需原版，主代理另附收據）：`strike_commander/workplace/root/`（SB16 設定）冷開機，旗標同 probe460 再加 `-sb-dma8 -sb-dma-log`，收據寫明完整旗標。兩次：
    - A（不帶 `-mode13-phase`）：預測 `start` 在第 2421489 步、Δ＝4170（若當下分頻不是 65536，照實記錄並重算）、IRQ7 未遞送、MENU 結束時 IRR7＝1、
      仍印找不到音效卡。記錄偵測期間 `PITDiv`、`0BAD` 每次呼叫的步數。
    - B（帶 `-mode13-phase`）：記錄 `start`、`complete`、`irq7` 的步數與 `CPUMode`（預期 `v86`）、處理常式進入 `B5C5:05C1` 的步數、`eoi`、
      `cs:[19C]` 的值、偵測回傳值；MENU 之後的停點、原因與總步數。停在 §2.3 的失敗即關閉時記下命令位元組與 `CS:IP`，作為後續規格的證據。
      停在別處時照實記錄，不改程式湊結果。
13. DOSBox-X 交叉對照（主代理收據項，可選）：以除錯器橋接在 `B5C5:05C1` 下中斷點，記錄 DSP `14h` 到 IRQ7 的模擬時間與 MENU 是否通過偵測。
14. 效能：選項關閉的 `BenchmarkStepRealMode` 與動工前交錯比較，不低於 97%（431 §7），記錄 load average。
15. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠；SingleStepTests 以掛載的語料實際執行、
    0 skip（掛載方式同 433 §8.2）；LE 路徑的測試（`go test ./internal/machine -run 'LE|DMA|DSP|SB16|IRQ7'`）不改而通過；`dma8237.go`、`sb_dsp.go`、`le_*.go`、
    `internal/cpu386`、`internal/state` 的 `git diff` 為空；`go test ./apps/...` 改動前後排序後逐行相同。

---

## 4. 未知與待決定事項

### 4.1 未知與風險

| 項目 | 現況 |
|---|---|
| 偵測當下的 PIT 分頻 | probe460 只記結束時的 65536；`040h`、`043h` 有寫入紀錄。Δ 依它而變，收據 A 記錄 |
| JEMM 對 IRQ7 的反射 | 435 §1.1 為靜態證據（IRQ0–7 一律反射，`0Dh`、`0Eh` 另判）；IRQ7 的執行期路徑未驗證，收據 B 記錄 |
| `20h`、`A0h` 的讀取者 | 選項開啟後 `20h` 讀值由 `FFh` 改為 IRR／ISR。JEMM 或遊戲是否讀它未知；收據記錄兩埠的讀取次數 |
| 遊戲內的播放路徑 | 靜態看到 `14h`、ADPCM `74h`–`77h`／`16h`／`17h`、`80h`、`D0h`／`D4h`（§1.2）。開啟後第一個未實作命令會失敗即關閉，用來界定下一份規格 |
| IRQ0、IRQ1 沒有 ISR | §2.6 的兩個差異。觸發案例的處理常式 `05C1` 不執行 `sti`，JEMM 的 ring 0 路徑在 `4BDE` 執行 `cli`（435 §3），目前看不到受影響的路徑 |
| 處理常式之後的 EOI 次序 | 真機上 IRQ7 的處理常式先讀 `22Eh` 再 EOI；先 EOI 再讀時這裡可能在 EOI 之後立刻再遞送一次（IRR7 尚未清）。本驅動的次序不受影響 |
| DSP 的延遲 | 真卡處理命令、重設都有微秒級延遲（DOSBox-X 重設 20 µs、忙碌週期）。本規格只算傳輸時間 |
| `22Eh` 的讀值 | 既有 `SoundBlasterDSP` 回 `80h`／0，DOSBox-X 回 `FFh`／`7Fh`（`sblaster.cpp:3178`）。bit7 的意義一致，其他位元不同，維持既有 |
| 回掃模型 | §1.4。不帶 `-mode13-phase` 時偵測仍失敗；帶了之後 SC 其他等回掃的畫面時間也會改變，既有收據不再適用 |
| 狀態檔缺口 | 選項關閉時 `picMask` 與 `SoundBlasterDSP` 不在 `SaveState` 內（現況），LoadState 後 `picMask` 回到初值。與本規格無關，另案處理 |

### 4.2 待決定（附建議）

1. **選項名稱與預設**：建議 `SetSBDMA8`／`-sb-dma8`，預設關閉（§2.1）。
2. **時基**：建議與 IRQ0 同一個機器時鐘（§2.4）。另一個做法是沿用 LE 的 1 微秒／指令，SC 的 Δ 會是 360 步、不帶 `-mode13-phase` 也能過偵測，
   但 DSP 時間比 IRQ0 快約 11.6 倍，語音字幕的起訖時間與遊戲時間對不上，等於把回掃模型的誤差搬到 DSP。
3. **SC 的回掃模型**：建議 SC 開 `-sb-dma8` 時一併開 `-mode13-phase`（觸發當下是 mode 13h），並重錄後續收據；或改用 `CRTCTiming`。由使用者決定。
4. **未實作的 DSP 命令**：建議失敗即關閉（§2.3）。只記錄會讓驅動等一個永遠不來的 IRQ，症狀離成因很遠。
5. **狀態檔格式**：建議版本 4 加第二個 gob 值（§2.9）。另一個做法是在 `internal/state` 加第三段；舊版程式讀檔時會靜默丟掉第三段，所以不採用。
6. **傳輸中遮罩或改暫存器**：建議先失敗即關閉，出現證據時照 DOSBox-X 的暫停語意另立規格。
7. **IRQ0、IRQ1 的 ISR**：建議不在本規格加（§2.6 的理由），風險由 §4.1 追蹤。

# 444：一般機器的 SB16 8 位元單次 DMA、ADPCM、靜音區塊與 IRQ7

狀態：**READY**（第 2 版，依審查修訂）
日期：2026-10-17
前置：[`190-opl-vgm-dump`](190-opl-vgm-dump.md) §8、[`190-irq0-calibration`](190-irq0-calibration.md)、[`198-dosbox-cycles`](198-dosbox-cycles.md)、
[`197-key-hold-and-typematic`](197-key-hold-and-typematic.md) §3.1、
[`009-keyboard-irq1`](009-keyboard-irq1.md)、[`012-input-keyboard-irq-and-mouse-scale`](012-input-keyboard-irq-and-mouse-scale.md)、
[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、
[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)、[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)、
[`443-segmented-32bit-code-and-stack`](443-segmented-32bit-code-and-stack.md)。LE 路徑的對照：`242`–`245`、`247`、`296`、`304`、`305`、`309`。
修訂：434 §5.3、435 §2.1 的對應條文已依本規格改寫（各註「由 444 修訂」）。本規格 §2.2、§2.3、§2.4、§2.8、§2.10、§3.4、§3.11、§4.1、§4.2 已依
[`445-legacy-sb16-dma8-pause-resume`](445-legacy-sb16-dma8-pause-resume.md) 改寫（各註「由 445 修訂」）；§5 收據不動。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/hardware/` 下的檔案。驅動程式位移為 `B5C5:0000` 起的映像位移（§1.2）。
程式位置以 commit `71d0d53` 為準。推論等級：已證實、強推論、假說、未知。

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
| 2425604、2425608 | `0A1←FF`、`021←00` | 還原遮罩（§1.2 的 `04B6`，結果仍是放行） |

### 1.2 驅動程式的偵測與播放流程

以 probe460 的讀檔紀錄從 `workplace/root/LILTRE.TRE` 位移 793944 取 4579 bytes（SHA-256
`68b0e62af80a317949501bfc446b1962f2353a41bfe2c488ded7634dab8ef949`），在容器內以 `objdump -D -b binary -m i8086` 反組譯（本機 scratchpad，不進版控）。
映像開頭是「Copyright (C) 1991,1992 Miles Design, Inc.」。下列為靜態證據，執行期次序與 §1.1 的埠寫入吻合，整體等級為強推論。

| 位移 | 內容 |
|---|---|
| `0C7B`–`0D83` | 偵測主程式。`cs:FA`–`cs:100` 依基底算出 `226h`、`22Ah`、`22Ch`、`22Eh`；`cs:102`＝IRQ（7）、`cs:104`＝DMA 通道（1）。依序：`1D9`（等 `22Ch` bit7 清除後寫 `D3h`）、`0BAD(16)`、`0283`（重設，回 0 即失敗）、`sti`、`cs:[19C]=0`、`0407(A6h)`、`0449(B5C5:05C1)`、`07EB(0000:0000, 長度 4)`，接著 `DI=22h` 次迴圈：`0BAD(1)` 後比較 `cs:[19C]==1`，成立就跳出；最後 `04B6` 還原並以 `SI`（1 或 0）回傳 |
| `05C1` | 偵測用 IRQ 處理常式：`in al,[cs:100]`（讀 `22Eh`）、`cs:[19C]=1`、`cs:[106]=0`、`out 20h,20h`（IRQ ≥ 8 時另寫 `A0h`）、`iret`。不執行 `sti` |
| `0BAD` | 等 n 次垂直回掃：`DX=[0040:0063]+6`（`3DAh`），每次先等 bit3 立起、再等 bit3 落下 |
| `0283` | `226←01`、讀 `226h` 20 次、`226←00`，最多 16 次「等 `22Eh` bit7、讀 `22Ah`」直到讀到 `AAh` |
| `0407` | 經 `0228` 寫 `40h`、TC。`0228` 以 `22Ch` bit7 輪詢，逾時則重設後重試一次 |
| `0449` | `int 21h AH=35h/25h` 存舊向量、設新向量（向量 `08h+IRQ`）；`in A1h`／`in 21h` 存起來，`AND` 掉 IRQ 位元後寫回 |
| `04B6` | `pushf; cli` 之後：`21h`、`A1h` 以 `(目前值 OR IRQ 位元) AND 存下的舊值` 寫回（舊值 bit7 為 0，所以仍放行）；`int 15h AX=1209h BX=4753h`（JEMM 私有 API），回 0 時直接寫 IVT 還原向量，否則 `int 21h AH=25h`；最後 `0530` 的 `popf` 恢復 IF |
| `07EB`、`0735` | 以 `段×16+位移` 算線性位址，`≥ A0000h` 時以 `int 67h AX=DE06h`（VCPI 取實體位址）換算；區塊長度限制在 4 KB 頁內；呼叫 `0562` 程式 DMA，再呼叫 `06F6` 送 DSP 命令與長度 |
| `0562` | `out 0Ah, ch|4`、頁暫存器（`80h+[F2h+ch]`，表為 `87h/83h/81h/82h`）、`out 0Ch`、位址兩次、計數兩次、`out 0Bh, 48h|ch`、`out 0Ah, ch` |
| `06F6`、`00EA` | 以 `cs:[114]` 查命令表 `00EA`：位移 0–3 為 `14h`、`75h`、`77h`、`17h`（第一個區塊，ADPCM 帶參考位元組），4–7 為 `14h`、`74h`、`76h`、`16h`（後續區塊）。`07DB` 在第一個區塊後把 `cs:[114]` 加 4 |
| `09B6`–`09DB` | 靜音區塊：先以 `0407` 設 TC，再送 `80h` 與兩個長度位元組，設 `cs:[106]=1`、`cs:[1D6]=1` |
| `05E8`、`0678` | 播放用 IRQ 處理常式。IRQ 為 7 時先呼叫 `0535`，回值不是 `FFFFh` 就當成假 IRQ，直接 EOI 返回 |
| `0535`–`055D` | `cs:[1D6]` 不為 0（靜音區塊）時清除並回 `FFFFh`；否則從埠 `通道×2+1`（通道 1 為 `03h`）連讀兩次組成計數。沒有先清 flip-flop |
| 其他 | `03F7` 送 `D0h`、`03A7` 送 `D4h`，另有 `D1h`、`D3h`。沒有 `1Ch`、`48h`、`C6h` 等自動初始化命令 |

所以偵測成立的條件是：DSP `14h` 傳完 4 bytes 後 IRQ7 送達 `B5C5:05C1`，而且在 34 次回掃等待結束之前。
遊戲內播放走 `14h`、ADPCM `74h`–`77h`、`16h`、`17h` 與靜音 `80h`，每個 DMA 區塊結束時處理常式要求 DMA 計數讀回 `FFFFh`（`0535`）。
`07EB` 的 VCPI 換算說明這支驅動把 DMA 位址當實體位址處理；偵測的緩衝區在 `0000:0000`，低於 `A0000h`，不經換算。
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
| 時間窗變短的原因 | `0x3DA` 走行為模型：`(portTicks>>4)&1` 決定 bit3（`machine.go` 的 `In8`），回掃週期由輪詢次數決定，一次 `0BAD(1)` 約百餘道指令 | 強推論（4115 與 34 次輪詢的量級吻合） |
| 4 bytes 的傳輸時間 | `4×90 µs＝360 µs` | 已證實（TC＝`A6h`） |
| 360 µs 換成指令數時鐘 | §2.4：`4·90·636085·315e6/(1e6·264·65536)＝4169.12`，取上整 4170 步 | 計算 |
| 開 `-mode13-phase` 的時間窗 | 每次回掃一個 `VGAFrameEvery`（預設 165000 步），34 次約 561 萬步 | 強推論（`mode13_time.go` 的 `mode13InputStatus`） |

使用者定案（2026-10-17）：本遊戲一律開 `-mode13-phase`。開了之後 IRQ7 在 4170 步左右到，落在時間窗內。

不開 `-mode13-phase` 時（§3 收據 A，作預測檢驗），IRQ7 在第 2421489＋4170 步左右才到期，晚於時間窗。這時 `04B6` 已執行 `pushf; cli`，
IMR 的 bit7 仍是 0（`(目前值|80h)&00h`，sb.tsv 最後一筆 `021←00`），IRQ7 因 IF=0 等待；`0530` 的 `popf` 恢復 IF 之前，IVT[0Fh] 已還原成 `0080:003C`。
依 §2.7 第 5 步，IRQ7 由 BIOS 預設處理常式消耗：IRR7 清除、不設 ISR7、不遞送，`IRQ7Defaulted` 加 1；偵測仍失敗。

---

## 2. 規則

### 2.1 選項

- `Machine.SetSBDMA8(on bool) error`，預設關閉。開啟的條件：`SoundBlaster()` 為真、`programLoaded` 為假、`len(m.Mem)==MemSize`。
  違反時回錯誤，狀態不變。開啟後不可關閉（再呼叫 `SetSBDMA8(false)` 回錯誤）。
- 新增存取函式 `Machine.SoundBlaster() bool`（回 `soundBlasterPresent`，與 `AdLib()` 同類）與 `Machine.SBDMA8() bool`。
- `sb8` 開啟後呼叫 `SetSoundBlaster(false)` 會 panic（該函式沒有錯誤回傳；關掉 DSP 而保留 DMA 與 IRQ7 的組合沒有意義）。
- 新狀態放在 `sb8 *sbDMA8`（nil 表示關閉）；新程式碼都在 `m.sb8 != nil` 之內。
- `cmd/probe` 加 `flag.Bool` 旗標 `-sb-dma8`（放在 `-sound-blaster` 旁，說明文字引用本規格），沒有同時給 `-sound-blaster` 時 `die`。
  另加 `-sb-dma-log <檔>`，把 §2.8 的觀測紀錄寫成 TSV，欄位與 §2.8 的表同序，第一列為欄名。
- 硬體組態固定為基底 `220h`、8 位元 DMA 通道 1、IRQ 7，與觸發案例的 `SC.CFG` 相同。其他組態屬後續規格。

### 2.2 8237 主片（通道 0–3）

沿用 `dma8237.go` 的 `DMA8237` 型別當暫存器檔（位址、計數、頁、模式、單一遮罩、全遮罩、flip-flop），在一般機器新建一個實例。
傳輸引擎另寫，不抽出 `LEOPLPorts` 的 `advanceDMA`。理由：LE 的 `advanceDMA` 綁在 `LEOPLPorts` 的欄位、1 微秒／指令的時鐘、逐 byte 的取樣信用、
PCM 擷取與 16 位元通道上，有 242–309 共十餘份 CONFORMED 收據，抽成共用的風險高於重寫一個只做單次傳輸的引擎。
`dma8237.go`、`le_*.go` 的 `git diff` 為空，LE 的測試不改。

| 埠 | 開啟時的行為 |
|---|---|
| `00h`–`07h` 寫 | `DMA8237.Out8`：依 flip-flop 寫低／高位元組，`Base` 與 `Current` 同時更新 |
| `00h`–`07h` 讀 | 走 `DMA8237.In8` 的規則：依 flip-flop 取低／高位元組並切換 flip-flop；對應 `Known` 位元缺時不切換、回 `FFh` 並累計 `SBDMAUnknownReads`（診斷，不進狀態）。傳輸進行中的通道 1 位址與計數改用 §2.5 的即時值，flip-flop 與 `Known` 規則相同，不寫回暫存器 |
| `08h` 讀 | 狀態暫存器：bit0–3 為各通道的 TC，讀後清除；bit4–7 為 0（DOSBox-X `dma.cpp:388-397`）。`DMA8237` 沒有 TC 欄位，TC 位元存在 `sbDMA8` |
| `08h` 寫（命令）、`09h` 寫（請求） | 值為 0 時接受不動作；非 0 失敗即關閉（§2.7 末） |
| `0Ah` | 單一遮罩（`DMA8237.Out8`） |
| `0Bh` | 模式：`Mode[ch]=v&FCh`（`DMA8237.Out8`） |
| `0Ch` | 清 flip-flop |
| `0Dh` | 主清除：flip-flop 清除、四通道遮罩；另清 TC 位元（`dma.cpp:336-344`） |
| `0Eh`、`0Fh` | 清全遮罩、寫全遮罩 |
| `87h`、`83h`、`81h`、`82h` | 通道 0–3 的頁暫存器，讀寫（`DMA8237` 的 `dmaPageChannel`）。`80h`、`84h`–`86h`、`88h` 等其他埠維持現況 |

傳輸進行中，下列寫入失敗即關閉：

- 遮罩通道 1：`0Ah`、`0Fh`、`0Dh` 的寫入使 `Mask` 的 bit1 由 0 變 1。
- 改寫通道 1：`02h`、`03h`、`83h`，或 `0Bh` 且 `v&3==1`。

真機會暫停或改道，屬後續規格（DOSBox-X 以 `DMA_MASKED`、`DMA_UNMASKED` 事件暫停，`sblaster.cpp:3636-3667`）。TC 自動設遮罩不算寫入。
本條只適用於未暫停的傳輸；被 `D0h` 暫停的傳輸期間的寫入見 445 §2.6（由 445 修訂）。

### 2.3 DSP 命令

一般機器沿用 `SoundBlasterDSP`（`sb_dsp.go`）的命令解析。開啟時接上 `StartDMA`、`CancelDMA`，以及本規格新增的兩個回呼。
`StartAutoDMA`、`StartSB16AutoDMA`、`Start16DMA` 維持 nil。`ControlDMA8` 由 445 §2.1 接上（由 445 修訂）。

`sb_dsp.go` 的增補（LE 行為不變）：

- 新欄位 `StartADPCMDMA func(cmd byte, n uint32) bool` 與 `StartSilence func(n uint32) bool`。
- `pending==0` 且 `v` 為 `74h`、`75h`、`76h`、`77h`、`16h`、`17h` 時：`StartADPCMDMA` 為 nil 就照現況回 false（不設 `pending`）；非 nil 才設 `pending=v`，
  收兩個長度位元組，以 `n=hi×256+lo+1` 呼叫 `StartADPCMDMA(v, n)`，回值即 `Out8` 的回值。`80h` 對 `StartSilence` 同理。
- `LEOPLPorts` 不設這兩個欄位，所以 LE 收到這些命令時與現況逐位元組相同。驗收第 15 項以測試確認。

| 命令或埠 | 行為 | DOSBox-X |
|---|---|---|
| `226h←1`、`226h←0` | 既有重設；`22Ah` 回 `AAh`。`CancelDMA`：進行中的傳輸或靜音取消（DMA 依 §2.5 的進度寫回暫存器，不設 TC），IRR7 若尚未被 CPU 接受則清除 | `DSP_DoReset` `sblaster.cpp:1489-1500`；`DSP_Reset`（1426 起）清 `pending_8bit` 但不降 PIC 線。本規格降線，與 LE 的 `CancelDMA`（`le_opl_ports.go:61-69`）一致 |
| `D1h`、`D3h` | 只改 `SpeakerOn`，無輸出 | `2309`、`2313` |
| `40h` TC | `RateNumerator=1000000`、`RateDenominator=256−TC` | `2129-2134`：`256000000/(65536−TC<<8)`，與 `1e6/(256−TC)` 相同 |
| `41h` 取樣率 | 既有：5000–45000 Hz，`RateDenominator=1` | |
| `14h` lo hi | 8 位元 PCM，`StartDMA(n)`，每個 DMA byte 一個取樣 | `2113-2120`、`1325`（`dma.total=1+len`） |
| `74h`、`75h` lo hi | 4 位元 ADPCM（`75h` 帶參考位元組），`StartADPCMDMA`，每個 DMA byte 兩個取樣 | `2180-2186`；`1193-1195`（`dma.mul=(1<<SB_SH)/2`） |
| `76h`、`77h` lo hi | 2.6 位元 ADPCM（`77h` 帶參考位元組），每個 DMA byte 三個取樣 | `2187-2193`；`1189-1191`（`/3`） |
| `16h`、`17h` lo hi | 2 位元 ADPCM（`17h` 帶參考位元組），每個 DMA byte 四個取樣 | `2212-2218`；`1185-1187`（`/4`） |
| `80h` lo hi | 靜音：不用 DMA，`n` 個取樣後拉 IRQ7 | `2219-2222`：`PIC_AddEvent(DSP_RaiseIRQEvent, 1000·(1+len)/freq)`；`3819-3823` 經 `SB_RaiseIRQ(SB_IRQ_8)` |
| `E1h` | 既有：回 `04h 05h` | `2368` |
| `22Ah` 讀 | 既有：取回覆位元組 | |
| `22Ch` 讀 | 既有：重設中回 `80h`，否則 0（不忙） | |
| `22Eh` 讀 | 既有：清 `IRQPending`，有回覆時 `80h`。開啟時另外：若 IRR7 為 1 且尚未被 CPU 接受，清 IRR7 | `3158-3163`：清 `pending_8bit` 並 `PIC_DeActivateIRQ`；`pic.cpp:95-106` 的 `lower_irq` 清 IRR |
| `SoundBlasterDSP.Out8` 對 `224h`–`22Fh` 回 false 的寫入 | 失敗即關閉，錯誤含命令位元組與 DSP 半狀態。涵蓋 `1Ch`、`C6h`、`B0h`、自動初始化 ADPCM（`7Dh`、`7Fh`、`1Fh`）及其他未實作命令。`D0h`、`D4h` 由 445 §2.3 處理，被拒絕的組合同樣失敗即關閉，訊息由 445 寫入（由 445 修訂） | |

ADPCM 的參考位元組不另計時間：與 DOSBox-X 相同，`n` 個 DMA byte 一律乘每 byte 的取樣數（`GenerateDMASound` 以 byte 計 `dma.left`，`sblaster.cpp:986-1030`）。
DOSBox-X 的 `dma.mul` 是 `(1<<14)/3` 這類截斷整數（`140`、`1214`），本規格用精確分數，差異小於萬分之一。
`48h`（設定區塊大小、不啟動傳輸）沿用既有行為。選項關閉時 `SoundBlasterDSP` 回 false 的寫入維持現況（不報錯）。不解碼 ADPCM，不輸出聲音。

### 2.4 傳輸啟動與時間

同一時間只有一個進行中的傳輸（DMA 或靜音）。

DMA 傳輸（`14h` 與 ADPCM）在下列條件全部成立時回 true；任一不成立回 false，由 §2.3 的規則失敗即關閉：

1. 沒有進行中的傳輸。被 `D0h` 暫停的傳輸先依 445 §2.7 隱式取消，再檢查第 2–8 項（由 445 修訂）。
2. 通道 1 的位址、計數兩個暫存器的高低位元組都寫過（`Known[2]==3`、`Known[3]==3`），頁暫存器寫過（`PageKnown[1]`）。
3. `Mode[1]==48h`（單次、讀、遞增、不自動初始化）。其他模式（含 `58h` 自動初始化、`68h` 遞減、需求或區塊模式）屬後續規格。
4. 通道 1 未遮罩（`Mask&2==0`）。
5. `n ≤ Current[3]+1`。`n` 較大時真機的 DMA 先到 TC、DSP 等不到資料而不發 IRQ，屬後續規格。
6. 來源範圍（§2.5 第 2 步的 `n` 個實體位址）不碰 `A0000h`–`BFFFFh`。VGA 區（含 planar 視窗）不能當 DMA 來源：`peekPhysRaw` 在 planar 下回 0，與真機不同。
7. `RateNumerator>0`、`RateDenominator>0`。
8. 下表的計算沒有溢位。

靜音（`80h`）只檢查第 1、7、8 項。

到期時刻與 IRQ0 同一個時鐘。取樣數 `N`：`14h` 為 `n`，`74h`／`75h` 為 `2n`，`76h`／`77h` 為 `3n`，`16h`／`17h` 為 `4n`，`80h` 為 `n`。

| 時鐘 | 單位 | 間隔 Δ |
|---|---|---|
| 週期時鐘（`CycleClock`） | `CPU.Cycles` | `⌈N·D·hz / R⌉`，`hz=CPUHz`，為 0 時用 `DefaultCPUHz`（同 `recalcIRQ0`，`machine.go:1318-1320`） |
| 指令數時鐘，`IRQ0Every>0` | `Steps` | `⌈N·D·IRQ0Every·315000000 / (R·264·PITDiv)⌉` |
| 指令數時鐘，`IRQ0Every==0` | `Steps` | `⌈N·D·165000·315000000·pct / (R·264·17000·100)⌉` |

- `R`、`D` 為 `SoundBlasterDSP` 的 `RateNumerator`、`RateDenominator`；`PITDiv` 為 `m.PITDivisor()`；`pct` 為 `ClockPercent()`。
- 第二列近似 `N·D/R` 秒乘 `StepsPerSecondNow()`（`opl_vgm.go:28`），第三列近似乘 `StepsPerSecondScaled()`；DSP 與 IRQ0、`-dump-vgm` 用同一個時基。
- 分頻的影響：`IRQ0Every` 未釘死時由 `recalcIRQ0` 設成 `stepsPerTick(IRQ0Base, PITDiv)`，即 `round(IRQ0Base·PITDiv/17000)`（`pit.go:121`），
  `IRQ0Every/PITDiv` 近似常數，乘積約為 11.58 M 步／秒，Δ 與分頻無關，只差四捨五入造成的幾步。`IRQ0Pinned` 時，或分頻很小使 `IRQ0Every` 被夾到
  `MinIRQ0Every`（`machine.go:1354-1356`）時，Δ 隨分頻改變。收據仍記 `PITDiv`。
- 以 128 位元整數計算（`math/bits.Mul64`、`Div64`），不用浮點。商超過 `2^63`，或開始時刻加 Δ 溢位 `uint64` 時，啟動回 false（失敗即關閉），不 panic。Δ 至少為 1。
- 開始時刻取 `Out8` 當下的 `m.Steps`（`Step` 在 `CPU.Step` 之前已遞增，等於本道指令的序號）或 `m.CPU.Cycles`。到期時刻＝開始時刻＋Δ，存成絕對值。
- 傳輸期間程式改 PIT 分頻、改時鐘比例，不重算到期時刻。
- 決定性：Δ 只由暫存器值與時鐘欄位決定，與主機時間、執行速度無關。

觸發案例：`N=4`、`D=90`、`R=1000000`、`IRQ0Every=636085`、`PITDiv=65536`，Δ＝`⌈4169.12⌉`＝4170 步。

### 2.5 傳輸完成與進度

`Step` 每一步在 `tick` 之前判斷：有進行中的傳輸，且 `m.Steps ≥ 到期`（指令數時鐘；比較時 `m.Steps` 尚未遞增，等於上一道指令的序號）或
`m.CPU.Cycles ≥ 到期`（週期時鐘），就完成。所以 Δ＝1 時，完成發生在啟動那一道的下一個 `Step` 開頭。分段模式（ring 0）下照樣完成。完成依序：

1. DMA 傳輸：通道 1 的 `Current[2] = (Current[2]+n) & FFFFh`（頁不進位，8237 位址計數器 16 位元）；`Current[3] = (Current[3]−n) & FFFFh`。
   `n == 舊 Current[3]+1` 時到 TC：計數成 `FFFFh`、設通道 1 的 TC 位元、設遮罩（`dma.cpp:513-549`）。驅動的播放處理常式以此判斷真假 IRQ（§1.2 的 `0535`）。
   靜音不碰 DMA。
2. 觀測：DMA 傳輸以 `peekPhysRaw`（`physical.go`，無 VGA latch 與讀取監看的副作用）讀 `n` 個實體位址 `Page[1]<<16 | ((Addr+i)&FFFFh)` 算 SHA-256，記進 §2.8。
   不寫任何記憶體，不改模擬狀態。DMA 不經 CPU 的分頁與 A20 閘（實體位址，強推論；驅動以 VCPI `DE06h` 自行換算，§1.2）。
3. DSP：`IRQPending` 原本為假時拉 IRQ7 線（IRR7＝1）；接著 `IRQPending=true`。原本為真時不再拉（邊緣只發生一次，`sblaster.cpp:792-819`）。
4. 傳輸結束，`DMA8Completions++`（靜音計 `SilenceCompletions`）。

進度（讀 DMA 暫存器、`CancelDMA` 用）：已傳 `k = min(n, ⌊經過·n/Δ⌋)`，經過＝現在減開始（同一單位）。即時位址＝`(起點位址+k)&FFFFh`，
即時計數＝`(起點計數−k)&FFFFh`。進度只影響讀值與取消時寫回的暫存器，不提早觸發 TC。

### 2.6 PIC 主片

選項開啟時以下列最小模型處理 IRQ7；IRQ0、IRQ1 的閘門（`machine.go` 的 `tick`、`keyboard.go` 的 `keyTick`、`keyhold.go` 的 `timedKeyTick`）不改。

| 項目 | 規則 |
|---|---|
| IMR | 沿用 `picMask`（`21h` 寫入已保存整個位元組）；IRQ7 參考 bit7。初值不變（現況為 0） |
| IRR7、ISR7 | 新增兩個位元。IRR7 由 §2.5 第 3 步拉起，由 `22Eh` 讀取（§2.3）、`CancelDMA`、CPU 接受或 BIOS 預設處理（§2.7 第 5 步）清除。ISR7 由 CPU 接受時設起，由 EOI 清除 |
| `20h` 寫，OCW2 | `20h`（非特定 EOI）：ISR7 為 1 時清除，否則不動作（IRQ0、IRQ1 沒有 ISR 模型，它們的 EOI 落在這裡）。`60h`–`67h`（特定 EOI）：`67h` 清 ISR7，其餘不動作。`00h`、`40h` 接受不動作。旋轉（bit7 為 1）與設定優先序（`C0h`–`C7h`）失敗即關閉 |
| `20h` 寫，OCW3（`v&18h==08h`） | bit1–0 為 `10b`／`11b` 時選讀 IRR／ISR；bit2（poll）或 bit6（特殊遮罩）為 1 時失敗即關閉 |
| `20h` 寫，ICW1（`v&10h`） | 失敗即關閉。JEMM 不重新程式化 PIC（435 §1.1），向量基底固定 `08h` |
| `20h` 讀 | 依 OCW3 選擇，初值選 IRR。IRR：bit0＝`irq0Pending`、bit7＝IRR7。ISR：bit7＝ISR7。其餘位元 0（`pic.cpp:337-353`） |
| `A0h` 寫 | OCW2 的 EOI（`20h`、`60h`–`67h`）與 `00h`、`40h` 接受不動作；OCW3 保存讀取選擇；ICW1、旋轉、設定優先序失敗即關閉 |
| `A0h` 讀 | 回 0（從片 IRQ 不建模） |
| `A1h` | 保存從片 IMR，讀回保存值。初值 `FFh`（等於現況的讀值）。不影響任何遞送 |

優先序：IRQ0 > IRQ1 > IRQ7，由 `Step` 內的判斷次序實現（§2.7）。

IRQ0、IRQ1 不加 ISR（§4.2 第 7 項）。已知差異：真機上 IRQ0 處理常式在 EOI 之前執行 `sti` 時 IRQ7 被 ISR0 擋住，這裡會被遞送；
ISR7 與 IRQ0 同時在服務中時，IRQ0 處理常式的非特定 EOI 在真機清 ISR0，這裡清 ISR7。

### 2.7 IRQ7 遞送

新增每步旗標 `m.eventThisStep`：`Step` 開頭清除；`tick` 的 `startCallback` 成功、週期遠呼叫、IRQ0 的 `raiseIRQ`，以及 `keyTick`、`timedKeyTick` 的 `raiseIRQ`
設起。`Step` 在鍵盤分支之後、`m.Steps++` 之前判斷 IRQ7，條件依序：

1. `m.sb8 != nil`、IRR7＝1、ISR7＝0、`picMask` bit7＝0。
2. `eventThisStep` 為假（197 §3.1 的「同一道指令只送一個事件」）。
3. `IF=1`。
4. 分段模式（`CPU.Segmented()`，ring 0）：不遞送、不消耗；IRQ7 分支自己呼叫 `segWait`（同一步已呼叫過時 `segWait` 本身會略過）。
   `segWait` 的「有待處理 IRQ」條件（`machine.go:1645`）加上「第 1 步成立的 IRQ7」，錯誤字串（`:1651`）加列 IRR7、ISR7、IMR bit7。434 §5.3 已依此修訂。
5. `ReadSup16(0x0F*4+2) == StubSeg`：視為 BIOS 預設處理常式。IRR7 清除、不設 ISR7、不遞送，`IRQ7Defaulted++`，記觀測 `defaulted`。
   V86 下 JEMM 也反射到這個 IVT 項，判斷相同。
6. `ivtGuardIRQ(0x0F)`：實模式查 433 的 IDTR；V86 呼叫 `prepareV86Delivery`（435 §2.4），失敗時 `irqErr` 由 `Step` 經 `takeIRQErr` 回傳。
7. 消耗：IRR7＝0、ISR7＝1、`IRQ7Deliveries++`、`eventThisStep=true`、記觀測 `irq7`、`raiseIRQ(0x0F)`：V86 時 `commitV86Delivery`，否則 `CPU.Interrupt(0x0F)`。

435 §2.1 的事件表已加入 IRQ7（由 444 修訂）。

觸發案例的路徑（強推論）：V86 → JEMM IDT 閘 `0Fh` → `4BDE` → `4C03`–`4C3B` 反射到 IVT[0Fh]＝`B5C5:05C1` → 處理常式在 V86 讀 `22Eh`、寫 `20h`、`iret`。

時間窗與決定性：IRQ7 的到期時刻在長度高位元組那一道指令就固定（§2.4）。之後能不能在時間窗內被接受，只取決於 IF、遮罩、ISR7、IVT[0Fh] 與分段模式，
這些都是模擬狀態，不受主機影響。

致命錯誤：§2.2–§2.6 的失敗即關閉存進黏著欄位 `m.sb8Err`。`Step` 的三處回傳（`machine.go:1479`、`1499`、`1520` 的 `if err != nil || m.pfOK`）
加上 `m.sb8Err != nil`，`err` 為 nil 時回 `sb8Err`；之後每一步在 `Step` 開頭直接回 `sb8Err`，不執行指令。只有 `Restore`、`LoadState` 清除。
發生錯誤的那一道 `out` 已執行完（`Ports`、`PortLog` 已寫）。

### 2.8 觀測紀錄

給 M3 語音字幕用的唯讀紀錄。不進狀態檔，不影響執行。

`Machine.SBDMALog []SBDMAEvent`（上限 4096 筆，滿了丟最舊的並累計 `SBDMALogDropped`），以及 `Machine.OnSBDMA func(SBDMAEvent)`（非 nil 時每筆呼叫，
與 `onOPL` 同類）。`Restore` 清空 `SBDMALog` 並把 `SBDMALogDropped` 歸零，比照 `PortLog`（`snapshot.go:249`）。欄位：

| 欄位 | 內容 |
|---|---|
| `Kind` | `start`、`complete`、`cancel`、`irq7`（CPU 接受）、`defaulted`（§2.7 第 5 步）、`eoi`（ISR7 清除）、`pause`、`resume`（445 §2.9；由 445 修訂） |
| `Step`、`Cycles` | 事件當下的 `m.Steps`、`m.CPU.Cycles` |
| `CPUMode` | `real`、`v86`、`seg`（分段模式） |
| `CS`、`IP` | 事件當下的段值與位移（`start` 為寫長度高位元組的那一道） |
| `Command`、`Format` | DSP 命令位元組；類型 `pcm8`、`adpcm4`、`adpcm4ref`、`adpcm26`、`adpcm26ref`、`adpcm2`、`adpcm2ref`、`silence` |
| `Length`、`Samples` | `n` 與 §2.4 的 `N` |
| `RateNum`、`RateDen`、`TimeConstant`、`TCKnown`、`SpeakerOn` | DSP 的取樣率設定與喇叭狀態 |
| `IRQ0Every`、`PITDiv`、`Clock` | 換算 Δ 用的時鐘欄位；`Clock` 為 `steps` 或 `cycles` |
| `Channel`、`DMAMode`、`Page`、`Addr`、`Count` | 開始時的通道 1 設定（`Count` 為程式寫入的計數）；靜音為 0 |
| `Phys` | 開始實體位址 `Page<<16|Addr`；靜音為 0 |
| `Due` | 到期時刻（單位見 `Clock`） |
| `Transferred` | `complete` 為 `n`；`cancel` 為進度 `k`；`pause`、`resume` 為凍結的進度（由 445 修訂） |
| `SHA256` | 只在 `start` 與 `complete` 各算一次（§2.5 第 2 步的方法），靜音為空；`cancel`、`pause`、`resume` 為空（由 445 修訂）。兩者不同表示傳輸期間 CPU 改寫了緩衝區 |

偵測用的 4 bytes 傳輸（`Format=pcm8`、`Length=4`、`Phys=0`）同樣會記錄。消費端先以 `Kind` 過濾，再依 `Format`、`Length`、`Phys` 與對應的讀檔紀錄過濾。
暫停過的傳輸 `start.Due ≠ complete.Due`（`complete.Due` 是 445 §2.5 平移後的到期時刻）；被隱式取消的傳輸沒有 `complete`，以與下一個 `start` 同一步的 `cancel` 結束
（由 445 修訂）。語音段落如何由多個區塊組成、字幕鍵的格式，留給 M3 的規格。

### 2.9 狀態檔

- gob 型別不變，錨點測試的常數不變（`internal/state/baseline_anchor_test.go`）。434 的 `PM` 欄位只放保護模式狀態，本規格不用它。
- 選項關閉：`SaveState` 照舊寫 `Version: 3`，與現況逐位元組相同。
- 選項開啟：`machineState.Version` 寫 4，並以同一個 `gob.Encoder` 緊接著編碼第二個值 `sbDMA8State`。`internal/state` 的機器段有 8 bytes 長度前綴，
  第二個值落在同一段內，不影響 DOS 段。
- `sbDMA8State` 是全部欄位匯出的鏡像結構（gob 忽略函式與未匯出欄位，不能直接編 `SoundBlasterDSP` 或 `DMA8237`）。自帶 `V int`（1），內容：
  `SoundBlasterDSP` 除回呼以外的全部欄位（含未匯出的命令半狀態、混音器、回覆佇列）；主片 `DMA8237` 全部欄位（含 flip-flop）與 TC 位元；`picMask`；
  從片 IMR；IRR7、ISR7；OCW3 讀取選擇（主、從）；傳輸的進行旗標、命令、`n`、`N`、開始與到期時刻、時鐘單位、開始時的通道 1 位址與計數；
  `DMA8Completions`、`SilenceCompletions`、`IRQ7Deliveries`、`IRQ7Defaulted`。
- `LoadState`：接受版本 2–4。版本 4 以同一個 `gob.Decoder` 再解 `sbDMA8State`，`V` 不認得時回錯誤；選項開關與「版本是否為 4」不一致時回錯誤，
  放在任何寫入之前（與 432 的 PM／profile 檢查相同）。所以版本 3 的檢查點不能載入到開啟選項的機器，SC 開 `-sb-dma8` 後要從冷開機重錄檢查點。
  `LegacyState` 改為 `Version < 3`。成功時清 `sb8Err`。舊版程式讀到版本 4 時因版本超出範圍而拒絕，失敗即關閉。
- `StateDigest`：接受版本 3 與 4。摘要輸入依序為：既有 JSON、`Ext` 的 SHA-256（有 `Ext` 時）、`sbDMA8State` 的 JSON 的 SHA-256（版本 4 時）。版本 3 的摘要不變。
- `picMask` 在選項關閉時不在 `SaveState` 內（現況缺口，`state.go:159-226` 沒有它；`Snapshot` 有）。本規格只在選項開啟時補存，關閉時不動，以守錨點。
- `Snapshot`／`Restore`：加入同一組欄位的深拷貝。`Restore` 在任何寫入之前判定快照與機器的 `sb8` 開關是否一致，不一致時 panic（比照 `snapshot.go:218-223`
  的 profile、`Ext` 檢查）。`SoundBlasterDSP` 的回呼在 `Restore` 後重新接到同一台機器。`Restore` 清 `sb8Err`。

### 2.10 不在範圍

聲音輸出、ADPCM 解碼與混音（另立規格）；自動初始化 DMA（`1Ch`、`C6h`、`7Dh`、`7Fh`、`1Fh`、`58h` 模式）；`D0h`／`D4h` 暫停與恢復（由 445 處理；由 445 修訂）；
16 位元 DMA 與第二組控制器；其他基底、DMA 通道、IRQ；從片 PIC 的 IRQ；PIC 重新程式化；IRQ0、IRQ1 的 ISR；DSP 命令處理時間與忙碌週期
（DOSBox-X `DSP_busy_cycle`）、重設的 20 µs 延遲、PIC 遞送延遲；8259 的假 IRQ7；dosgolem 內建 EMM（非 JEMM）下以 page frame 為 DMA 緩衝區時的對應
（DOSBox-X `dma.cpp:68-74` 的 `UpdateEMSMapping`）；`0x3DA` 的時間模型（§1.4）。

### 2.11 改動點

| 位置 | 改動 |
|---|---|
| 新檔 `internal/machine/sb_dma8.go` | `sbDMA8` 型別、`SetSBDMA8`、`SBDMA8`、`SoundBlaster`、回呼、§2.4 的時間計算、§2.5 的完成與進度、§2.6 的 PIC 埠、§2.7 的遞送、§2.8 的紀錄、`sbDMA8State` |
| `internal/machine/sb_dsp.go` | §2.3 的兩個回呼欄位與 `74h`–`77h`、`16h`、`17h`、`80h` 的參數解析；回呼為 nil 時行為不變 |
| `internal/machine/machine.go` 的 `In8`、`Out8` | `m.sb8 != nil` 時，`00h`–`0Fh`、`20h`、`A0h`、`A1h`、`81h`–`83h`、`87h` 先交給 `sbDMA8`；`22Eh` 讀取後降 IRR7；DSP 寫入回 false 時設 `sb8Err` |
| `internal/machine/machine.go` 的 `Step`、`tick`、`segWait`；`keyboard.go`、`keyhold.go` | §2.7 的 `eventThisStep`、IRQ7 分支、三處回傳與開頭檢查、`segWait` 條件與錯誤字串 |
| `internal/machine/machine.go` 的 `SetSoundBlaster` | §2.1 的 panic |
| `internal/machine/state.go`、`state_digest.go`、`snapshot.go` | §2.9 |
| `cmd/probe/main.go` | `-sb-dma8`、`-sb-dma-log`；報告列出完成次數、IRQ7 遞送與預設處理次數、IRR7／ISR7 終值、`20h`／`A0h` 的讀取次數 |
| 不改 | `dma8237.go`、`le_*.go`、`internal/cpu`、`internal/cpu386`、`internal/state` |

選項關閉時熱路徑只多指標判斷（`Step` 的進入條件與三處錯誤條件），依 431 §7 量測。

---

## 3. 驗收

1. 選項關閉：錨點測試常數、`StateDigest` 不變；新增測試以 §1.1 的完整埠序列跑一段合成程式，`20h`、`A1h`、`08h`、`02h` 讀回 `FFh`，DMA 埠寫入只進
   `PortLog`，沒有 IRQ7，`SoundBlasterDSP` 的半狀態與現況相同。`go test ./internal/...` 中既有測試全部不改而通過。
2. `SetSBDMA8` 的條件：未開 Sound Blaster、`programLoaded` 為真、`Mem` 長度不符、開啟後再關閉，各回錯誤且狀態不變；`sb8` 開啟後 `SetSoundBlaster(false)` panic。
   probe 單給 `-sb-dma8` 時 `die`；`-sb-dma-log` 的檔案第一列為 §2.8 的欄名、欄數固定。
3. 實模式合成程式（machine 層測試，不讀原版檔），位元組在測試內組出：IVT[0Fh] 指向一段處理常式（讀 `22Eh`、寫旗標、`out 20h,20h`、`iret`），
   依 §1.1 的次序執行重設（含讀 `22Ah` 得 `AAh`）、`40h A6h`、讀改寫 `21h`／`A1h`、DMA 程式設定、`14h 03h 00h`、`sti` 後輪詢旗標。期望：
   - 完成發生在 `m.Steps`（遞增前）首次 ≥ 「長度高位元組那一道的序號＋Δ」的 `Step` 開頭，IRQ7 在同一步或之後第一個允許遞送的步被接受。
     完成步號與遞送步號在測試內寫死；Δ 另以 `math/big` 獨立驗算，不呼叫被測函式；
   - 處理常式執行後旗標為 1、IRR7＝0、ISR7＝0、`IRQPending`＝false；
   - 通道 1 `Current[2]=4`、`Current[3]=FFFFh`、遮罩位元設起；不先清 flip-flop 連讀兩次 `03h` 得 `FFh`、`FFh`（同 `0535`）；讀 `08h` 得 bit1，再讀得 0；
   - 觀測紀錄依序為 `start`、`complete`、`irq7`、`eoi`，`start`、`complete` 的 `SHA256` 等於測試以 `Mem[0:4]` 獨立算出的值。
   時鐘變體各一例：指令數時鐘（預設）、`IRQ0Pinned`、`IRQ0Every=0` 且 `ClockPercent=50`、週期時鐘（`CPUHz` 任一值與 `CPUHz=0`）。
4. ADPCM 與靜音：`74h`、`75h`、`76h`、`77h`、`16h`、`17h` 各一例，Δ 依 2、3、4 倍取樣數；`80h` 一例，不改 DMA 暫存器、`Phys=0`、到期後拉 IRQ7。
   傳輸進行中（未暫停）再送任何啟動命令失敗即關閉；暫停中的情形見 445 §2.7（由 445 修訂）。
5. V86：沿用 `v86deliver_test.go` 的 `TestV86DeliverIRQ` 夾具，IRQ7 經 IDT 閘 `0Fh` 進 ring 0，框架 9 個 dword、`EIP` 為遞送當下的 `IP`、清 IF、
   進分段模式；ring 0 的 stub 寫 `out 20h,20h` 後 `iretd` 回 V86，ISR7 清除、`CS:IP` 回原處。
6. BIOS 預設處理：IVT[0Fh] 為 StubSeg 時（實模式與 V86 各一例）不遞送、IRR7 清除、ISR7 為 0、`IRQ7Defaulted` 加 1、紀錄有 `defaulted`。
   另以 §1.2 `04B6` 的形狀（`pushf; cli`、還原 IMR 為放行、改寫 IVT[0Fh] 為 StubSeg、`popf`）且到期落在 `cli` 之後，結果為 `defaulted`。
7. 分段模式：IRR7 成立、IF=1 時不遞送、不消耗，`segWait` 計數（只有 IRQ7 待處理時也計）；回 V86 後遞送；連續一百萬步失敗即關閉，錯誤訊息列出 IRQ7。
   傳輸在分段模式期間到期時照樣完成。
8. 遮罩：`21h` bit7 設起時到期後不遞送、IRR7 保留；清除後下一個允許的步遞送。ISR7 為 1 時不遞送，EOI 後遞送。
   `22Eh` 讀取在遞送前清 IRR7 時不遞送；`IRQPending` 已為真時再次完成不重拉 IRR7。
9. 優先序與單一事件：同一步 IRQ0 與 IRQ7 都可送時先送 IRQ0；同一步有回呼啟動或週期遠呼叫時不送 IRQ7。
10. PIC 埠：`20h` 讀依 OCW3 回 IRR／ISR；`67h` 清 ISR7；`60h`–`66h`、`00h`、`40h` 不動作；ICW1、旋轉、設定優先序、poll、特殊遮罩各一例失敗即關閉；`A1h` 讀回寫入值。
11. DMA 與 DSP 的失敗即關閉（各一例）：模式 `58h`、`68h`；通道遮罩中啟動；`n > 計數+1`；位址或頁未寫；來源碰到 `A0000h`–`BFFFFh`；傳輸中經 `0Ah`、`0Fh`、`0Dh`
    遮罩通道 1；傳輸中寫 `02h`、`03h`、`83h`、`0Bh`（通道 1）；`08h`／`09h` 寫非 0；DSP `1Ch`、`C6h` 序列、`7Dh`、無傳輸或靜音中的 `D0h`（由 445 修訂）；Δ 的商超過 `2^63`（以極端 `IRQ0Pinned` 構造）。
    錯誤在寫入的那一步由 `Step` 回傳，之後每一步回同一錯誤且不執行指令，`Restore` 後清除。傳輸中遮罩通道 0、2、3 或改寫它們的暫存器不報錯。
12. 取消與進度：傳輸一半時 `226←1`，暫存器依進度 k 更新、不設 TC、不拉 IRR7，紀錄有 `cancel`。傳輸中讀通道 1 的位址與計數得進度值，flip-flop 照常切換，
    未寫過的暫存器回 `FFh` 且不切換。
13. 時刻固定：傳輸中改 PIT 分頻、改 `IRQ0Every`（釘死）不改到期時刻。
14. 狀態檔：在 `start` 與到期之間 `SaveState`／`LoadState`、`Snapshot`／`Restore`，接著跑過完成與處理常式再多 100 步，最終 `SaveState` 位元組與不中斷的執行相同；
    選項開啟寫版本 4、關閉寫版本 3；選項與版本不一致時兩個方向都拒絕且機器不變；`sbDMA8State.V` 不認得時拒絕；`Restore` 開關不一致時 panic 且機器不變；
    `StateDigest` 讀版本 4 並能偵測 IRR7 的改變；`Restore` 後 `SBDMALog` 為空。紀錄超過 4096 筆時保留最新 4096 筆、`SBDMALogDropped` 正確。
15. LE 不變：`LEOPLPorts` 收到 `74h`、`80h` 時與改動前一樣回 false；LE 路徑的測試（`go test ./internal/machine -run 'LE|DMA|DSP|SB16|IRQ7'`）不改而通過。
16. 突變自檢（各至少一個測試失敗）：Δ 不乘 `D`；Δ 改用 1 微秒／步；ADPCM 不乘取樣倍數；靜音動到 DMA；忽略 `PITDiv`（只在 `IRQ0Pinned` 的例子有效）；
    忽略 `ClockPercent`（只對 `IRQ0Every==0` 那列有意義）；傳輸中改分頻會重算到期；比較用遞增後的 `Steps`；遮罩 bit7 不擋；ISR7 不擋；非特定 EOI 不清 ISR7；
    `67h` 不清；`22Eh` 不降 IRR7；`IRQPending` 已真仍重拉；TC 不設遮罩；計數不成 `FFFFh`；即時讀值不切換 flip-flop；實體位址不含頁；分段模式下遞送；
    `segWait` 不計 IRQ7；同一步送兩個事件；StubSeg 時仍遞送或改為擱置；V86 下改走 IVT；重設不取消傳輸；`sb8Err` 不黏著；存讀檔漏掉到期時刻；
    選項關閉仍寫版本 4；`LoadState` 接受不一致；`Restore` 不清 `SBDMALog`。
17. 觸發案例（需原版，主代理另附收據）：`strike_commander/workplace/root/`（SB16 設定）冷開機，旗標同 probe460 再加 `-sb-dma8 -sb-dma-log`，`-steps` 給足
    到 MENU 結束之後（預設 20,000,000 可能不夠），收據寫明完整旗標與步數上限。
    - B（主路線，帶 `-mode13-phase`）：開 `-mode13-phase` 後回掃等待變長，步號與 probe460 不同，不能沿用 2421489。記錄偵測期間的 `PITDiv` 與 `IRQ0Every`、
      `start`、`complete`、`irq7` 的步數與 `CPUMode`（預期 `v86`）、處理常式進入 `B5C5:05C1` 的步數、`eoi`、`cs:[19C]` 的值、偵測回傳值；
      `20h`、`A0h` 的讀取次數；MENU 之後的停點、原因與總步數。停在 §2.3 的失敗即關閉時記下命令位元組與 `CS:IP`，作為後續規格的證據。
      停在別處時照實記錄，不改程式湊結果。
    - A（預測檢驗，不帶 `-mode13-phase`）：預測 `start` 在第 2421489 步、Δ＝4170（`PITDiv` 不同時照實記錄並重算）、紀錄有 `defaulted` 而沒有 `irq7`、
      仍印找不到音效卡。記錄 `0BAD` 每次呼叫的步數、`cli` 與 `popf` 的步數、IVT[0Fh] 還原的步數。
18. DOSBox-X 交叉對照（主代理收據項，可選）：以除錯器橋接在 `B5C5:05C1` 下中斷點，記錄 DSP `14h` 到 IRQ7 的模擬時間與 MENU 是否通過偵測。
19. 效能：選項關閉的 `BenchmarkStepRealMode` 與動工前交錯比較，不低於 97%（431 §7），記錄 load average。
20. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠；SingleStepTests 以掛載的語料實際執行、
    0 skip（掛載方式同 433 §8.2）；`dma8237.go`、`le_*.go`、`internal/cpu386`、`internal/state` 的 `git diff` 為空；`go test ./apps/...` 改動前後排序後逐行相同。

---

## 4. 未知與已決定事項

### 4.1 未知與風險

| 項目 | 現況 |
|---|---|
| 偵測當下的 PIT 分頻 | probe460 只記結束時的 65536；`040h`、`043h` 有寫入紀錄。未釘死時 Δ 幾乎不受影響（§2.4），收據仍記錄 |
| JEMM 對 IRQ7 的反射 | 435 §1.1 為靜態證據（IRQ0–7 一律反射，`0Dh`、`0Eh` 另判）；IRQ7 的執行期路徑未驗證，收據 B 記錄 |
| `20h`、`A0h` 的讀取者 | 選項開啟後 `20h` 讀值由 `FFh` 改為 IRR／ISR。審查以靜態反組譯確認 JEMM（`4E54`、`58FD`、`5907`、`4E6F`）與驅動（`05C1`、`05E8`、`0668`、`06E6`）只寫不讀這兩埠；MENU、STRIKE 本身未知，收據記錄兩埠的讀取次數 |
| 遊戲內的播放路徑 | `14h`、ADPCM、`80h` 已納入。觸發案例在第 131427746 步送 `D0h`（§5），`D0h`／`D4h` 由 445 接上；idle、靜音中與改寫 DMA 後的組合仍失敗即關閉（由 445 修訂） |
| IRQ0、IRQ1 沒有 ISR | §2.6 的兩個差異。偵測的處理常式 `05C1` 與播放的 `05E8`／`0678` 都不執行 `sti`，JEMM 的 ring 0 路徑在 `4BDE` 執行 `cli`（435 §3），目前看不到受影響的路徑 |
| 處理常式之後的 EOI 次序 | 先 EOI 再讀 `22Eh` 時，這裡可能在 EOI 之後立刻再遞送一次（IRR7 尚未清）。本驅動先讀後 EOI，不受影響 |
| ADPCM 參考位元組的時間 | 真卡上參考位元組只產生一個取樣；本規格照 DOSBox-X 以每 byte 取樣數計，`75h` 等每區塊多算 1–3 個取樣的時間 |
| DSP 的延遲 | 真卡處理命令、重設都有微秒級延遲（DOSBox-X 重設 20 µs、忙碌週期）。本規格只算傳輸時間 |
| `22Eh` 的讀值 | 既有 `SoundBlasterDSP` 回 `80h`／0，DOSBox-X 回 `FFh`／`7Fh`（`sblaster.cpp:3178`）。bit7 的意義一致，其他位元不同，維持既有 |
| `-mode13-phase` 的代價 | SC 其他等回掃的畫面時間會變，430–443 的收據與檢查點不再適用；版本 3 的檢查點本來也不能載入到開啟 `-sb-dma8` 的機器 |
| 狀態檔缺口 | 選項關閉時 `picMask` 與 `SoundBlasterDSP` 不在 `SaveState` 內（現況），LoadState 後 `picMask` 回到初值。與本規格無關，另案處理 |

### 4.2 已決定（2026-10-17）

1. **`-sb-dma8` 預設關閉**（主代理決定）。這是共用層，開啟後 `20h`、`A1h`、DMA 埠的讀值與 IRQ 遞送改變，`apps/` 下的既有收據可能改變。理由同 439、441。
2. **時基用機器時鐘**（主代理決定）。DSP 與 IRQ0、`-dump-vgm` 同一個時基，語音字幕的起訖時間才與遊戲時間一致。LE 的 1 微秒／指令會讓 DSP 時間比 IRQ0 快約 11.6 倍，
   等於把回掃模型的誤差搬到 DSP。
3. **本遊戲一律開 `-mode13-phase`**（使用者定案）。時間窗變短的真因是 `0x3DA` 的行為模型，修在回掃模型這一側；收據 B 為主路線，A 保留作預測檢驗。
4. **未實作的 DSP 命令失敗即關閉**（主代理決定）。只記錄會讓驅動等一個永遠不來的 IRQ，症狀離成因很遠。
5. **狀態檔版本 4 加第二個 gob 值**（主代理決定）。gob 型別與錨點不動；在 `internal/state` 加第三段的話，舊版程式讀檔時會靜默丟掉它，版本 4 則讓舊版程式拒絕。
6. **傳輸中遮罩通道 1 或改寫其暫存器失敗即關閉**（主代理決定）。暫停語意要等出現證據再照 DOSBox-X 另立規格，避免沒有原版證據的行為。
   `D0h` 暫停之後的寫入改由 445 §2.6 允許；未暫停時本條不變（由 445 修訂）。
7. **IRQ0、IRQ1 不加 ISR**（主代理決定）。BIOS 的 `int 08h` stub（`machine.go` 的 `initVectors`，`biosTimerOff`）沒有 EOI，加了之後 IRQ0 在第一次之後永遠被擋；
   差異由 §4.1 追蹤。
8. **範圍納入 ADPCM 與 `80h`**（主代理決定）。驅動的播放路徑靜態上只用這些單次命令（§1.2），一次納入可以讓遊戲內播放走到下一個未知點，觀測紀錄也涵蓋語音。
9. **IVT[0Fh] 為 StubSeg 時視為 BIOS 預設處理常式**（依審查）。擱置會讓 IRQ7 在之後某個不相干的時刻送進新裝的處理常式；消耗 IRR7 與真機 BIOS 的預設行為較接近，
   收據 A 的結果也因此可預測。

## 5. 實作收據（2026-10-17）

commit：`021ee31`。

- §3 單元測試：容器內 `go test -count=1 ./internal/... ./cmd/probe ./session ./oracle` 13 個套件 ok；SingleStepTests 316 PASS、0 SKIP；`gofmt`、`go vet` 無輸出。
  突變 31 條各至少一個測試失敗（「LoadState 接受不一致」由 `sb8Apply` 的 nil 指標 panic 抓到，不是斷言）。`dma8237.go`、`le_*.go`、`internal/cpu`、`internal/cpu386`、
  `internal/state` 無改動；LE 收到 `74h`–`77h`、`16h`、`17h`、`80h` 照舊回 false。`go test -v ./apps/...` 與改動前相同（只差一個 map 走訪順序造成的 Skip 訊息）。
- 效能（`BenchmarkStepRealMode`，選項關閉，`--cpuset-cpus 6` 綁單核，交錯新舊各 5 次，load average 約 3）：最快值改動前 27.91、改動後 28.62 ns/op，約 97.5%。
- 實作與規格字面的差異：`228h`、`229h` 是 OPL 別名（190 §8），失敗即關閉的判斷放在別名重對映之後，所以不會停；§2.5「Δ＝1」的舉例與正式規則不符，照正式規則實作
  並寫死步號；`segWait` 在 IRQ7 條件成立後、檢查 IF 前呼叫（IF=0 的步也歸零計數）。`OnSBDMA` 不為 nil 時不追加 `SBDMALog`（同 `onOPL`）。
- 觸發案例（`strike_commander` 的 `workplace/logs/probe470.log`、`workplace/scratch/p470/sbdma.tsv`）：SB16 設定（`-ma -ds`）、`-mode13-phase -sound-blaster -adlib -sb-dma8`
  冷開機。偵測：第 4938225 步 `14h` 長度 4、第 4942395 步完成並遞送 IRQ7、驅動在 `B5C5:05E3` 送 EOI；第二次偵測同樣通過。MENU 播 59750 bytes 取樣
  （第 16823080 步至 79099541 步，完成時遞送 IRQ7）。第 125256614 步開始 18430 bytes，第 131427746 步驅動送 `D0h`（暫停），本規格不涵蓋而失敗即關閉，
  由規格 445 處理。`20h` 被讀 1926463 次、`A0h` 0 次（讀者未查）。

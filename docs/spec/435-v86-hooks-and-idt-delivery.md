# 435 — V86 事件經 IDT 遞送、回 V86 與服務層在 V86 下的規則

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移；`X` 為 XMS 區塊線性位址（本案 `110000h`）。
JEMM 的路徑分析出自 `strike_commander` 的 `workplace/re/jemm-api/img.dis` 靜態反組譯（本機），推論等級逐項標示。

---

## 1. 為什麼要做

436 的觸發案例停在 SCCD 映像 `0x2BE`（`0110:02BE`）的 `INT 2Fh`（`AX=4300h`）：V86 的第一個需要 IDT 的事件。之後 SCCD 會以 `INT 21h` 執行 `MENU.EXE`
等程式（`docs/re/002`），全部在 V86 下經 JEMM。

### 1.1 JEMM 的 IDT 與處理流程

| 項目 | 內容 | 等級 |
|---|---|---|
| IDT | 基底 `X+1530h`、界限 `7FFh`；256 項皆為 386 中斷閘（type `Eh`、DPL 0、選擇子 `18h`），偏移 `47DEh+4n`（entry_stage1 `4072`–`4085` 的迴圈；第 104610 步傾印） | 已證實 |
| 小樁 | `47DE+4n` 為 `E8 rel16 CF`：`call 4BDE` | 已證實 |
| 進入框架 | 從 V86 進入時換到 `SS0:ESP0=50h:1400h`（TSS `10h`），`SS` 選擇子 `50h` 為 B=0，以 16 位元 `SP` 推入 32 位元閘的 9 個 dword（`GS`、`FS`、`DS`、`ES`、`SS`、`ESP`、`EFLAGS`、`CS`、`EIP`）加錯誤碼；`4BDE` 的 `cmp sp,13D6h`（`4BE1`）區分有無錯誤碼：無錯誤碼進 `4BDE` 時 `SP=13DAh`、有錯誤碼 `13D6h` | 強推論（數值與程式常數吻合） |
| V86 的 `INT n` | IOPL=3，閘 DPL 0 < CPL 3，觸發 `#GP(n*8+2)`；JEMM 在 `4FDC` 解碼 `CD nn`，於 `4EE8`–`4F5C` 改 V86 堆疊（推 `FLAGS`、`CS`、`IP`，清 IF、TF）、讀 IVT[nn] 寫回框架，`4F60` 以空選擇子載入 `DS`／`ES`／`GS`／`FS`、`popad`、`iretd` 回 V86 | 強推論 |
| JEMM 自己處理的 INT | `15h`（`57CD`：`AX=1209h` 私有 API、`AH=87h`／`88h`）、`67h`（`65DB` EMS／VCPI）、`INT 3`／`INT 1` 走溫開機；其他（含 `21h`、`2Fh`）反射 | 強推論 |
| IRQ | PIC 不重新程式化，IRQ0–7 在向量 `08h`–`0Fh`；除 `0Dh`、`0Eh` 以錯誤碼旗標區分外，一律反射 IVT（`09h` 先 `in al,60h` 檢查 Ctrl-Alt-Del 等） | 已證實（靜態）／強推論（執行期） |

所以 V86 下 dosgolem 要提供：V86 事件經 IDT 遞送到 ring 0（含堆疊切換與框架），以及 ring 0 的 `iretd` 回 V86。服務層則要在 V86 下仍能被觸發：
遊戲的 `INT 21h` 經 JEMM 反射到 IVT[21h]＝`0080:0400`（StubSeg 的 `CD F2 CF`），這個 `CD F2` 若再經 IDT，JEMM 會反射到 IVT[F2h]（同樣是 StubSeg 的
`CD F2 CF`），永遠不會到服務層。

## 2. 遞送機制

### 2.1 事件種類

| 事件 | 來源 | 向量 | 錯誤碼 | DPL 檢查 |
|---|---|---|---|---|
| 軟體中斷 | V86 的 `CD nn`（§4 的服務層例外除外） | nn | 無 | 有：IOPL<3 改送 `#GP(0)`（`cpu.cpp:1302-1309`）；閘 DPL < 3 改送 `#GP(nn*8+2)`（`1317-1320`） |
| `INT 3`、`INTO` | `CC`、`CE` | 3、4 | 無 | 同上（軟體中斷） |
| 外部中斷 | IRQ0（`tick`）、IRQ1（`keyTick`、`timedKeyTick`） | `08h`、`09h` | 無 | 無 |
| CPU 例外 | V86 的除以零（0）、`BOUND` 越界（5）、`#GP(0)`（13，見下） | 0、5、13 | 13 有 | 無 |

V86 下改送 `#GP(0)` 的情況（取代 436 §4 的「失敗即關閉」）：IOPL<3 的 `cli`、`sti`、`pushf`、`popf`、`iret`、`INT n`；`hlt`；特權指令（`mov crN`、`mov drN`、
`lgdt` 等 433 以 `Sys.User()` 判定者）；I/O 點陣圖拒絕。`#GP` 的框架 `EIP` 是該指令起點（`cpu.cpp:1181`；`core_normal.cpp:115-116`）；軟體中斷成功時是下一道。
例外與 IRQ 不看 IOPL、不看閘 DPL。

### 2.2 遞送步驟（V86 → ring 0，照 DOSBox-X `CPU_Interrupt` 保護模式分支，`cpu.cpp:1300-1475`）

由 Machine 執行（核心以哨兵 `cpu.ErrV86Event` 回報，附 `V86Event{Kind, Vector, HasError, ErrorCode, FrameEIP}`；核心把 `IP` 設回本道指令起點，不改其他狀態）。
第 1–4 步只讀，失敗時狀態不變並失敗即關閉：

1. IDT：`vector*8+7 ≤ IDTR.limit`，否則失敗（真機 `#GP(vector*8+2+EXT)`，屬 S6）。讀閘（`ReadSup32`×2）。
2. 閘：386 中斷閘（type `Eh`）或 386 陷阱閘（type `Fh`）、present；軟體中斷另做 §2.1 的 DPL 檢查（檢查失敗改遞送 `#GP`，回到第 1 步，向量 13）。
   286 閘、任務閘失敗即關閉。
3. 目標碼段：閘選擇子經 434 的描述元解碼；須為非一致碼段、present、DPL 0、D=0（D=1 屬 32 位元碼段，後續規格）。
4. 新堆疊：`TR` 有效；讀 `SS0`、`ESP0`（`TR.base+8`、`+4`）；`SS0` 非空、`TI=0`、在 GDT 界限內、RPL＝DPL＝0、可寫資料段、present（`cpu.cpp:1365-1394`）。
   以 `SS0` 的快取與 B 位元算出推入後的 `SP`：B=0 時以 16 位元 `SP` 環繞。推入的 9 或 10 個 dword 所在頁以 `peekXlatPage` 探測首尾（`m.paging` 為真時），
   並依 434 §4.1 檢查 `SS0` 界限（本案向下展開、界限 0）；任一失敗即關閉。
5. 進入：`SetEFlagsHi`、核心旗標的修改都在推入之後。依序：
   - 以 `SS0` 為堆疊（`SetSegCache(SS)`、`Seg[SS]=SS0`、`SP`＝`ESP0` 低 16 位元，B=0）；
   - 推入（32 位元，`WriteSup`）：`GS`、`FS`、`DS`、`ES`、舊 `SS`、舊 `ESP`、`EFLAGS`（完整 32 位元，含 `VM=1`）、舊 `CS`、`FrameEIP`、有錯誤碼時錯誤碼；
   - `DS`、`ES`、`FS`、`GS` 載入空選擇子（快取不可用）；
   - `CS`＝閘選擇子（RPL 改為 0）、快取取自描述元；`IP`＝閘偏移低 16 位元（高 16 位元非 0 失敗即關閉）；
   - 清 `VM`、`TF`、`NT`、`RF`；中斷閘另清 `IF`；
   - `SetSegmented(true)`、`SetPMCPL(0)`。
6. 這一步回 nil。

`Ticks++`、`onTick` 在 IRQ0 被接受、進入第 1 步之前執行（同現行 `tick` 在 `Interrupt(0x08)` 前的順序）；遞送失敗即關閉時 IRQ0 已被消耗，
在診斷中寫明。`kbdData` 同理。

### 2.3 回 V86：ring 0 的 `iretd`

分段模式、CPL 0、`NT=0` 的 `66 CF`，彈出的 `EFLAGS` 有 `VM=1` 時（照 `cpu.cpp:1600-1640`）：

1. 讀（不改狀態）`SS:SP` 起 9 個 dword：`EIP`、`CS`、`EFLAGS`、`ESP`、`SS`、`ES`、`DS`、`FS`、`GS`；`EIP` 高 16 位元非 0 失敗即關閉（DOSBox-X 截為
   低 16 位元；刻意從嚴）；`EFLAGS` bit 18–31 非 0 失敗。
2. `SetSegmented(false)`、清六個段快取、`SetEFlagsHi(VM 與 RF)`（DOSBox-X `FMASK_ALL|FLAG_VM`）、核心 `SetFlags`（低 16 位元）、`IP`、
   `Seg[CS/SS/ES/DS]`、`FSeg`、`GSeg`（段值）、`SP`＝彈出的 `ESP` 低 16 位元，`Hi[SP]`＝高 16 位元。CPL 由 `VM` 推得為 3。
3. 由核心回哨兵 `cpu.ErrV86Return`，Machine 執行第 2 步（`SetEFlagsHi` 要先 `SetSegmented(false)`，434 §3.2）。

分段模式 `NT=0` 的 `iret`／`iretd` 若彈出 `VM=0`（同特權或外層返回）：失敗即關閉（JEMM 的 ring 0 巢狀路徑 `4C3F` 屬後續）。

## 3. 分段模式下的事件

分段模式（ring 0）期間：

- IRQ 照 434 §5.3 延後（`IF=0` 不報錯；`IF=1` 且有待處理 IRQ 一百萬步失敗即關閉）。JEMM 的處理常式在 `4BDE` 執行 `cli`，ring 0 期間通常 `IF=0`。
- 軟體中斷、CPU 例外照 434 失敗即關閉（保護模式內的 IDT 遞送與 `#PF` 屬 S6）。

## 4. 服務層在 V86 下的規則

V86 下執行 `CD nn` 時，在 IDT 遞送之前先判斷是否由服務層處理：**本道指令的 `CS` 等於 `StubSeg`，且 IVT[nn]（線性讀）的段值為 `StubSeg`
或 `fromOwnStub` 成立**（同 `internal/dos` 的 `handle` 現行判斷，`dos.go` 約 686–729 行）。成立時照實模式路徑呼叫 `IntHook`，不經 IDT；
不成立時走 §2。

理由：StubSeg 代表 BIOS／DOS 的實作。真機上 BIOS 與 DOS 的程式碼在 V86 下原生執行，dosgolem 以 `CD nn` 陷入代替那些程式碼，所以 StubSeg 內的
陷入不經 JEMM。遊戲自己的 `INT 21h`（`CS` 不是 StubSeg）照真機經 IDT 與 JEMM，JEMM 反射到 IVT[21h]＝`0080:0400` 後，`CD F2` 在 StubSeg 內由服務層處理。
IVT 被遊戲掛鉤（段值不是 StubSeg）時，StubSeg 內的 `CD nn` 也經 IDT（例如 BIOS 計時器 stub `0080:0420` 內的 `INT 1Ch` 在遊戲掛了 1Ch 時）。

| 項目 | V86 下的行為 |
|---|---|
| 服務層讀寫客體 | 照現行 `SvcAddr`（轉換開啟時為線性位址）；寫入 V86 堆疊上的 `FLAGS`（`fixStackedCF`）照舊 |
| 阻塞型服務（`int 21h AH=01/07/08` 等） | 照舊以 `Rewind` 退回 `CD` 起點；下一步重新判斷，仍由服務層處理 |
| `keyTick`、`timedKeyTick` 讀 IVT[09] | 由 `ReadPhys16` 改為線性讀（與 `handle` 一致；V86 開分頁時兩者可能不同） |
| IRQ0、IRQ1 | 依 §2 經 IDT；`ivtGuardIRQ` 的 V86 分支（436）改為遞送 |
| `startCallback`、`periodic` 的 `FarCall` | V86 下照常執行（以 V86 的 `SS:SP` 推入，段值×16）；分段模式下照 434 延後。436 §4 的 V86 延後取消 |
| `Interrupt`、`FarCallWithReturn` 第二道防線 | `Interrupt` 在 V86 仍失敗即關閉（V86 不得走 IVT 直送）；`FarCallWithReturn` 的 V86 條件移除 |

## 5. 核心與 Machine 的改動點

| 位置 | 改動 |
|---|---|
| `internal/cpu/sys.go` 的 `ivtGuard` | V86 分支：先判斷 §4 的服務層條件（只在 `doInt`），成立則放行到 `IntHook`；否則回 `ErrV86Event`（軟體中斷，含 §2.1 的 IOPL 與 DPL 檢查交給 Machine）。`divideError`、`divideFault`、`BOUND` 回例外事件 |
| `internal/cpu/v86.go` 與 `ops.go`、`ops386.go`、`ops186.go` 的 V86 失敗點（436 §4） | 改為回 `ErrV86Event`（`#GP(0)`，`FrameEIP`＝指令起點） |
| `internal/cpu/ops386.go` 的 `66 CF`（分段模式、`NT=0`） | §2.3 |
| `internal/machine/handoff.go` 的 `finishStep` | 處理 `ErrV86Event`、`ErrV86Return` |
| `internal/machine/sysregs.go` 的 `ivtGuardIRQ` | V86 分支改為「交給 §2 遞送」：回傳一個不呼叫 `Interrupt` 的指示，`tick`／`keyTick`／`timedKeyTick` 改呼叫 Machine 的遞送函式 |
| `internal/machine/machine.go` 的 `tick` | 回呼閘門改為 `!segmented` |
| `internal/machine/keyboard.go`、`keyhold.go` | IVT[09] 改線性讀 |

## 6. 不在範圍

保護模式內（ring 0）的例外與 IDT 遞送、`#PF`、`#DB`（S6）；ring 0 巢狀返回（`iretd` 彈出 `VM=0`）；任務閘；32 位元碼段；VCPI；回實模式（S9）；
V86 的 x87；中斷遮蔽（436 §4 的刻意差異維持）。

## 7. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 無改動；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變。
2. 遞送：合成 GDT、IDT、TSS（仿 §1.1）：
   - V86 `CD nn`、閘 DPL 0：送 `#GP(nn*8+2)` 到向量 13，框架 `EIP` 為 `CD` 起點、錯誤碼正確、`SP` 在推入後為 `ESP0-40`；
   - 閘 DPL 3 的 `CD nn`：直接送向量 nn，框架 `EIP` 為下一道；IOPL<3 的 `CD nn` 送 `#GP(0)`；
   - IRQ0、IRQ1：送向量 8、9，無錯誤碼，`IF` 被清（中斷閘）、陷阱閘不清；
   - 除以零送向量 0；
   - 框架 9 個 dword 的值與順序、`DS`／`ES`／`FS`／`GS` 為空選擇子、`CS`／`IP`／`CPL`、`segmented=true`、`VM=0`；
   - 第 1–4 步每個失敗條件各一例，失敗時狀態不變。
3. 回 V86：ring 0 `iretd` 彈出 `VM=1` 的框架後，`CS:IP`、`SS:SP`、四個資料段、旗標（含 IOPL）還原，`segmented=false`；`EIP` 高半非 0 失敗；彈出 `VM=0` 失敗。
4. 往返：合成「V86 `CD 21` → ring 0 處理常式模擬 JEMM 反射（改 V86 堆疊、讀 IVT、`iretd`）→ StubSeg `CD F2` → 服務層 → `CF`」，驗證服務層只被呼叫一次、
   回到 `CD 21` 的下一道，`CF` 回寫正確。
5. 服務層條件：`CS`≠StubSeg 時不呼叫 `IntHook`；IVT[nn] 不指 StubSeg 時 StubSeg 內的 `CD nn` 經 IDT；`fromOwnStub` 情形由服務層處理；阻塞服務的 `Rewind` 重跑仍由服務層處理。
6. 回呼：V86 下 `startCallback`、`periodic` 照常執行；分段模式下延後不消耗。
7. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7）；`BenchmarkStepV86Paged` 不低於 11.58 M 道／秒。照實記錄。
8. 突變自檢：服務層條件不看 `CS`、軟體中斷不做 DPL 檢查、框架少推資料段、`FrameEIP` 對 `#GP` 用下一道、`iretd` 回 V86 不還原資料段、
   IRQ0 遞送不清 `IF`，各至少一個測試失敗。
9. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
   - 第一個事件 `INT 2Fh`：記錄進入 `18h:4812`（向量 13 小樁）時的 `SP`（預期 `13D8h`，即進 `4BDE` 時 `13D6h`）、框架內容與錯誤碼 `17Ah`；
   - JEMM 反射後回到 V86 的 `0080:00BC`（IVT[2Fh]），`CD 2F` 在 StubSeg 內由服務層處理；
   - 繼續執行，照實記錄最後停點、原因、步數，以及途中 `EXEC` 了哪些程式（`MENU.EXE` 等）。
10. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

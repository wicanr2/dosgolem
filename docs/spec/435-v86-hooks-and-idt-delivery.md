# 435 — V86 事件經 IDT 遞送、回 V86 與服務層在 V86 下的規則

狀態：**READY**（第 3 版，依第 2 輪複審修正）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、[`436-tss-task-return-v86-entry`](436-tss-task-return-v86-entry.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移；`X` 為 XMS 區塊線性位址（本案 `110000h`）。
JEMM 的路徑分析出自 `strike_commander` 的 `workplace/re/jemm-api/img.dis` 靜態反組譯（本機），推論等級逐項標示。

---

## 1. 為什麼要做

436 的觸發案例停在 SCCD 映像 `0x2BE`（`0110:02BE`）的 `INT 2Fh`（`AX=4300h`），這是 V86 的第一個需要 IDT 的事件。之後 SCCD 會以 `INT 21h`
執行 `MENU.EXE` 等程式（`docs/re/002`），全部在 V86 下經 JEMM。

### 1.1 JEMM 的 IDT 與處理流程

| 項目 | 內容 | 等級 |
|---|---|---|
| IDT | 基底 `X+1530h`、界限 `7FFh`；256 項皆為 386 中斷閘（type `Eh`、DPL 0、選擇子 `18h`），偏移 `47DEh+4n`（entry_stage1 `4072`–`4085` 的迴圈；第 104610 步傾印） | 已證實 |
| 小樁 | `47DE+4n` 為 `E8 rel16 CF`：`call 4BDE`；`4BDE` 以返回位址算出向量 | 已證實 |
| 進入框架 | 從 V86 進入換到 `SS0:ESP0=50h:1400h`（TSS `10h`），`50h` 為 B=0，推入 9 個 dword（`GS`、`FS`、`DS`、`ES`、`SS`、`ESP`、`EFLAGS`、`CS`、`EIP`）加錯誤碼。`4BE1` 的 `cmp sp,13D6h` 區分有無錯誤碼：進 `4BDE` 時無錯誤碼 `SP=13DAh`、有錯誤碼 `13D6h` | 強推論（數值與程式常數吻合） |
| V86 的 `INT n` | IOPL=3，閘 DPL 0 < CPL 3，觸發 `#GP(n*8+2)`；JEMM 在 `4FDC` 解碼 `CD nn`，`4EE8`–`4F5C` 改 V86 堆疊（推 `FLAGS`、`CS`、`IP`，清 IF、TF）、讀 IVT[nn] 寫回框架，`4F60` 以空選擇子載入 `DS`／`ES`／`GS`／`FS`、`popad`、`iretd` 回 V86 | 強推論 |
| JEMM 自己處理的 INT | `15h`（`57CD`：`AX=1209h` 私有 API；`AH=87h` 回 `AX=FFFFh`、`AH=88h` 回 0，所以 dosgolem 的 `int15` 這兩個子功能在 V86 下不會被呼叫）、`67h`（`65DB` EMS／VCPI）；`INT 1`／`INT 3` 走溫開機；其他（含 `21h`、`2Fh`）反射 | 強推論 |
| IRQ | PIC 不重新程式化，IRQ0–7 在向量 `08h`–`0Fh`；除 `0Dh`、`0Eh` 以錯誤碼旗標區分外，一律反射 IVT。IRQ 路徑（`4C03`–`4C3B`）以 `67` 前綴的 `[esp+n]` 讀框架；`09h` 先 `in al,60h` | 已證實（靜態）／強推論（執行期） |
| `#GP(0)` 非 `CD`／`CC`／`CF`／`0F 20`／`0F 22` | 走 `5046`、`504C` 的偽例外回呼，不是反射 | 強推論 |

所以 V86 下 dosgolem 要提供：V86 事件經 IDT 遞送到 ring 0（含堆疊切換與框架）、ring 0 的 `iretd` 回 V86，以及服務層在 V86 下的觸發規則：
遊戲的 `INT 21h` 經 JEMM 反射到 IVT[21h]＝`0080:0400`（StubSeg 的 `CD F2 CF`），這個 `CD F2` 若再經 IDT，JEMM 會反射到 IVT[F2h]（同樣是 StubSeg 的
`CD F2 CF`），永遠到不了服務層。

## 2. 遞送機制

### 2.1 事件種類

| 事件 | 來源 | 向量 | 錯誤碼 | IOPL 檢查 | 閘 DPL 檢查 | 框架 `EIP` |
|---|---|---|---|---|---|---|
| 軟體中斷 | V86 的 `CD nn`（§4 的服務層例外除外） | nn | 無 | IOPL<3 改送 `#GP(0)`（`cpu.cpp:1302-1309`） | 閘 DPL < 3 改送 `#GP(nn*8+2)`（`1317-1320`） | 下一道 |
| `INTO`（`CE`，OF=1） | | 4 | 無 | 有 | 有 | 下一道 |
| `INT 3`（`CC`） | | 3 | 無 | 無（`prefix_none.h:1054` 的 `NoIOPLCheck`） | 有 | 下一道 |
| 外部中斷 | IRQ0（`tick`）、IRQ1（`keyTick`、`timedKeyTick`） | `08h`、`09h` | 無 | 無 | 無 | 遞送當下的 `IP` |
| `#DE` | 除以零、商溢位（16、32 位元，含 `AAM 0`） | 0 | 無 | 無 | 無 | 指令起點（含前綴） |
| `#BR` | `BOUND` 越界 | 5 | 無 | 無 | 無 | 指令起點 |
| `#GP(0)` | IOPL<3 的 `cli`、`sti`、`pushf`、`popf`、`iret`；`hlt`；特權指令（433 以 `Sys.User()` 判定者：`mov crN`、`mov drN`、`lgdt`、`lidt`、`lmsw`、`clts`）；I/O 點陣圖拒絕 | 13 | 0 | 無 | 無 | 指令起點 |

`#DE`、`#BR`、`#GP` 的框架 `EIP` 是指令起點（DOSBox-X 386 核心的 `PRE_EXCEPTION` 為空，`core_normal.cpp:34`；迴圈末才 `SAVEIP`，`183-220`）。
既有核心的 16 位元 `divideError` 沿用 8086 的「下一道」慣例，V86 下不走它。

V86 下的 `0F 00` 群組、`lar`、`lsl`（真機 `#UD`）維持 436 的失敗即關閉，不在本規格範圍。

### 2.2 核心回報事件

- 新增 `cpu.V86Event{Kind, Vector, HasError, ErrorCode, FrameEIP, Reason}`（`Reason` 保留 436 的診斷字串，例如「點陣圖」「界限」）。核心以 `c.v86Event(ev)`
  記錄：事件存在 `c.ev`，同時以 `failf` 設佔位錯誤。`Step` 既有的 `if c.fault != nil` 分支統一處理：`c.ev != nil` 時把 `IP` 設回 `opIP`、回哨兵
  `cpu.ErrV86Event`，並把事件移到另一份副本後清掉 `c.ev`（避免殘留事件讓之後的一般 `failf` 被誤判）。熱路徑不增加判斷。`c.V86Event()` 讀取副本，內容在下一次 `Step` 前有效。
- 同一道指令先記了一般 `failf`、後記事件時，以先記的為準（現行 `failf` 只留第一個）。
- 半狀態：記事件的位置在任何寫入之前。`div`／`idiv`（`alu.go` 的 `div8`／`div16`／`idiv8`／`idiv16` 在回 false 前會先 `divFlags`）與 `AAM 0`
  （`ops.go` 先 `c.logic(0,8)`）在處理函式內以區域變數保留進入時的 `Flags`，V86 記 `#DE` 時還原（386 的 `#DE` 不改旗標）。
- 436 的 V86 失敗點（`cpu.go` 的 `v86Fail`；`v86.go` 的 `v86Popf`、`v86Iretd`、`v86IOPermit`；`ops.go` 的 `cli`／`sti`／`pushf`／`iret`／`hlt`；`ops386.go` 的
  `66 9C`／`66 9D`；`ops186.go` 的 `insOuts`）改為記 `#GP(0)` 事件；`v86Iretd` 只有 IOPL<3 那支改為事件，`EIP` 高半非 0 仍是一般失敗（刻意從嚴）。
  `sys.go` 的 `privileged` 在 V86 下記 `#GP(0)` 事件後仍回非 nil 錯誤，呼叫端照舊提早 return，不執行指令本體。分段模式（ring 0）的失敗照 434。

### 2.3 遞送步驟（V86 → ring 0，照 `CPU_Interrupt` 保護模式分支，`cpu.cpp:1300-1475`）

由 Machine 執行，分成兩段：`prepareV86Delivery(ev) (plan, error)` 只讀不寫（第 1–4 步），`commitV86Delivery(plan)` 只寫且不會失敗（第 5 步）。
prepare 失敗時狀態不變並失敗即關閉（讀取時 `walk` 設 A 位元與填 TLB 是讀取本身的效果，不算狀態改變）；讀取留下分頁紀錄（`m.pfOK`）時以 `stepFault` 回報。

1. 軟體中斷的 IOPL 檢查（§2.1）；不通過則事件改為 `#GP(0)`、框架 `EIP` 改為指令起點（`Step` 已把 `IP` 設回 `opIP`，直接用 `c.IP`）。
2. IDT：`vector*8+7 ≤ IDTR.limit`，否則失敗即關閉（真機 `#GP(vector*8+2+EXT)`，屬 S6）。讀閘（`ReadSup32`×2）。軟體中斷先做閘 DPL 檢查（DOSBox-X 在型別檢查之前，
   `cpu.cpp:1317`）；不通過則改為 `#GP(vector*8+2)`、框架 `EIP` 改為起點，回到本步重讀向量 13 的閘（改送的 `#GP` 屬例外，不再做 IOPL 與 DPL 檢查）。
   接著：閘須為 386 中斷閘（type `Eh`）或 386 陷阱閘（type `Fh`）、present；286 閘、任務閘失敗即關閉。
3. 目標碼段：閘選擇子 `TI=0`、在 GDT 界限內，以 `DecodeDescriptor` 解碼；須為非一致碼段、present、DPL 0、D=0（D=1 屬 32 位元碼段，後續規格）；閘偏移高 16 位元須為 0。
4. 新堆疊：`TR` 有效（436 §3.2 第 1 步的判定）；讀 `ESP0`（`TR.base+4`，32 位元）與 `SS0`（`+8`）；`SS0` 非空、`TI=0`、在 GDT 界限內、RPL＝DPL＝0、
   可寫資料段、present（`cpu.cpp:1372-1394`）。`SS0` 的 B=1 失敗即關閉（本案 B=0）。
   推入的 n 個 dword（無錯誤碼 9、有錯誤碼 10）：第 k 個（k 從 0 起，順序同第 5 步）位於 `SS0.base + uint16(ESP0 − 4(k+1))`（B=0 時位移 16 位元環繞，
   dword 本身不環繞；同 DOSBox-X `CPU_Push32` 以 mask 取新 SP）。每個 dword 的 `new_sp..new_sp+3` 都依 434 §4.1 檢查 `SS0` 界限（本案向下展開、界限 0），
   `m.paging` 為真時以 `peekXlatPage` 探測每個 dword 的首尾 byte；任一失敗即關閉。
5. commit，依序：
   - 先取舊值：`GS`、`FS`、`DS`、`ES`、`SS`、`ESP`（`Hi[SP]:SP`）、`EFLAGS`（`uint32(Flags) | uint32(eflagsHi)<<16`，含 `VM=1`）、`CS`。
   - 載入新堆疊：`Seg[SS]=SS0`、`SetSegCache(SS, …)`、`SP`＝`ESP0` 低 16 位元、`Hi[SP]`＝`ESP0` 高 16 位元（真機整個載入 `ESP0`；DOSBox-X 在 B=0 時保留舊高半，
     `cpu.cpp:1410`。刻意對齊真機：JEMM 的 IRQ 路徑以 `[esp+n]` 定址，舊高半非 0 會讀錯）。
   - 推入（`WriteSup8`，各 4 bytes）：`GS`、`FS`、`DS`、`ES`、舊 `SS`、舊 `ESP`、`EFLAGS`、舊 `CS`、`FrameEIP`、有錯誤碼時錯誤碼；`SP` 依序遞減。
   - `DS`、`ES`、`FS`、`GS` 載入空選擇子（快取不可用）。
   - `CS`＝閘選擇子（RPL 改為 0），快取取自描述元；`IP`＝閘偏移低 16 位元（高 16 位元非 0 已在第 3 步失敗即關閉）。
   - 旗標：`SetSegmented(true)` 之後 `SetEFlagsHi`（清 `VM`，`RF` 不動，同 DOSBox-X `cpu.cpp:1465-1472`）、核心旗標清 `TF`、`NT`，中斷閘另清 `IF`；`SetPMCPL(0)`。
6. 回 nil。commit 之後同一個 `Machine.Step` 內，若遞送發生在 `tick`（IRQ），`CPU.Step()` 接著執行 ring 0 的第一道指令（與實模式送 IRQ 相同）。

### 2.4 IRQ 的遞送順序

`ivtGuardIRQ`（`internal/machine/sysregs.go`）的 V86 分支改為呼叫 `prepareV86Delivery`：失敗時存 `irqErr` 並回 true（位置與 433／436 相同，在 IF、PIC 遮罩、
待處理、`KeyEvery`／`nextKey`、定時鍵到期與 `StubSeg` 判斷之後，消耗狀態之前），成功時把 plan 存起來回 false。`tick`／`keyTick`／`timedKeyTick` 在原本呼叫
`Interrupt` 的位置改呼叫 `m.raiseIRQ(vec)`：V86 時 `commitV86Delivery(plan)`，否則 `CPU.Interrupt(vec)`。順序固定為：
守門（含 prepare）→ `irq0Pending=false`／彈出佇列 → `Ticks++` → `onTick` → `kbdData`（鍵盤）→ `raiseIRQ`。

`Step` 在 `tick` 之後才評估 `m.CPU.Segmented()`，所以同一步之後的 `keyTick` 走分段模式的延後路徑，不會重複遞送。IRQ 路徑的錯誤由 `Step` 直接以
`takeIRQErr()` 回傳（不經 `finishStep`）；prepare 讀取留下分頁紀錄（`m.pfOK`）時，`takeIRQErr` 的結果以 `stepFault` 包成 `*PageFaultError`。

### 2.5 回 V86：ring 0 的 `iretd`

分段模式、`NT=0` 的 `66 CF`（16 位元 `CF` 維持 434 的失敗即關閉），且 `c.Sys.CPL()==0`：

1. 以 `SS` 快取讀 `SS:SP` 起 3 個 dword（`EIP`、`CS`、`EFLAGS`），位移 16 位元環繞，做 434 §4.1 的界限檢查。彈出的 `EFLAGS` 有 `VM=1` 時再讀 6 個 dword
   （`ESP`、`SS`、`ES`、`DS`、`FS`、`GS`）。`VM=0`（同特權或外層返回，JEMM 的 ring 0 巢狀路徑 `4C3F`）失敗即關閉，屬後續。
2. `EIP` 高 16 位元非 0 失敗即關閉（DOSBox-X 截為低 16 位元，`cpu.cpp:1612`；刻意從嚴）；`EFLAGS` bit 18–31 非 0 失敗（DOSBox-X 以遮罩略過，
   `cpu.cpp:1622`；刻意從嚴）。
3. 核心把 9 個值存進 `c.V86Return()`，`IP` 設回起點，回哨兵 `cpu.ErrV86Return`。
4. Machine：`SetSegmented(false)`、清六個段快取、`SetEFlagsHi`（只依彈出值設 `VM`，`RF` 不動；DOSBox-X `FMASK_ALL|FLAG_VM` 不含 `RF`）、核心 `SetFlags`
   （低 16 位元，含 IOPL）、`IP`、`Seg[CS/SS/ES/DS]`、`FSeg`、`GSeg`（段值）、`SP`／`Hi[SP]`＝彈出的 `ESP`。CPL 由 `VM` 推得為 3（`cpu.cpp:1608-1636`）。
   434 的一百萬步計數歸零。

`finishStep` 處理 `ErrV86Event`、`ErrV86Return` 時與 `ErrTaskReturn` 相同：`m.pfOK` 為真時走 `stepFault`，不遞送。

## 3. 分段模式下的事件

分段模式（ring 0）期間：IRQ 照 434 §5.3 延後（JEMM 的處理常式在 `4BDE` 執行 `cli`，ring 0 期間通常 `IF=0`）；軟體中斷、CPU 例外照 434 失敗即關閉
（保護模式內的 IDT 遞送與 `#PF` 屬 S6）。

## 4. 服務層在 V86 下的規則

V86 下執行 `CD nn`、`CC`、`CE` 時，`doInt` 的流程為：**先呼叫 `IntHook`；回 false 才記軟體中斷事件**（§2.2），不呼叫 `Interrupt`。V86 以外維持現行順序
（先 `ivtGuard` 後 `IntHook`）。`ivtGuard` 的 V86 分支只留給 `divideError`、`divideFault`、`BOUND` 三處。

服務層的條件只寫在 `internal/dos` 的 `handle` 一處：開頭加上「`d.M.VM()` 為真且本道指令的 `CS`（`c.Op()` 的段值）不等於 `StubSeg` 時回 false」，
其餘沿用現行判斷（IVT[nn] 的段值為 `StubSeg` 或 `fromOwnStub`，`dos.go` 約 686–729 行）。比較的是段暫存器值，不是線性位址。
`dpmi_real_mode.go`、`le_real_irq7.go`、`cmd/shots` 等其他 `IntHook` 使用者的 `Sys` 為 nil，不受影響。

理由：StubSeg 代表 BIOS／DOS 的實作。真機上 BIOS 與 DOS 的程式碼在 V86 下原生執行，dosgolem 以 `CD nn` 陷入代替那些程式碼，所以 StubSeg 內的陷入不經 JEMM。
遊戲自己的 `INT 21h`（`CS` 不是 StubSeg）照真機經 IDT 與 JEMM，反射到 IVT[21h]＝`0080:0400` 後，`CD F2` 在 StubSeg 內由服務層處理。SCCD 的 `INT 15h AX=1209h`
同理先到 JEMM；JEMM 不認得的 `int 15h` 子功能反射到 IVT[15h]＝`0080:0408` 的 `CD F4`，再由服務層處理。PSP:0 的 `CD 20` 與 EMSSeg:0 的 `CD F6` 不在
StubSeg，經 JEMM 反射到 StubSeg 內的對應 stub 後由服務層處理（兩跳）。IVT 被遊戲掛鉤（段值不是 StubSeg）時，StubSeg 內的 `CD nn` 也經 IDT（例如 BIOS 計時器
stub `0080:0420` 內的 `INT 1Ch` 在遊戲掛了 1Ch 時）。

| 項目 | V86 下的行為 |
|---|---|
| 旗標回寫 | V86 下被服務的中斷都由 JEMM 反射而來，堆疊上有 JEMM 推的 `FLAGS`、`CS`、`IP`，stub 結尾的 `CF` 會把 `FLAGS` 彈回。所以服務層處理 per-vector stub（`CS:IP` 為 `StubSeg:StubOff(n)`，即 `CD n; CF`）之後，把 `c.Flags` 的 `CF`、`ZF` 寫回 `SS:SP+4` 的 `FLAGS`（只改這兩位元；服務未動旗標時寫入等值）；`F2`／`F3`／`F4` 照現行 `fixStackedCF`（`CF`）。far call 形式的 `F5`（XMS）、`F6`（EMS）、`F7`、`F8`（字型）、`F9`（回呼）不寫。**回寫（含 `fixStackedCF`）只在服務返回後 `Seg[CS]==StubSeg` 且 `IP` 不等於本道起點時才做**：EXEC 啟動子行程時 `SS:SP` 已換成子行程（`exec.go` 的 `enterProgram`），寫入會改到子行程記憶體（COM 的 `SP=FFFEh` 時 `SP+4` 繞到 `PSP:0002`）；`Rewind` 退回起點的阻塞服務也不寫。`4Ch`、`INT 20h`、`27h` 終止後 `CS` 回到 StubSeg，照寫。同一條件也套用到實模式的 `fixStackedCF` |
| 讀 IVT | `handle` 與 `keyTick`、`timedKeyTick` 一律以 `ReadSup16` 讀（宿主存取，不受 V86 的使用者分頁權限影響）；profile 關閉時等同現行 |
| 阻塞型服務（`int 21h AH=01/07/08` 等） | 照舊以 `Rewind` 退回 `CD` 起點；下一步重新判斷，仍由服務層處理 |
| 埠 `60h` | JEMM 的 IRQ1 路徑在反射前 `in al,60h`；現行讀埠 `60h` 直接回 `kbdData`、不消耗（`machine.go` 約 1096 行），維持現況並加回歸測試 |
| `startCallback`、`periodic` 的 `FarCall` | V86 下照常執行（以 V86 的 `SS:SP` 推入，段值×16）；分段模式下照 434 延後。436 §4 的 V86 延後取消，`tick` 的閘門改為 `!segmented` |
| 第二道防線 | `Interrupt` 在 V86 仍失敗即關閉（V86 不得走 IVT 直送）；`FarCallWithReturn` 的 V86 條件移除 |

## 5. 改動點

| 位置 | 改動 |
|---|---|
| `internal/cpu/ops.go` 的 `doInt`（`CD`、`CC`、`CE`） | §4：V86 時先 `IntHook`，回 false 記軟體中斷事件 |
| `internal/cpu/sys.go` 的 `ivtGuard` 與三個呼叫點（`ops.go` 的 `divideError`、`wide.go` 的 `divideFault`、`ops186.go` 的 `BOUND`） | V86 分支改記 `#DE`／`#BR` 事件 |
| `internal/cpu/alu.go` 的 `div8`／`div16`／`idiv8`／`idiv16` 呼叫端與 `AAM` | §2.2 的旗標還原 |
| `internal/cpu/cpu.go` 的 `v86Fail`、`v86.go`、`ops.go`、`ops386.go`、`ops186.go`、`sys.go` 的 `privileged` | §2.2：改記 `#GP(0)` 事件 |
| `internal/cpu/ops.go` 的 `hlt` | V86 記 `#GP(0)`；分段模式維持 `errf` |
| `internal/cpu/ops386.go` 的 `66 CF`（分段模式、`NT=0`） | §2.5 |
| `internal/cpu/cpu.go` 的 `Step` fault 分支、`Interrupt`、`FarCallWithReturn` | §2.2 的統一處理；§4 的第二道防線 |
| 新檔 `internal/cpu/v86event.go`、`internal/machine/v86deliver.go` | 事件型別與存取器；prepare／commit 與回 V86 |
| `internal/machine/handoff.go` 的 `finishStep` | 新哨兵各一個分支，帶 `!m.pfOK`，放在 `ErrModeSwitch` 之前 |
| `internal/machine/sysregs.go` 的 `ivtGuardIRQ`；`machine.go` 的 `tick`；`keyboard.go`、`keyhold.go` | §2.4；IVT 改 `ReadSup16` |
| `internal/dos/dos.go` 的 `handle` | §4 的 `CS` 條件與旗標回寫 |

要改的既有測試：`internal/cpu/v86_test.go` 的 `TestV86CliSti`、`TestV86PushfPopf`、`TestV86Iret`、`TestV86IOBitmap`、`TestV86HltAndInterrupts` 與第二道防線
那段（改為期待 `ErrV86Event`、`IP` 回起點、未動 I/O；`FarCallWithReturn` 在 V86 不再失敗）；`internal/machine/v86gate_test.go` 的 `TestV86IRQ0Gate`、
`TestV86KeyIRQGate` 與回呼延後段（改為期待遞送與照常執行）；`internal/machine/sysinsn_test.go` 的 `TestSysPrivilegedInUserMode`（改為期待 `#GP(0)` 事件）。
重構 `ivtGuard` 後重跑 434 的 `segmode_test`、`TestIVTGuardCPU`、`TestIVTGuardIRQ`。

## 6. 不在範圍

保護模式內（ring 0）的例外與 IDT 遞送、`#PF`、`#DB`（S6）；ring 0 巢狀返回（`iretd` 彈出 `VM=0`）；任務閘；32 位元碼段；VCPI；回實模式（S9）；
V86 的 x87 與 `#UD`；中斷遮蔽（436 §4 的刻意差異維持）。JEMM 以 GDT 選擇子 `C0h`（基底 `CS<<4`、界限 `FFFFFh`、byte 粒度）讀 V86 指令與堆疊，
`CS<<4+IP` 超過 1 MB（HMA）時 434 的界限檢查會失敗即關閉（真機 `#GP`），罕見，記為已知限制。

## 7. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 無改動；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變；`Step` 的熱路徑不新增分支。
2. 遞送：合成 GDT、IDT、TSS（仿 §1.1）：
   - V86 `CD nn`、閘 DPL 0：送 `#GP(nn*8+2)` 到向量 13，框架 `EIP` 為 `CD` 起點、錯誤碼正確、推入後 `SP=ESP0-40`；
   - 閘 DPL 3 的 `CD nn`：直接送向量 nn，框架 `EIP` 為下一道；IOPL<3 的 `CD nn`、`INTO` 送 `#GP(0)`；IOPL<3 的 `INT 3` 不送 `#GP(0)`（只做 DPL 檢查）；
   - IRQ0、IRQ1：送向量 8、9，無錯誤碼；中斷閘清 `IF`、陷阱閘不清；
   - 16 位元與 32 位元 `div 0`、`BOUND` 越界：框架 `EIP` 為起點，框架內 `EFLAGS` 等於指令前的旗標；
   - 框架 9 個 dword 的值與順序、`DS`／`ES`／`FS`／`GS` 為空選擇子、`CS`／`IP`／`CPL`、`segmented=true`、`VM=0`；
   - V86 的 `ESP=ABCD1234h` 時送 IRQ0：進入後 `ESP=ESP0-36`（高半取自 `ESP0`），以 `67` 前綴的 `[esp+4]` 讀到預期值；
   - 推入跨過 `SP=0` 環繞時每個 dword 的位址正確；
   - prepare 每個失敗條件各一例，失敗時狀態不變；分頁紀錄回 `*PageFaultError`。
3. 回 V86：ring 0 `iretd` 彈出 `VM=1` 的框架後，`CS:IP`、`SS:SP`（含 `Hi[SP]`）、四個資料段、旗標（含 IOPL）還原，`segmented=false`；`EIP` 高半非 0、`EFLAGS` 高位元、
   彈出 `VM=0`、CPL≠0 各失敗一例；16 位元 `CF` 失敗。
4. 往返：合成「V86 `CD 21` → ring 0 處理常式模擬 JEMM 反射（改 V86 堆疊、讀 IVT、`iretd`）→ StubSeg `CD F2` → 服務層 → `CF`」，服務層只被呼叫一次、
   回到 `CD 21` 的下一道，`CF` 回寫正確；同樣以 per-vector stub 驗 `INT 16h AH=01` 的 `ZF` 與 `INT 13h` 的 `CF` 回寫；V86 下 EXEC 啟動子行程後子行程 `SS:SP+4`
   的位元組不變；阻塞服務 `Rewind` 時不回寫。
5. 服務層條件：`CS`≠StubSeg 時不服務；IVT[nn] 不指 StubSeg 時 StubSeg 內的 `CD nn` 經 IDT；`fromOwnStub` 情形由服務層處理；阻塞服務的 `Rewind` 重跑仍由服務層處理；
   讀埠 `60h` 兩次得到同一值。
6. 回呼：V86 下 `startCallback`、`periodic` 照常執行；分段模式下延後不消耗。
7. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7）；`BenchmarkStepV86Paged` 不低於 11.58 M 道／秒。照實記錄。
8. 突變自檢：服務層條件不看 `CS`、關掉 `doInt` 的 V86 分支（往返測試必須失敗）、軟體中斷不做 DPL 檢查、`INT 3` 做 IOPL 檢查、框架少推資料段、
   `#GP` 的 `FrameEIP` 用下一道、`#DE` 不還原旗標、不載入 `ESP0` 高半、`iretd` 回 V86 不還原資料段、IRQ0 遞送不清 `IF`、per-vector stub 不寫回旗標、回寫不看 `CS`、事件取走後不清 `c.ev`，
   各至少一個測試失敗。
9. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
   - 第一個事件 `INT 2Fh`：進入 `18h:4812`（向量 13 小樁）時 `SP=13D8h`；框架 `EIP=000002BEh`、`CS=0110h`、`ESP=000000D2h`、`SS=0216h`、`ES=2000h`、
     `DS=01EAh`、`EFLAGS` 含 `VM`、`IOPL=3`，錯誤碼 `0000017Ah`（436 收據第 104610 步）；
   - 記錄 JEMM 的 `[3ED3]`、`[3F38]`（非 0 會走 `4F6E` 的另一條反射路徑）；
   - JEMM 反射後：V86 的 `SS:SP=0216:00CC`，`[00CC]=02C0h`、`[00CE]=0110h`、`[00D0]` 為原 `FLAGS`（`IF=1`），`CS:IP=0080:00BC`，`IF=TF=0`；
     `CD 2F` 在 StubSeg 內由服務層處理；
   - 繼續執行，照實記錄最後停點、原因、步數，以及途中 `EXEC` 了哪些程式（`MENU.EXE` 等）。
10. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

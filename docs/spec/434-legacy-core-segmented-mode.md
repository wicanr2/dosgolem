# 434 — 既有核心的分段模式（16 位元保護模式）

狀態：**READY**（第 3 版，依第 2 輪複審修正引用與清單）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移（`strike_commander` `docs/re/003` §1），
實模式時 JEMM 映像在段 `2000h`，`2000:xxxx` 的偏移即 img 位址。反組譯在 `strike_commander` 的 `workplace/re/jemm-api/img.dis`（本機）。

---

## 1. 為什麼要做

433 的觸發案例停在 JEMM img `0x4203`（`mov cr0,eax` 設 PE）。之後到 `ltr` 之前，JEMM 在保護模式執行 16 位元碼（`docs/re/004` §1）：

| img 範圍 | 內容 | CS |
|---|---|---|
| `4206`–`43A2` | `lldt`、載入 `SS`／`ES`／`DS`、把 `0xA000` bytes 自身映像複製到 XMS、改寫 GDT 基底、`lidt`、`lgdt`、重載段暫存器 | 實模式殘留快取（基底 `20000h`） |
| `43A2` | `jmp 18h:43A7` | 選擇子 `18h` |
| `43A7`–`4583` | 堆積（`62B5` 與 `637D`–`6597` 一族）、頁表與 `CR0.PG`（`53CE`，含 `52BD`）、頁 0 別名（`573F`）、填 SCCD 結構 | `18h` |
| `4586` | `ltr cx`（`CX=F0h`），之後 `NT=1` 的 `iretd` 進 V86 任務 | `18h` |

路徑上的指令含 `66`／`67`／`64`／`65` 前綴、ModRM32、`enter`／`leave`、`loop`（16 位元 `CX`）、`lods`（無 `67`）。沒有 `in`／`out`／`int`／`hlt`／`iret`／
`pushf`／`popf`（`pushfd`／`popfd`／`iretd` 在 `ltr` 之後）。依 431 第 2 版，這段由既有核心以「分段模式」執行；本規格的驗收停在 img `0x4586` 的 `ltr`。

`4443` 的 `cmp word [3ED3],0` 為 0 時跳到 `44F7`，略過 VCPI 客戶模式分支（`444C`–`44F1`：`mov ss,F8h`、`popad`、`66 call fword`）。`[3ED3]` 只在
`3FBB` 寫入；交棒當下 `2000:3ED3` 為 0（已證實，probe 傾印），且同值隨複製進 XMS 副本（強推論），所以這個分支不在驗收路徑上。

### 1.1 交棒當下的狀態（已證實，433 收據之後的 probe）

`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile` 停在 `2000:4206` 時：

- `X ＝ [2000:3EA9]`（XMS 區塊線性位址，4 KB 對齊）＝ `00110000h`。`X` 來自 SCCD 呼叫框架與 430 的 XMS 配置，不能由反組譯推得；
  §9 第 10 項的期望值都寫成 `X` 的函數。
- 第一份 GDT（`lgdt` 16 位元形式，基底 `021400h`、界限 `012Fh`）的相關項：

| 選擇子 | 基底 | 界限 | access | 旗標（G、D/B） | 說明 |
|---|---|---|---|---|---|
| `08h` | `021400h` | `0007h` | `82h` | 0 | LDT（基底即 GDT 本身） |
| `10h` | `021D98h` | `2068h` | `8Bh` | 0 | 386 TSS（忙碌） |
| `18h` | `020000h` | `FFFFFh` | `9Ah` | 0 | 16 位元碼段，可讀，非一致 |
| `20h` | `020000h` | `FFFFFh` | `92h` | 0 | 資料，可寫 |
| `28h` | `0` | `FFFFFh` | `92h` | G=1 | 資料，4 GB |
| `50h` | `020000h` | `0` | `96h` | 0 | 資料，向下展開，可寫，B=0 |
| `F0h` | `021D30h` | `0067h` | `89h` | 0 | 386 TSS（可用） |

這些項的 access byte accessed 位元都是 0。`80h` 的基底由 `62B5` 執行時改寫（`62FE`、`6305`），交棒當下的值不是期望值。

## 2. 分段模式的狀態

- `SegCache`（433 §3.2 的型別與 `Attr` 編碼）由 `internal/machine` 移到 `internal/cpu`，machine 以型別別名沿用。
- 既有核心新增未匯出欄位 `segmented bool` 與 `cache [6]SegCache`，索引與既有常數相同（`ES`、`CS`、`SS`、`DS`、`FS`、`GS`，`internal/cpu/cpu.go:33-46`）。
  只經下列方法讀寫，供 Machine 使用：`Segmented() bool`、`SetSegmented(on bool)`、`SegCache(i int) SegCache`、`SetSegCache(i int, c SegCache)`。
- 「不可用」的快取：`Attr` 的 P 位元（bit 7）為 0，且 `Base`、`Limit` 為 0。
- 不變式：`segmented=true` ⇒ `PE=1`、`VM=0`、profile 開啟 ⇒ 432 的 `xlat=true` ⇒ `CPU.Code==nil`（`machine.go` 的 `syncCodeFastPath` 已因 `xlat` 清除）。

### 2.1 `PE=1`、`VM=0` 而 `segmented=false`

只有測試以 `SetCR0`／`SetVM` 直接造出這個狀態（432 的 `paging_test.go:567`、`674`、`733-741`，433 的 `sysinsn_test.go:368-390`，
`bench_step_test.go:86`）。它維持 432 的語意：段值×16 定址、經線性層。保留這個狀態讓上列測試不變。

`LoadState` 讀到版本 1、2 的 `PM` 且 `PE=1`、`VM=0` 時回錯誤（這種存檔只可能來自 433 的停擺狀態，不能可靠恢復）。拒絕放在 `LoadState`，不在 `decodePM`，
所以直接呼叫 `decodePM` 的測試不受影響；經 `LoadState` 的 `TestPMV1Layout`（`linear_test.go:627-690`，手造 `CR0=80000019h`、`VM=0`）要改成 `VM=1` 或 `PE=0`。

### 2.2 `PM` 版本 3

`encodePM` 改輸出版本 3：版本 2 的 80 bytes 之後接：

| 位移 | 長度 | 欄位 |
|---|---|---|
| 80 | 1 | `segmented`（0 或 1） |
| 81 | 1 | 保留，0 |
| 82 | 60 | `cache[0..5]`，每項 `Base` 4、`Limit` 4、`Attr` 2 |

共 142 bytes。`decodePM` 支援版本 1、2（`segmented=false`、快取全 0）與 3。`LoadState` 另驗：

- `segmented` 只能是 0 或 1；保留 byte 為 0；
- `segmented=0` 時六個快取全 0；`segmented=1` 時 `PE=1`、`VM=0`、`pmCPL=0`；
- 各 `Attr` 的 bit 8–11、13 為 0；`cache[CS]` 與 `cache[SS]` 的 D/B（bit 14）為 0。

433 的 `PM` 測試要改：版本 2 的逐欄位位移測試改成「手造版本 2 可讀」；往返長度 80 改 142；「未知版本號」改用 4；長度不足與尾端多餘 bytes 依版本 3
重新檢查。`Snapshot` 的 `pmFields` 增加 `segmented` 與快取（`snapshot.go:197`），`applyPM` 以 `SetSegmented`／`SetSegCache` 還原。
`StateDigest` 在 profile 開啟時因 `PM` 變長而改變，profile 關閉時不變。

## 3. 進入與離開

### 3.1 取代 433 的交棒接收端

刪除 433 的下列程式與測試，改為本節行為：

- `internal/machine/handoff.go`：`ErrPMCoreNotAvailable`、`Handoff`、`PMCore`、`modeNormal`／`modeHalt`／`modePM`、`recalcMode`、`setPMCore`、`setHalt`、
  `rejudgeHalt`、`handoffHaltErr`、`handoff`、`segsToPM`／`segsFromPM`（連同 `cpu386` import）。`finishStep` 保留並改寫。
- `machine.go`：`pm`、`entered`、`halt`、`mode` 欄位（約 674–678 行）與 `Step` 開頭的模式分派（約 1428–1439 行）。
- `linear.go`：`recalcPM` 裡的 `recalcMode()`（約 41 行）；`SetProtectedModeProfile` 的 `m.entered, m.halt = false, nil`（約 86 行）改為
  `SetSegmented(false)` 並清快取。
- `pmstate.go`：`applyPM` 結尾的 `rejudgeHalt`。
- 測試：`handoff_test.go` 整檔改寫為 §9 第 4 項；`sysinsn_test.go:366` 附近改寫註解；433 §9 第 7 項的突變「`Restore` 不重新判定」「`Enter` 失敗不停擺」
  「分頁紀錄加哨兵不停擺」失去對象，由本規格 §9 第 9 項取代。
- 仍寫 cpu386 的註解：`linear.go` 約 156、208 行，`cmd/probe/main.go` 約 227 行的旗標說明。

`cpu.ErrModeSwitch` 與 433 §5 的 PE 邊緣判定保留。`finishStep` 收到 `ErrModeSwitch` 時：

1. 呼叫 `m.enterSegmented()`：六個快取設為實模式推得值（基底＝段值×16、界限 `FFFFh`，CS 屬性 `009Bh`，其餘 `0093h`，同 433 §5），
   `SetSegmented(true)`，`SetPMCPL(0)`。
2. 同一步有分頁紀錄時回 `*PageFaultError`（此時已進分段模式）；否則回 nil。這取代 432 §10 第 1 項「哨兵不處理」對 `ErrModeSwitch` 的處置：
   `CR0.PE` 已是 1，不進分段模式反而會以實模式語意執行保護模式碼。

### 3.2 離開與不變式的守門

- `SetCR0` 在 `segmented=true` 時拒絕清 PE（回錯誤，`CR0` 不改）；`mov cr0` 因此失敗即關閉（S9）。
- `SetVM(true)`、設 `VM` 的 `SetEFlagsHi`、`SetPMCPL(≠0)` 在 `segmented=true` 時拒絕。`VM` 由 0 變 1 的路徑（`iretd`、任務切換）在本規格都失敗即關閉。
- `Restore`、`LoadState` 依 `PM` 的 `segmented` 欄位還原，不重新推導。
- `SetProtectedModeProfile` 切換時 `SetSegmented(false)`、快取清 0。

## 4. 位址計算

### 4.1 規則

分段模式下，每次記憶體存取以**指令指定的段暫存器索引**（預設段或段前綴，含 `FS`／`GS`）取該暫存器的快取，不以段值查找。
兩個段暫存器可以持有同一選擇子而快取不同。JEMM 的例子：img `42AD` 的 `lgdtw` 把 GDTR 換成 XMS 副本（`X+1400h`）後，`42B6`、`42BE` 經 `ES`
（快取基底 `X`）改寫副本的項 `20h`；此時副本的項 `20h` 仍是舊基底 `20000h`，以選擇子查表會寫到舊 GDT，之後 `42C3` 重載的 `DS` 與 `43A2` 的
`CS` 基底都會錯。

- 線性位址 ＝ 快取 `Base` ＋ 偏移（32 位元環繞），再交給 432 的線性層（分頁、A20）。
- 偏移：16 位元定址時為 16 位元，多 byte 存取的各 byte 偏移各自 16 位元環繞（沿用既有核心；DOSBox-X 以基底加 16 位元位移不回繞，界限大於
  `FFFFh` 的段在偏移 `FFFFh` 的 word 存取會不同。刻意差異，JEMM 路徑不觸發）。`67` 前綴時為 ModRM32 的完整 32 位元有效位址，多 byte 存取以 32 位元相加。
- 界限檢查（刻意從嚴：DOSBox-X 預設 `do_seg_limits` 關，`cpu.cpp:78`，界限設為 `~0`）：每個實際存取的 byte 都檢查；`lea` 不存取、不檢查。
  - 向上展開：偏移 ≤ `Limit`。
  - 向下展開：偏移 > `Limit`，且 ≤ `FFFFh`（B=0）或 `FFFFFFFFh`（B=1）。
- 型別檢查（刻意從嚴：DOSBox-X 只在載入時查型別）：寫入唯讀資料段或碼段、讀取不可讀碼段、經不可用快取存取。
- 違反界限或型別時失敗即關閉，診斷寫明段暫存器、選擇子、偏移、界限。`#GP`／`#SS` 的遞送屬 S6。
- 取指令：快取 `cache[CS].Base + IP`，`IP` 16 位元環繞，同樣檢查界限。
- 堆疊：`push`／`pop`／`call`／`ret`／`enter`／`leave` 經 `SS` 快取；`SS` 的 B=1 失敗即關閉（`4206`–`4586` 路徑上的 `SS` 都是 B=0；JEMM 其他路徑的
  `110h`、`118h` 是 B=1，屬後續）。
- `segmented=false` 時，位址計算與 433 完成時逐位元組相同（以段值×16，`67` 定址超過 `FFFFh` 照舊記 fault）。

### 4.2 既有核心的改法

既有核心目前以段「值」與 16 位元偏移傳遞位址，改為：

- `operand` 改帶段索引與 `uint32` 偏移（`modrm.go:3-12`）；`decodeModRM` 與 `decodeModRM32` 填索引；`decodeModRM32` 在分段模式不截斷（`modrm.go:144`）、
  不記 64 KB fault（`modrm.go:140-142`）；`moffs32`（`ops386.go:108-111`）同樣處理。
- `dataSeg(def)` 回段索引；`segVal` 只用於需要段值的地方（推入段暫存器、診斷）。
- 記憶體原語（`read8/16/32`、`write8/16/32`、`fetch8`）改吃段索引與 `uint32` 偏移；`segmented=false` 時以 `segVal(i)<<4` 加 16 位元偏移，與現況相同。
- 繞過原語、直接 `Bus.Read8(Linear(...))` 的位置（`ops.go:277`、`281`、`393`，`modrm.go:151`、`159`）與直接以 `c.Seg[ES]`、`c.Seg[SS]` 當位址段的位置
  （`string.go:55`，`ops386.go:416`、`481`、`486`，`ops186.go:99`、`104`、`150`，`cpu.go:324`、`329`，`ops.go:792`）改用索引原語；直接讀 `operand.seg`／`off`
  的位置（`sys.go:106-133`、`ops0f.go:139-143`、`ops.go:324`、`329`、`753`、`761`、`ops186.go:38-39`、`ops386.go:465`、`473`、`modrm.go:166`、`174`）一併改。
  多 byte 的 `off+2`、`off+4` 改以偏移寬度相加。
- `BT` 系的位址調整（`ops0f.go:235-240`：`int32(o.off)+adj`、64 KB 的 `failf`、`uint16(eff)` 截斷）改以偏移寬度計算，64 KB 檢查只在 `segmented=false`。
- `lea`（`ops.go:226`、`ops386.go:274`）取 `m.rm.off` 處依運算元寬度轉型，或改用 `ea32`。
- 分段模式的分支只放在原語內一處（`if c.segmented`）；熱路徑的實模式分支在前。實作收據列出最終改動的位置清單（以 grep 對照本節）。

`Interrupt` 讀 IVT 的 `read16(0, …)`（`cpu.go:343-344`）在分段模式不會執行（§5.3）。

## 5. 段暫存器

### 5.1 載入：`loadSeg(i int, sel uint16) bool`

既有核心所有「載入新段值」的位置都經 `loadSeg`：

| 位置 | 指令 |
|---|---|
| `ops.go:120`、`129`、`133` | `pop es`／`pop ss`／`pop ds` |
| `ops.go:234`、`236`、`242` | `mov sreg, r/m16`（含 `FS`、`GS`） |
| `ops.go:264`、`451`、`756`、`762` | 遠跳、遠呼叫（`9A`、`EA`、`FF /3`、`FF /5`） |
| `ops.go:324`、`329` | `les`、`lds` |
| `ops.go:340`、`345` | `retf imm16`、`retf` |
| `ops0f.go:64`、`66`、`147`–`151` | `pop fs`／`pop gs`、`lss`／`lfs`／`lgs` |
| `ops386.go:168`–`172` | 32 位元 `pop sreg` |
| `cpu.go:344`、`371` | `Interrupt`、`FarCall`／`FarCallWithReturn`：分段模式不會經 Machine 到達（§5.3）；函式開頭另加 `segmented` 時 `failf` 當第二道防線 |

「還原成本道指令起點」的位置（`ops186.go:46`、`wide.go:290`、`cpu.go:383`）只把 `CS:IP` 設回 `opCS:opIP`，CS 快取不變，不經 `loadSeg`。
`pop cs`（`ops.go:125`）、`mov cs`（`8E /1`）、`iret`（`ops.go:356`）與 `66` 形式在分段模式失敗即關閉。實作時以 grep `Seg\[`、`FSeg`、`GSeg`
的賦值確認沒有遺漏，收據列出最終清單。

`segmented=false` 時 `loadSeg` 只寫段值，行為不變。`segmented=true` 時：

1. 空選擇子：`(sel & 0xFFFC) == 0`（RPL 忽略；`04h` 不算空）。`CS`、`SS` 載入空選擇子失敗即關閉（`CS` 見 `cpu.cpp:1796-1798`）；`DS`／`ES`／`FS`／`GS`
   允許，快取設為不可用。（DOSBox-X `cpu.cpp:3037-3040` 只設段值與基底 0，之後照常存取；本規格經它存取失敗即關閉，刻意從嚴。）
2. 讀描述元：`TI=0` 查 GDT，`TI=1` 查 LDT（`LDTR` 快取；不可用時失敗）。`索引×8＋7 > 表界限` 失敗（Intel 語意；DOSBox-X 是
   `(sel &^ 7) >= limit`，`include/cpu.h:481`、`500`、`505`，只在界限不是 `8n−1` 時不同，本案 GDT `12Fh`、LDT `7` 相同）。描述元以 `ReadSup32`
   經 432 的線性層讀取；分頁錯誤依 432 §10 第 1 項回報。
3. 檢查（照 DOSBox-X `CPU_SetSegGeneral`，`cpu.cpp:2976-3078`，與 `CPU_JMP`，`cpu.cpp:1785-1835`；違反時失敗即關閉，不送例外）：
   - `SS`：RPL＝CPL＝DPL；可寫資料段；present。
   - `DS`／`ES`／`FS`／`GS`：資料段與非一致可讀碼段須 RPL ≤ DPL 且 CPL ≤ DPL；一致可讀碼段免檢；其他型別失敗；present。
   - `CS`（遠跳、遠呼叫、遠返回）：非一致碼段，RPL ≤ CPL、DPL＝CPL；present；D=0。一致碼段、TSS、閘門、D=1 失敗即關閉。
     新 `CS` 值為 `(sel &^ 3) | CPL`。遠跳與遠呼叫不改 CPL（`cpu.cpp:1833` 只把 CPL 併入新 CS 值）。遠返回時彈出的 `CS` RPL ≠ CPL 失敗即關閉。
4. 通過後才寫入段值與快取；失敗時段值與快取都不寫。
5. 不寫回 accessed 位元。DOSBox-X 不寫（`CPU_SetSegGeneral` 只讀描述元）；386 真機會寫。JEMM 不讀該位元，靜態上沒有 `lar`、`lsl`、`verr`、
   `verw`、`arpl`（`docs/re/004` §2）。

本規格只支援 CPL 0：`enterSegmented` 設 `pmCPL=0`，之後沒有路徑改變它；`SysBus.CPL()` 不為 0 時任何段載入失敗即關閉。

### 5.2 `SysBus` 增補

`internal/cpu.SysBus` 增加：

- `CPL() uint8`
- `ReadSup32(addr uint32) uint32`（Machine 已有 `ReadSup32`，`linear.go:226`）
- `LDTR() (sel uint16, c SegCache)`、`SetLDTR(sel uint16, c SegCache)`、`TRSel() uint16`

`cpu/sys_test.go` 的 `fakeSys` 與 machine 測試中包住 `SysBus` 的替身跟著補。

### 5.3 中斷、IRQ 與回呼

- CPU 側：`doInt`、`divideError`、`divideFault`、`BOUND` 的 433 守門擴充為「`IVTMismatch()` 或 `segmented`」，分段模式下失敗即關閉，診斷註明
  保護模式 IDT 遞送屬 435／S6。`doInt` 的檢查仍在 `IntHook` 之前。
- Machine 側依 431 §3 延後，不報錯：`segmented=true` 時 `tick` 的 `startCallback`、`periodic` 的 `FarCall`，以及 `keyTick`、`timedKeyTick` 都不執行、
  不消耗狀態（`startCallback` 與 `periodic` 在 `machine.go` 約 1563–1572 行，位於 IF 檢查之前，閘門要放在它們前面）。IRQ0 的時鐘照常累積並設
  `irq0Pending`（`machine.go` 約 1600–1606 行），只延後遞送：閘門放在遞送段（約 1608 行起）之前，`Ticks++`、`onTick`、`Interrupt(0x08)` 都不執行。
  `VGAFrameEvery` 的幀計數照常（不經 CPU）。
- 431 §3 的失敗條件：分段模式遇到 `hlt` 失敗即關閉；有待處理 IRQ（`irq0Pending` 或鍵盤佇列非空）且 `IF=1` 而連續一百萬步仍在分段模式，失敗即關閉。
  計數在離開分段模式或條件不成立時歸零，存在 Machine，不進狀態檔。

## 6. 指令

分段模式下依保護模式語意執行的指令；其餘沿用既有實作，只改位址計算（§4）與段載入（§5）。

| 指令 | 分段模式行為 | DOSBox-X |
|---|---|---|
| `EA ptr16:16`、`FF /5`（16 位元） | 遠跳，§5.1 的 `CS` 規則 | `CPU_JMP` |
| `9A`、`FF /3`（16 位元） | 同特權遠呼叫：推 `CS`、`IP` 各 2 bytes | `CPU_CALL` 的非一致碼段同特權分支 |
| `CB`、`CA iw`（16 位元） | 同特權遠返回 | `CPU_RET` |
| `66` 形式的遠轉移與 `66 CB` | 失敗即關閉（只在 VCPI 分支 `44F1`，本案不可達） | |
| `CF`、`66 CF` | 失敗即關閉（S4、435） | |
| `0F 00 /2`（`lldt r/m16`） | 空選擇子使 `LDTR` 不可用；否則須在 GDT（TI=1 失敗，Intel 語意；DOSBox-X 依 TI 查現行 LDT，`include/cpu.h:497-509`，刻意從嚴），描述元 S=0、type 2、present，載入 `LDTR` | `include/cpu.h:524-541` |
| `0F 00 /3`（`ltr`） | 失敗即關閉，診斷含「S4」。**本規格的驗收停點** | |
| `0F 00 /0`、`/1`（`sldt`、`str`） | 讀 `LDTR`、`TR` 選擇子（`TR` 目前恆 0） | |
| `0F 00 /4`、`/5`、`0F 02`、`0F 03` | 失敗即關閉（JEMM 沒用） | |
| `0F 01` 群組、`mov CRn`／`DRn`、`clts` | 433 的處理函式；`execSys0F`（`sys.go:48-51`）依 `segmented` 把 `0F 00` 分流到本表 | |
| `mov cr0` 設 `PG` | 經 `SetCR(0)`（432 的 `SetCR0` 重算 `paging`），不回哨兵 | |
| `pushf`／`popf`／`66` 形式 | 不改：CPL 0 下既有實作已符合 431 §4.1；CPL>0 不在本規格 | |
| `hlt` | 失敗即關閉（§5.3） | |
| `in`／`out`／字串 I/O | CPL ≤ IOPL 時照既有路徑；否則失敗即關閉（I/O 許可點陣圖屬 S4） | |
| x87 ESC | `FNINIT`、`FNSTCW`、`FLDCW`、`FWAIT` 以外失敗即關閉 | |

`67` 前綴下的隱含位址指令（字串、`xlat`、`loop` 用 `ECX`）照舊失敗即關閉（`ops386.go:69-96`）；路徑上的 `loop`、`lods` 不帶 `67`。

## 7. 觀測

分段模式下 `CS` 是選擇子，`cpu.Addr(Seg[CS], IP)` 不是線性位址。新增兩個 Machine 方法，兌現 431 §4 的兩種語意：

- `PCLinear() uint32`：下一道（步間）的線性位址。分段模式回 `cache[CS].Base + IP`，否則 `cpu.Linear(Seg[CS], IP)`。
- `InsnLinear() uint32`：本道指令起點的線性位址，由 `insnCS`／`insnIP` 與記下的 CS 基底算出（分段模式在 `Step` 記 `insnCS` 時一併記基底）。

| 位置 | 用途 | 分段模式的處置 |
|---|---|---|
| `internal/machine/machine.go:1473` | `Coverage` | 分段模式用 `PCLinear()`；否則維持 `cpu.Addr`（20 位元遮罩）不變；超出 `len(Coverage)` 不記 |
| `internal/machine/probe.go:215` | `atBreak` | 分段模式用 `PCLinear()`；否則維持 `cpu.Addr`。`AddBreak`（`probe.go:113`）存 `seg*16+off`，分段模式的位址（可大於 1 MB）只能經新增的 `AddLinearBreak(addr uint32)` 設定 |
| `cmd/probe/main.go:765` | `CS:IP` 落入視訊區的護欄 | 改用 `PCLinear()` |
| `internal/machine/probe.go:228` 的 `Insn()` | 指令起點 | 照舊回選擇子:偏移；需要線性位址者用 `InsnLinear()` |

下列以 `seg:off` 記錄或比對的觀測照舊，分段模式下的值是「選擇子:偏移」，屬已知語意：`physical.go:63`、`80`，`bios_rom.go:54`，`vga.go:670`，
`cmd/probe/main.go:433-434`、`864-872`、`958-962`，`cmd/probe/observe.go:136-160`、`263`，`-regs-at`、`TraceSegs`、`OpAddr()`。
`oracle`、`session`、LE 與 DPMI 路徑不在 `-pm-profile` 下使用，不改；其中以段值換算線性位址的位置（如 `oracle/run.go:122`）在分段模式下不保證。

## 8. 不在範圍

`iretd` 與任務切換、`ltr`、TSS、V86 進入（S4）；`#GP`／`#SS`／`#NP`／`#PF` 的遞送（S6、435）；保護模式 IDT 遞送與 IRQ（435）；32 位元碼段、
B=1 堆疊與 `66` 遠轉移；CPL 3 保護模式；一致碼段與閘門的轉移；accessed 位元寫回；回實模式（S9）。

## 9. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 與 LE 路徑 `git diff` 為空。
2. SingleStepTests 以掛載的語料實際執行、315 個子測試全過、0 skip，`knownGaps` 不變；掛載方式同 433 §8.2。
3. `PM` 版本 3：逐欄位位移；手造版本 1、2 可讀；版本 1、2 帶 `PE=1`、`VM=0` 被拒；§2.2 的驗證各一例；`Snapshot`／`Restore` 與 `SaveState`／`LoadState`
   對 `segmented` 與六個快取往返；還原 `segmented=1` 後 `CPU.Code==nil`，且 `Step` 以分段模式繼續。
4. 進入：合成程式在實模式設 PE 後 `segmented=true`、快取為實模式推得值、`CPL=0`；同一步有分頁紀錄時回 `*PageFaultError` 且已進分段模式；
   `SetCR0` 直接設 PE 不進分段模式（§2.1 的狀態照 432 語意執行）；分段模式下 `SetCR0` 清 PE、`SetVM(true)`、`SetEFlagsHi` 設 VM 都被拒。
5. 位址：仿 img `42AD`–`42BE` 的合成程式（ES 以舊表載入、換 GDTR、經 ES 寫新表）證明以索引取快取；`67` 定址的有效位址 `> FFFFh` 在分段模式可用、
   在實模式仍記 fault（ModRM32 與 `moffs32` 各一例）；`67` 定址的 dword 寫入跨過 `FFFFh` 以 32 位元相加；向上與向下展開界限邊界各一例（剛好在內、剛好超出）；
   寫入唯讀段、讀不可讀碼段、經不可用快取存取失敗即關閉；`lea` 超出界限不失敗；`CS` 基底非零時取指令正確；`FS`／`GS` 前綴走各自的快取。
6. 段載入：§5.1 每條檢查各一個失敗例與一個成功例（含 `CS` 空選擇子、`04h` 不算空）；GDT 與自建 LDT 各一例；表界限邊界；accessed 位元不被寫回；
   失敗時段值與快取不變；§5.1 表中每個位置在分段模式下都經 `loadSeg`（逐一以合成程式觸發；`Interrupt`、`FarCall` 兩列改驗直接呼叫時失敗即關閉）；
   `pop cs`、`mov cs`、`iret` 失敗即關閉；分段模式下 `SetPMCPL(3)` 被拒。
7. 指令：§6 每列至少一例；`lldt` 後以 `TI=1` 選擇子載入成功（自建 LDT）；`mov cr0` 設 PG 不回哨兵。§5.3：分段模式下 `INT`、`IntHook` 攔截的 `INT 21h`、
   `div 0` 失敗即關閉；IRQ0、按鍵、`startCallback`、`periodic` 延後且不消耗；`hlt` 失敗；`IF=1` 且有待處理 IRQ 一百萬步後失敗（`IF=0` 時不失敗）。
8. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄 load average；若主機仍不安靜，
   收據記錄全部樣本與最快值，本項與 432、433 同列開放，不阻擋其他驗收。
9. 突變自檢：以段值查找快取、略過界限檢查、略過型別檢查、略過 `loadSeg`（§5.1 表中任選三個位置各自突變）、失敗時仍寫入段值、寫回 accessed 位元、
   `decodeModRM32` 在分段模式仍查 64 KB、`moffs32` 在分段模式截斷、`startCallback` 在分段模式照常執行，各至少一個測試失敗。
10. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`。先確認 A20 已開（SCCD 映像 `0x1B80`–`0x1B97`
    的 8042 序列，430 §2.4），再以 `-regs-at` 量出第一次到達 img `4245` 的步數 N，以 `-dump-mem-at` 在步數 N 傾印。期望（`X` 為實測值）：
    - 複製迴圈後（步數 N，在 `4249` 寫舊 GDT 之前）：線性 `X`–`X+9FFFh` 與線性 `20000h`–`29FFFh` 逐位元組相同（此時 PG=0，線性即實體）。
    - 新 GDTR＝(`X+1400h`, `12Fh`)，新 IDTR＝(`X+1530h`, `7FFh`)。
    - `jmp 18h:43A7` 之後 `CS=0018h`，快取基底 `X`、界限 `FFFFFh`、D=0。
    - 停在 img `0x4586` 的 `ltr`：診斷含 `0018:4586` 與「S4」，`PCLinear()`＝`X+4586h`；`CR0=80000011h`；`CR3` 4 KB 對齊且 ≥ `X+8000h`。
    - 副本 GDT（線性 `X+1400h` 起 `130h` bytes）的項 `18h`、`20h`、`50h` 基底為 `X`，`80h` 為 `X+7830h`，`F0h` 為 `X+1D30h`。
    - 記錄（不斷言）：總步數；停點時實體 `0`–`FFFh` 是否與交棒時相同（`573F` 頁 0 別名的推論為假說）；`CR3` 所指 PDE[0..3] 與前幾個 PTE。
    - 若在此之前停在別的指令，照實記錄停點、原因與步數。
11. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

## 10. 實作收據（2026-10-16）

commit：`c64ca5f`（階段 1：分段狀態、`PM` 版本 3、以 `enterSegmented` 取代 433 交棒接收端）、`84d8799`（階段 2：段索引與 32 位元偏移、`loadSeg`、
`SysBus` 增補）、`7a1817a`（階段 3：中斷與回呼延後、`0F 00` 分流、觀測、I/O 與 `hlt`）。

### 驗收結果

- 第 1–7、9、11 項：容器內 `gofmt -l` 無輸出，`go build`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle`
  13 個套件全部 ok；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變；錨點測試通過；`internal/cpu386`、`internal/state` 無改動。每階段由主代理在獨立容器重跑後提交。
- 第 9 項突變自檢：11 個突變全被測試抓到，已還原。
- 第 8 項效能：profile 關閉的 `BenchmarkStepRealMode` 對 `a846473` 交錯兩輪各 10 對。第 1 輪最快值 old 31.24、new 33.98 ns/op（91.9%）；
  `operand` 壓成 16 bytes 後第 2 輪 old 28.62、new 29.12（98.3%），單對比值 77.5%–112.8%；1 分鐘 load average 10–12，15 分鐘約 30。
  **未能證明穩定不低於 97%**，與 432、433 同列開放。
- 第 10 項觸發案例（`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`）：
  - 執行 104587 道後停止：`0018:4586 … 0F 00 /3 LTR：TSS、任務切換與進 V86 屬 S4，失敗即關閉`。img `4245` 第一次到達是第 63332 步，`43A7` 是第 63422 步。
  - `X`＝`110000h`。第 63332 步時線性 `X`–`X+9FFFh` 與 `20000h`–`29FFFh` 逐位元組相同。
  - 停點前一步（第 104585 步存檔）：`CR0=80000011h`、`CR3=0011C000h`（≥ `X+8000h`）、GDTR＝(`111400h`, `12Fh`)、IDTR＝(`111530h`, `7FFh`)、
    `segmented=1`；CS、DS、ES、SS 快取基底 `X`（CS 屬性 `009Ah`、界限 `FFFFFh`；SS `0096h`、界限 0），FS、GS 基底 0、界限 4 GB（選擇子 `28h`）。
  - 副本 GDT：項 `08h` `X+1400h`、`10h` `X+1D98h`、`18h`／`20h`／`50h` `X`、`80h` `X+7830h`、`F0h` `X+1D30h`，皆符合。
  - 記錄：PDE[0..3]＝`118027h`、`119007h`、`11A007h`、`11B007h`；線性 `0`–`FFFh`（分頁後的線性視角）與交棒時相同。
  - `IF=0`，計時器與鍵盤中斷送出 0 次。

### 實作期間的處置

1. 版本 3 的欄位驗證放在 `decodePM`；「版本 1、2 帶 `PE=1`、`VM=0`」放在 `LoadState`。
2. 偏移寬度跟著運算元：只有分段模式的 ModRM32 與 `moffs32` 以 32 位元相加；堆疊、字串、`enter`、取指令一律 16 位元環繞。
3. 多 byte 存取先檢查全部 byte，任一失敗則整筆不讀不寫。
4. `SS` 的 B=1 只在堆疊原語檢查。
5. 遠指標載入先讀完兩個欄位再載入段暫存器，失敗時通用暫存器也不寫；遠呼叫先載入 CS 再推舊的 `CS:IP`。
6. 只可執行、不可讀的碼段可以載入 CS；取指令只查界限。
7. 一百萬步計數放在已會進入的延後分支；待處理條件只看 `irq0Pending` 與鍵盤佇列，不看 `picMask`。
8. I/O 檢查不通過時讀回 `FFh`、寫入丟棄並記錯。
9. `operand` 的段索引為 `uint8`，整個值 16 bytes。
10. `sldt`、`str` 一律寫 16 位元。

### 已知限制（probe）

- `-dump-mem-at` 的位址是「IDA 位址」，即線性位址加 `EF00h`（本次載入的映像基底）；收據的位址已換算。
- `-dump-mem` 只涵蓋 1 MB 內的 `m.Mem`，給 1 MB 以上的範圍會 panic（`cmd/probe/main.go` 的 `writeOneMemDump`，既有問題，未改）。
- `BreakAt` 存 `seg*16+off` 不遮罩，`atBreak` 在實模式以 `cpu.Addr`（20 位元）比對，兩者在 1 MB 以上不一致（既有問題，未改）。

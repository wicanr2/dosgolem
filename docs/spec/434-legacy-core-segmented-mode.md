# 434 — 既有核心的分段模式（16 位元保護模式）

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移（`strike_commander` `docs/re/003` §1），
執行期 JEMM 映像載入在段 `2000h`，實模式時 `2000:xxxx` 的偏移即 img 位址。

---

## 1. 為什麼要做

433 的觸發案例停在 JEMM img `0x4203`（`mov cr0,eax` 設 PE）。之後到 `ltr` 之前，JEMM 在保護模式執行 16 位元碼（`docs/re/004` §1）：

| img 範圍 | 內容 | CS |
|---|---|---|
| `4206`–`43A2` | `lldt`、載入 `SS`／`ES`／`DS`、把 `0xA000` bytes 自身映像複製到 XMS、改寫 GDT 基底、`lidt`、`lgdt`、重載段暫存器 | 實模式殘留快取（基底 `20000h`） |
| `43A2` | `jmp 18h:43A7` | 選擇子 `18h` |
| `43A7`–`4583` | 堆積（`62B5`）、頁表與 `CR0.PG`（`53CE`）、頁 0 別名（`573F`）、填 SCCD 結構 | `18h` |
| `4586` | `ltr cx`（`CX=F0h`），之後 `NT=1` 的 `iretd` 進 V86 任務 | `18h` |

路徑上約 50 種指令，含 `66`／`67`／`64` 前綴、`enter`／`leave`、`popad`、`loop`、`lods`（`docs/re/004` 的 `sys_jemm_annot.tsv` 與 img 反組譯）。
依 431 第 2 版，這段由既有核心以「分段模式」執行。本規格的驗收停在 img `0x4586` 的 `ltr`（任務暫存器屬後續規格 S4）。

### 1.1 交棒當下的狀態（已證實，433 收據之後的 probe）

`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile` 停在 `2000:4206` 時：

- `[2000:3EA9]`（XMS 區塊線性位址）＝ `00110000h`。
- 第一份 GDT（`lgdt` 16 位元形式，基底 `021400h`、界限 `012Fh`）的相關項：

| 選擇子 | 基底 | 界限 | access | 旗標（G、D/B） | 說明 |
|---|---|---|---|---|---|
| `08h` | `021400h` | `0007h` | `82h` | 0 | LDT |
| `10h` | `021D98h` | `2068h` | `8Bh` | 0 | 386 TSS（忙碌） |
| `18h` | `020000h` | `FFFFFh` | `9Ah` | 0 | 16 位元碼段，可讀 |
| `20h` | `020000h` | `FFFFFh` | `92h` | 0 | 資料，可寫 |
| `28h` | `0` | `FFFFFh` | `92h` | G=1 | 資料，4 GB |
| `50h` | `020000h` | `0` | `96h` | 0 | 資料，向下展開，可寫，B=0 |
| `F0h` | `021D30h` | `0067h` | `89h` | 0 | 386 TSS（可用） |

access byte 的 accessed 位元在這些項都是 0。

## 2. 分段模式的狀態

既有核心（`internal/cpu.CPU`）新增：

- `segmented bool`：分段模式旗標。只由 Machine 經方法設定（§3），核心不自行切換。
- `Cache [6]SegCache`：段描述元快取，索引與既有常數相同（`ES`、`CS`、`SS`、`DS`、`FS`、`GS`，`internal/cpu/cpu.go:33-46`）。
  `SegCache` 由 `internal/machine` 移到 `internal/cpu`（欄位與 433 §3.2 的 `Attr` 編碼不變），machine 以型別別名沿用。

`segmented=false` 時 `Cache` 不被讀取，所有路徑與 433 完成時相同。

`EFLAGS.VM=1`（V86）時 `segmented` 必須為 false；V86 由 435 與 S4 處理。本規格內 `segmented=true` 恆伴隨 `PE=1`、`VM=0`。

### 2.1 `PM` 版本 3

`encodePM` 改輸出版本 3：版本 2 的 80 bytes 之後接：

| 位移 | 長度 | 欄位 |
|---|---|---|
| 80 | 1 | `segmented`（0 或 1，其他值載入失敗） |
| 81 | 1 | 保留，必須為 0 |
| 82 | 60 | `Cache[0..5]`，每項 `Base` 4、`Limit` 4、`Attr` 2 |

共 142 bytes。`decodePM` 支援版本 1、2（`segmented=false`、快取全 0）與 3。`LoadState` 另驗：`segmented=1` 時 `PE=1`、`VM=0`；
各 `Attr` 的 bit 8–11、13 為 0。433 的測試要改：版本 2 的逐欄位位移測試改成「手造版本 2 可讀」；往返長度 80 改 142；
「未知版本號」改用 4；長度不足與尾端多餘 bytes 依版本 3 重新檢查。`StateDigest` 在 profile 開啟時因 `PM` 變長而改變，profile 關閉時不變。

## 3. 進入與離開

### 3.1 取代 433 的交棒接收端

433 的 `PMCore`、`Handoff`、`entered`、`modePM`、停擺（`halt`、`setHalt`、`rejudgeHalt`）與 `ErrPMCoreNotAvailable` 刪除，相關測試改寫為本節的行為。
`cpu.ErrModeSwitch` 與 433 §5 的 PE 邊緣判定保留。

`Machine.Step` 收到 `ErrModeSwitch`（`finishStep`）時：

1. 依 432 §10 第 1 項先處理分頁紀錄；同一步有紀錄時，仍執行第 2 步，再回 `*PageFaultError`。
2. 呼叫 `m.enterSegmented()`：以實模式段值設定六個快取（基底＝段值×16、界限 `FFFFh`，CS 屬性 `009Bh`，其餘 `0093h`，與 433 §5 相同），
   設 `segmented=true`，`SetPMCPL(0)`。
3. 沒有分頁紀錄時這一步回 nil。

`enterSegmented` 也呼叫 `syncCodeFastPath`：`segmented=true` 時 `CPU.Code` 一律為 nil（取指令要加 CS 基底）。

### 3.2 離開

- `mov cr0` 使 PE 由 1 變 0：失敗即關閉（S9），`CR0` 不改。
- `VM` 由 0 變 1 的路徑（`iretd`、任務切換）在本規格都失敗即關閉，所以 `segmented` 不會在本規格內變回 false。
- `Restore`、`LoadState` 依 `PM` 的 `segmented` 欄位還原，不重新推導。
- `SetProtectedModeProfile` 切換時 `segmented=false`、`Cache` 清零。

## 4. 位址計算

分段模式下，每次記憶體存取以**指令指定的段暫存器索引**（預設段或段前綴）取該暫存器的快取，不以段值查找。
兩個段暫存器可以持有同一選擇子而快取不同（JEMM img `4257`–`425C` 之間，`ES` 已以改寫後的 GDT 重載為基底 `110000h`，`DS` 仍是
舊快取 `20000h`），所以以段值查找是錯的。

- 線性位址 ＝ `Cache[i].Base + offset`（32 位元環繞），再交給 432 的線性層（分頁、A20）。
- 偏移寬度：16 位元定址時 16 位元，各 byte 各自環繞（同既有核心 `read16` 的規則）；`67` 前綴時用 ModRM32 的完整 32 位元有效位址，
  **不套實模式的 64 KB 檢查**（`modrm.go` 的 `decodeModRM32` 目前在 `ea > FFFFh` 時記 fault，分段模式改查段界限）。
- 界限檢查（刻意從嚴：DOSBox-X 預設 `do_seg_limits` 關，不查界限，`cpu.cpp` 的 `Segs.limit` 設為 `~0`）：
  - 向上展開：存取的每個 byte 偏移 ≤ `Limit`。
  - 向下展開：偏移 > `Limit`，且 ≤ `FFFFh`（B=0）或 `FFFFFFFFh`（B=1）。
  - 違反時失敗即關閉，診斷寫明段暫存器、選擇子、偏移、界限。`#GP`／`#SS` 的遞送屬 S6。
- 型別檢查：寫入唯讀資料段或碼段、讀取不可讀碼段、經空選擇子存取，皆失敗即關閉。
- 取指令：`Cache[CS].Base + IP`，`IP` 16 位元環繞；`CS` 屬性 D=1 不會出現（§5）。
- 堆疊：`SS` 屬性 B=0 時用 `SP`；B=1 失敗即關閉（JEMM 的 `SS` 都是 B=0）。
- `segmented=false` 時位址計算與 433 完成時逐位元組相同；實模式效能門檻見 §9 第 8 項。

## 5. 段暫存器載入

分段模式下，既有核心所有寫段暫存器的位置都經同一個方法 `loadSeg(i int, sel uint16) bool`。目前的寫入點：
`ops.go:120`、`125`、`129`、`133`（`pop sreg`）、`228`–`240`（`mov sreg`，含 `FS`／`GS`）、`264`、`451`、`756`、`762`（遠跳與遠呼叫）、
`324`、`329`（`les`／`lds`）、`340`、`345`、`356`（`retf`／`iret`）；`ops0f.go:64`、`66`（`pop fs/gs`）、`147`–`151`（`lss`／`lfs`／`lgs`）；
`ops386.go:168`–`172`（32 位元 `pop sreg`）；`cpu.go:344`（`Interrupt`）、`371`（`FarCall`）；`ops186.go:46`、`wide.go:290`、`cpu.go:383`（回到指令起點）。
實作時再以 grep 確認沒有遺漏，收據列出最終清單。

`segmented=false` 時 `loadSeg` 只寫段值，行為不變。`segmented=true` 時：

1. 讀描述元：`TI=0` 查 GDT（`GDTR`），`TI=1` 查 LDT（`LDTR` 快取）；索引×8＋7 超過表界限即失敗。描述元以監督者身分經 432 的線性層讀取
   （432 §補充驗收第 7 項）。
2. 檢查（照 DOSBox-X `CPU_SetSegGeneral`，`cpu.cpp:2976-3060`；違反時失敗即關閉，不送例外）：
   - `SS`：選擇子非空；RPL＝CPL＝DPL；可寫資料段；present。
   - `DS`／`ES`／`FS`／`GS`：空選擇子允許，快取標成不可用（之後經它存取失敗即關閉）；否則為資料段或可讀碼段，非一致碼段時 RPL ≤ DPL 且 CPL ≤ DPL；present。
   - `CS`（遠轉移）：照 `CPU_JMP` 的非一致碼段分支（`cpu.cpp:1785-1830`）：RPL ≤ CPL、DPL＝CPL；present。一致碼段、TSS、閘門與 D=1 碼段
     本規格不支援，失敗即關閉。新 `CS` 值為 `(sel &^ 3) | CPL`。
3. 寫入段值與快取。**不寫回 accessed 位元**（DOSBox-X 不寫，`CPU_SetSegGeneral` 只讀描述元）。
4. 本規格只支援 CPL 0；`CS` 載入後 `SetPMCPL(新 CS 的 RPL)`，結果不為 0 時失敗即關閉（ring 3 保護模式屬後續）。

`Interrupt`、`FarCall` 與 `IntHook` 在分段模式下不可用：`doInt`、例外、`tick`／`keyTick`／`timedKeyTick` 送 IRQ 與回呼前檢查
`segmented`，為真時失敗即關閉（`doInt` 的檢查在 `IntHook` 之前，與 433 §2 的 `IDTR` 守門同一位置）。IRQ 依 431 §3 留在待處理：
`segmented=true` 且 `IF=0` 時，`tick` 等不送也不報錯；`IF=1` 且有待處理 IRQ 時才失敗即關閉（IDT 遞送屬後續）。JEMM 這段 `IF=0`
（433 收據）。

## 6. 指令

分段模式下，下列指令依保護模式語意執行；其餘指令沿用既有實作，只改位址計算（§4）與段載入（§5）。

| 指令 | 分段模式行為 | DOSBox-X |
|---|---|---|
| `EA ptr16:16`、`FF /5`（16 位元） | 遠跳，§5 的 `CS` 規則 | `CPU_JMP` |
| `9A`、`FF /3`（16 位元） | 同特權遠呼叫：推 `CS`、`IP` 各 2 bytes，§5 的 `CS` 規則 | `CPU_CALL` 的非一致碼段同特權分支 |
| `CB`、`CA iw`（16 位元） | 同特權遠返回；彈出的 `CS` RPL ≠ CPL 失敗即關閉 | `CPU_RET` |
| `66` 形式的遠轉移與 `66 CB` | 失敗即關閉（JEMM 這段沒有） | |
| `CF`、`66 CF` | 失敗即關閉（`iretd` 屬 S4、435） | |
| `0F 00 /2`（`lldt r/m16`） | CPL 0；空選擇子使 LDTR 不可用；否則須為 GDT 中的 LDT 描述元（type 2）且 present，載入 `LDTR` 快取 | `CPU_LLDT` |
| `0F 00 /3`（`ltr`） | 失敗即關閉（S4）。**本規格的驗收停點** | |
| `0F 00 /0`、`/1`（`sldt`、`str`） | 讀 `LDTR`、`TR` 選擇子 | |
| `0F 00 /4`、`/5`、`0F 02`、`0F 03` | 失敗即關閉（JEMM 沒用） | |
| `0F 01` 群組、`mov CRn`／`DRn`、`clts` | 433 的處理函式，特權檢查依 CPL（`Sys.User()`） | |
| `mov cr0` 設 `PG` | 經 `SetCR(0)`（432 的 `SetCR0` 重算 `paging`），不回哨兵 | |
| `pushf`／`popf`／`66` 形式 | 431 §4.1 的保護模式規則 | `cpu.cpp:774-788` |
| `hlt` | 失敗即關閉（431 §3） | |
| `in`／`out`／字串 I/O | CPL ≤ IOPL 時照既有路徑；否則失敗即關閉（I/O 許可點陣圖屬 S4） | |
| x87 ESC | `FNINIT`、`FNSTCW`、`FLDCW`、`FWAIT` 以外失敗即關閉 | |

`67` 前綴下的隱含位址指令（字串、`xlat`、`loop` 用 `ECX`）目前失敗即關閉（`ops386.go:69-96`），本規格不補；JEMM 路徑上的 `loop`、`lods`
不帶 `67`。

## 7. 觀測

分段模式下 `CS` 是選擇子，`cpu.Addr(Seg[CS], IP)` 不是線性位址。新增 `Machine.PCLinear() uint32`：分段模式回 `Cache[CS].Base + IP`，
否則回 `cpu.Linear(Seg[CS], IP)`。下列以段值換算 `CS:IP` 的消費者改用它：

| 位置 | 用途 | 分段模式的處置 |
|---|---|---|
| `internal/machine/machine.go:1473` | `Coverage` | 改用 `PCLinear()`；超出 `len(Coverage)` 時不記 |
| `internal/machine/probe.go:215` | `atBreak` 線性斷點 | 改用 `PCLinear()` |
| `cmd/probe/main.go:765` | `CS:IP` 落入視訊區的護欄 | 改用 `PCLinear()` |

以 `seg:off` 文字比對的觀測（`-regs-at`、`TraceSegs`、`insnCS`／`insnIP`、`OpAddr()`）照舊，分段模式下的值是「選擇子:偏移」。
`oracle`、`session`、LE 與 DPMI 路徑不在 `-pm-profile` 下使用，不改；其中以段值換算線性位址的位置（`oracle/run.go:122` 等）在
分段模式下不保證，寫入本規格的已知限制。

## 8. 不在範圍

`iretd` 與任務切換、`ltr`、TSS、V86 進入（S4）；`#GP`／`#SS`／`#NP`／`#PF` 的遞送（S6、435）；保護模式 IDT 遞送與 IRQ（435）；
32 位元碼段與 `66` 遠轉移；CPL 3 保護模式；一致碼段、閘門；accessed 位元寫回；回實模式（S9）。

## 9. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 與 LE 路徑 `git diff` 為空。
2. SingleStepTests 以掛載的語料實際執行、315 個子測試全過、0 skip，`knownGaps` 不變；掛載方式同 433 §8.2。
3. `PM` 版本 3：逐欄位位移；手造版本 1、2 可讀；§2.1 的驗證各一例；`Snapshot`／`Restore` 與 `SaveState`／`LoadState` 對 `segmented` 與六個快取往返；
   載入 `segmented=1` 的狀態後 `Step` 以分段模式繼續（不停擺）。
4. 進入：合成程式在實模式設 PE 後，`segmented=true`、快取為實模式推得值、`CPL=0`、`Code=nil`；同一步有分頁紀錄時回 `*PageFaultError` 且已進分段模式。
5. 位址：DS 與 ES 持有同一選擇子、快取不同時，各自依索引存取（負對照：改成以段值查找會失敗）；`67` 定址的有效位址 `> FFFFh` 在分段模式可用、
   在實模式仍記 fault；向上與向下展開的界限邊界各一例（剛好在內、剛好超出）；寫入唯讀段、空選擇子存取失敗即關閉；`CS` 基底非零時取指令正確。
6. 段載入：§5 每條檢查各一個失敗例與一個成功例；GDT 與 LDT 各一例；表界限邊界；accessed 位元不被寫回；`CS` 載入 D=1 失敗即關閉；
   §5 列出的每個寫入點在分段模式下都經 `loadSeg`（以合成程式逐一觸發，或以突變自檢證明）。
7. 指令：§6 每列至少一例；`lldt` 載入後以 `TI=1` 的選擇子載入成功；`mov cr0` 設 PG 不回哨兵；`mov cr0` 清 PE 失敗即關閉；
   分段模式下 `INT`、`IntHook` 攔截的 `INT 21h`、`IF=1` 時的待處理 IRQ0 失敗即關閉，`IF=0` 時 IRQ0 留在待處理不報錯。
8. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄 load average。
9. 突變自檢：以段值查找快取、略過界限檢查、略過 `loadSeg`（任選三個寫入點各自突變）、寫回 accessed 位元、`decodeModRM32` 在分段模式仍查 64 KB、
   進入時不清 `Code`，各至少一個測試失敗。
10. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
    - 複製迴圈（img `4232`–`4243`）結束時，實體 `110000h`–`119FFFh` 與同一時點的線性 `20000h`–`29FFFh` 逐位元組相同（`-dump-mem-at` 同一步各傾印一份）。
    - `jmp 18h:43A7` 之後 `CS=0018h`，快取基底 `110000h`、界限 `FFFFFh`、16 位元。
    - 停在 img `0x4586` 的 `ltr`（`0018:4586`），診斷寫明 S4；此時 `CR0` 的 PE、PG 皆為 1，`CR3` 為 `53CE` 設定的頁目錄。
    - 若在此之前停在別的指令，照實記錄停點、原因與已執行步數。
11. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

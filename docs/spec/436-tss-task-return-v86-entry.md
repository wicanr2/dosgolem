# 436 — TSS、`ltr` 與任務返回進入 V86

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（第 2 版，§8 的 S4）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、
[`433-real-mode-system-instructions`](433-real-mode-system-instructions.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)

編號說明：本規格是 431 §8 的 S4。觸發案例先碰到 S4，才碰到 `435-v86-hooks-and-idt-delivery`，所以先做本規格；435 改以本規格為前置。

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移；`X` 為 XMS 區塊線性位址（本案 `110000h`，434 §1.1）。

---

## 1. 為什麼要做

434 的觸發案例停在 JEMM img `0x4586` 的 `ltr cx`（`CX=F0h`）。之後 JEMM 進 V86（img `4586`–`459F`）：

```text
4586  0f 00 d9      ltr  cx            ; TR ← F0h
4589  66 9c         pushfd
458b  66 58         pop  eax
458d  80 cc 40      or   ah,40h        ; NT
4590  66 50         push eax
4592  66 9d         popfd              ; EFLAGS.NT=1
4594  66 cf         iretd              ; NT=1：任務返回到 TSS F0h 的 back link
4596  fc            cld                ; 以下在 V86（CS=2000h）
4597  0f b2 26 97 3e lss sp,[3E97h]
459c  33 c0         xor  ax,ax
459e  fb            sti
459f  cb            retf               ; 回 SCCD
```

第 104585 步（`ltr` 前一步）的兩個 TSS（已證實，probe 傾印；欄位依 386 TSS 佈局）：

| 欄位 | TSS `F0h`（基底 `X+1D30h`，界限 `67h`，可用） | TSS `10h`（基底 `X+1D98h`，界限 `2068h`，忙碌） |
|---|---|---|
| back link | `10h` | 0 |
| `SS0:ESP0` | `50h:1400h` | `50h:1400h` |
| `CR3` | 0 | `0011C000h` |
| `EIP` | 0 | `4596h` |
| `EFLAGS` | `2` | `00023202h`（VM、IOPL=3、IF） |
| 通用暫存器 | 0 | 0，`ESP=200h` |
| `ES`／`CS`／`SS`／`DS`／`FS`／`GS` | `20h`×4，0，0 | 全部 `2000h` |
| `LDT` | `08h` | `BBh` |
| I/O 點陣圖位移 | `68h` | `68h`（點陣圖 `2000h` bytes，靜態全 0，`docs/re/004` §1） |

所以 `iretd` 讀 TSS `F0h` 的 back link `10h`，切到 V86 任務：`CS:IP=2000:4596`、`EFLAGS=00023202h`、`SS:SP=2000:0200`。之後的 V86 事件
（`INT n`、IRQ、例外）經 IDT 遞送屬 435。

## 2. 任務暫存器

433 §2 已有 `TR` 的選擇子與快取（存於 `PM`）。本規格不改 `PM` 格式。

`SysBus` 增加：`TR() (sel uint16, c SegCache)`、`SetTR(sel uint16, c SegCache)`、`WriteSup8(addr uint32, v uint8)`、`ReadSup16(addr uint32) uint16`、
`SetCR3`（以 `SetCR(3, v)` 既有方法即可，不另加）。

### 2.1 `ltr r/m16`（分段模式，CPL 0）

照 DOSBox-X `CPU_LTR`（`cpu/cpu.cpp:2452-2477`）：

1. 空選擇子（`(sel & FFFCh)==0`）：失敗即關閉（DOSBox-X 設為無效 TR 且不報錯；刻意從嚴，JEMM 不會這樣用）。
2. `TI=1` 或超出 GDT 界限：失敗即關閉（真機 `#GP`）。
3. 描述元須為可用的 386 TSS（S=0、type 9）；286 TSS 與忙碌 TSS 失敗即關閉（DOSBox-X 接受 286 可用 TSS；刻意從嚴）。
4. not present 失敗即關閉（真機 `#NP`）。
5. 載入 `TR` 選擇子與快取（基底、展開後界限、屬性），並把 GDT 中該描述元的 type 設為忙碌（access byte `|= 02h`，以 `WriteSup8` 寫回）。

434 §6 表中 `ltr` 一列改為本節行為。

## 3. 任務返回

### 3.1 觸發

分段模式下執行 `CF` 或 `66 CF`，且 `EFLAGS.NT=1`：核心不改任何狀態，回傳哨兵 `cpu.ErrTaskReturn`（`errors.Is` 可判），`IP` 前進到下一道
（`opIP`＋指令長度），供 Machine 存入舊 TSS 的 `EIP`。`NT=0` 的 `iret`／`iretd` 仍失敗即關閉（435）。DOSBox-X 的任務返回不看運算元大小
（`cpu.cpp:1585-1597`）。

### 3.2 Machine 的處置（`finishStep` 收到 `ErrTaskReturn`）

步驟照 DOSBox-X `CPU_SwitchTask` 的 `TSwitch_IRET` 路徑（`cpu.cpp:905-1091`）；任一檢查失敗即關閉，且在第 4 步之前失敗時不改任何狀態：

1. 目前 `TR` 必須有效（DOSBox-X `CPU_CHECK_COND`，真機 `#TS`）。目前 TSS 不忙碌時 DOSBox-X 只記 log，本規格照做（不失敗）。
2. 讀 back link：`ReadSup16(TR.base)`。
3. 新 TSS：`TI=0`、在 GDT 界限內、386 TSS、**忙碌**（type `B`）、present；否則失敗即關閉（DOSBox-X `E_Exit`）。讀新 TSS 的 `CR3`、`EIP`、`EFLAGS`、
   八個通用暫存器、六個段值與 `LDT`（偏移依 386 TSS 佈局；以 `ReadSup*`）。
4. 清目前 TSS 描述元的忙碌位元（access byte `&^= 02h`，寫回 GDT）。
5. 把目前狀態寫入目前 TSS：`EFLAGS`（完整 32 位元，清 `NT`）、`EIP`（第 3.1 節的下一道）、八個通用暫存器（32 位元）、六個段值（各寫 32 位元，高半 0，
   同 DOSBox-X `mem_writed`）。不寫 `CR3`、`LDT`、back link。
6. 新選擇子等於目前 `TR` 時照 DOSBox-X 的特例處理（不載入新內容）；本案不會發生，失敗即關閉即可（刻意從嚴）。
7. 新 `CR3` 與目前不同時 `SetCR(3, newCR3)`（432 清 TLB）；相同時不動（DOSBox-X 同）。
8. 載入新內容：
   - `EFLAGS` 的 `VM=1`（本規格只支援這種）：新 `EIP` 高 16 位元必須為 0，否則失敗即關閉。依序 `SetSegmented(false)`、清六個段快取、`SetEFlagsHi`（`VM`、`RF`）、
     核心 `SetFlags`（低 16 位元，含 `IOPL`、`NT`、`IF`）、`IP`、通用暫存器（低半進 `R`，高半進 `Hi`）、六個段值（直接寫，V86 以段值×16 定址）。
     `pmCPL` 不改（V86 的 CPL 由 432 從 `VM` 推得為 3；DOSBox-X `CPU_SetCPL(3)`）。
   - `LDT`：VM=1 時不載入 `LDTR`（照 DOSBox-X，`cpu.cpp:1028-1033` 只在非 V86 任務呼叫 `CPU_LLDT`；386 真機會載入 `BBh`。刻意對齊 oracle，
     JEMM 之後若以 `TI=1` 選擇子載入段，屆時再核對）。
   - `EFLAGS` 的 `VM=0`（保護模式任務）：失敗即關閉（本案沒有）。
9. `TR` 改為新選擇子，快取取自新描述元（仍為忙碌）。
10. 這一步回 nil；同一步有分頁紀錄時依 432 §10 第 1 項回 `*PageFaultError`，且第 4 步之後的寫入要能被分頁錯誤中斷：描述元與 TSS 的讀取全部在
    第 4 步之前完成，第 4、5 步的寫入目標頁在第 3 步先以讀取探測（`ReadSup8` 各一次）；探測有分頁紀錄就在第 4 步前停止。

434 §3.2 的守門（分段模式下拒絕設 `VM`）由第 8 步先 `SetSegmented(false)` 再設 `VM` 滿足。

## 4. V86 執行（本規格範圍內）

進入 V86 後由既有核心以 432 的 V86 語意執行（段值×16、經線性層、CPL 3 的使用者分頁權限）。本規格補下列規則，其餘屬 435：

| 項目 | 規則 | DOSBox-X |
|---|---|---|
| `cli`、`sti` | IOPL=3 時照常；IOPL<3 失敗即關閉（`#GP` 遞送屬 435） | `CPU_CLI`／`CPU_STI` |
| `pushf`／`pushfd` | IOPL=3 時照常，`pushfd` 推入的高半清 `VM`、`RF`（431 §4.1）；IOPL<3 失敗即關閉 | `cpu.cpp:1560`、`1572` 一帶 |
| `popf`／`popfd` | IOPL=3 時照常，但不改 `IOPL`（bits 12–13）、不改 `VM`、`RF`；IOPL<3 失敗即關閉 | `CPU_POPF`，`cpu.cpp:774-797` |
| `iret`／`iretd` | IOPL=3 時從 V86 堆疊彈出，旗標只改 `FMASK_NORMAL|NT`（不改 `IOPL`）；IOPL<3 失敗即關閉 | `cpu.cpp:1533-1572` |
| `in`／`out`／字串 I/O | 依 `TR` 的 I/O 點陣圖：讀 `TSS+66h` 的位移，超過 TSS 界限或點陣圖中對應位元為 1 時失敗即關閉；否則照常 | `CPU_IO_Exception`，`cpu.cpp:1093-1111` |
| `INT n`、`INT3`、`INTO`、`BOUND` 越界、除以零 | 失敗即關閉，診斷註明 435（`doInt` 的檢查仍在 `IntHook` 之前） | |
| `hlt`、特權指令（`mov crN`、`lgdt` 等） | 失敗即關閉（433 已依 `Sys.User()` 處理） | |
| IRQ（`IF=1` 且有待處理 IRQ0 或按鍵） | 失敗即關閉，診斷註明 435；`IF=0` 時留在待處理 | |
| `startCallback`、`periodic` 的 `FarCall` | 延後，不執行、不消耗（同 434 §5.3） | |

`ErrTaskReturn` 之後 `segmented=false`、`VM=1`，434 的分段模式閘門不再適用；上表的 Machine 側閘門以 `VM=1` 且 profile 開啟為條件。

## 5. 不在範圍

V86 事件經 IDT 遞送、`#GP` 反射、從保護模式 `iretd` 回 V86（435）；保護模式任務、`jmp`／`call` 到 TSS 或任務閘、286 TSS；`#TS`／`#NP` 的遞送；
V86 下的 x87；回實模式（S9）。

## 6. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 無改動；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變。
2. `ltr`：§2.1 每條各一例（空選擇子、`TI=1`、超出界限、資料段、286 TSS、忙碌 TSS、not present 失敗；可用 386 TSS 成功）；成功後 GDT 描述元變忙碌、
   `TR` 快取正確；失敗時 `TR` 與 GDT 不變。
3. 任務返回：合成 GDT 與兩個 TSS（仿 §1 的表）：
   - 執行後 `VM=1`、`segmented=false`、`CS:IP`、`SS:SP`、`EFLAGS`、通用暫存器（含高半）、六個段值與新 TSS 相同；`TR` 為新選擇子；
   - 舊 TSS 描述元不再忙碌，新 TSS 仍忙碌；
   - 舊 TSS 的 `EFLAGS`（無 `NT`）、`EIP`（`iretd` 的下一道）、通用暫存器與段值（32 位元寫入）正確，`CR3`、`LDT`、back link 未被寫；
   - `CR3` 相同時不清 TLB、不同時清；`LDTR` 不變；
   - §3.2 每個失敗條件各一例（`TR` 無效、back link 指向非忙碌 TSS、`TI=1`、超出界限、新 TSS `VM=0`、新 `EIP` 高半非 0），失敗時狀態不變；
   - `16` 位元 `CF` 帶 `NT=1` 同樣任務返回；`NT=0` 的 `iret` 仍失敗即關閉；
   - 寫入目標頁不存在時在第 4 步前停止、狀態不變。
4. V86：§4 每列至少一例（IOPL=3 與 IOPL<3 各一）；`popf` 不改 IOPL；`pushfd` 高半清 VM；I/O 點陣圖位元為 0 照常、為 1 失敗、位移超過界限失敗；
   IRQ0 在 `IF=1` 失敗、`IF=0` 延後；回呼延後不消耗。
5. `PM` 往返：任務返回後 `SaveState`／`LoadState` 與 `Snapshot`／`Restore` 還原 `VM=1`、`TR`、`segmented=false`，`Step` 照 V86 繼續。
6. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄；主機不安靜時與 432–434 同列開放。
7. 突變自檢：`ltr` 不設忙碌、任務返回不清舊 TSS 忙碌、舊 TSS 的 `EFLAGS` 不清 `NT`、存入的 `EIP` 用指令起點、`popf` 在 V86 改 IOPL、略過 I/O 點陣圖、
   V86 的 `INT` 在 `IntHook` 之後才檢查，各至少一個測試失敗。
8. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
   - `ltr` 之後 `TR=F0h`，GDT 中 `F0h` 的 access byte 為 `8Bh`；
   - `iretd` 之後 `VM=1`、`CS:IP=2000:4596`、`SS:SP=2000:0200`、`EFLAGS=00023202h`、`TR=10h`；GDT 中 `F0h` 回到 `89h`、`10h` 仍為 `8Bh`；
     TSS `F0h` 的 `EIP=4596h`、`EFLAGS` 無 `NT`、`CS=18h`；
   - 之後停在第一個需要 435 的事件（預期是 `IF=1` 後的待處理 IRQ0，或 SCCD 的第一個 `INT`）；照實記錄停點、原因、步數與當時的 `CS:IP`。
9. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

# 431 — 混合 CPU：實模式與 V86 用既有核心，保護模式用 cpu386

狀態：**DRAFT**（第 1 版）
日期：2026-10-15
前置：[`429-dos-sft-chain`](429-dos-sft-chain.md)、[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`197-cpu386-real-mode-subset`](197-cpu386-real-mode-subset.md)、[`014-dos4gw-flat-descriptors`](014-dos4gw-flat-descriptors.md)

---

## 1. 為什麼要做

《Strike Commander》光碟版要在 Origin 的 JEMM 底下執行。JEMM 從實模式自己切進保護模式，建 GDT、IDT、頁表與 TSS，
再以硬體任務返回進入 V86，之後所有 DOS 程式（`MENU.EXE`、`STRIKE.EXE` 等）都在 V86 裡跑（證據：
`strike_commander_cht_dosgolem` 的 `docs/re/003-jemm-api.md`、`004-cpu-requirements.md`、`006-dosboxx-oracle-smoke.md`）。

使用者的定案（2026-10-15，記於該專案 `AGENTS.md` §2）：

- 不就地重整 `internal/cpu`，擴充 `internal/cpu386` 負責保護模式。
- 混合執行：`EFLAGS.VM=1` 時由既有 `internal/cpu` 執行，`INT n`、IRQ、例外交回 cpu386 走 IDT。

本規格定義兩個核心的分工、共享狀態與交接點。它是架構規格：指令覆蓋（cpu386 的 16 位元碼段、TSS 任務切換、分頁錯誤等）
另立規格，依本規格的介面逐步補。

### 實模式交給誰

觸發案例在實模式就碰到系統指令：SCCD 的 `smsw ax`（映像 `0x1FE9`），JEMM 在切保護模式前的 `sgdt`、`sidt`、`lgdt`、`lidt`
與 `mov eax,cr0`／`mov cr0,eax`（JEMM img `0x41E8`–`0x4203`）。有兩種做法：

- (a) 既有核心在實模式補這幾道系統指令；`mov cr0` 把 PE 設起時交棒給 cpu386。
- (b) 開啟本規格時由 cpu386 從程式開頭就執行實模式。

採 (a)。(b) 要 cpu386 執行整套 16 位元實模式程式，正是使用者否決「全部在 cpu386」的理由。

## 2. 啟用方式

`Machine.SetProtectedModeProfile(on bool) error`，預設關閉；`programLoaded`（`429`）為真時回錯誤；開啟時要求
`ExtendedMemoryKB() > 0`（`430`），否則回錯誤。關閉時，本規格的所有行為都不發生：既有核心遇到本規格新增的系統指令
仍照現狀失敗即關閉，既有收據不變。`cmd/probe` 加 `-pm-profile`，與 `-ext-mem-kb` 同樣在 `machine.New()` 之後、
`if *loadState == ""` 區塊外設定。

## 3. 執行模式與負責核心

| 模式 | 判定 | 執行的核心 |
|---|---|---|
| 實模式 | `CR0.PE=0` | 既有 `internal/cpu` |
| 保護模式 | `CR0.PE=1` 且 `EFLAGS.VM=0` | `internal/cpu386` |
| V86 | `CR0.PE=1` 且 `EFLAGS.VM=1` | 既有 `internal/cpu`，經 V86 轉接層（§6） |

任一時刻只有一個核心在執行。`Machine.Step` 依目前模式呼叫對應核心的 `Step`，每道指令計一步，兩個核心共用 `m.Steps`。

## 4. 共享狀態表

這張表是本規格的核心：每一項狀態只有一個擁有者，另一邊只看得到擁有者給的視圖。

### 4.1 由 Machine 擁有、兩個核心都讀寫

新增結構 `machine.SysRegs`（放 `Machine`，進 `Snapshot` 與狀態檔；狀態檔欄位依 `430` §3 的程序在「只加欄位」commit 加入）：

| 欄位 | 內容 | 初值 |
|---|---|---|
| `CR0`、`CR2`、`CR3` | 控制暫存器 | `CR0=00000010h`（FPU 存在；DOSBox-X `src/cpu/cpu.cpp:2582`、`3632-3634`），其餘 0 |
| `DR[8]` | 除錯暫存器（`DR4`／`DR5` 不用） | 0 |
| `GDTR`、`IDTR` | 基底 32 位元、界限 16 位元 | 基底 0、界限 0（`IDTR` 界限 `03FFh`，實模式中斷表） |
| `LDTR`、`TR` | 選擇子＋描述元快取（基底、界限、型別） | 0 |

- 寫入 `CR0`：一律補上 `00000010h`（同 DOSBox-X `cpu.cpp:2582`）。
- 寫入 `CR3`：重建 §6.2 的轉換表。
- 寫入 `CR0` 使 PE 改變、或 PG 改變：交接或重建轉換表，見 §5。

### 4.2 交接時整份搬移（擁有者是正在執行的核心）

| 狀態 | 既有核心 | cpu386 | 交接規則 |
|---|---|---|---|
| 通用暫存器 | `R[8]`（低 16）＋`Hi[8]`（高 16） | `R[8] uint32` | 逐一組合／拆開，高半不丟 |
| 指令位址 | `IP uint16` | `EIP uint32` | 交給既有核心時 `EIP` 高 16 必須為 0，否則失敗即關閉 |
| 旗標 | `Flags uint16` | `EFlags uint32` | 低 16 位元直接搬；高 16（`RF`、`VM`、`AC`）由 Machine 的 `EFlagsHi` 保存，既有核心執行期間不變。既有核心在本規格開啟時，旗標位元 12–14（IOPL、NT）照 386 語意保存，不強制成 8086 的值 |
| 段暫存器 | `Seg[4]`＋`FSeg`、`GSeg` | 選擇子＋描述元快取 | 見 §4.3 |

### 4.3 段描述元快取

- 實模式與 V86：快取由段值推得（基底＝段值×16、界限 `FFFFh`、16 位元、可讀寫）。
- 實模式→保護模式（§5.1）：交給 cpu386 時，六個段的快取照實模式推得的值建立，選擇子＝原段值。386 在 `PE=1` 之後、
  重新載入段暫存器之前，沿用舊快取繼續執行（JEMM img `0x4203`–`0x43A2` 依賴這點：`PE=1` 後仍以實模式 CS 執行，
  直到遠跳到 `18h:43A7`）。
- 保護模式→V86（§5.2）：由 cpu386 依 `iretd`／任務切換載入段值，既有核心只用段值。
- V86→保護模式（§5.3）：cpu386 從 GDT 讀 IDT 閘指定的碼段描述元建立 CS 快取；資料段依 386 規則清為 0。

## 5. 交接點

### 5.1 實模式→保護模式

既有核心執行 `mov cr0,r32`，寫入值的 PE 為 1：完成這道指令（`IP` 前進、`CR0` 更新）後回傳哨兵錯誤 `ErrModeSwitch`；
`Machine.Step` 收到後依 §4.2、§4.3 把狀態交給 cpu386，下一步起由 cpu386 執行。哨兵不是錯誤，不進診斷的失敗計數。

### 5.2 保護模式→V86

cpu386 以 `iretd` 彈出的 `EFLAGS.VM=1`，或任務切換載入的 TSS `EFLAGS.VM=1` 時，完成後交棒給既有核心。本規格只定義
交棒介面；`iretd` 到 V86 的堆疊格式與任務切換由後續規格（S4）實作。

### 5.3 V86→保護模式

既有核心在 V86 遇到下列事件時不自己處理，回傳 `V86Event{Kind, Vector, ErrorCode, InstrIP}`，由 Machine 交給 cpu386
依 IDT 遞送（堆疊切換到 TSS 的 `SS0:ESP0`，推 `GS`、`FS`、`DS`、`ES`、`SS`、`ESP`、`EFLAGS`、`CS`、`EIP`，有錯誤碼時再推
錯誤碼；中斷閘清 `IF`、`TF`、`NT`、`VM`、`RF`）。框架大小必須精確：JEMM 以 `cmp sp,13D6h` 判斷入口（`004` §1）。

| 事件 | 條件 | 遞送 | 回報給 cpu386 的 `EIP` |
|---|---|---|---|
| 軟體中斷 `INT n`（`CD nn`） | 一律（例外見 §6.3） | 依 IDT 第 n 項；閘 DPL < 3 時改遞 `#GP`，錯誤碼 `n*8+2` | 下一道指令 |
| `INT 3`（`CC`）、`INTO`（`CE`，OF=1） | 同上 | 同上（`INT 3` 向量 3、`INTO` 向量 4） | 下一道指令 |
| IOPL 敏感指令 | IOPL < 3 時的 `CLI`、`STI`、`PUSHF`、`POPF`、`IRET`、`INT n` | `#GP(0)` | 本道指令 |
| 特權指令 | `HLT`、`LGDT`、`LIDT`、`LMSW`、`CLTS`、`MOV CRn`、`MOV DRn`、`LLDT`、`LTR`、`INVD`、`WBINVD` | `#GP(0)` | 本道指令 |
| 其他 CPU 例外 | `#DE`、`#UD` 等 | 依向量 | 依 386 規則 |
| 硬體中斷 | PIC 有待處理 IRQ 且 `IF=1` | 依 IDT（PIC 映射的向量） | 下一道指令 |

IOPL=3 時 `CLI`、`STI`、`PUSHF`、`POPF`、`IRET` 在 V86 原生執行（JEMM 的 V86 任務 IOPL=3，`004` §1）。I/O 指令依 TSS 的
I/O 權限點陣圖判斷：對應位元為 1 時 `#GP(0)`，否則原生執行。上述 IOPL 語意引用前要以 Intel 386 手冊核對並寫入本節
（第 1 輪審查的待辦）。

### 5.4 保護模式→實模式

不在本規格範圍。cpu386 執行 `mov cr0` 清除 PE 時失敗即關閉，診斷寫明「需要 S9」。JEMM 的卸載路徑（`UN`）需要保留
非 `段×16` 基底的快取在實模式執行，既有核心表示不了。

## 6. V86 轉接層

既有核心在 V86 時，記憶體、`INT`、特權指令經由轉接層，不改既有核心的實模式路徑。

### 6.1 實作位置

既有核心新增可選的 `V86` 介面欄位（nil 時行為不變）：`OnInt(n, instrIP) (handled bool)`、`OnPrivileged(op) error`、
`OnIO(port, size, write) (allowed bool)`。Machine 在 V86 時裝上，在其他模式時移除。既有核心的 `Bus` 在 V86 時換成分頁
匯流排（§6.2）。

### 6.2 分頁匯流排（不產生錯誤）

`CR0.PG=1` 時，線性位址經 `CR3` 的兩層頁表轉成實體位址，再走 `ReadPhys*`／`WritePhys*`（`430` §2.1）。

- 轉換表：每 4 KB 一項的陣列，在寫 `CR3`、`CR0.PG` 改變時整份重建；平時查表不讀頁表。JEMM 每次改頁表後都重載 `CR3`，
  沒有 `invlpg`（`004` §4），所以這個快取時機與 JEMM 的用法一致。
- 範圍：V86 只需前 1 MB 加 HMA（`0x000000`–`0x10FFEF`）；cpu386 的監督者存取查同一套頁表，表外的位址逐次讀頁表。
- 頁不存在、或 V86 存取遇到 `U/S=0`、寫入遇到 `R/W=0`：本規格失敗即關閉，診斷寫明線性位址與 PTE。`#PF` 遞送另立規格，
  前提是量到 STRIKE 依賴範圍監看（`SR`）命中（`003` §6）。
- A20：`PG=1` 時不套 A20 遮罩（JEMM 自己映射 HMA）；`PG=0` 時沿用既有路徑。
- 取指令快路徑（`CPU.Code`）在 `PG=1` 時停用。

### 6.3 dosgolem 服務層在 V86 下

dosgolem 的 DOS／BIOS 服務是在既有核心執行 `INT n`、且向量指向 `StubSeg` 時，由 `IntHook` 接管。V86 下規則改為：

- `INT n` 的指令位址在 `StubSeg` 內，且 IVT 第 n 項指向 `StubSeg`：照舊由服務層處理，不交給 cpu386。這相當於 BIOS 本身
  在 V86 裡執行：遊戲的 `INT 21h` 先經 JEMM（`#GP`）反射到 IVT，跳進 `StubSeg` 的 trampoline，trampoline 的 `CD F2`
  才由服務層接手。
- 其他 `INT n`（包括遊戲程式自己的 `INT 21h`）：依 §5.3 交給 cpu386。

這條規則讓 JEMM 看得到所有它要攔的中斷（例如 `INT 15h AX=1209h`），同時服務層不必重寫。

### 6.4 計時器與鍵盤

`Machine.tick()` 在實模式照舊呼叫既有核心的 `Interrupt(08h)`。在保護模式與 V86 改為設定 PIC 待處理 IRQ，由目前執行的
核心在指令邊界依 §5.3 遞送。

## 7. 既有核心在實模式新增的系統指令

只在本規格開啟時生效；關閉時照現狀失敗即關閉。

| 指令 | 實模式行為 | 依據 |
|---|---|---|
| `smsw r/m16`（`0F 01 /4`） | 讀 `CR0` 低 16 位元 | DOSBox-X `CPU_SMSW` |
| `lmsw r/m16`（`0F 01 /6`） | 只改 `CR0` 低 4 位元，不能清除 PE；設 PE 時依 §5.1 交棒 | DOSBox-X `cpu.cpp` `CPU_LMSW` |
| `sgdt`／`sidt m`（`0F 01 /0`、`/1`） | 存界限（16 位元）與基底；16 位元運算元存 24 位元基底、第 6 byte 依 386 規則 | 待核對手冊 |
| `lgdt`／`lidt m`（`0F 01 /2`、`/3`） | 載入；16 位元運算元只取 24 位元基底 | 同上 |
| `mov r32,crN`、`mov crN,r32`（`0F 20`、`0F 22`） | 讀寫 `SysRegs`；寫 `CR0` 依 §4.1、§5.1 | DOSBox-X `CPU_SET_CRX` |
| `mov r32,drN`、`mov drN,r32`（`0F 21`、`0F 23`） | 讀寫 `SysRegs.DR` | |
| `clts`（`0F 06`） | 清 `CR0.TS` | |
| `lldt`、`ltr`、`sldt`、`str`（`0F 00 /0–/3`） | 實模式下為 `#UD`：失敗即關閉 | 386 規則 |

上表「待核對」的位元組格式在實作前以 Intel 386 手冊或 DOSBox-X 原始碼核對並補行號。

## 8. 不在範圍（後續規格）

| 編號 | 內容 |
|---|---|
| S2 | cpu386 執行 16 位元碼段（D=0）、從 GDT 讀描述元、遠轉移經選擇子、JEMM 需要的指令 |
| S4 | TSS、`ltr`、`iretd` 帶 NT 的任務返回 |
| S6 | `#PF` 遞送、`#DB` 與除錯暫存器觸發 |
| S9 | 保護模式→實模式 |
| — | VCPI、`invlpg`、4 MB 頁、`WP` |

## 9. 驗收

以合成程式驗證，不讀原版檔：

1. 關閉時不變：錨點測試（`internal/state/baseline_anchor_test.go`）常數不變；既有核心遇到 §7 的指令仍失敗即關閉。
2. 啟用條件：`ext-mem` 為 0 時開啟回錯誤；載入後開啟回錯誤。
3. 實模式系統指令：§7 每列一例（含 `smsw` 讀到 `0010h`、`lmsw` 不能清 PE、`lldt` 失敗即關閉）。
4. 實模式→保護模式：合成程式設 PE 後，下一道指令由 cpu386 執行；通用暫存器（含高半）、旗標、段值、`IP` 交接後一致；
   CS 快取為實模式推得的值。
5. 共享狀態不漂移：在兩核心間來回交接（以測試替身代替尚未實作的 S2／S4）後，`SysRegs`、高 16 位元暫存器與 `EFlagsHi`
   與預期值逐位相同；`Snapshot`／`Restore` 涵蓋 `SysRegs` 與目前模式。
6. V86 事件：§5.3 表每列一例，檢查回報的 `V86Event`（向量、錯誤碼、`InstrIP`）；IOPL=3 時 `CLI` 等原生執行；
   I/O 權限點陣圖位元為 1 時回報 `#GP(0)`。
7. 服務層規則（§6.3）：`StubSeg` 內的 `INT 21h` 由服務層處理；遊戲碼的 `INT 21h` 回報給 cpu386。
8. 分頁匯流排：`B0000`、`C0000`、HMA 的別名（`003` §3 的映射表）經轉換表讀寫到預期的實體位址；改頁表但不重載 `CR3`
   時沿用舊轉換（與快取時機一致）；頁不存在時失敗即關閉並帶線性位址。
9. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile` 通過 `smsw`，
   進 JEMM 後停在 S2 尚未支援的第一道保護模式指令。
10. `tools/go.sh test ./internal/... ./cmd/probe` 全綠。

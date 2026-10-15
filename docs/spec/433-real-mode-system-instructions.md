# 433 — 既有核心的實模式系統指令與實模式→保護模式交棒

狀態：**DRAFT**（第 1 版）
日期：2026-10-15
前置：[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（契約）、
[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)（已實作）、
[`197-cpu386-real-mode-subset`](197-cpu386-real-mode-subset.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

觸發案例在實模式就碰到系統指令：SCCD 的 `smsw ax`（映像 `0x1FE9`，dosgolem 目前停在這裡，`430` §6 收據），JEMM 切保護模式前的
`sgdt`、`sidt`（img `0x41E8`、`0x41ED`）、`mov eax,cr0`、`mov cr0,eax`（`0x41FC`、`0x4203`）與 `lgdt`、`lidt`（`strike_commander`
`docs/re/004-cpu-requirements.md` §1.1）。431 定案由既有核心在實模式補這幾道，PE 設起時交棒給 cpu386。

本規格補：(1) Machine 的完整 `SysRegs`；(2) 既有核心在實模式（與 V86 內不特權者）的系統指令；(3) 實模式→保護模式的交棒點；
(4) `-pm-profile` 旗標；(5) 作用中核心的觀測視圖。cpu386 的分段模式在 434；本規格的交棒在 434 之前停在明確的失敗即關閉。

## 2. `SysRegs`

在 432 已有的 `CR0`、`CR3`、`EFlagsHi`、`pmCPL` 之外，Machine 增加：

| 欄位 | 規則 | 依據 |
|---|---|---|
| `CR2` | 讀寫 | |
| `DR0`–`DR7` | `DR4`／`DR5` 為 `DR6`／`DR7` 的別名；寫 `DR6` 為 `(v\|FFFF0FF0h)&FFFFEFFFh`；寫 `DR7` 為 `(v\|400h)&FFFF2FFFh`；初值全 0 | `cpu/cpu.cpp:2695-2749`、`3643` 起 |
| `GDTR`、`IDTR` | 基底 32 位元、界限 16 位元；初值 `GDTR` 基底 0 界限 0、`IDTR` 基底 0 界限 `03FFh` | |
| `LDTR`、`TR` | 選擇子＋快取（基底、界限、屬性 16 位元）；本規格只保存，載入在 434／S4 | |

所有寫入經 Machine 的方法（431 §4）。`PM` 編碼提升為版本 2，在版本 1 的 16 bytes 之後依序加：`CR2`（4）、`DR0`–`DR3`、`DR6`、`DR7`（各 4）、
`GDTR` 基底（4）界限（2）、`IDTR` 基底（4）界限（2）、`LDTR` 選擇子（2）基底（4）界限（4）屬性（2）、`TR` 同 `LDTR`（12），共 16＋4＋24＋6＋6＋12＋12
＝ 80 bytes。讀取支援版本 1（缺的欄位取初值）與版本 2；`LoadState` 的驗證比照 432 §10 第 6 項，另驗 `DR6`、`DR7` 的固定位元。

## 3. 既有核心的系統指令介面

既有核心新增可選欄位 `Sys SysBus`（nil 時行為完全不變：這些指令照現狀失敗即關閉）。`SysBus` 由 Machine 實作，方法：`CR(n) (uint32, error)`、
`SetCR(n, v) error`、`DR(n) uint32`、`SetDR(n, v)`、`GDTR()`／`SetGDTR`、`IDTR()`／`SetIDTR`。Machine 只在 profile 開啟時裝上 `Sys`。

## 4. 指令

只在 `Sys` 非 nil 時生效。實模式下全部執行；V86 下標「特權」者由 435 交給 cpu386（本規格不實作 V86 路徑，只保證特權判斷的入口存在）。

| 指令 | 行為 | 特權 | 依據 |
|---|---|---|---|
| `smsw r/m16`（`0F 01 /4`） | 16 位元形式寫 `CR0` 低 16 位元；`66` 前綴的暫存器形式寫整個 `CR0` | 否 | `core_normal/prefix_0f.h:113-115`、`prefix_66_0f.h:108-110` |
| `lmsw r/m16`（`0F 01 /6`） | 只改 `CR0` 低 4 位元，不能清除 PE；設 PE 時交棒（§5） | 是 | `cpu/cpu.cpp:2788-2796`（`lmsw_allow_clear_pe_bit` 預設 false） |
| `sgdt`／`sidt m`（`0F 01 /0`、`/1`） | 存 16 位元界限與完整 32 位元基底（386 不遮成 24 位元）；暫存器運算元為 `#UD` | 否 | `prefix_0f.h:74-81`、`91-93` |
| `lgdt`／`lidt m`（`0F 01 /2`、`/3`） | 16 位元運算元只取 24 位元基底，`66` 前綴取整個 dword；暫存器運算元為 `#UD` | 是 | `prefix_0f.h:85`、`89`、`107-112`；`prefix_66_0f.h:80`、`84` |
| `mov r32,crN`（`0F 20`） | 讀 `CR0`、`CR2`、`CR3`（`CR3` 讀回遮 `FFFFF000h`）；`CR1`、`CR4` 起為 `#UD` | 是 | `cpu.cpp:2656-2673` |
| `mov crN,r32`（`0F 22`） | 經 Machine 的 setter；寫 `CR0` 使 PE 由 0 變 1 時交棒（§5）；寫 `CR0` 使 PE 由 1 變 0 由 432 的 `SetCR0` 擋下 | 是 | `cpu.cpp:2580-2636` |
| `mov r32,drN`、`mov drN,r32`（`0F 21`、`0F 23`） | 讀寫 §2 | 是 | `cpu.cpp:2695-2749` |
| `clts`（`0F 06`） | 清 `CR0.TS` | 是 | `prefix_0f.h:151-155` |
| `lldt`、`ltr`、`sldt`、`str`（`0F 00 /0–/3`）、`lar`、`lsl`（`0F 02`、`0F 03`） | 實模式為 `#UD`：本規格失敗即關閉（DOSBox-X 送 IVT 第 6 項；刻意從嚴，寫入診斷） | — | `prefix_0f.h:22`、`128`、`141` |

`#UD` 在實模式的處理：失敗即關閉，診斷寫明位址與 opcode；遞送到 IVT 第 6 項在有程式依賴時另立規格。

`mov crN`／`mov drN` 的 ModRM 一律視為暫存器形式（`mod` 欄位忽略，386 規則）；運算元一律 32 位元，與 `66` 前綴無關。

## 5. 實模式→保護模式交棒

- 既有核心執行 `mov cr0,r32` 或 `lmsw`，寫入使 PE 由 0 變 1：完成這道指令（`IP` 前進、`CR0` 經 setter 更新、衍生欄位重算），然後回傳
  哨兵 `ErrModeSwitch`。
- `Machine.Step` 收到哨兵：
  - 同一步有分頁錯誤紀錄時，依 432 §10 第 1 項回分頁錯誤，哨兵不處理。
  - 否則依 431 §4 交接：通用暫存器（`R`＋`Hi`）、`IP`、旗標低 16 位元、段值經單一映射函式交給 cpu386；段快取設為實模式推得的值
    （基底＝段值×16、界限 `FFFFh`、16 位元、可讀寫、present）。
  - 434 實作前：交接函式回傳錯誤 `ErrPMCoreNotAvailable`，`Step` 失敗即關閉，診斷寫明交棒位址與 `CR0`。哨兵不進失敗計數，但這個錯誤進。
- `-pm-profile` 關閉時這條路徑不存在（`Sys` 為 nil，`mov cr0` 照現狀失敗即關閉）。

## 6. `-pm-profile` 旗標

`cmd/probe` 加 `-pm-profile`，在 `configureMem`（430）裡、`SetExtendedMemoryKB` 與 `SetEMSPresent` 之後呼叫 `SetProtectedModeProfile(true)`；
與 `-load-state` 同給時，值必須與狀態檔一致（比照 430 §3）。

## 7. 作用中核心的觀測視圖

新增 `Machine.ActiveOp() (cs uint16, eip uint32)`、`Machine.EFlags() uint32`、`Machine.IF() bool`。本規格內作用中核心永遠是既有核心，
這三者回傳既有核心的值（`EFlags()` 合上 `EFlagsHi`）；434 起依模式切換。下列消費者改用它們（第 3 輪審查的 grep 結果，實作時補齊並列入收據）：

- `tick()`、`keyTick()`、`timedKeyTick()` 讀 IF 的地方。
- `Write8` 影片寫入紀錄與 `VRAMSites` 的 `OpAddr()`。
- `Machine.Step` 的 Coverage、`insnCS`／`insnIP`、`TraceSegs`；`probe.go` 的 `atBreak`。
- `session/digest.go` 的 CPU 摘要、`session/observer.go`、`oracle/state.go`、`oracle/run.go` 的護欄與查表、`cmd/probe/main.go` 的 `CS:IP ≥ A0000` 護欄。

profile 關閉時這些視圖與直接讀 `m.CPU` 完全相同；`session` 的 CPU 摘要在 profile 關閉時不變。

## 8. 不在範圍

cpu386 的執行（434）、V86 路徑與 `#GP` 遞送（435）、`ltr`／`lldt` 的保護模式語意與任務切換（S4）、`#UD`／`#GP` 送 IVT。

## 9. 驗收

1. profile 關閉不變：錨點語意常數不變；既有核心遇到 §4 的指令仍失敗即關閉；**SingleStepTests 語料實際掛入並全部通過**
   （`~/cht/dosgolem/testdata` 唯讀掛載；收據記錄執行的檔數與案例數，不得是 skip）。
2. `SysRegs`：每個欄位的寫入規則與初值各一例；`PM` 版本 2 逐欄位佈局；版本 1 的狀態檔可讀、缺欄位取初值；驗證失敗的條件各一例。
3. 指令：§4 每列至少一例（含 `smsw` 讀到 `0010h`、`66` 前綴 `smsw eax` 讀整個 `CR0`、`lmsw` 不能清 PE、`sgdt` 存完整 32 位元基底、`lgdt`
   16 位元取 24 位元、`66 lgdt` 取 32 位元、暫存器運算元的 `lgdt` 失敗即關閉、`mov cr4` 失敗即關閉、`mov eax,cr3` 遮低 12 位元、`DR6`／`DR7`
   固定位元、`lldt` 失敗即關閉）。
4. 交棒：合成程式設 PE 後，`Step` 回 `ErrPMCoreNotAvailable`（434 前），診斷含位址；`IP` 已越過 `mov cr0`；`CR0` 為新值；同一步有分頁紀錄時回分頁錯誤。
   以測試替身（記錄交接內容的假 cpu386 介面）驗證暫存器、旗標、段值與段快取的交接內容，含段索引映射的雙向往返與互換負對照。
5. 觀測視圖：profile 關閉時與直接讀 `m.CPU` 相同；`session` 的 CPU 摘要不變。
6. probe：`-pm-profile` 與 `-ext-mem-kb 0` 同給時回錯誤；與 `-load-state` 的一致性規則各一例。
7. 突變自檢：把 `lmsw` 改成可清 PE、把 `sgdt` 改成遮 24 位元、把段索引映射的 CS 與 SS 對調，各至少一個測試失敗。
8. 效能：profile 關閉時 `BenchmarkStepRealMode` 與 432 完成後的版本交錯比較（431 §7）。
9. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile` 通過 `smsw`、XMS 與 A20 檢查，進 JEMM，
   執行 `sgdt`、`sidt`、`mov eax,cr0`，在 img `0x4203` 的 `mov cr0,eax` 交棒，停在 `ErrPMCoreNotAvailable`。若在此之前停在別的指令，照實記錄。
10. `go build ./...`、`go vet`、`tools/go.sh test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

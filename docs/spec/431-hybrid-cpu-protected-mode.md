# 431 — 混合 CPU 架構：實模式與 V86 用既有核心，保護模式用 cpu386

狀態：**DRAFT**（第 4 版）
日期：2026-10-15
前置：[`429-dos-sft-chain`](429-dos-sft-chain.md)、[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`015-execution-speed`](015-execution-speed.md)、[`197-cpu386-real-mode-subset`](197-cpu386-real-mode-subset.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

《Strike Commander》光碟版要在 Origin 的 JEMM 底下執行。JEMM 從實模式自己切進保護模式，建 GDT、IDT、頁表與 TSS，
再以硬體任務返回進入 V86，之後所有 DOS 程式都在 V86 裡跑（`strike_commander_cht_dosgolem` 的 `docs/re/003`、`004`、`006`）。

使用者定案（2026-10-15，該專案 `AGENTS.md` §2）：擴充 `internal/cpu386` 負責保護模式；`EFLAGS.VM=1` 時由既有
`internal/cpu` 執行，`INT n`、IRQ、例外交回 cpu386 走 IDT。

本規格是架構契約：分工、狀態擁有權、交接點與共同規則。實作拆成四份規格，各自 READY 才動手：

| 規格 | 範圍 | 前置 |
|---|---|---|
| `432-linear-memory-and-paging` | Machine 的實體／線性存取分層、A20、兩層頁表、共用 TLB、服務層位址 | 430 |
| `433-real-mode-system-instructions` | 既有核心在實模式的系統指令、`SysRegs`、實模式→保護模式交棒、`-pm-profile` | 432 |
| `434-cpu386-segmented-mode` | cpu386 的分段模式機制（以暫存器索引的段快取、CS 基底取指令、Machine 匯流排轉接） | 433 |
| `435-v86-hooks-and-idt-delivery` | 既有核心的 V86 掛鉤、V86 事件經 IDT 遞送、服務層在 V86 下的規則、IRQ 與回呼 | 434 |

指令覆蓋（JEMM 的 16 位元碼段指令、TSS 任務切換、`#PF`、回實模式）是更後面的規格（§8）。

### 1.1 實模式交給既有核心

觸發案例在實模式就碰到系統指令（SCCD 的 `smsw`、JEMM 切保護模式前的 `sgdt`、`lgdt`、`mov cr0` 等）。既有核心在實模式
補這幾道，設 PE 時交棒給 cpu386（433）。不採「cpu386 從程式開頭執行實模式」，因為那要 cpu386 執行整套 16 位元實模式
程式，正是使用者否決的方向。

### 1.2 cpu386 現況

cpu386 是平坦核心：段查詢以選擇子為鍵、描述元由主機登錄，取指令不加 CS 基底，預設 32 位元運算元（`internal/cpu386/cpu.go`
約 26–83、271–278 行）。JEMM 的保護模式程式跑在基底非零的 16 位元碼段，所以要加「分段模式」（434）。LE／DOS4GW 路徑
維持平坦模式，行為不變。

## 2. 啟用與目標型號

- `Machine.SetProtectedModeProfile(on)`，預設關閉；細節在 433。關閉時 432–435 的行為都不發生，既有收據不變。
- 目標 CPU 固定為 386（`197` §5：`AC` 立不起來）。型號相依的行為照 386：`EFLAGS.AC`、`ID` 不可改；`CR4` 存取為 `#UD`；
  分頁權限：使用者存取時 PDE 與 PTE 的 U/S 取 AND、寫入時 R/W 取 AND，監督者不受限（無 `WP`）；DOSBox-X 的現行路徑是 `cpu/paging.cpp:150-172` 的 `translate_array` 加 `210-230` 的 `fault_table`，不分型號；`DR7` 寫入照 `cpu/cpu.cpp:2713`。
  DOSBox-X 對拍用 `cputype=386`。

## 3. 執行模式

| 模式 | 判定 | 執行的核心 |
|---|---|---|
| 實模式 | `CR0.PE=0` | 既有 `internal/cpu` |
| 保護模式 | `CR0.PE=1` 且 `VM=0` | `internal/cpu386`（分段模式） |
| V86 | `CR0.PE=1` 且 `VM=1` | 既有 `internal/cpu`，經 V86 掛鉤 |

- 任一時刻只有一個核心執行，只在指令邊界交接。模式由 `CR0.PE` 與 `VM` 推得，不另存。
- 兩核心共用 `m.Steps` 與 `Cycles`。cpu386 在 `DOSBoxCost` 開啟時每道指令計 1（字串指令每次迭代計 1），關閉時也計 1；
  這與既有核心的 8086 成本表不同，`CycleClock` 的 IRQ0 在保護模式期間節奏因此不同，寫入 434 的已知限制。
- 保護模式期間不遞送 IRQ，留在待處理，回 V86 或實模式後遞送；同一條 IRQ 的待處理合併為一筆（同 8259 的 IRR）。JEMM 在 ring 0
  長時間執行時 IRQ0 因此延後。cpu386 在保護模式遇到 `HLT`，或有待處理 IRQ 且 `IF=1` 而連續一百萬步未離開保護模式時，
  失敗即關閉並寫明原因，不安靜卡住。保護模式內的 IDT 遞送需要時另立規格。

## 4. 狀態擁有權

| 狀態 | 擁有者 | 說明 |
|---|---|---|
| `CR0`、`CR2`、`CR3`、`DR0`–`DR7`、`GDTR`、`IDTR`、`LDTR`、`TR`（含快取） | Machine 的 `SysRegs` | 433 |
| 段描述元快取 ×6 | cpu386 | 434。既有核心在實模式與 V86 由段值推得基底，不用快取 |
| `CPL` | Machine 判定 | 實模式 0；V86 為 3（由 `PE` 與 `VM` 推得，432）；保護模式由 cpu386 經 434 提供 |
| `EFLAGS` 低 16 位元 | 作用中核心 | 熱路徑不動 |
| `EFLAGS` 高 16 位元（`RF`、`VM`；386 的 `AC`、`ID` 恆 0） | Machine 的 `EFlagsHi` | 既有核心執行期間不變；`Machine.EFlags()` 組合出完整值 |
| 通用暫存器、`IP`／`EIP`、段值 | 作用中核心 | 交接時搬移（433、434） |
| `FPUCW` | Machine 共用 | 其餘 x87 狀態只在 cpu386；V86 下 `FNINIT`、`FNSTCW`、`FLDCW`、`FWAIT` 以外的 ESC 失敗即關閉 |
| TLB | Machine | 432；不存檔，`Restore`／`LoadState` 後清空 |
| 指令位址視圖 | Machine 的 `ActiveOp()` | 取代直接讀 `m.CPU.Seg[CS]`／`IP` 的觀測點（433 列出全部消費者） |

段暫存器索引兩邊順序不同（既有核心 `Seg[4]` 為 `ES, CS, SS, DS`，FS、GS 另放 `FSeg`、`GSeg`；cpu386 `Seg[6]` 為
`CS, DS, ES, FS, GS, SS`），交接一律經單一映射函式。

### 4.1 旗標共同規則

- 保留位元：bit 1 為 1，bit 3、5、15 為 0（與既有核心 `SetFlags` 的 386 分支相同，`internal/cpu/cpu.go:196-202`）。
- V86 下既有核心：`POPF` 在 IOPL≠3 時 `#GP(0)`、通過時不改 IOPL；`IRET` 只改 `FMASK_NORMAL|NT`；`PUSHFD` 高半清 `VM`、`RF`
  （`cpu.cpp:774-797`、`1560`、`1572`）。
- 保護模式下 cpu386（分段模式）：`POPF`／`POPFD` 不改 `VM`、`RF`；CPL>0 時不改 IOPL；CPL>IOPL 時不改 IF（`cpu.cpp:774-788`）。
  平坦模式（LE）維持現狀。

### 4.2 狀態檔

使用者定案（2026-10-15）：`machineState` 只新增一個不透明欄位 `PM []byte`（`omitempty`），內容自帶版本號與編碼，存放 432–435 的全部
保護模式狀態（`PMProfile`、控制與除錯暫存器、描述表暫存器、`EFlagsHi`、`EIP` 高 16 位元、cpu386 段快取與 FPU、待處理 IRQ）。
模式與 CPL 讀回時推得，不另存。profile 關閉時 `PM` 為 nil，`StateDigest` 不變；gob 位元組錨點在 432 新增此欄位的 commit 改變一次
（430 之後第二次），之後 433–435 只改 `PM` 的內部格式，不再改 gob 型別。

## 5. 交接點

| 交接 | 觸發 | 規格 |
|---|---|---|
| 實模式→保護模式 | 既有核心執行 `mov cr0` 或 `lmsw` 使 PE 由 0 變 1，完成該道指令後回傳哨兵 `ErrModeSwitch` | 433 |
| 保護模式→V86 | cpu386 的 `iretd` 彈出 `VM=1`，或任務切換載入 `VM=1` | 後續規格 S4 |
| V86→保護模式 | 既有核心在 V86 遇到 `INT n`、IOPL 敏感指令、特權指令、例外、IRQ，回傳 `V86Event`，由 cpu386 依 IDT 遞送 | 435 |
| 保護模式→實模式 | 不支援，失敗即關閉 | 後續規格 S9 |

`V86Event` 帶 `Kind`、`Vector`、`HasError`、`ErrorCode`、`InstrIP`、`NextIP`。轉 `#GP`、`#UD` 與 IOPL 敏感的例外，框架的 `EIP` 用
`InstrIP`（`cpu.cpp:1181`；`core_normal.cpp:115-116`）；成功遞送的軟體中斷用 `NextIP`。

## 6. 服務層

dosgolem 的 DOS／BIOS 服務在 V86 下仍由 `StubSeg` 的 trampoline 觸發：遊戲的 `INT 21h` 先經 JEMM（`#GP`）反射到 IVT，跳進
`StubSeg`，trampoline 的 `CD F2` 才由服務層接手。判斷規則、旗標回寫與阻塞型服務在 435；服務層的位址計算在 432。

## 7. 效能門檻

基準是 `015-execution-speed`（`cmd/probe` 約 24.9 M 道／秒；即時遊玩下限 `StepsPerSecond` 約 11.58 M）。

- profile 關閉：432–435 每份實作後，同一容器內對固定合成程式交錯跑新舊各三次取最快，不低於修改前的 97%。
- 新增合成迴圈的 `BenchmarkStepRealMode`、`BenchmarkStepV86`、`BenchmarkStepV86Paged`，不依賴原版檔；基準測試要在行為 commit 之前
  先提交，才能在修改前的版本編出比較用的測試程式。同一容器交錯跑新舊各五次取最快，落在門檻附近時重跑，收據記錄 load average。
- V86 分頁迴圈不低於 11.58 M 道／秒；量不到時另立 TLB 與取指令優化的規格，不在功能規格裡順手改。

## 8. 後續規格

| 編號 | 內容 |
|---|---|
| S2 | cpu386 分段模式的指令覆蓋：JEMM 的 16 位元碼段、16 位元 ModRM、遠轉移經選擇子、`iretd`、`retfd`、`66 FF /3 /5` |
| S4 | TSS、`ltr`、`iretd` 帶 NT 的任務返回、進 V86 |
| S6 | `#PF`、`#DB`、TF 單步、除錯暫存器觸發 |
| S9 | 保護模式→實模式（JEMM 的卸載路徑） |
| — | 保護模式內的 IDT 遞送、VCPI、`invlpg`、4 MB 頁、`WP`、`#NM` |

## 9. 驗收

本規格本身不產生程式，是 432–435 的契約：432–435 各自必須與本規格一致，發現矛盾時先改本規格並重審。各份規格自己的驗收要另外涵蓋：

1. 關閉時不變：錨點測試常數不變；cpu386 平坦模式（LE 路徑）不變，並以「新舊 cpu386 對同一合成 LE 程式逐步比對暫存器與記憶體
   雜湊」的差分測試證明（FD2、MOO2 收據測試缺原版時會 skip，不能單獨作證據）。
2. 既有核心有改動的規格，`SingleStepTests` 語料要實際抓下來跑且全部通過（`CLAUDE.md`），不以「缺語料 skip 的全綠」代替。
3. §7 的效能門檻。
4. 合成的 GDT、IDT、TSS、頁表測試是單元測試，不能代替原版證據；需要原版的驗收缺檔時 skip，不得以合成品通過。

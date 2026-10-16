# 431 — CPU 架構：既有核心執行實模式、16 位元保護模式與 V86

狀態：**READY**（第 2 版：依使用者 2026-10-16 定案改分工；與 434 一併審查兩輪）
日期：2026-10-15；第 2 版 2026-10-16
前置：[`429-dos-sft-chain`](429-dos-sft-chain.md)、[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`015-execution-speed`](015-execution-speed.md)、[`197-cpu386-real-mode-subset`](197-cpu386-real-mode-subset.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

《Strike Commander》光碟版要在 Origin 的 JEMM 底下執行。JEMM 從實模式自己切進保護模式，建 GDT、IDT、頁表與 TSS，
再以硬體任務返回進入 V86，之後所有 DOS 程式都在 V86 裡跑（`strike_commander_cht_dosgolem` 的 `docs/re/003`、`004`、`006`）。

使用者定案（2026-10-16，該專案 `AGENTS.md` §2）：既有 `internal/cpu` 加「分段模式」，執行 16 位元保護模式碼與 V86；
`internal/cpu386` 不改，只在遇到 32 位元碼段時再接（後續規格）。理由見 §1.2。

本規格是架構契約：分工、狀態擁有權、交接點與共同規則。實作拆成四份規格，各自 READY 才動手：

| 規格 | 範圍 | 前置 |
|---|---|---|
| `432-linear-memory-and-paging` | Machine 的實體／線性存取分層、A20、兩層頁表、共用 TLB、服務層位址 | 430 |
| `433-real-mode-system-instructions` | 既有核心在實模式的系統指令、`SysRegs`、實模式→保護模式交棒、`-pm-profile` | 432 |
| `434-legacy-core-segmented-mode` | 既有核心的分段模式：段描述元快取、從 GDT／LDT 載入選擇子、CS 基底取指令、32 位元偏移、遠轉移、保護模式系統指令、觀測視圖、`PM` 版本 3 | 433 |
| `435-v86-hooks-and-idt-delivery` | 既有核心的 V86 掛鉤、經 IDT 遞送中斷與例外（含特權切換與堆疊切換）、服務層在 V86 下的規則、IRQ 與回呼 | 434 |

TSS 任務切換、`#PF`、回實模式與 32 位元碼段是更後面的規格（§8）。

### 1.1 實模式交給既有核心

觸發案例在實模式就碰到系統指令（SCCD 的 `smsw`、JEMM 切保護模式前的 `sgdt`、`lgdt`、`mov cr0` 等），由既有核心補（433）。
PE 由 0 變 1 時，同一顆核心切進分段模式（434），不換核心。

### 1.2 為什麼不用 cpu386

JEMM 從切保護模式到進 V86 之間執行的碼全在 16 位元碼段（GDT 選擇子 `18h`，D=0），以 `66`、`67` 前綴使用 32 位元運算元與定址
（`strike_commander` `docs/re/004` §1，路徑上約 50 種指令）。cpu386 是 DOS/4GW 用的 32 位元平坦核心：

- 不處理 `67` 前綴，只有 32 位元 ModRM（`internal/cpu386/address32.go:6-46`，`cpu.go:548`、`6077-6078`）；
- 取指令不加 CS 基底（`cpu.go:271-301`），far call 與 `retf` 要求 CS 基底為 0（`cpu.go:2662`、`6029-6031`）；
- 運算元大小以區域布林 `operand16` 在各分支各自處理，約 75 處只支援 32 位元（`cpu.go:544-583` 起）；
- `FS`、`GS` 前綴（`64`、`65`）未處理。

既有核心已有 `66`／`67` 前綴、ModRM32／SIB 與 `FS`／`GS`（`docs/spec/012`、`197`），SCCD 與 JEMM 實模式段已由它執行。
所以 16 位元保護模式交給既有核心，cpu386 與 LE／DOS4GW 路徑一行不改。

## 2. 啟用與目標型號

- `Machine.SetProtectedModeProfile(on)`，預設關閉；定義於 432 §2，433 加 `-pm-profile` 旗標與 `SysRegs`。關閉時 432–435 的行為都不發生，既有收據不變。
- 目標 CPU 固定為 386（`197-cpu386-real-mode-subset` §5：`AC` 立不起來）。型號相依的行為照 386：`EFLAGS.AC`、`ID` 不可改；`CR4` 存取為 `#UD`；
  分頁權限：使用者存取時 PDE 與 PTE 的 U/S 取 AND、寫入時 R/W 取 AND，監督者不受限（無 `WP`）；DOSBox-X 一般核心的路徑是 `cpu/paging.cpp:150-172` 的 `translate_array` 加 `210-230` 的 `fault_table`，不分型號（同檔 `708-719` 的舊 386 分支只供 `InitPageUserROHandler`／`InitPageCheckOnly` 使用，語意不同，對拍時不以它為準）；`DR7` 寫入照 `cpu/cpu.cpp:2713`。
  DOSBox-X 對拍用 `cputype=386`。

## 3. 執行模式

| 模式 | 判定 | 執行的核心 |
|---|---|---|
| 實模式 | `CR0.PE=0` | 既有 `internal/cpu` |
| 保護模式 | `CR0.PE=1` 且 `VM=0` | 既有 `internal/cpu`（分段模式）；CS 載入 D=1 的碼段時失敗即關閉（後續規格） |
| V86 | `CR0.PE=1` 且 `VM=1` | 既有 `internal/cpu`，經 V86 掛鉤 |

- 模式由 `CR0.PE` 與 `VM` 推得，不另存。三種模式都由既有核心執行，`m.Steps` 與 `Cycles` 照既有成本表計算。
- 保護模式期間不遞送 IRQ，留在待處理，回 V86 或實模式後遞送。合併只適用電平型的 IRQ0（現行 `irq0Pending` 布林，維持
  `machineState` 頂層欄位）；按鍵佇列（`keyQueue`）每個事件都保留、不合併。`startCallback`、`periodic` 的 `FarCall`、`keyTick`、
  `timedKeyTick` 在保護模式期間全部延後、留在佇列。JEMM 在 ring 0 長時間執行時 IRQ0 因此延後。保護模式遇到 `HLT`，
  或有待處理 IRQ 且 `IF=1` 而連續一百萬步未離開保護模式時，失敗即關閉並寫明原因，不安靜卡住。保護模式內的 IDT 遞送需要時另立規格。

## 4. 狀態擁有權

| 狀態 | 擁有者 | 說明 |
|---|---|---|
| `CR0`、`CR2`、`CR3`、`DR0`–`DR7`、`GDTR`、`IDTR`、`LDTR`、`TR`（含快取） | Machine 的 `SysRegs` | 433 |
| 段描述元快取 ×6 | 既有核心 | 434。分段模式才使用；實模式與 V86 由段值推得基底 |
| `CPL` | Machine 判定 | 實模式 0；V86 為 3（由 `PE` 與 `VM` 推得，432；DOSBox-X `cpu/cpu.cpp:1047`、`1624`）；保護模式在載入 CS 時經 Machine 的 `SetPMCPL` 寫入（434） |
| `EFLAGS` 低 16 位元 | 既有核心 | 熱路徑不動 |
| `EFLAGS` 高 16 位元（`RF`、`VM`；386 的 `AC`、`ID` 恆 0） | Machine 的 `EFlagsHi` | 432 |
| 通用暫存器、`IP`、段值 | 既有核心 | `EIP` 高 16 位元：16 位元碼段恆 0，32 位元碼段屬後續規格 |
| `FPUCW` | 既有核心 | 保護模式與 V86 下 `FNINIT`、`FNSTCW`、`FLDCW`、`FWAIT` 以外的 ESC 失敗即關閉 |
| TLB | Machine | 432；不存檔，`Restore`／`LoadState` 後清空 |
| 指令位址視圖 | Machine 的觀測方法 | 取代直接讀 `m.CPU.Seg[CS]`／`IP` 的觀測點；分「指令起點」與「下一道」兩種語意（434 定義並列出全部消費者） |

`CR0`、`EFlagsHi`、`pmCPL` 的寫入只能經 Machine 的方法（432 的 setter、433 的 `mov cr0`、434、S4 的交接），由方法重算 432 的衍生欄位
（`xlat`、`paging`、`cplUser`）；核心不得直接改這些欄位。

### 4.1 旗標共同規則

- 保留位元：bit 1 為 1，bit 3、5、15 為 0（與既有核心 `SetFlags` 的 386 分支相同，`internal/cpu/cpu.go:196-202`）。
- V86 下既有核心：`POPF` 在 IOPL≠3 時 `#GP(0)`、通過時不改 IOPL；`IRET` 只改 `FMASK_NORMAL|NT`；`PUSHFD` 高半清 `VM`、`RF`
  （`cpu.cpp:774-797`、`1560`、`1572`）。
- 保護模式（分段模式）：`POPF`／`POPFD` 不改 `VM`、`RF`；CPL>0 時不改 IOPL；CPL>IOPL 時不改 IF（`cpu.cpp:774-788`）。

### 4.2 狀態檔

使用者定案（2026-10-15）：`machineState` 只新增一個不透明欄位 `PM []byte`（`omitempty`），內容自帶版本號與編碼，存放 432–435 的全部
保護模式狀態（`PMProfile`、控制與除錯暫存器、描述表暫存器、`EFlagsHi`、分段模式旗標與段描述元快取）。IRQ0 的待處理維持既有的頂層欄位
`IRQ0Pending`，不遷入。
模式與 CPL 讀回時推得，不另存。profile 關閉時 `PM` 為 nil，`StateDigest` 不變；gob 位元組錨點在 432 新增此欄位的 commit 改變一次
（430 之後第二次），之後 433–435 只改 `PM` 的內部格式，不再改 gob 型別。

## 5. 交接點

| 交接 | 觸發 | 規格 |
|---|---|---|
| 實模式→保護模式 | `mov cr0` 或 `lmsw` 使 PE 由 0 變 1，完成該道指令後回傳哨兵 `ErrModeSwitch`（433）；Machine 讓同一顆核心進分段模式 | 433、434 |
| 保護模式→V86 | `iretd` 彈出 `VM=1`，或任務切換載入 `VM=1` | 後續規格 S4 |
| V86→保護模式 | V86 遇到 `INT n`、IOPL 敏感指令、特權指令、例外、IRQ，產生 `V86Event`，依 IDT 遞送 | 435 |
| 保護模式→實模式 | 不支援，失敗即關閉 | 後續規格 S9 |
| 16 位元碼段→32 位元碼段 | CS 載入 D=1 的描述元 | 失敗即關閉，後續規格 |

`V86Event` 帶 `Kind`、`Vector`、`HasError`、`ErrorCode`、`InstrIP`、`NextIP`。轉 `#GP`、`#UD` 與 IOPL 敏感的例外，框架的 `EIP` 用
`InstrIP`（`cpu.cpp:1181`；`core_normal.cpp:115-116`）；成功遞送的軟體中斷用 `NextIP`。

## 6. 服務層

dosgolem 的 DOS／BIOS 服務在 V86 下仍由 `StubSeg` 的 trampoline 觸發：遊戲的 `INT 21h` 先經 JEMM（`#GP`）反射到 IVT，跳進
`StubSeg`，trampoline 的 `CD F2` 才由服務層接手。判斷規則、旗標回寫與阻塞型服務在 435；服務層的位址計算在 432。

## 7. 效能門檻

基準：`cmd/probe` 約 24.9 M 道／秒（`015-execution-speed`）；即時遊玩下限約 11.58 M 道／秒（`machine.StepsPerSecond()`，`internal/machine/pit.go`）。

- profile 關閉：432–435 每份實作後，同一容器內對固定合成程式交錯跑新舊各五次取最快，不低於修改前的 97%。
- 新增合成迴圈的 `BenchmarkStepRealMode`、`BenchmarkStepV86`、`BenchmarkStepV86Paged`，不依賴原版檔；基準測試要在行為 commit 之前
  先提交，才能在修改前的版本編出比較用的測試程式。同一容器交錯跑新舊各五次取最快，落在門檻附近時重跑，收據記錄 load average。
- V86 分頁迴圈不低於 11.58 M 道／秒；量不到時另立 TLB 與取指令優化的規格，不在功能規格裡順手改。

## 8. 後續規格

| 編號 | 內容 |
|---|---|
| S4 | TSS、`ltr`、`iretd` 帶 NT 的任務返回、進 V86 |
| S6 | `#PF`、`#DB`、TF 單步、除錯暫存器觸發 |
| S9 | 保護模式→實模式（JEMM 的卸載路徑） |
| — | 32 位元碼段（JEMM 選擇子 `98h`、`128h`，`int15` 的 `CU`、`XC` 複製常式）：既有核心加 D=1 預設，或接 cpu386 |
| — | 保護模式內的 IDT 遞送、VCPI、`invlpg`、4 MB 頁、`WP`、`#NM` |

## 9. 驗收

本規格本身不產生程式，是 432–435 的契約：432–435 各自必須與本規格一致，發現矛盾時先改本規格並重審。各份規格自己的驗收要另外涵蓋：

1. 關閉時不變：錨點測試常數不變；`internal/cpu386` 與 LE 路徑沒有改動（`git diff` 為空）。
2. 既有核心有改動的規格，`SingleStepTests` 語料要實際抓下來跑且全部通過（`CLAUDE.md`），不以「缺語料 skip 的全綠」代替。
3. §7 的效能門檻。
4. 合成的 GDT、IDT、TSS、頁表測試是單元測試，不能代替原版證據；需要原版的驗收缺檔時 skip，不得以合成品通過。

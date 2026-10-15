# 431 — 混合 CPU：實模式與 V86 用既有核心，保護模式用 cpu386

狀態：**DRAFT**（第 2 版，依第 1 輪審查改寫）
日期：2026-10-15
前置：[`429-dos-sft-chain`](429-dos-sft-chain.md)、[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`197-cpu386-real-mode-subset`](197-cpu386-real-mode-subset.md)、[`014-dos4gw-flat-descriptors`](014-dos4gw-flat-descriptors.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

《Strike Commander》光碟版要在 Origin 的 JEMM 底下執行。JEMM 從實模式自己切進保護模式，建 GDT、IDT、頁表與 TSS，
再以硬體任務返回進入 V86，之後所有 DOS 程式都在 V86 裡跑（`strike_commander_cht_dosgolem` 的 `docs/re/003`、`004`、`006`）。

使用者定案（2026-10-15，該專案 `AGENTS.md` §2）：擴充 `internal/cpu386` 負責保護模式；`EFLAGS.VM=1` 時由既有
`internal/cpu` 執行，`INT n`、IRQ、例外交回 cpu386 走 IDT。

本規格定義分工、共享狀態、交接點與兩個核心各自要補的基礎機制。指令覆蓋另立規格（§10）。

### 1.1 實模式交給誰

觸發案例在實模式就碰到系統指令：SCCD 的 `smsw ax`（映像 `0x1FE9`），JEMM 切保護模式前的 `sgdt`、`sidt`、`lgdt`、`lidt`、
`mov eax,cr0`／`mov cr0,eax`（JEMM img `0x41E8`–`0x4203`）。採 (a)：既有核心在實模式補這幾道，設 PE 時交棒給 cpu386。
不採 (b)「cpu386 從程式開頭執行實模式」，因為那要 cpu386 執行整套 16 位元實模式程式，正是使用者否決的方向。

### 1.2 cpu386 的現況與要補的

cpu386 目前是平坦核心：`Seg [6]uint16` 只存選擇子，描述元放在以選擇子為鍵、由主機登錄的 `Descriptors` map，
`Descriptor` 只有基底、界限、可寫；取指令把 `EIP` 直接當線性位址，不加 CS 基底；預設運算元為 32 位元
（`internal/cpu386/cpu.go` 約 48、59–83、271–277 行）。JEMM 的保護模式程式跑在基底非零的 16 位元碼段（選擇子 `18h`，
`004` §2），所以本規格要替 cpu386 加一套「分段模式」（§5），只在本規格開啟時使用；既有 LE／DOS4GW 路徑維持平坦模式，
行為不變。

## 2. 啟用方式

`Machine.SetProtectedModeProfile(on bool) error`，預設關閉。檢查順序：`programLoaded` 為真回錯誤；開啟時
`ExtendedMemoryKB() == 0` 回錯誤；`len(m.Mem) != MemSize` 回錯誤。關閉時本規格的所有行為都不發生，既有收據不變。
`cmd/probe` 加 `-pm-profile`，在 `configureMem`（`430`）之後、`if *loadState == ""` 區塊外設定。

目標 CPU 型號固定為 386（`197` §5 已定 `AC` 立不起來）。下列型號相依的行為照 386 固定，不跟 DOSBox-X 的
`cputype` 預設：`EFLAGS.AC`、`ID` 不可改；`CR0` 讀回不遮 486 位元；`CR4` 存取為 `#UD`；分頁 U/S 取 PDE 與 PTE 的
AND（`hardware/paging.cpp:708-719` 的 386 分支）；`DR7` 寫入照 `cpu.cpp:2713`。DOSBox-X 交叉對照時用 `cputype=386`，
`006` 的設定要補記這一項。

## 3. 執行模式與負責核心

| 模式 | 判定 | 執行的核心 |
|---|---|---|
| 實模式 | `CR0.PE=0` | 既有 `internal/cpu` |
| 保護模式 | `CR0.PE=1` 且 `EFLAGS.VM=0` | `internal/cpu386`（分段模式） |
| V86 | `CR0.PE=1` 且 `EFLAGS.VM=1` | 既有 `internal/cpu`，經 V86 轉接（§7） |

任一時刻只有一個核心在執行，且只在指令邊界交接。`Machine.Step` 依模式呼叫對應核心；兩個核心共用 `m.Steps`。
cpu386 每道指令依 DOSBox-X 的成本表累加 `Cycles`（與既有核心同一個計數器，`CycleClock` 的 IRQ0 靠它），
cpu386 沒有成本資料的指令以 1 計並在診斷計數。

## 4. 共享狀態

每一項狀態只有一個擁有者。

### 4.1 Machine 擁有，兩個核心都讀寫

| 狀態 | 內容與規則 |
|---|---|
| `Mode` | 實模式、保護模式、V86 |
| `CR0`、`CR2`、`CR3` | 寫 `CR0` 一律補 `00000010h`（`cpu/cpu.cpp:2582`），初值 `00000010h`（`cpu.cpp:3632-3634`）；讀 `CR3` 遮 `FFFFF000h`（`cpu.cpp:2673`）；`CR1`、`CR4` 起為 `#UD` |
| `DR0`–`DR7` | `DR4`／`DR5` 為 `DR6`／`DR7` 的別名；寫 `DR6` 為 `(v\|FFFF0FF0h)&FFFFEFFFh`（`cpu.cpp:2708`）；寫 `DR7` 為 `(v\|400h)&FFFF2FFFh`（`cpu.cpp:2713`）；初值全 0（同 DOSBox-X，`cpu.cpp:3643` 起） |
| `GDTR`、`IDTR` | 基底 32 位元、界限 16 位元；`IDTR` 初值基底 0、界限 `03FFh` |
| `LDTR`、`TR` | 選擇子＋描述元快取 |
| 段快取 ×6 | 每個段暫存器：選擇子、基底、界限、型別、DPL、D/B、向下展開、present |
| `CPL` | 實模式 0；V86 3；保護模式取 CS 快取的 DPL |
| `EFLAGS` | 完整 32 位元的正規值（§4.3） |
| `FPUCW` | x87 控制字；兩核心共用（§4.4） |
| 分頁 TLB | §6 |

段暫存器索引兩邊順序不同：既有核心 `ES=0, CS=1, SS=2, DS=3`，FS／GS 另放（`internal/cpu/cpu.go` 約 33–46 行）；
cpu386 `CS=0, DS=1, ES=2, FS=3, GS=4, SS=5`（`cpu386/cpu.go` 約 26–33 行）。交接一律經單一映射函式，驗收雙向測試。

### 4.2 交接時搬移

| 狀態 | 既有核心 | cpu386 | 規則 |
|---|---|---|---|
| 通用暫存器 | `R`（低 16）＋`Hi`（高 16） | `R uint32` | 逐一組合／拆開 |
| 指令位址 | `IP uint16` | `EIP uint32` | 交給既有核心時 `EIP` 高 16 須為 0，否則失敗即關閉（DOSBox-X `cpu.cpp:1612` 會遮掉；這裡刻意從嚴） |
| 段值 | `Seg[4]`＋`FSeg`、`GSeg` | 選擇子 | 段快取在 Machine（§4.1），兩核心都不各自保存 |
| `Halted` | 既有核心 | 無 | 交接時必須為 false；V86 的 `HLT` 走 `#GP(0)`（§8.2） |

診斷欄位（既有核心的 `opCS/opIP`、`DivErrors`）由 Machine 提供「作用中核心的指令位址」視圖，`Write8` 的影片寫入紀錄等
都改讀這個視圖。

### 4.3 旗標

Machine 保存完整 32 位元 `EFLAGS`。既有核心執行時看到低 16 位元；交還時以下列規則合回：

- 保留位元：bit 1 為 1，bit 3、5、15 為 0（兩核心一致；cpu386 的 `POPFD` 目前直接寫入，分段模式下改用同一套正規化）。
- 高 16 位元（`RF`、`VM`、`AC`）在既有核心執行期間不變。
- V86 下既有核心的旗標寫入照 DOSBox-X：
  - `POPF`：IOPL≠3 時 `#GP(0)`；通過時 IOPL 位元不變（`cpu.cpp:774-788`，CPL>0 遮掉 IOPL）。
  - `IRET`：只改 `FMASK_NORMAL|NT`，不改 IOPL（`cpu.cpp:1560`、`1572`）。
  - `PUSHFD` 的高半清 `VM` 與 `RF`（`cpu.cpp:797`）。
- 實模式下既有核心照舊。

### 4.4 FPU

既有核心只有 `FPUCW`，x87 算術不做；cpu386 有完整 FPU 狀態。本規格下：

- `FPUCW` 由 Machine 共用。cpu386 的其餘 FPU 狀態只在保護模式使用。
- V86 遇到 `FNINIT`、`FNSTCW`、`FLDCW`、`FWAIT` 以外的 ESC 指令：失敗即關閉，診斷寫明位址。STRIKE 是否在 V86 使用 x87
  算術待量測（`004` §2 只確認 JEMM 與 SCCD 不用）。
- `CR0.EM`／`TS` 下的 `#NM` 不在範圍。

### 4.5 狀態檔與快照

新增的狀態欄位依 `430` §3 的程序：先在「只加欄位」的 commit 一次加完，之後不再增改。為了讓預設的 `StateDigest`
（JSON）不變，新欄位一律用指標或 slice（零值為 nil，`omitempty` 才會省略）。欄位清單：`PMProfile`、`Mode`、`SysRegs`
（§4.1 的控制、除錯、描述表暫存器與 `LDTR`／`TR` 快取）、段快取、`CPL`、`EFLAGSHi`、cpu386 的 FPU 狀態。

- `Snapshot`／`Restore` 與 `SaveState`／`LoadState` 依模式重接：`CPU.Bus`、`CPU.Code`（`syncCodeFastPath`）、V86 掛鉤。
- `PMProfile` 與機器設定不一致時 `LoadState` 回錯誤、`Restore` panic（同 `430` 的 `Ext` 規則）。
- TLB 不存；`Restore` 與 `LoadState` 後清空，按需重填（§6）。
- `session/digest.go` 只在 `PMProfile` 開啟時把上述狀態納入摘要，關閉時摘要不變。

## 5. cpu386 的分段模式

只在本規格開啟時使用。LE／DOS4GW 路徑維持平坦模式。

- 取指令：線性位址＝CS 快取基底＋`EIP`。資料存取：線性位址＝段快取基底＋有效位址，檢查界限（向下展開照 386 規則）。
- 預設運算元與位址大小由 CS 快取的 D 位元決定；`66`／`67` 前綴反轉。堆疊用 `SP` 或 `ESP` 由 SS 快取的 B 位元決定。
- 段暫存器載入：保護模式下從 GDT／LDT 讀描述元，依 386 規則檢查（present、型別、DPL／RPL／CPL），寫回 accessed 位元。
  實模式推得的快取在 `PE=1` 後沿用，直到該段重新載入（DOSBox-X `cpu.cpp:2580-2636` 不動段快取）。
- 主機掛鉤：本模式下 `Descriptors`、`SegmentRead*`、`SegmentLoadOK`、`IntHook`、`StepHook`、`PortOut`／`PortIn` 必須為 nil；
  `INT` 一律走 IDT，I/O 走 Machine。
- 記憶體經 Machine 的線性存取（§6）。

## 6. 線性位址與分頁

- 有效分頁＝`CR0.PE && CR0.PG`（DOSBox-X `cpu.cpp:2629-2632`）。
- `Machine.Read8`／`Write8`（既有核心的 Bus、服務層、cpu386 的分段模式都經過它）改為接受線性位址：有效分頁時先轉成實體位址，
  再走實體存取（`a < 0x100000` 走現有路徑含 VGA、ROM、監看；以上走 `Ext`）。沒有分頁時行為不變。服務層因此看到與 CPU
  相同的位址空間，不必改呼叫點。
- 轉換：兩層 4 KB 頁表。TLB 由 Machine 持有、兩核心共用，按需填（第一次存取該頁時讀 PDE、PTE），在寫 `CR3`、`CR0.PE`／`PG`
  改變、`SetA20` 時整份清除（同 DOSBox-X 的惰性 TLB，`hardware/paging.cpp:779-905`、`990-1006`、`1405-1414`）。
- 填 TLB 時依 386 規則把 PDE、PTE 的 accessed 位元（寫入時另加 dirty）寫回頁表（`paging.cpp:839-884`）。這是程式看得到的
  記憶體寫入，與 DOSBox-X 對拍時才一致。
- 權限：使用者存取（CPL=3，含 V86）檢查 U/S（PDE 與 PTE 取 AND）與寫入時的 R/W；監督者存取不檢查 R/W（386 無 `WP`）。
- 轉換後的實體位址依 A20 狀態遮 bit 20（DOSBox-X `hardware/memory.cpp:313-320`）。
- 錯誤：頁不存在或權限不符時，本規格不遞送 `#PF`：Machine 記下錯誤（線性位址、PTE、存取型別），`Machine.Step` 在該步結束後
  失敗即關閉。`#PF` 遞送另立規格（§10 S6），前提是量到 STRIKE 依賴範圍監看（`SR`）命中。
- 有效分頁時取指令快路徑（`CPU.Code`）停用；任何改變 `PE`、`PG`、A20 的寫入都呼叫 `syncCodeFastPath`。
- 多 byte 存取跨頁時逐 byte 轉換。

## 7. 交接點

### 7.1 實模式→保護模式

既有核心執行 `mov cr0,r32` 或 `lmsw`，寫入使 PE 由 0 變 1：完成這道指令後回傳哨兵 `ErrModeSwitch`；`Machine.Step` 依 §4
把狀態交給 cpu386（段快取維持實模式推得的值），下一步由 cpu386 執行。哨兵不進失敗計數。

### 7.2 保護模式→V86

cpu386 的 `iretd` 彈出 `EFLAGS.VM=1`，或任務切換載入的 TSS `EFLAGS.VM=1` 時，完成後交棒。實作在後續規格（S4）。

### 7.3 V86→保護模式

既有核心在 V86 遇到下表事件時不自己處理，回傳 `V86Event{Kind, Vector, HasError, ErrorCode, InstrIP, NextIP}`，由 Machine 交給
cpu386 依 IDT 遞送：

- 堆疊切到 TSS 的 `SS0:ESP0`，依序推 `GS`、`FS`、`DS`、`ES`（同時清為 0）、`SS`、`ESP`、`EFLAGS`（此時 VM 仍為 1）、`CS`、`EIP`，
  有錯誤碼再推錯誤碼（DOSBox-X `cpu.cpp:1414-1427`、`1444-1448`）。框架為 `0x24`／`0x28` bytes（無／有錯誤碼）。
- 推完後清 `TF`、`NT`、`VM`；中斷閘再清 `IF`，陷阱閘保留 `IF`（`cpu.cpp:1467-1472`）。`RF` 不動（DOSBox-X 不設 RF）。
- 只接受 32 位元閘；16 位元閘、目的碼段 DPL≠0、conforming 碼段：失敗即關閉（DOSBox-X 分別為 `E_Exit` 或 `#GP`，
  `cpu.cpp:1362-1364`、`1424`、`1439-1440`）。
- IDT 界限不足或閘不存在：`#GP`／`#NP`，錯誤碼 `n*8+2`（`cpu.cpp:1313`、`1336-1338`）。

| 事件 | 條件 | 遞送 | 框架的 `EIP` |
|---|---|---|---|
| `INT n`（`CD nn`）、`INTO`（`CE`，OF=1） | IOPL=3 且閘 DPL=3 | 依 IDT 第 n 項（`INTO` 為 4） | `NextIP` |
| 同上 | IOPL=3 且閘 DPL<3 | `#GP`，錯誤碼 `n*8+2`（`cpu.cpp:1317-1320`） | `InstrIP`（`cpu.cpp:1181`；`core_normal.cpp:115-116`） |
| 同上 | IOPL<3 | `#GP(0)`（`cpu.cpp:1302-1308`） | `InstrIP` |
| `INT 3`（`CC`） | 不看 IOPL（`core_normal/prefix_none.h:1054`）；閘 DPL<3 時 `#GP(3*8+2)` | 向量 3 | 同上兩列 |
| `CLI`、`STI`、`PUSHF`、`POPF`、`IRET` | IOPL<3 | `#GP(0)` | `InstrIP` |
| 特權指令：`HLT`、`LGDT`、`LIDT`、`LMSW`、`CLTS`、`MOV CRn`、`MOV DRn` | 一律 | `#GP(0)` | `InstrIP` |
| `LLDT`、`LTR`、`SLDT`、`STR`、`LAR`、`LSL` | 一律 | `#UD`（`core_normal/prefix_0f.h:22`、`128`、`141`） | `InstrIP` |
| 其他 CPU 例外（`#DE`、`#UD`） | 依 386 規則 | 依向量 | 依 386 規則 |
| 硬體中斷 | PIC 有待處理 IRQ 且 `IF=1` | 向量固定 `08h+irq`（JEMM 不重新程式化 PIC，`004` §1） | 下一道指令 |

IOPL=3 時 `CLI`、`STI`、`PUSHF`、`POPF`、`IRET` 在 V86 原生執行（`cpu.cpp:756-772`、`774-800`、`1542-1584`）。
`SMSW`、`SGDT`、`SIDT` 在 V86 不特權、原生執行（`prefix_0f.h:74-81`、`91-93`）；`smsw` 讀到 PE=1。
`TF=1` 時（單步）：本規格失敗即關閉，`#DB` 在 S6。

### 7.4 I/O 權限

V86 的 I/O 指令一律查 TSS 的 I/O 權限點陣圖（與 IOPL 無關，`cpu.cpp:1093-1111`）：

- TSS 必須是 386 型，否則 `#GP(0)`。
- 點陣圖偏移超過 TSS 界限時 `#GP(0)`；讀一個 16 位元字，檢查從埠號起連續 `size` 個位元，任一為 1 即 `#GP(0)`。
- 檢查在任何埠存取之前、以整道指令為單位完成（既有核心的字組 I/O 是兩次 byte 存取，不可在第二個 byte 才拒絕）。
  字串 I/O 只在指令開頭以 `DX` 檢查一次（`prefix_none.h:392-413`）。

### 7.5 保護模式→實模式

不在範圍（S9）。cpu386 執行 `mov cr0` 清除 PE 時失敗即關閉。

## 8. V86 轉接

### 8.1 既有核心的掛鉤

既有核心新增可選欄位 `V86 V86Hooks`（nil 時行為完全不變）。Machine 在進 V86 時裝上、離開時移除。掛鉤負責 §7.3 的判斷與
§7.4 的檢查；記憶體走 `Machine.Read8`／`Write8`（§6），不另換 Bus。

### 8.2 dosgolem 服務層在 V86 下

V86 下 `INT n` 的處理：

- 指令位址在 `StubSeg`，且（IVT 第 n 項的段是 `StubSeg`，或指令位址正是 `StubSeg:StubOff(n)`）：由服務層處理，不交給 cpu386。
  後一個條件對應現行的 `fromOwnStub`（`internal/dos/dos.go` 約 691–694、726–728 行）：程式先裝自己的處理常式、再 chain 回
  舊的 stub 時，stub 的 `CD n` 必須由服務層接，否則 JEMM 會把它再反射回程式，形成無限遞迴。
- 其他 `INT n`（包括遊戲程式自己的 `INT 21h`）：依 §7.3 交給 cpu386。遊戲的 `INT 21h` 經 JEMM（`#GP`）反射到 IVT，跳進
  `StubSeg` 的 trampoline，才由服務層接手。

旗標回寫：這條路徑上，JEMM 反射時已在 V86 堆疊推好 `IRET` 框架，服務層處理完接著執行 stub 的 `IRET`，會把旗標從框架彈回。
所以服務層在 V86 下處理完、且下一道是 `StubSeg` 內的 `IRET` 時，把服務結果的 `CF`、`ZF`、`SF`、`PF`、`AF`、`OF` 寫進堆疊上
`SS:SP+4` 的旗標字（現行只有 `CD F2`／`F3`／`F4` 補 `CF`，`dos.go` 約 736–744、829–838 行）。

阻塞型服務：`conIn` 等以 `Rewind` 重跑 `INT` 的服務，在 V86 反射路徑上執行時 `IF=0`（INT 已清 IF）。真 DOS 的等待迴圈會
`STI`；本規格規定：V86 下阻塞型服務重跑期間，Machine 照 `IF=1` 遞送 IRQ。驗收要證明計時器照常推進。

服務層看到的記憶體：服務層經 `Machine.Read8`／`Write8` 存取，有效分頁時同樣經過轉換（§6），與 CPU 一致。

### 8.3 計時器、鍵盤、回呼

- `tick()`、`keyTick()`、`timedKeyTick()`：實模式照舊。保護模式與 V86 改為設定 PIC 待處理 IRQ，由作用中核心在指令邊界依 §7.3
  遞送；`kbdData`、`KeyIRQs`、`nextKey` 仍在遞送當下才更新，順序不變。IF 一律讀 Machine 的 `EFLAGS`。
- `startCallback`、`periodic` 的 `FarCall`、滑鼠回呼：只在實模式與 V86 觸發；保護模式期間留在佇列。`callbackFrame` 加存
  `EFLAGSHi`。

## 9. 既有核心新增的系統指令（實模式與 V86）

只在本規格開啟時生效。V86 下特權者依 §7.3 交給 cpu386。

| 指令 | 行為 | 依據 |
|---|---|---|
| `smsw r/m16`（`0F 01 /4`） | 16 位元形式寫 `CR0` 低 16 位元；`66` 前綴的暫存器形式寫整個 `CR0`；實模式與 V86 皆原生 | `prefix_0f.h:113-115`、`prefix_66_0f.h:108-110` |
| `lmsw r/m16`（`0F 01 /6`） | 只改 `CR0` 低 4 位元，不能清除 PE；設 PE 時依 §7.1 交棒（V86 下特權） | `cpu.cpp:2788-2796`（`lmsw_allow_clear_pe_bit` 預設 false） |
| `sgdt`／`sidt m`（`0F 01 /0`、`/1`） | 存 16 位元界限與完整 32 位元基底（386 不遮成 24 位元）；實模式與 V86 皆原生 | `prefix_0f.h:74-81`、`91-93` |
| `lgdt`／`lidt m`（`0F 01 /2`、`/3`） | 16 位元運算元只取 24 位元基底，`66` 前綴取整個 dword；暫存器運算元為 `#UD`（V86 下特權） | `prefix_0f.h:85`、`89`、`107-112`；`prefix_66_0f.h:80`、`84` |
| `mov r32,crN`、`mov crN,r32`（`0F 20`、`0F 22`） | 讀寫 §4.1；寫 `CR0` 依 §7.1 | `cpu.cpp:2580-2673` |
| `mov r32,drN`、`mov drN,r32`（`0F 21`、`0F 23`） | 讀寫 §4.1 | `cpu.cpp:2695-2749` |
| `clts`（`0F 06`） | 清 `CR0.TS` | `prefix_0f.h:151-155` |
| `lldt`、`ltr`、`sldt`、`str`（`0F 00 /0–/3`）、`lar`、`lsl` | 實模式與 V86 皆 `#UD`。實模式下本規格失敗即關閉（DOSBox-X 送 IVT 第 6 項；刻意從嚴） | `prefix_0f.h:22`、`128`、`141` |

## 10. 後續規格

| 編號 | 內容 |
|---|---|
| S2 | cpu386 分段模式的指令覆蓋（JEMM 的 16 位元碼段、遠轉移經選擇子、`iretd`、`retfd`、`66 FF /3 /5` 等） |
| S4 | TSS、`ltr`、`iretd` 帶 NT 的任務返回、進 V86 |
| S6 | `#PF`、`#DB`、TF 單步、除錯暫存器觸發 |
| S9 | 保護模式→實模式 |
| — | VCPI、`invlpg`、4 MB 頁、`WP`、`#NM` |

## 11. 驗收

以合成程式驗證，不讀原版檔。

1. 關閉時不變：錨點測試常數不變；既有核心遇到 §9 的指令仍失敗即關閉；cpu386 平坦模式（LE 路徑）既有測試不變。
2. 開啟但程式不進 PE：除新增的 `PMProfile` 旗標外，記憶體、暫存器、狀態摘要與關閉時逐位相同。
3. 啟用條件：`ext-mem` 為 0 時回錯誤；載入後回錯誤。
4. 欄位盤點：以 `reflect` 列出 `cpu.CPU` 與 `cpu386.CPU` 的所有欄位，與白名單（共享、僅診斷、刻意不同步）比對，新增欄位未登記即失敗。
5. 往返恆等：隨機填滿共享狀態（含 `Hi`、FS／GS、旗標保留位元、`EFLAGSHi`），實模式→cpu386→實模式、V86→cpu386→V86 後
   逐位相同；段索引映射有互換的負對照。
6. 實模式系統指令：§9 每列一例（`smsw` 讀到 `0010h`、`lmsw` 不能清 PE、`sgdt` 存完整 32 位元基底、`lgdt` 16 位元取 24 位元）。
7. 實模式→保護模式：設 PE 後下一道由 cpu386 執行；暫存器、旗標、段值一致；CS 快取為實模式推得的值，在 16 位元碼段內以
   CS 基底取指令。
8. V86 事件：§7.3 每列一例，檢查 `V86Event` 的向量、錯誤碼、`InstrIP`、`NextIP`；框架為 `0x24`／`0x28` bytes；中斷閘清 IF、
   陷阱閘保留 IF。
9. V86 旗標：`popf 0` 後 IOPL 仍為 3；`IRET` 不改 IOPL；`pushfd` 不含 VM、RF；IOPL<3 時 `CLI` 回報 `#GP(0)`。
10. I/O 權限：字組跨點陣圖位元邊界、偏移超過界限、點陣圖字跨出 TSS 界限、`REP OUTSB` 只檢查一次、第二個 byte 被拒時第一個
    byte 不發生。
11. 服務層（§8.2）：`StubSeg` 內 IVT 指向 `StubSeg` 的 `INT 21h` 由服務層處理；IVT 已改指程式但指令在 `StubOff(n)` 也由服務層
    處理；IVT 指向程式且指令不在自己的 stub 時交給 cpu386；V86 下 `INT 16h AH=01h` 的 `ZF` 到得了呼叫端；阻塞型
    `INT 21h AH=08h` 在反射路徑下計時器照常推進。
12. 分頁：`003` §3 的映射（`B0000`、`C0000`–`FFFFF`、HMA）讀寫到預期實體位址；accessed／dirty 寫回；V86 寫入 R/W=0 頁回報
    錯誤、監督者寫入同一頁成功；A20 關閉時轉換後的實體位址遮 bit 20；PTE 已改、`CR3` 未重載、該頁未存取過時讀到新值；
    在三種模式各拍 `Snapshot`，`Restore` 到另一台機器後再 `Step`，Bus、`Code`、掛鉤都正確重接；服務層在映射狀態 1 讀到與
    CPU 相同的位元組。
13. 計時器與回呼：保護模式期間 IRQ 待處理、回呼留在佇列；回 V86 後依序遞送；`callbackFrame` 往返含 `EFLAGSHi`。
14. 突變自檢：刻意改壞段索引映射、V86 的 `POPF` 遮罩、`#GP` 的 `EIP` 選擇，對應測試失敗。
15. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile` 通過 `smsw`，
    進 JEMM 後停在 S2 尚未支援的第一道保護模式指令。
16. `tools/go.sh test ./internal/... ./cmd/probe ./session ./oracle` 全綠（需要顯示器的套件照現行以 `xvfb-run` 執行）。

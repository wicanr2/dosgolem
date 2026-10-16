# 432 — 實體與線性存取分層、A20、分頁與共用 TLB

狀態：**READY**（四輪審查；最後一輪的文字與契約補寫已套用於 §10）
日期：2026-10-15
前置：[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（架構契約）、[`015-execution-speed`](015-execution-speed.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

431 的混合 CPU 需要一條記憶體路徑同時服務既有核心（實模式與 V86）、cpu386（保護模式）與服務層。保護模式開啟後，線性位址要經
A20 與分頁才變成實體位址。現行 `Machine.Read8`／`Write8` 同時扮演「實體存取」與「CPU 的位址空間」，且 430 的 `ReadPhys8` 在
1 MB 以下呼叫 `Read8`（`internal/machine/extmem.go` 約 73–95 行）；直接讓 `Read8` 做分頁轉換，頁表走訪會遞迴。另外，JEMM 開分頁前
就以平坦選擇子寫 1 MB 以上的 XMS 區（430 §1），現行 `Read8` 卻把 HMA 以外的位址 `&0xFFFFF` 折回低記憶體（`machine.go` 約 807–826 行）；
386 只遮 bit 20（DOSBox-X `hardware/memory.cpp:1301-1317`，386 的 `address_bits=32`，`1935-1946`）。

## 2. 啟用條件與公開設定

「轉換開啟」（衍生欄位 `m.xlat`）＝ `PMProfile` 開啟且（`CR0.PE=1` 或既有核心在分段模式）。後者只在 442 的殘留快取實模式
（`segmented=true`、`PE=0`）成立，讓 1 MB 以上的快取基底經線性層存取（由 442 修訂）。其餘情形 `Read8`／`Write8` 與現行逐位元組相同；430 的測試
（例如 `extmem_test.go` 約 262–272 行「`Write8(0x10FFF0)` 折回」）不改。

本規格先於 433 實作，所以先提供下列公開設定（433 再擴充成完整的 `SysRegs` 與交接）：

| 方法 | 規則 |
|---|---|
| `SetProtectedModeProfile(on bool) error` | `programLoaded` 為真回錯誤；開啟時 `ExtendedMemoryKB() == 0` 回錯誤（否則 PE 由 0 變 1 時 HMA 內容會消失）；`len(m.Mem) != MemSize` 回錯誤 |
| `SetCR0(v uint32)`、`SetCR3(v uint32)` | `CR0` 一律補 `00000010h`，只保留 386 有意義的 bit 0–4 與 bit 31，其餘（含 bit 16 `WP`）寫入時清為 0；寫入後依 §5.1 作廢 TLB、重算衍生欄位、呼叫 `syncCodeFastPath` |
| `SetVM(on bool) error` | `CR0.PE=0` 時回錯誤；否則設 `EFlagsHi` 的 VM 位元並重算衍生欄位。真正的進出 V86 由 435 與 S4 處理；本規格的測試以它注入 V86 狀態，屬合成測試 |
| `SetPMCPL(cpl uint8)` | 保護模式的 CPL，由 434 的分段模式寫入；本規格內只供測試，預設 0 |

衍生欄位：`m.xlat`、`m.paging`（`xlat && CR0.PG`）、`m.cplUser`（`PE ? (VM ? true : pmCPL==3) : false`，`pmCPL` 在 434 前為 0）。
只在上述 setter、`Restore`、`LoadState` 時重算，`Read8` 熱路徑只讀衍生欄位。431 §4 規定 `CR0`、`EFlagsHi`、`pmCPL` 的所有寫入
（含 433 的 `mov cr0`、434、S4 的交接）都經這些方法，所以衍生欄位不會過期。

## 3. 兩層存取

### 3.1 實體層（不轉換）

把現行 `Read8`／`Write8` 的本體（`&0xFFFFF` 之後的 VGA 平面視窗、ROM 保護、讀寫監看、影片寫入紀錄、`Ext`）抽成
`readPhysRaw(a)`／`writePhysRaw(a, v)`，輸入是實體位址：

- `a < 0x100000`：現行低記憶體路徑。
- `0x100000 ≤ a < 0x100000 + len(Ext)`：`Ext`。
- 其餘：讀回 `0xFF`；寫入丟棄並計 `UnmappedPhysWrites`。

另有 `peekPhysRaw(a)`：不載 VGA latch、不觸發讀取監看（對應現行 `Peek8` 的實體部分）；以及頁表專用的 `pageRead32`／`pageWrite32`：
直接讀寫 `Mem`／`Ext`，不經 VGA、ROM 保護與監看，另計 `PageTableWrites`（診斷計數，不存檔）。

`ReadPhys*`／`WritePhys*`（430）改用實體層；430 §2.1 補一行修訂紀錄指向本規格。

`Read8` 的轉換關閉路徑也改為呼叫 `readPhysRaw`（只保留前面的 `hmaAddr` 與 `&0xFFFFF`），避免兩份本體。以隨機差分測試證明：對隨機
`a < 1 MB` 與各種監看、平面模式組合，新舊路徑逐位元組相同（`machine.go` 約 769–795 行的 `syncCodeFastPath` 判準表同步改寫，加入「轉換開啟」一欄）。

### 3.2 線性層（`Read8`／`Write8`）

- 轉換關閉：與現行相同。
- 轉換開啟：線性位址 `a`（32 位元）→ `m.paging` 時經 §5 轉換成實體位址，否則實體＝線性 → 對這個實體位址套 A20：
  `phys = a20 ? phys : phys &^ 0x100000` → 實體層。分頁時 A20 套在轉換之後的實體位址上。多 byte 存取（`Read16`、`Write16`）逐 byte 轉換；跨頁且第二頁錯誤時，第一個 byte 的寫入已發生（與逐 byte 的現行實作一致，
  寫入收據）。

### 3.3 監督者存取

新增 `ReadSup8`／`ReadSup16`／`ReadSup32`、`WriteSup8`／`WriteSup16`／`WriteSup32`：與線性層相同，但一律以監督者身分檢查權限
（不論目前 CPL）。供 435 經 IDT 遞送 V86 事件時讀 IDT、TSS、寫 ring 0 堆疊（讀 IDT、TSS 對應 DOSBox-X 以 `cpu.mpl=0` 標示的隱含特權存取，`cpu/paging.cpp:786`；
`cpu/cpu.cpp:662-673`、`840-859`、`1095-1104`；寫 ring 0 堆疊則對應 `CPU_SetCPL(cs_dpl)`（`cpu.cpp:1413`）之後的監督者身分）。不採全域旗標，避免設定後漏還原。cpu386 的 Bus 轉接在 434。

### 3.4 呼叫點分類

實作時依第 1 輪審查的 grep 結果列入收據；`LEMachine` 與 LE 專用匯流排（`le_machine.go`、`dpmi_real_mode.go`、`le_real_irq7.go`、
`le_protected_irq0.go`、`le_mouse_callback.go`）不在範圍。

| 類別 | 位置 | 改用 |
|---|---|---|
| CPU 的位址空間 | 既有核心的 `Bus`、既有核心分段模式（434） | 線性層 |
| 服務層的客體指標 | `internal/dos` 的 `cpu.Addr(` 呼叫，含 `fixStackedCF` | `SvcAddr`（§4）＋線性層 |
| 硬體與 BIOS 側 | `bda.go` 的鍵盤緩衝與 BDA、`bumpBDATicks`、文字畫面讀出、`cga.go`、`oracle/oracle.go` 的畫面讀出、`cmd/probe` 的 IVT 狀態輸出 | 實體層（`ReadPhys*`／`WritePhys*`） |
| 客體可見的 IVT | `keyboard.go`／`keyhold.go` 檢查 IVT[09h] 是否仍指 StubSeg、`internal/dos` 的 `handle` 讀 IVT | 線性層的監督者讀取（`ReadSup16`）；由 435 §4 取代原先的實體層：這個檢查的語意是「客體有沒有掛向量」，V86 下客體與 JEMM 都以線性位址讀寫 IVT |
| 載入與初始化 | `initBDA`、`initVectors`、`loader.go`、`WriteMCB`、`SetExtendedMemoryKB`、`SetEMSPresent`、PSP 命令列寫入 | 不變（發生在轉換關閉時），註明為實體語意 |
| 觀測與診斷（不得有副作用） | `pollWords`、`WatchWord` 登記、服務層的診斷回溯（`bios.go` 的 `noteKeyWord` 呼叫者、`files.go` 的 `noteMissingAccess`、`exec.go` 的 `CallSite`、`dos.go` 的 `noteCPU`）、`cmd/probe` 的 dump | §6 |
| 服務層的固定結構（手算 `seg*16`） | `int9.go` 與 `dos/bios.go`、`dos/dos.go` 的 BDA 存取、`VideoSeg`、`StubSeg`、EMS 頁框等 | 不改，經線性層；依賴 BDA 與 `StubSeg` 在 JEMM 的兩種映射狀態都恆等映射（§4） |
| 直接動 `Mem`／`Ext` | `sft.go`、`callback.go`、`Indexed`、`SegmentBytes`、`Find`、`session/digest.go` | 不變，實體語意 |
| oracle 的段:偏移 API | `oracle` 的 `Byte`、`Word`、`SetByte` 等，以及讀寫客體堆疊的 `oracle/run.go`、`oracle/state.go`、`oracle/call.go`（`cpu.Addr(ss,sp)` 加 `Read16`／`Write16`） | 位址仍以 `cpu.Addr` 計算（20 位元段:偏移，不在本規格範圍）；存取經線性層，轉換開啟時會走分頁，錯誤由 `PageFault()` 查詢 |

讀寫監看、`noteWrite`、影片寫入紀錄比對的是實體位址；轉換開啟時文件改稱「實體位址」。影片寫入紀錄與 `VRAMSites` 讀的
分段模式下 `m.CPU.OpAddr()` 回選擇子:偏移，線性位址改用 434 §7 的 `InsnLinear()`。

## 4. 服務層的位址計算

`internal/dos` 有 46 處 `cpu.Addr(seg, off)`，`Addr` 先遮 20 位元（`internal/cpu/cpu.go` 約 235–237 行）。轉換開啟且 A20 開時，
客體的 `FFFF:0010` 起是線性 `0x100000` 起，服務層卻會讀到 0 起。

新增 `Machine.SvcAddr(seg, off uint16) uint32`：轉換關閉時回 `cpu.Addr(seg, off)`（行為不變）；轉換開啟時回 `seg*16 + off`（不遮罩，
同 `cpu.Linear`，`cpu.go` 約 253–255 行）。46 處全數改用。手算的 `seg*16`（BDA、`VideoSeg`、`StubSeg`、EMS 頁框等固定結構，約 25 處）
本來就不遮罩，不改。

範圍限定：profile 開啟但仍在實模式、A20 由 XMS 打開時，服務層讀 HMA 的指標維持現狀（`cpu.Addr` 折回），不在本規格範圍。

服務層走 `SvcAddr` 加線性層，本來就看得到 JEMM 的別名，所以不需要「呼叫 DOS 時一定在映射狀態 0」的前提。只有服務層以手算位址存取的
固定結構（BDA 在線性 `0x400`、`StubSeg` 在 `0x800`）依賴恆等映射：兩者都在 0 頁，JEMM 的兩種映射狀態都不改 0 頁（`strike_commander`
`docs/re/003-jemm-api.md` §3），強推論。

加 `repolint` 規則（`internal/repolint` 已有 AST 檢查先例）：`internal/dos` 非測試檔不得呼叫 `cpu.Addr(`。沒有白名單：診斷回溯也改用 `SvcAddr`（§6）。

## 5. 分頁

- 有效分頁＝轉換開啟且 `CR0.PG=1`（DOSBox-X `cpu/cpu.cpp:2629-2632`）。
- 兩層 4 KB 頁表。頁目錄基底取 `CR3 & 0xFFFFF000`。PDE bit 7（PS）、`PCD`、`PWT` 在 386 忽略。不支援 4 MB 頁與 `WP`。
- 頁表走訪以 `pageRead32`／`pageWrite32` 存取實體位址，並套 A20 遮罩。這點刻意與 DOSBox-X 不同（DOSBox-X 的 `phys_readd` 不經 A20，
  `include/mem.h:242-264`）：真機的 A20 閘在匯流排上，走訪也受影響。差異只在 A20 關且頁表位於 1 MB 以上時出現，JEMM 開分頁前 A20
  已開。走訪位址超出 `Ext` 時讀到 `FFFFFFFFh`（同 DOSBox-X `mem.h:263`），即 P=1 的垃圾項。

### 5.1 TLB

- Machine 持有，兩個核心與服務層共用。按需填入：某頁第一次經轉換時才走頁表（同 DOSBox-X 的惰性 TLB，`cpu/paging.cpp:779-905`）。
- 條目：世代、實體頁號、權限等級（PDE 與 PTE 的 U/S、R/W 化簡成「監督者讀、監督者讀寫、使用者讀、使用者讀寫」四級，同
  `cpu/paging.cpp:150-172` 的 `translate_array`）、PTE 的實體位址、dirty 已寫回旗標。權限在查表時依 CPL 比對，CPL 改變不必作廢。
- 結構：兩層稀疏表。第一層以 4 MB 為單位，第二層是懶配置的 1024 筆陣列；不逐出，所以「舊 PTE 仍被使用」的行為不取決於碰撞
  （同 DOSBox-X 的完整大小 TLB，`include/paging.h:62`）。
- 作廢：寫 `CR3`、`CR0.PE` 或 `PG` 改變、`SetA20` 時，世代號加一（uint32，溢位時整份清除）。對照：`PAGING_SetDirBase`
  （`cpu/paging.cpp:1405-1414`）、`PAGING_Enable`（`1424-1436`）、A20 變動（`hardware/memory.cpp:1301-1317`）。A20 變動時作廢是對齊
  DOSBox-X；真機的 A20 閘在 TLB 之後、不作廢 TLB，這是已知差異（與 §5.2 採真機語意的立場不同，因為這裡跟 DOSBox-X 對拍較重要）。
- 不存檔；`Restore`、`LoadState` 後世代加一。

### 5.2 accessed 與 dirty

照 DOSBox-X 的順序（`cpu/paging.cpp:841-844`、`863`、`877-880`）：

- PDE 的 P=1 即寫回 PDE 的 accessed，早於 PTE 的存在與權限檢查。
- PTE 的 accessed（與寫入時的 dirty）在權限檢查通過後才寫回；權限錯誤時不改 PTE。
- 讀入的條目之後被寫入時：以條目記住的 PTE 位址讀改寫 dirty 位元，不重走權限；若 PTE 的實體頁已改變，仍用條目的舊實體頁
  （保持舊轉換語意，不像 DOSBox-X `paging.cpp:360-361` 那樣中止）。dirty 已寫回後不再改頁表。
- 不採 DOSBox-X 在權限錯誤時重讀頁表、屬性變了就清 TLB 重試的 `hack_check`（`paging.cpp:439-469`、`588-596`）：本規格採真機語意。
  與 DOSBox-X 對拍時這是已知差異。

### 5.3 權限與錯誤

- 使用者存取（`m.cplUser`）：PDE 與 PTE 的 U/S 任一為 0 即錯誤；寫入時 R/W 任一為 0 即錯誤（`cpu/paging.cpp:210-230` 的
  `fault_table`，WP=0）。監督者存取不檢查 U/S、R/W。
- 隱含監督者存取：經 §3.3 的 `ReadSup*`／`WriteSup*`，不檢查 U/S 與 R/W。
- 頁不存在（PDE 或 PTE 的 P=0）：錯誤。
- 本規格不遞送 `#PF`，錯誤紀錄的生命週期：
  - 型別 `PageFaultRecord{Linear uint32; PDE, PTE uint32; Write, User bool; Steps uint64}`。`Steps` 記錄產生時的 `m.Steps`（`tick`、
    `keyTick` 在 `m.Steps++` 之前執行，它們產生的紀錄因此帶上一步的序號，這是已知的序號語意）。同一步多次錯誤只保留第一筆。
    讀回 `0xFF`、寫入丟棄。
  - `Machine.Step` 的第一個動作（在 `tick`、`keyTick` 之前）把殘留的紀錄移到 `LastPageFault()` 並清空 `PageFault()`；之後在 `Step` 內
    產生的紀錄，於該步結束時讓 `Step` 回傳錯誤，失敗即關閉。`CPU.Step()` 自己也回錯誤時，以 CPU 的錯誤為主，分頁紀錄附在錯誤訊息後。
  - `Step` 以外的存取（oracle 的 `Byte`、`SetByte`、probe 的 dump 等）產生的紀錄由 `PageFault()` 查詢，不影響下一個 `Step`。
  - 觀測類存取（§6）不產生紀錄。
  - `#PF` 遞送在後續規格 S6。

### 5.4 取指令快路徑

轉換開啟時 `CPU.Code` 一律停用（A20 開時現行已停用，所以 JEMM 場景沒有新損失）；所有 setter、`Restore`、`LoadState` 呼叫
`syncCodeFastPath`。`syncCodeFastPath` 的判準看 `m.xlat`，所以 442 的殘留快取實模式在 A20 關閉時也停用 `CPU.Code`；離開分段模式的路徑
（442 的 `leaveSegmented`）同樣經 `recalcPM` 呼叫它（由 442 修訂）。

## 6. 觀測

- `Peek8`、`Peek16` 簽章不變：轉換開啟時以客體線性視角、監督者身分轉換（不因 U/S、R/W 失敗），只有頁不存在時回 `0xFF`；另增
  `PeekOK(a) (uint8, bool)`，頁不存在時回 `(0xFF, false)`，其餘回 `(值, true)`。
- 觀測的轉換：TLB 條目世代符合就用（看到 CPU 此刻會看到的，含舊轉換）；不符就無副作用地走頁表，不填 TLB、不寫 accessed／dirty、
  不產生錯誤紀錄。
- `pollWords` 與 `WatchWord` 登記：轉換開啟時改用 `Peek16`；轉換關閉時維持 `Read16`（`Peek` 對平面 VGA 視窗與讀取監看的行為不同，
  改了會讓轉換關閉的路徑變動）。
- 服務層的診斷回溯：一律以 `SvcAddr` 計算位址；轉換開啟時以 `Peek` 讀取，轉換關閉時維持現行的 `Read8`（同 `pollWords` 的理由）。

## 7. 狀態檔

依 431 §4.2：本規格實作開頭的「只加欄位」commit 在 `machineState` 加一個 `PM []byte`（`omitempty`），更新錨點測試的 gob 位元組常數，
並修改錨點測試檔頭與 `internal/machine/state.go` 的政策註解（「語意錨點永不變；gob 位元組在 430、432 各變一次，之後保護模式狀態只改
`PM` 的內部格式」）。`PM` 的內容（版本 1，共 16 bytes，小端序）：

| 位移 | 大小 | 欄位 |
|---|---|---|
| 0 | 2 | 版本號，`1` |
| 2 | 1 | `PMProfile`（0 或 1） |
| 3 | 1 | `pmCPL` |
| 4 | 4 | `CR0` |
| 8 | 4 | `CR3` |
| 12 | 2 | `EFlagsHi` |
| 14 | 2 | 保留，寫 0 |

433–435 以提升版本號在尾端增加內容，讀取時支援所有舊版本（舊版本缺的欄位取初值）。profile 關閉時 `PM` 為 nil。

`LoadState`：`PM` 與機器設定（profile 開關）不一致時回錯誤，檢查在任何寫入之前。`Snapshot`／`Restore` 保存同樣的內容；`Restore`
沒有錯誤回傳，profile 開關不一致時 panic（比照 430 對 `Ext` 長度的處理），訊息寫明兩邊設定。

## 8. 不在範圍

`#PF` 遞送、`invlpg`、4 MB 頁、`WP`、保護模式的 CPL 來源（434）、`SysRegs` 其餘欄位與 `-pm-profile` 旗標（433）、oracle 段:偏移 API 的
視角、profile 開啟但仍在實模式時服務層的 HMA 指標。

## 9. 驗收

測試放置：Machine 層放 `internal/machine`；需要服務層的放 `internal/state` 或以公開 setter 從 `internal/dos` 測。

1. 轉換關閉不變：錨點測試的語意常數不變；430 全部測試不改照常通過；`internal/dos` 改用 `SvcAddr` 後既有測試全綠；隨機差分測試
   （§3.1）通過；profile 開、`PE=0`、A20 開時 `Read8(0x100000)`、`Read8(0x10FFF0)` 與 profile 關閉相同。
2. 實體層：轉換開啟、頁表把線性 0 頁映到別處時 `ReadPhys8(0)` 仍讀實體 0；把 PDE／PTE 放在「線性映到別處」的位址，走訪仍讀實體位址
   （不遞迴）。
3. 無分頁（`PE=1`、`PG=0`）與分頁時的 A20：分頁開啟、A20 關閉時，轉換後落在 `0x1xxxxx` 的實體位址被遮成 `0x0xxxxx`；`0x300000` 在 A20 關時落在 `0x200000`；A20 開時 `0x10FFF0` 以上不折回（轉換關閉會折回，以測試釘住兩者
   不同）；`Ext` 範圍外讀 `0xFF`、寫入計數；`PE=0` 而 `PG=1` 時不分頁。
4. 分頁映射：以合成頁表重現 `003-jemm-api.md` §3 的映射（`B0000`–`B7FFF`、`C0000`–`FFFFF` 指向 `B8000`，HMA 指向 `A0000`），讀寫到預期
   實體位址；線性 `F0000` 映到 `B8000` 時寫入成功；線性頁映到 `ROMBase` 以上的實體頁時寫入被丟棄並計 `ROMWrites`；跨頁且兩頁實體
   不連續的 `Read16`、`Write16` 正確。
5. 舊轉換：改 PTE 而未重載 `CR3`、該頁未存取過時讀到新值，已存取過時讀到舊值；寫 `CR3` 後讀到新值；A20 切換後舊條目作廢；
   不同頁號在第一層、第二層的條目互不影響。
6. accessed 與 dirty：權限錯誤時 PDE 的 accessed 已設、PTE 的 accessed 與 dirty 未設；讀不設 dirty；讀後寫設一次 dirty，第二次寫入
   `PageTableWrites` 不增加。
7. 權限（合成情境，只存在於測試）：以 `SetVM(true)` 注入 V86 並由既有核心執行 `mov [mem],al`：寫 R/W=0 頁使 `Step` 失敗、紀錄含線性位址與 PTE；PDE 與 PTE 的
   U/S 各自為 0 的兩個方向都錯誤；`SetVM(false)`、`SetPMCPL(0)` 時同一道指令寫 R/W=0 頁成功（此時依 431 應由 cpu386 執行，這裡只驗 Machine 的權限判斷）；P=0 分別
   發生在 PDE、PTE；CPL=3 時 `ReadSup8` 讀 U/S=0 頁、`WriteSup8` 寫 R/W=0 頁成功；`SetVM(true)` 在 `PE=0` 時回錯誤。
8. 錯誤紀錄生命週期：`PageFaultRecord` 各欄位正確；同一步兩次錯誤只留第一筆；錯誤發生一次後，下一個沒有錯誤的 `Step` 回 nil 且
   `LastPageFault()` 是上一筆；`Step` 外的 `Read8` 錯誤不讓下一個 `Step` 失敗，可由 `PageFault()`
   查到；`Peek8` 不產生紀錄。
9. 服務層：轉換開啟時 `SvcAddr(0xFFFF, 0x10) == 0x100000` 且不等於 `cpu.Addr(0xFFFF, 0x10)`；轉換關閉時兩者相等；在合成的映射狀態 1 下，
   服務層與既有核心讀同一個線性位址得到相同 byte；`repolint` 規則對新增的 `cpu.Addr(` 呼叫失敗。
10. 觀測：`Peek8` 不改 accessed、不填 TLB、不產生紀錄；對 U/S=0 頁仍讀得到；`PeekOK` 對不存在的頁回 `(0xFF, false)`；轉換關閉時
    `pollWords` 與診斷回溯的行為不變。
11. 狀態與快照：`PM` 版本 1 的位元組佈局逐欄位測試；`PM` 往返（profile 開）；profile 關時 `PM` 為 nil、語意錨點不變；`PM` 與機器設定不一致時 `LoadState` 回錯誤且機器不變；
    同一 profile 下，`PE=1` 的機器 `Restore` `PE=0` 的快照（及反向）後衍生欄位、TLB 世代、`CPU.Code` 正確；profile 開關不一致的快照
    `Restore` 時 panic。
12. 效能（431 §7）：`BenchmarkStepRealMode` 在行為 commit 之前先提交；轉換關閉時對固定合成程式交錯跑新舊各五次取最快，不低於修改前的
    97%；新增 `BenchmarkStepV86`（`SetVM` 注入，無分頁）與 `BenchmarkStepV86Paged`（合成恆等頁表），後者不低於 11.58 M 道／秒
    （431 §7；量不到時依 431 §7 另立優化規格，本規格的功能驗收不因此失敗，但收據要記錄量測值）；`BenchmarkMemLinear` 與
    `BenchmarkMemPaged` 的數值記入收據，不做斷言；連續 10 萬次 `SetCR3` 以 `testing.AllocsPerRun` 量測為 0 次配置。
13. `go build ./...`、`go vet ./...`、`tools/go.sh test ./internal/... ./cmd/probe ./session ./oracle` 全綠（需顯示器的套件照現行以
    `xvfb-run`）。

## 10. 補充規則（第 4 輪複審）

以下規則優先於前文中與之衝突的敘述。

1. **哨兵與分頁錯誤**：`CPU.Step()` 回傳控制流哨兵（433 的 `ErrModeSwitch`、435 的 `V86Event`）且同一步有分頁紀錄時，`Machine.Step`
   回分頁錯誤，哨兵不遞送。錯誤型別匯出為 `*PageFaultError`（含 `PageFaultRecord`）；與 CPU 錯誤並存時以 `%w` 包住 CPU 錯誤，
   `errors.Is`、`errors.As` 都可用。
2. **profile 關閉時的 setter**：`SetCR0`、`SetCR3`、`SetEFlagsHi`、`SetVM`、`SetPMCPL` 在 profile 關閉時回錯誤（`SetCR0`、`SetCR3` 的簽章
   因此回傳 `error`）。`SetProtectedModeProfile` 只能在 `CR0` 為初值時切換，切換後重算衍生欄位；開啟時另要求 `m.CPU.Model == Model80386`。
   `SetPMCPL` 要求 `cpl <= 3`。`SetCR0` 把 PE 由 1 寫成 0 而 `VM=1` 時回錯誤（431 §5：保護模式→實模式不支援）。
3. **`EFlagsHi`**：新增 `SetEFlagsHi(hi uint16) error`，只接受 RF（bit 0）與 VM（bit 1），`VM=1` 需 `PE=1`；`SetVM` 是它的包裝。
   `EFlagsHi = EFLAGS >> 16`。
4. **公開存取器**：`PageFault() (PageFaultRecord, bool)`、`LastPageFault() (PageFaultRecord, bool)`；`CR0()`、`CR3()`、`PMCPL()`、
   `EFlagsHi()`；`Translating() bool`（即 `m.xlat`，供 `internal/dos` 的診斷回溯選擇 `Peek` 或 `Read8`）；`PeekOK16(a) (uint16, bool)`。
5. **錯誤紀錄細節**：PDE 的 P=0 時 `PTE` 欄位填 0。TLB 命中而權限錯誤時，紀錄的 PTE 由條目記住的 PTE 位址重讀，PDE 由 `CR3` 與線性位址
   重算位址後讀取（皆經 `pageRead32`，無副作用）。P=0 與權限錯誤的走訪不填 TLB（沒有負向快取）。`Step` 第一動作在沒有殘留紀錄時
   不動 `LastPageFault()`。`Step` 以外存取連續產生多筆時，`PageFault()` 非空就不覆蓋。`Restore`、`LoadState` 清空兩個紀錄。
6. **`LoadState` 對 `PM` 的驗證**（全部回錯誤，且在任何寫入之前）：未知版本號、長度小於該版本的大小、`PM[2]`（`PMProfile`）不為 1、
   `pmCPL > 3`、`CR0` 含 386 保留位元、`EFlagsHi` 含 RF 與 VM 以外的位元、`VM=1` 而 `PE=0`。
7. **監督者存取的用途**：除了 435 的 IDT、TSS 與 ring 0 堆疊，434 的分段模式載入段描述元時讀 GDT、LDT（通常 U/S=0），以及
   435 的 I/O 權限點陣圖，都經 `ReadSup*`。
8. **字組監看的位址語意**：`pollWords`、`WatchWord` 在轉換開啟時用 `Peek16`，位址是線性視角，與讀寫監看（實體位址）不同。`m.xlat`
   改變的那一步重新取樣 `last` 值，不發出變化通知。
9. **走訪順序**：跟隨 DOSBox-X（`cpu/paging.cpp:837-844`）：先讀 PDE、再讀 PTE，然後寫 PDE 的 accessed，最後做 PTE 的存在與權限檢查。
   頁目錄自我映射時結果因此與 DOSBox-X 相同。
10. **固定結構的分類**：BDA 與 `StubSeg` 依賴恆等映射；`VideoSeg` 與 EMS 頁框在服務層經線性層時看得到 JEMM 的別名，這是正確行為。
    手算 `seg*16` 的清單包含 `exec.go` 的 `psp*16`（約 203、213、493 行）與 `int10text.go`；直接動 `Ext` 的列包含
    `internal/dos/xms_ext.go` 約 288 行。
11. **A20 作廢的立場**：A20 變動時作廢 TLB 對齊 DOSBox-X，是因為 A20 在 JEMM 執行期間不會改變（SCCD 於載入 JEMM 前開啟，`002` §1），
    作廢與否不影響觸發案例，選擇與對照組一致較容易判讀差異；「真機的 A20 閘在 TLB 之後、不作廢 TLB」是推論（假說），未查證手冊。
    `hack_check` 會改變遊戲可觀察的行為，所以那裡採真機語意。
12. **`CR0` 讀回與 DOSBox-X 的差異**：本規格清除 386 保留位元，DOSBox-X 只補 `FPUPRESENT`、不清其他位元（`cpu.cpp:2582`），對拍時
    `CR0` 讀回可能不同，屬已知差異。

驗收補充（併入 §9 執行）：

- 第 1 項：取指令落在不存在頁、讀回 `0xFF` 被解成哨兵時，`Step` 回 `*PageFaultError` 且 `errors.As` 取得紀錄。
- 第 2、3、6 項：每條錯誤條件各一例。
- 第 5 項：TLB 命中的權限錯誤紀錄含正確 PDE、PTE；P=0 後修好 PTE 而不重載 `CR3`，再存取成功（證明沒有負向快取）。
- 第 8 項：`m.xlat` 改變那一步不產生變化通知。
- 第 9 項：頁目錄自我映射的合成頁表，accessed 位元結果與預期相同。


## 11. 實作收據（2026-10-15）

commit：`418edb2`（基準）、`92d6138`（只加 `PM` 欄位）、`09314fb`（C1：兩層存取、`SvcAddr`、setter、`PM` 編碼）、C2（分頁與 TLB，
見 `git log`）。

### 驗收結果

- 第 1–11、13 項與 §10 補充驗收：測試通過（容器內 `go build ./...`、`go vet`、`go test ./internal/... ./cmd/probe ./session ./oracle`）。
  錨點語意常數全程不變；gob 位元組常數只在 `92d6138` 改變一次。
- 突變自檢：C1 的熱路徑捷徑與監看條件 5 個突變、C2 的 accessed 順序、U/S 改 OR、拿掉 TLB 作廢，皆被測試抓到，已還原。
- 第 12 項效能：
  - `BenchmarkStepV86Paged` 15.8–15.9 M 道／秒，高於 11.58 M。`BenchmarkStepV86` 20.4–22.0 M。`BenchmarkMemLinear` 147–200 M 讀／秒，
    `BenchmarkMemPaged`（TLB 命中）97.5–111 M。
  - profile 關閉的 `BenchmarkStepRealMode` 與 `418edb2` 交錯比較（C1 五輪、C2 二十五對），主機 load average 13–30，單次樣本差 2–3 倍。
    最快值新舊相當（C2：新 28.26、舊 28.79 ns/op），但單輪比值落在 87%–112%，**未能證明穩定不低於 97%**。待主機安靜時重量，
    本項維持開放。

### 實作期間的處置

1. `SetProtectedModeProfile` 切換成功時把 `CR3`、`EFlagsHi`、`pmCPL` 回初值；同值呼叫回 nil。檢查順序：`programLoaded`、`Ext`、`len(Mem)`、
   `Model`、同值、`CR0` 非初值。
2. `LoadState` 讀入的 `CR0` 一律補 `0x10`；`PM` 尾端多出的 bytes 不報錯，位移 14 的保留欄位不驗證。
3. `SetVM(false)` 在 `PE=0` 時不報錯。
4. `pageWrite32` 寫到範圍外時丟棄、不計 `UnmappedPhysWrites`；`PageTableWrites` 只在值有改變時遞增（同 DOSBox-X 只在需要時更新）。
5. 診斷回溯除 `CallSite` 外，`exec.go` overlay 紀錄的 `PBRaw` 也改 `diagRead8`。
6. `cmd/probe` 的 dump 用 `peek8`／`peek16`；B8000 與 CGA 輸出歸硬體側，用 `ReadPhys8`。
7. 錯誤紀錄的 PDE 是 accessed 寫回之後的值；PTE 的 P=0 時紀錄讀到的 PTE 值。
8. `*PageFaultError` 的 `Error()` 是 CPU 錯誤訊息加分頁錯誤說明（§10 第 1 項優先於 §5.3）；CPU 錯誤與分頁錯誤並存的真實整合情境要到
   433、435 有哨兵後才能測，目前以自訂錯誤驗證包裝語意。
9. `tlbMarkDirty` 不檢查 PTE 的 P：條目填入後 PTE 被清成 P=0，dirty 仍寫上（不重走權限）。
10. 純 `machine` 的合成測試與基準一律 `IRQ0Every = 0`：沒有掛服務層時，`int 1Ch` 的預設 stub 會遞迴（既有行為，不在本規格範圍）。
11. TLB 第一層陣列以指標在第一次填入時配置，避免放在 `Machine` 結構內拖慢實模式。

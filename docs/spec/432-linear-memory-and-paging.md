# 432 — 實體與線性存取分層、A20、分頁與共用 TLB

狀態：**DRAFT**（第 1 版）
日期：2026-10-15
前置：[`430-physical-memory-and-xms-unification`](430-physical-memory-and-xms-unification.md)、
[`431-hybrid-cpu-protected-mode`](431-hybrid-cpu-protected-mode.md)（架構契約）、[`015-execution-speed`](015-execution-speed.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

431 的混合 CPU 需要一條記憶體路徑同時服務三種使用者：既有核心（實模式與 V86）、cpu386（保護模式）、dosgolem 服務層。
保護模式開啟後，線性位址要經 A20 與分頁才變成實體位址。現行 `Machine.Read8`／`Write8` 同時扮演「實體存取」與
「CPU 的位址空間」兩個角色，且 430 的 `ReadPhys8` 在 1 MB 以下呼叫 `Read8`（`internal/machine/extmem.go` 約 73–95 行）。
若直接讓 `Read8` 做分頁轉換，頁表走訪會遞迴（第 2 輪審查阻擋 B1）。

另一個問題：JEMM 在開分頁前就以平坦選擇子寫到 1 MB 以上的 XMS 區（430 §1），但現行 `Read8` 把 HMA 以外的位址一律
`&0xFFFFF` 折回低記憶體（`machine.go` 約 800–813 行）。386 與 DOSBox-X 只遮 bit 20（`hardware/memory.cpp:313-320`）。

本規格把存取分成兩層，並定義 A20、分頁與 TLB。

## 2. 啟用條件

本規格的轉換只在 `PMProfile`（433 的 `SetProtectedModeProfile`）開啟且 `CR0.PE=1` 時生效，稱為「轉換開啟」（`m.xlat`）。
其餘情形（profile 關閉，或實模式）`Read8`／`Write8` 與現行逐位元組相同；430 的測試（例如 `extmem_test.go` 約 262–272 行
「`Write8(0x10FFF0)` 折回」）不改。

本規格先於 433 實作，所以 `CR0`、`CR3` 與 `PMProfile` 旗標在本規格定義為 `Machine` 欄位（433 再擴充成完整的 `SysRegs`）。
測試以 `export_test.go` 直接設定這些欄位。

## 3. 兩層存取

### 3.1 實體層（不轉換）

把現行 `Read8`／`Write8` 的本體（HMA／`Ext`、`&0xFFFFF`、VGA 平面視窗、ROM 保護、讀寫監看、影片寫入紀錄）抽成
`readPhysRaw(a)`／`writePhysRaw(a, v)`：輸入是**實體位址**：

- `a < 0x100000`：現行低記憶體路徑（含 VGA、ROM、監看）。
- `0x100000 ≤ a < 0x100000 + len(Ext)`：讀寫 `Ext`（430）。
- 其餘：讀回 `0xFF`；寫入丟棄並計 `UnmappedPhysWrites`。

`ReadPhys*`／`WritePhys*`（430）改用實體層。430 §2.1 補一行修訂紀錄指向本規格。

### 3.2 線性層（`Read8`／`Write8`）

- 轉換關閉：與現行相同（先 `hmaAddr`，其餘 `&0xFFFFF`）。
- 轉換開啟：線性位址 `a`（32 位元）→ 有效分頁（`CR0.PG=1`）時經 §5 轉成實體位址，否則實體＝線性 → 套 A20：
  `phys = a20 ? phys : phys &^ 0x100000`（同 DOSBox-X；對實模式可達範圍，結果與 `&0xFFFFF` 相同）→ 實體層。

### 3.3 呼叫點分類

| 類別 | 呼叫點（實作時以 grep 補齊，列入收據） | 改用 |
|---|---|---|
| CPU 的位址空間 | 既有核心的 `Bus`、服務層、cpu386 分段模式（434） | 線性層 |
| 硬體與 BIOS 側（不應被客體頁表改道，也不應觸發 accessed／dirty 寫回） | `bda.go` 的鍵盤緩衝與 BDA 存取、`bumpBDATicks`、`keyboard.go`／`keyhold.go` 讀 IVT、文字畫面讀出、`cga.go`、頁表走訪本身 | 實體層 |
| 載入與初始化（實模式，轉換關閉） | `initBDA`、`initVectors`、`loader.go`、`WriteMCB`、`SetExtendedMemoryKB`、`SetEMSPresent` | 不變，註明為實體語意 |
| 觀測 | `pollWords`、`Peek8`／`Peek16` | §6 |

讀寫監看、`noteWrite`、影片寫入紀錄比對的是實體位址。轉換關閉時與現行相同；轉換開啟時文件與工具說明改稱「實體位址」。

## 4. 服務層的位址計算

服務層現在以 `cpu.Addr(seg, off)` 算位址（`internal/dos` 約 46 處，另有約 26 處手算 `seg*16`），`Addr` 先遮 20 位元
（`internal/cpu/cpu.go` 約 235–237 行）。A20 開或分頁時，客體的 `FFFF:0010` 起是線性 `0x100000` 起，服務層卻會讀到 0 起。

新增 `Machine.SvcAddr(seg, off uint16) uint32`：轉換關閉時回 `cpu.Addr(seg, off)`（行為不變），轉換開啟時回
`seg*16 + off`（不遮罩，同既有核心的 `Linear`，`cpu.go` 約 253–255 行）。`internal/dos` 所有以段:偏移算位址再交給 Machine 的
地方改用它；`fixStackedCF` 也在內。

## 5. 分頁

- 有效分頁＝轉換開啟且 `CR0.PG=1`（DOSBox-X `cpu/cpu.cpp:2629-2632`：PE=0 時停用分頁）。
- 兩層 4 KB 頁表。頁目錄基底取 `CR3 & 0xFFFFF000`。PDE、PTE 以實體層讀取，位址同樣套 A20。
- 不支援 4 MB 頁、`PCD`／`PWT`（忽略）、`WP`（386 沒有）。

### 5.1 TLB

- Machine 持有，兩個核心與服務層共用。按需填入：某頁第一次經轉換時才走頁表（同 DOSBox-X 的惰性 TLB，
  `cpu/paging.cpp:779-905`、`990-1006`）。
- 作廢：寫 `CR3`、`CR0.PE` 或 `PG` 改變、`SetA20` 時整份作廢（`cpu/paging.cpp:1405-1414`）。以世代號實作：每筆記錄填入時的世代，
  世代不符視為無效，作廢只是世代加一，不清整個陣列。
- 結構：以線性頁號直接索引的平面陣列（不用 Go map）。V86 只需 `0x000`–`0x10F` 頁；cpu386 的監督者存取可到 4 GB，
  超出陣列範圍的頁走第二層的小型直接映射快取，同樣以世代作廢。
- 不存檔；`Restore`、`LoadState` 後作廢。

### 5.2 accessed 與 dirty

依 386 規則寫回：填 TLB 時把 PDE 與 PTE 的 accessed 位元設起；寫入存取時把 PTE 的 dirty 設起（DOSBox-X `cpu/paging.cpp:839-884`）。
TLB 記住該頁的 dirty 是否已寫回，已寫回就不再改頁表。寫回經實體層。

### 5.3 權限與錯誤

- CPL 來源：實模式 0；V86 3；保護模式由 434 提供，434 實作前以 0 計。
- 使用者存取（CPL=3）：PDE 與 PTE 的 U/S 取 AND（386 規則，`cpu/paging.cpp:708-719`），任一為 0 即錯誤；寫入時 R/W 取 AND，
  為 0 即錯誤。
- 監督者存取（CPL<3）：不檢查 U/S 與 R/W。
- 頁不存在（PDE 或 PTE 的 P=0）：錯誤。
- 本規格不遞送 `#PF`：Machine 記下第一筆錯誤（線性位址、PDE、PTE、存取型別、CPL），讀回 `0xFF`、寫入丟棄；`Machine.Step`
  在該步結束後回傳錯誤，失敗即關閉。`#PF` 遞送在後續規格 S6。

### 5.4 取指令快路徑

轉換開啟時 `CPU.Code` 一律停用；任何改變 `PE`、`PG`、A20、`PMProfile` 的寫入都呼叫 `syncCodeFastPath`。

## 6. 觀測

- `Peek8`／`Peek16`（觀測器入口）：轉換開啟時以客體線性視角轉換，但不寫 accessed／dirty、不填 TLB、不記錄錯誤（頁不存在時回
  `0xFF` 並由回傳值標示）。
- `pollWords`（每步呼叫 `Read16`）改用 `Peek16`，避免觀測觸發 accessed 寫回或錯誤。

## 7. 狀態檔

431 §4.2 的「只加欄位」commit 在本規格實作開頭進行：一次加完 432–435 需要的所有狀態欄位（指標或 slice），更新錨點測試的
gob 位元組常數並修改測試檔頭與 `internal/machine/state.go` 的政策註解（這是 430 之後的第二次）。之後 433–435 不再增改欄位。
本規格實際寫入的是 `PMProfile`、`CR0`、`CR3`（其餘欄位由 433–435 開始使用）。

## 8. 不在範圍

`#PF` 遞送、`invlpg`、4 MB 頁、`WP`、保護模式的 CPL 來源（434）、`SysRegs` 其餘欄位與 `-pm-profile` 旗標（433）。

## 9. 驗收

1. 轉換關閉不變：錨點測試的語意常數（記憶體雜湊、兩個 `StateDigest`）不變；430 全部測試不改照常通過；`internal/dos`
   改用 `SvcAddr` 後既有測試全綠。
2. 實體層：`ReadPhys*` 不再經過線性層（以「轉換開啟、頁表把 0 頁映到別處」時 `ReadPhys8(0)` 仍讀實體 0 證明）；頁表走訪不遞迴。
3. 線性層無分頁（`PE=1`、`PG=0`）：`0x300000` 在 A20 關時落在 `0x200000`；`0x110000` 不折回；`Ext` 範圍外讀 `0xFF`、寫入計數。
4. 分頁：以合成頁表重現 `strike_commander` `docs/re/003` §3 的映射（`B0000`–`B7FFF`、`C0000`–`FFFFF` 指向 `B8000`，HMA 指向
   `A0000`），讀寫到預期實體位址；accessed、dirty 寫回且 dirty 只寫一次；改 PTE 而未重載 `CR3`、該頁未存取過時讀到新值，已存取
   過時讀到舊值；寫 `CR3` 後讀到新值；A20 關閉時轉換後的實體位址遮 bit 20。
5. 權限：V86（CPL=3）寫 R/W=0 頁、讀 U/S=0 頁回報錯誤並使 `Step` 失敗；CPL=0 寫同一頁成功；P=0 回報錯誤含線性位址與 PTE。
6. TLB 世代：連續 10 萬次寫 `CR3` 不配置記憶體、不變慢（基準測試）；超出平面陣列範圍的監督者存取正確轉換。
7. 服務層：轉換開啟、A20 開時，服務層以 `SvcAddr(0xFFFF, 0x0010)` 讀寫的 byte 與既有核心以 `FFFF:0010` 讀到的相同；在映射狀態 1
   （合成頁表）下兩者也相同。
8. 觀測：`Peek8` 不改頁表 accessed 位元、不記錄錯誤；`pollWords` 不觸發錯誤。
9. 快照：轉換開啟、分頁中拍 `Snapshot`，`Restore` 到另一台同設定機器後 TLB 作廢、讀寫結果相同。
10. 效能（431 §7）：轉換關閉時固定合成程式不低於修改前 97%；新增 `BenchmarkStepRealMode`、`BenchmarkMemV86Paged`（後者以合成頁表
    與迴圈存取量測每秒存取數，作為 435 之後 V86 門檻的基準）。
11. `tools/go.sh test ./internal/... ./cmd/probe ./session ./oracle` 全綠（需顯示器的套件照現行以 `xvfb-run`）。

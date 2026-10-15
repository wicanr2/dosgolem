# 429 — DOS 系統檔案表（SFT）鏈

狀態：**READY**（三輪審查）
日期：2026-10-15
前置：[`012-mcb-chain`](012-mcb-chain.md)（list of lists 的 `[BX-2]`）

---

## 1. 觸發案例

《Strike Commander》光碟版啟動器 `SCCD.EXE`（9,232 bytes，SHA-256
`5dbfe18c528f51b9b1b0c989c1578cb297ba56c64038be7ca9d903dd88e93d01`）在映像
位移 `0x2028` 的函式：

```text
2028: enter 4,0
202C: mov ax,3001h / int 21h        ; DOS 版本 < 2 就回 0
2037: mov ah,52h / int 21h          ; ES:BX = list of lists
203B: add bx,4                      ; [BX+4] = 第一張 SFT 的 far pointer
2046: les bx,[bp-4] / add cx,es:[bx+4]   ; 累加每張表的檔案數
204D: les bx,[bp-4] / 取 [bx+0]、[bx+2] 為下一張表
205D: cmp word [bp-4],0FFFFh / jne 2046  ; offset 為 FFFFh 才停
```

呼叫端（映像 `0x1A7A`）比較回傳值 `cmp ax,19h / jae`，總數少於 25 就以錯誤碼
`0Eh` 結束，等於檢查 `CONFIG.SYS` 的 `FILES=`。

dosgolem 的 `AH=52h` 回 `0070:0010`，但 `0070:0014` 沒有寫入，SFT 指標是
`0000:0000`。走訪從中斷向量表開始，永遠碰不到 `FFFFh`。實測：dosgolem
`b360e4c` 的 `cmd/probe -exe SCCD.EXE` 在 `0294:0806`–`0822` 迴圈到
5,000 萬步上限（`strike_commander_cht_dosgolem` 的 `docs/re/001-m0-feasibility.md`）。

## 2. 規則

新增常數 `SFTSeg = 0x0060`（第一張表）與 `SFT2Seg = 0xF000`（第二張表）。
`MCBSeg`（同為 `0x0060`）目前只有宣告、沒有任何程式讀寫，本規格不沿用它的名稱，
免得讀者以為 SFT 是 MCB 鏈的一部分。

`Machine.SetDOSFiles(n)` 設定 SFT 的檔案總數，`DOSFiles()` 讀回。

- `n == 0` 是預設值，行為與本規格之前完全相同：不寫入任何位元組。
- `n` 在 6 到 255 之間時，`LoadEXE`、`LoadCOM` 在 `initMCB()` 之後呼叫 `initSFT()` 寫入下表。
  `initSFT` 一律直接寫 `m.Mem`，不經 `Write8`／`Write16`／`WriteBytes`：第二張表在 ROM 區，`Write8` 對
  `ROMBase` 以上的寫入會丟棄並計 `ROMWrites`（`internal/machine/machine.go` 的 `Write8`）。
- 其他值回錯誤。下界 6 沿用 DOSBox-X（`dos_classes.cpp:136`，`files=` 小於 6 即結束）；
  上界 255 時第二張表佔 `F0000`–`F39DE`（`6 + 251*59 = 14,815` bytes），不碰 8×8 字型（`F000:FA6E`，
  `199-rom-font-8x8`）與開機尾段（`FFFF0`，`191-bios-rom-tail`）。
- 判斷「已載入」用新增的未匯出旗標 `programLoaded`，由 `LoadEXE`、`LoadCOM`、`LoadState` 設為真；
  不用 `ImageLen`（舊狀態檔可能為 0）。`programLoaded` 為真時呼叫 `SetDOSFiles`（含 `n == 0`）一律回錯誤，
  不安靜失效。同一 Machine 不支援重載後改設 SFT。
- `initSFT` 顯式寫入整個表範圍（含清零的項目），不只寫非零位元組；`MemPoison` 非零時，
  表內未命名的位元組也必須是 0。
- `LoadState` 不讀寫 `DOSFiles`：狀態檔的記憶體裡有沒有鏈，由當初存檔的那次執行決定，
  `DOSFiles()` 在 `LoadState` 後回 0。

| 線性位址 | 內容 |
|---|---|
| `LOLSeg*16 + 0x14` | 第一張表的 far pointer：`0060:0000`（offset 在前） |
| `0060:0000` | 下一張表 far pointer：`F000:0000` |
| `0060:0004` | 第一張表的檔案數：`4` |
| `0060:0006` 起 | 4 筆 59-byte 項目，全部清零；第 0–2 筆位移 `0x20` 寫 `"CON "`，第 3 筆寫 `"AUX "` |
| `F000:0000` | `FFFF:FFFF`（鏈尾） |
| `F000:0004` | 第二張表的檔案數：`n - 4` |
| `F000:0006` 起 | `n - 4` 筆 59-byte 項目，全部清零 |

依據：DOSBox-X `src/dos/dos_classes.cpp` `DOS_InfoBlock::SetLocation` 第 129–168 行：
表頭 6 bytes（下一張 far pointer、檔案數），項目 59 bytes（`include/dos_inc.h`
`SftHeaderSize = 6`、`SftEntrySize = 59`）；第二張表的下一張指標寫 `0xFFFFFFFF`；
預設項目名稱寫在位移 `0x20`。

與 DOSBox-X 的差異與理由：

- DOSBox-X 的第一張表在 info block 段的 `+0xCC`，有 5 筆。dosgolem 的 `LOLSeg`
  （`0x0700`–`0x07FF`）之後緊接 `StubSeg`，放不下。改放 `0x0600`–`0x06F1`，放 4 筆共
  242 bytes，第 5 筆（`PRN`）不寫。
- DOSBox-X 的第二張表由 `DOS_GetMemory` 配在 DOS 私有記憶體。dosgolem 傳統記憶體
  的配置由 MCB 鏈決定，搶一塊會改變所有程式可用的記憶體。改放 BIOS ROM 區 `F000` 段：
  - 不放 `E000` 段：JEMM（`430` 的觸發案例）把 EMS 頁框設在 `E000h`，頁框映射後 SFT 會被 EMS 頁蓋掉。
    `C000`–`CFFF` 是 UMB、`D000` 是內建 EMS 頁框，也都不行。
  - `F0000` 起目前沒有內容（`initROM` 只寫 `FFFF0` 起，8×8 字型在 `FFA6E` 起）。
- 已知限制：
  - 程式對 SFT 的寫入（例如在鏈尾追加新表以擴充 FILES 的常駐程式改寫 `F000:0000` 的 next 指標）會被 ROM
    保護安靜丟棄，只在 `ROMWrites` 留痕。這是刻意取捨：本規格不模擬 SFT 項目內容。
  - 真 DOS 的 SFT 在傳統記憶體。JEMM 的遊戲映射狀態把 `C0000`–`FFFFF` 別名到 `B8000`（`430` 觸發案例的
    `003-jemm-api.md` §3），該狀態下本設計讀不到第二張表，原版讀得到。已知 SCCD 在 JEMM 啟動前走鏈；
    `STRIKE.EXE` 是否在映射狀態 1 走鏈未驗證。
- `191-bios-rom-tail` 寫 `F0000`–`FFFEF` 維持 0；本規格開啟時例外，在 191 加一行註記指向這裡。
- `docs/spec/008-ems-paging.md` §2 寫頁框段是 `0xE000`，與程式（`EMSFrameSeg = 0xD000`）
  不符，該段已過期；以程式為準。本規格在 008 加一行註記指向這裡。

### 為什麼預設關閉

既有收據記的是當時的記憶體內容。整機記憶體有多處被雜湊：`session/digest.go`、
Buck Rogers 收據的 `memory_sha256`、`cmd/state-compare` 的機器摘要。預設開啟會讓所有
程式在 `0x0600`、`0x0714`、`F0000` 多出位元組，舊收據重跑會得到不同雜湊，而畫面照常、
沒有測試會報錯。依 dosgolem `CLAUDE.md`「為了重跑既有收據而存在的機制」一節，
新行為以選項開啟。

## 3. 介面

- `machine.(*Machine).SetDOSFiles(n int) error`、`DOSFiles() int`。
- `cmd/probe` 加旗標 `-dos-files N`（預設 0），宣告放在其他旗標旁（`main.go` 約 215 行）。
  旗標檢查抽成可測的純函式 `validateDOSFiles(n int, loadState string) error`：與 `-load-state`
  同時給時回錯誤（狀態檔整份覆蓋記憶體）。`SetDOSFiles` 在 `if *loadState == ""` 區塊內、
  讀檔之後、`LoadEXE`／`LoadCOM` 之前呼叫（`main.go` 約 330–336 行）。
- 存檔狀態：SFT 位元組在記憶體內，隨既有記憶體快照保存；`DOSFiles` 只影響載入時的寫入，
  不另外存。
- EXEC 子程式：SFT 在共用記憶體，子程式看到同一條鏈；`initSFT` 不在 `LoadProgramAt`
  （EXEC 路徑）重寫。

## 4. 不在範圍

- SFT 項目的內容（開檔時填入 handle 狀態）。項目全部清零，只有預設名稱。
- `AH=52h` 的其他欄位；FCB SFT（`[BX+1Eh]`）。
- `oracle.Options`、`cmd/run`、`session`：之後有程式需要時另開規格接上。
- LE／DOS4GW 路徑（`LEMachine` 有獨立記憶體）。

## 5. 驗收

測試放置：走鏈要呼叫 `int 21h AH=52h`，`internal/machine` 不能引用 `internal/dos`（循環），
所以第 1、5 項放 `internal/dos`，用 `newTest` 與 `call(m, d, 0x21, 0x5200)`（`dos_test.go`）。
其餘放 `internal/machine`。`MemPoison` 是全域變數，相關測試以 `t.Cleanup` 還原，且不得用
`t.Parallel`。

1. 契約測試（EXE 與 COM 各一，`internal/dos`）：`SetDOSFiles(25)` 後載入最小程式，從 `AH=52h`
   回傳的 `ES:BX` 起，先斷言 `[BX+4]` 是 `0060:0000`，再走鏈：offset 為 `FFFFh` 即停，最多走
   16 張表（超過即失敗）。逐表核對表頭、項目數、`CON`／`AUX` 名稱位置，總數等於 25；載入後 `ROMWrites == 0`。
2. 預設不變，兩層錨點（與 `430` 驗收 1(b) 共用同一個測試夾具與前置 commit）：
   (a) `MemPoison` 為 0 與 `0xCC` 各跑一次：未呼叫 `SetDOSFiles` 時載入後，`0x0600`–`0x06FF`、
       `0x0714`–`0x0717`、`F0000`–`F3FFF` 與 `New()` 剛建好時逐位元組相同。
   (b) 先提交只含測試的前置 commit，以未修改的 loader 算出固定程式載入後整機記憶體的
       SHA-256，寫成測試常數（同型做法見 `le_machine_test.go`）；實作 commit 後，`n = 0`
       的雜湊必須等於該常數。
3. 差異範圍（`MemPoison = 0xCC` 下）：`n = 25` 與 `n = 0` 載入同一支程式，整機記憶體的差異
   位址集合恰好等於 `0x0600`–`0x06F1`、`0x0714`–`0x0717`、`F0000`–`F0000+6+21*59-1` 的聯集
   （每個位址都要不同，因為 poison 值不是 0 也不是表內容）。
4. 邊界：`SetDOSFiles(5)`、`(256)`、`(-1)` 回錯誤；`(6)` 第二張表 2 筆；`(255)` 第二張表
   251 筆，並斷言 `0xF0000 + 6 + 251*59 <= 0xFFA6E`（字型起點）；載入後呼叫（含 `n = 0`）回錯誤。
5. 常數守門（`internal/dos` 或 `internal/machine`）：`SFTSeg*16 + 6 + 4*59 <= LOLSeg*16`；
   `SFT2Seg*16 + 6 + 251*59` 小於 8×8 字型起點與開機尾段；`SFT2Seg` 不與 `umbStart`–`umbEnd`、`EMSFrameSeg` 及 `E000` 段重疊。
6. 存讀往返（`internal/machine`）：`n = 25` 載入後 `SaveState`，以新的 `New()` 機器 `LoadState`，
   三段記憶體相同，且 `DOSFiles()` 回 0、`SetDOSFiles` 回錯誤。`Snapshot`／`Restore` 一併保存 `programLoaded`：
   對新機器 `Restore` 之後 `SetDOSFiles` 回錯誤。
7. 負對照（walker 本身要能失敗）：在第 1 項的機器上，載入後把鏈尾改寫成 `0000:0000`、把第一張表
   檔案數改成 5，walker 各自報錯（前者撞到 16 張上限）。改寫一律直接改 `m.Mem`（`F000` 段經 `Write16` 會被
   ROM 保護吞掉，測試會變成假紅）。
8. `validateDOSFiles` 單元測試：`(30, "x.state")` 回錯誤；`(30, "")`、`(0, "x.state")` 不回錯誤。
9. 觸發案例（需要原版素材，缺檔 skip，不算進第 10 項的「全綠」，另附 probe 收據）：
   `cmd/probe -exe SCCD.EXE -dos-files 30` 不再於 `0294:0806` 迴圈，並走到
   `int 2Fh AX=4300h`（映像 `0x206C` 的 `mov ax,4300h`，`int 2Fh` 在 `0x206F`）或更後面。
10. `tools/go.sh test ./internal/machine ./internal/dos ./cmd/probe` 全綠。

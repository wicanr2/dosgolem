# 430 — 延伸實體記憶體與 XMS 統一

狀態：**DRAFT**（第 1 版）
日期：2026-10-15
前置：[`011-xms`](011-xms.md)、[`014-ems`](014-ems.md)、[`189-a20-gate-and-hma-addressing`](189-a20-gate-and-hma-addressing.md)、
[`429-dos-sft-chain`](429-dos-sft-chain.md)

---

## 1. 觸發案例與目標

《Strike Commander》光碟版的啟動器 `SCCD.EXE` 載入 Origin 的 V86 監控程式 JEMM。證據在
`strike_commander_cht_dosgolem` 的 `docs/re/002-sccd-launch-flow.md` 與 `004-cpu-requirements.md`：

| SCCD／JEMM 的需求 | dosgolem 現況 |
|---|---|
| `int 2Fh AX=4310h` 取得 XMS 入口後，把入口前 5 bytes 改寫成遠跳（標準 XMS 掛鉤協定），原 5 bytes 存起來供還原 | 入口是 `CD F5 CB`，不是可掛鉤形狀 |
| XMS `AH=09h` 配置約 5,000 KB、`AH=0Ch` Lock 取實體位址，位址須 ≤ `0xFE0000`（SCCD 映像 `0x37E`） | 區塊在 Go map，Lock 回合成位址 `0x01000000` 起 |
| JEMM 以平坦選擇子把 `0xA000` bytes 映像複製到該實體位址，之後在那裡建頁表、EMS 頁 | 1 MB 以上只有 64 KB HMA |
| SCCD 用 8042（`out 64h,D1h`、`out 60h,DFh`）開 A20 | 8042 寫入只記錄，不影響 A20 |
| 有 EMM 時 SCCD 改走 VCPI 路徑並失敗；SCCD 要看不到 `EMMXXXX0` | `int 67h` 一律指向內建 EMM |

本規格只處理記憶體與 DOS 服務層，CPU 的保護模式與分頁另立規格（S1–S5）。

## 2. 規則

### 2.1 延伸實體記憶體（選項，預設關閉）

`Machine.SetExtendedMemoryKB(kb int) error`，`kb` 為 0（預設）或 1,024 到 64,512 之間、1,024 的倍數。

- `kb == 0`：行為與本規格之前完全相同（`Mem` 1 MB、HMA 獨立 64 KB、XMS 區塊在 Go map）。
- `kb > 0`：新增 `Ext []byte`，大小 `kb*1024`，代表實體位址 `0x100000` 起的延伸記憶體。
  - HMA 改為 `Ext[0:HMASize]`，不再有獨立陣列；A20 開時 `段:偏移` 到 `0x100000`–`0x10FFEF` 讀寫 `Ext`。
  - 新增實體位址存取 `ReadPhys8／16／32`、`WritePhys8／16／32(a uint32)`：`a < 0x100000` 走現有 `Read8`／`Write8`
    （含 VGA、ROM、監看），`0x100000 ≤ a < 0x100000+len(Ext)` 讀寫 `Ext`，其餘讀回 `0xFF`、寫入丟棄並計數
    （`UnmappedPhysWrites`）。這是 S3 分頁匯流排與 cpu386 的唯一入口。
  - 實模式核心看到的位址仍以 A20 決定：A20 關時 `0x100000` 以上折回低記憶體（現有行為），開時只有 HMA 範圍可達。
  - 必須在載入程式前呼叫；`programLoaded`（`429-dos-sft-chain` §2）為真時回錯誤。

### 2.2 XMS 區塊實體化（`kb > 0` 時）

- 可配置範圍是 `Ext` 中 HMA 之後：實體 `0x110000` 起到 `Ext` 結尾，以 1 KB 為單位、首次適配（first fit）配置。
  `AH=08h` 回報的最大可用區塊與總可用量由此計算（不再是常數 8,192 KB）。
- `AH=0Ch` Lock 回傳區塊的真實實體位址；`AH=0Bh` Move 以實體位址複製（handle 0 仍代表 `段:偏移`）。
- `AH=0Fh` 重新配置大小：本規格只支援縮小或原地擴大，否則回錯誤碼 `A0h`（與現有錯誤碼表一致）。
- 區塊內容從 Go map 搬入 `Ext`；狀態存檔一併存 `Ext`（見 §3）。

### 2.3 可掛鉤的 XMS 入口（`kb > 0` 時）

依 XMS 3.0 規格的掛鉤協定，入口改為 `EB 03 90 90 90 CD F5 CB`（短跳過 3 個 NOP 到 trampoline）。
放在 `StubSeg:XMSTrapOff`，佔 8 bytes，不超過下一個 stub（`biosTimerOff = specialStubBase + 0x20`）。
掛鉤程式改寫前 5 bytes 後，trampoline 仍在 `+5`。`kb == 0` 時維持 `CD F5 CB`。

### 2.4 8042 A20（`kb > 0` 時）

依 8042 規格：`out 64h,D1h` 之後的下一個 `out 60h,v`，以 `v` 的位元 1 設定 A20（呼叫 `SetA20`）。
狀態埠 `64h` 的輸入緩衝區滿位元（bit 1）回 0，讓輪詢迴圈結束。其他 8042 命令維持只記錄。
`v` 的位元 0 為 0 時（要求重設）只記錄，不重設機器。
依據：DOSBox-X `src/hardware/keyboard.cpp`：第 825–827 行 `0xd1` 設 `CMD_SETOUTPORT`；第 666–678 行收到下一個
資料 byte 時 `MEM_A20_Enable((val & 2)>0)`，位元 0 為 0 時在預設設定下只記錄警告。

### 2.5 不安裝內建 EMM（選項）

`Machine.SetEMSPresent(false)`（預設 true）：`initVectors` 不寫 `EMSSeg` 的 driver header 與
`EMMXXXX0` 簽章，`int 67h` 指向一般 stub；`EMMXXXX0` 檔名開啟回「找不到檔案」。

## 3. 介面

- `machine`：`SetExtendedMemoryKB`、`ExtendedMemoryKB`、`ReadPhys*`、`WritePhys*`、`UnmappedPhysWrites`、`SetEMSPresent`。
- `cmd/probe`：`-ext-mem-kb N`、`-no-ems`，與 `-load-state` 同給時以錯誤結束（理由同 `429` §3）。
- 狀態檔：`Ext` 長度非零時加一段區塊（長度＋內容）；舊狀態檔沒有這段時 `Ext` 為空。狀態檔版本號加一，
  讀舊版號照舊。

## 4. 不在範圍

- 分頁、保護模式、cpu386 對 `Ext` 的存取路徑（S1–S3）。
- `int 2Fh AX=0803h`、SFT（另案）。
- UMB（`AH=10h`／`11h`）維持現有 `C000`–`CFFF` 行為。

## 5. 驗收

1. 預設不變：未呼叫兩個新選項時，既有測試全綠；固定程式載入後整機記憶體 SHA-256 等於前置 commit 釘住的值
   （同 `429` §5 第 2 項 b 的做法），XMS 入口仍是 `CD F5 CB`。
2. 延伸記憶體：`kb = 8192` 時 `ReadPhys8`／`WritePhys8` 在 `0x0FFFFF`、`0x100000`、`0x10FFEF`、`0x10FFF0`、
   `0x8FFFFF`、`0x900000`（越界）逐點驗；A20 開時實模式 `FFFF:0010` 與 `WritePhys8(0x100000)` 是同一 byte。
3. XMS：配置 5,000 KB 後 Lock 回傳位址 ≥ `0x110000` 且 ≤ `0xFE0000`；以 `WritePhys` 寫入、XMS Move 讀回一致；
   兩個區塊不重疊；釋放後可再配置；`AH=08h` 數值隨配置變化。
4. 掛鉤入口：以合成程式依 XMS 協定改寫入口前 5 bytes 為遠跳到自己的處理常式，再呼叫 XMS，處理常式先被執行，
   鏈回原入口（跳到 `+5`）後服務照常。
5. 8042：`out 64h,D1h; out 60h,DFh` 後 A20 開；`DDh` 後關；未接 `D1h` 的 `out 60h` 不影響 A20。
6. `SetEMSPresent(false)`：`int 67h` 向量的段 `:000A` 不是 `EMMXXXX0`；開 `EMMXXXX0` 失敗。
7. 狀態往返：`kb = 8192` 寫入 `Ext` 後存讀，`Ext` 逐位元組相同；讀舊版狀態檔不出錯。
8. 觸發案例（需原版，缺檔 skip，另附 probe 收據）：`SCCD.EXE` 搭配 `-dos-files 30 -ext-mem-kb 8192 -no-ems`
   通過 SFT、EMM、XMS 檢查與映像 `0x37E` 位址檢查，`call 0:12` 抵達 JEMM 入口（停在第一道保護模式指令前即可）。
9. `tools/go.sh test ./internal/machine ./internal/dos ./cmd/probe` 全綠。

# 001 — 範圍與 MVP

狀態：**READY**
日期：2026-09-04
依據：`~/cht/rich2/docs/spec/082-parity-oracle-emulator.md`（評估與量測）

---

## 1. 這個東西是什麼

**一個無頭、決定性、可以當 Go 套件 import 的 DOS 執行器**，
目的只有一個：讓《大富翁2》remake 的對拍從「隔著 X 看畫面」變成
「在同一個行程裡讀原版的記憶體」。

它**不是**通用模擬器，也不打算變成。判準是：

> 跑得動 `RUN_full.EXE`，而且跑出來的畫面與狀態與 DOSBox-X 一致。

其他 DOS 程式跑不跑得動不在範圍內；跑得動是副產物，不是目標。

## 2. 為什麼不用現成的

| 方案 | 為什麼不 |
|---|---|
| DOSBox-X | 943k 行 C++。**它不解決問題**——現在慢與不穩來自「只能從外面看畫面」，不是模擬器本身。它的除錯器是 ncurses 互動介面，不是可程式化的 API |
| 現成 Go x86 模擬器 | 只有玩具等級（`tiny_x86_emu`、`owlinux1000/x86emulator`），沒有 DOS 層 |
| unicorn／QEMU | `rich2` 走過（`docs/re/005`），能跑；但它是 C 函式庫、要 CGO、而且觀測面要自己再包一層 |

**DOSBox-X 留著當交叉 oracle 與時序參考實作**（`~/cht/dosbox`）。
本專案畫出來的每一張，都要拿它的索引截圖驗過才算數——
否則就是拿自己驗自己。

## 3. 三個讓它可行的量測（出處 `rich2/docs/spec/082` §2）

1. `RUN_full.EXE` 主程式區 52,892 bytes 只用到 **62 個助憶碼，全部 8086**。
2. **不需要 x87**：全檔 876 個 `INT 34h–3Dh`，浮點走 Microsoft 浮點模擬器，
   而那個模擬器**連結在 binary 裡面**，執行期只跑整數指令。
3. 系統服務面很窄，而且 `rich2/tools/dosemu.py` 已經走過一遍
   （跑到防拷畫面、圖形管線整條正確）。

## 4. MVP 的定義

**MVP ＝ 能在 `go test` 裡把原版跑到一個指定的畫面，並逐點比對。**

拆成兩個各自可驗收的切片：

### MVP-A（先做）：CPU 核心

- 8086 real mode 整數指令集，含字串指令、中斷、旗標。
- **驗收：SingleStepTests/8088 v2 全綠**（323 個 opcode 檔、每檔 10,000 筆）。
- 不含：x87、386 以上、保護模式、分頁、匯流排週期精確。

> 為什麼先做這個：它是**唯一一個可以在沒有原版素材、沒有 DOS 層、
> 沒有畫面的情況下獨立驗收到底的部分**。做完就知道 CPU 對不對，
> 不必等整條鏈接起來才發現。

### MVP-B：跑到防拷畫面

- MZ 載入器、PSP、記憶體控制區塊。
- `int 21h` 子集、`int 10h` 顯示卡偵測與 mode 13h、BIOS 資料區、PIT。
- **驗收：`RUN_full.EXE` 停在防拷密碼畫面，`0xA0000` 的 320×200 色號陣列
  與 DOSBox-X 的 Ctrl+F5 索引截圖逐點相同。**

MVP-B 過了就證明整條路走得通；沒過就表示估錯了，而那時的沉沒成本
只有 CPU 核心——它有 SingleStepTests 保證，可以留著。

## 5. MVP 之後（不在 MVP 範圍，先寫下來免得漂）

| | 內容 |
|---|---|
| M2 | 輸入與時序：鍵盤、`int 33h` 滑鼠、`int 08h`／`1Ch`；過防拷、進主選單、進棋盤 |
| M3 | 儀器層：breakpoint／watchpoint／call trace／RND 記錄／savestate |
| M4 | Go API 與 `parity` 套件；把 `rich2` 現有 54 支 DOSBox 腳本收斂成宣告式對拍表 |
| M5 | 迴歸：重跑 `rich2/docs/playtest/` 既有的 parity 收據 |

## 6. 硬規則

1. **SDD**：反組譯／量測 → 規格（`docs/spec/`，標 `DRAFT`／`READY`）→ 才實作。
   **只有 READY 的規格可以動手。**
2. **不得散布原版素材。** 本儲存庫不含 `RUN.EXE`、`.PIX`、`.PAK` 或任何原版檔案；
   測試靠玩家自備，缺檔就 skip，不用自製代用品。
3. **建置與測試一律走 docker**（`tools/go.sh`），不裝到系統環境。
4. **測試語料不進版控**：SingleStepTests 761 MB，用 `tools/fetch_cputests.sh`
   抓到 gitignore 的 `testdata/`。
5. **推論標籤要誠實**：confirmed／強證據／假說／未知。

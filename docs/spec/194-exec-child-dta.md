# 194 — EXEC：子行程的 DTA 指到它自己的 PSP:0080h

狀態：**READY**（行為照 DOSBox-X 原始碼；觸發案例有寫入監看的直接證據，見 §2）
日期：2026-09-11
前置：[`009-exec`](009-exec.md)、[`027-dos-dta-find-first`](027-dos-dta-find-first.md)

---

## 1. 規則

`AH=4Bh AL=00h` 載入並執行子程式時，**目前的 DTA（Disk Transfer Area）改成
子行程 PSP 的 `0080h`**。子行程結束（`AH=4Ch`／`int 20h`）時**不還原** DTA。
`AL=03h`（載入 overlay）不動 DTA。監督佇列推出來的下一支程式，比照 EXEC 設成
它自己的 `PSP:0080h`。

依據：DOSBox-X `src/dos/dos_execute.cpp`

- `DOS_Execute` 在 `LOAD`／`LOADNGO` 分支切換 PSP 之後立刻
  `dos.dta(RealMake(newpsp.GetSegment(),0x80))`（第 870–873 行附近）；
- `DOS_Terminate`（第 111 行起）還原 PSP、SS:SP、暫存器、`22h`–`24h` 向量，
  **沒有**碰 DTA。

## 2. 為什麼要有這條：子程式的 FindFirst 會寫進父程式的記憶體

DTA 是 `AH=4Eh`／`4Fh`（FindFirst／FindNext）寫結果的地方，一筆 43 bytes。
父程式常把 DTA 設在自己的區域變數上（在堆疊裡）。EXEC 不換 DTA 的話，
子程式的第一個 FindFirst 就把 43 bytes 寫進父程式堆疊。

實例（Borland C++ 2.0，`BCC -ms HELLO.C`，BCC 以 `spawn` 叫 `TLINK`）：

1. BCC 在 EXEC 前設的 DTA 在自己的堆疊 `0EFF:9D4x` 附近。
2. TLINK 找 `tlink.cfg`（`AH=4Eh`），DOS 把搜尋結果寫進那塊——
   `cmd/run -watch 18D4A-18D57` 看到第 1,093,611 步由 `1E2A:0342`
   （TLINK 的 `int 21h` 下一道）一次改寫 12 個位元組，全部落在 BCC 的 `spawn` 堆疊框上。
3. TLINK 正常結束、`hello.exe` 正確產出；BCC 回來後 `retf 0Ah` 從被蓋掉的框彈出
   `9400:1697`，跳進一片 0，最後在一段走訪串列的迴圈裡空轉到指令上限。

單獨執行 TLINK（沒有父行程）完全正常，產出的 `hello.exe` 與 BCC 驅動那次逐位元組相同——
問題只在「子行程沿用父行程的 DTA」。

## 3. 驗收

1. 契約測試：父程式把 DTA 設在自己的緩衝區，EXEC 一支會做 `AH=4Eh` 的子程式；
   子程式執行期間 DTA 是 `子PSP:0080h`，父程式緩衝區的內容**一個位元組都沒變**。
   反面對照：同一支測試在修改前要失敗。
2. `tools/go.sh test ./internal/dos` 全綠。
3. `BCC -ms HELLO.C` 在 `cmd/run -cpu 8086` 下跑完、回傳碼 0，產出可執行的 `hello.exe`。

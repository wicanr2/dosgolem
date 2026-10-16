# 437 — 分段模式的 32 位元遠轉移（`66` 前綴）

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。JEMM 位址為 img 位移；`X` 為 XMS 區塊線性位址（本案 `110000h`）。

---

## 1. 為什麼要做

435 的觸發案例停在 `0018:5DA0` 的 `66 36 FF 2E 4C 3F`（`jmp m16:32 ss:[3F4C]`），434 §6 把分段模式的 `66` 遠轉移列為失敗即關閉。這是 JEMM `int15`
`SP` 子功能（`BX=5350h`，img `5CDF`–`5DE2`）：在 ring 0 執行呼叫者碼段內偏移 `EDX` 的常式（`docs/re/003` 第 42 行）。

| 項目 | 內容 | 等級 |
|---|---|---|
| 呼叫者 | `MENU.EXE` 檔案位移 `22EE3`：`xor cx,cx`、`EDX=9Eh`，常式為搖桿讀取（port `201h` 計時迴圈，結尾 16 位元 `retf`，檔案 `22F23`）。`STRIKE.EXE` 另有調色盤上傳（`3211B`）與搖桿讀取（`6DAFB`）兩處，同樣 `CX=0` | 已證實（靜態） |
| GDT 改寫 | `128h` 基底＝V86 的 `CS×16`，`118h`＝`DS×16`，`120h`＝`ES×16`；`CX=0` 時清 `128h` 的 D 與 `110h` 的 B（`5CE4`–`5CFA`） | 已證實（靜態） |
| 第 1971307 步的 GDT 副本 | `110h`：基底 `X`、界限 0、`96h`、B=0；`118h`：基底 `25030h`、界限 `3Fh`、`96h`、B=1；`120h`：基底 `FFFF0h`、界限 `FFFFFh`、`92h`、G=1、B=1；`128h`：基底 `221E0h`、界限 `FFFFFh`、`9Ah`、D=0。`[3F4C]`＝偏移 `0000009Eh`、選擇子 `0128h` | 已證實（probe 傾印） |
| 跳躍前的堆疊 | `SS=110h`（B=0），推入 `18h`、`5DA6h`（16 位元幀）；常式以 16 位元 `retf` 回 `18h:5DA6`（`SP_return_point`），之後 `5DE6` 的 `popad`、`iretd` 回 V86 | 已證實（靜態） |
| `CX≠0` 的分支 | 推入 32 位元幀 `[00005DA6][00000018]`，目標段 D=1；三個已知呼叫者都不走 | 已證實（靜態） |

所以本案只需要：分段模式下 `66` 前綴的遠轉移以 16:32 遠指標執行，目標仍是 D=0 碼段。D=1 碼段（JEMM 的 `98h`，`int15` 的 `CU`、`XC`）屬下一份規格。

## 2. 範圍

分段模式（`segmented=true`）下，`66` 前綴改變下列指令的遠指標與堆疊幀寬度（照 DOSBox-X `CPU_JMP`／`CPU_CALL`／`CPU_RET` 的 `use32` 參數，
`cpu/cpu.cpp:1785-1835` 起；`core_normal/prefix_66.h` 的 `EA`、`9A`、`CA`、`CB`、`FF /3`、`FF /5`）：

| 指令 | 行為 |
|---|---|
| `66 EA ptr16:32` | 讀 4 bytes 偏移、2 bytes 選擇子，遠跳 |
| `66 FF /5 m16:32` | 從記憶體讀 4 bytes 偏移、2 bytes 選擇子（段前綴適用），遠跳 |
| `66 9A ptr16:32`、`66 FF /3 m16:32` | 遠呼叫：推入 32 位元 `CS`（高 16 位元 0）與 32 位元 `EIP`（下一道），各 4 bytes |
| `66 CB`、`66 CA iw` | 遠返回：彈出 32 位元 `EIP`、32 位元 `CS`（取低 16 位元），`CA` 另加 `iw` |

- 段載入一律經 434 §5.1 的 `loadSeg`（`CS` 規則：非一致碼段、RPL ≤ CPL、DPL＝CPL、present、D=0；遠返回另驗 RPL＝CPL）。
- 目標偏移高 16 位元非 0：失敗即關閉（D=0 碼段的 `IP` 只有 16 位元；真機會在取指令時因超過界限而 `#GP`，DOSBox-X 以 `reg_eip` 存 32 位元。刻意從嚴）。
- 堆疊：`SS` 的 B=0 時推入與彈出以 `SP` 計算，16 位元環繞（434 §4.1）；B=1 照 434 失敗即關閉。
- 失敗時 `CS:IP`、`SP` 與堆疊都不改（先讀完、驗證完再寫）。
- 實模式與 V86 的 `66` 遠轉移維持現狀（`ops386.go` 的 `execute32` 現行行為）。

## 3. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 無改動；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變。
2. §2 表每列至少一例：`66 FF /5` 帶 `36` 段前綴讀遠指標；`66 9A` 與 `66 FF /3` 推入 8 bytes 且內容正確；`66 CB` 與 `66 CA 04 00` 彈出 8 bytes（加 4）。
3. 失敗例：目標偏移高半非 0、目標為 D=1 碼段、目標為資料段、RPL＞CPL、遠返回 RPL≠CPL、`SS` 為 B=1；失敗時狀態不變。
4. 混合：`66 FF /5` 跳到 D=0 段後，該段以 16 位元 `retf`（`CB`）返回由 16 位元 `push` 組成的幀（仿 JEMM `5D9B`–`5DA0` 與 MENU 常式）。
5. 突變自檢：`66 FF /5` 只讀 2 bytes 偏移、`66 9A` 推入 4 bytes、`66 CB` 彈出 4 bytes、略過偏移高半檢查，各至少一個測試失敗。
6. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄。
7. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
   - `0018:5DA0` 之後 `CS=0128h`、`IP=009Eh`、`CS` 快取基底 `221E0h`；
   - MENU 的常式以 `retf` 回 `0018:5DA6`，`SP_return_point` 之後經 `5DE6` 回 V86；
   - 繼續執行，照實記錄最後停點、原因、步數、`EXEC` 紀錄與視訊模式變化。
8. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

# 437 — 分段模式的 32 位元遠轉移（`66` 前綴）

狀態：**READY**（第 2 版，依審查補實作順序與邊界）
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
| 第 1971307 步（停點前兩步）的 GDT 副本 | `110h`：基底 `X`、界限 0、`96h`、B=0；`118h`：基底 `25030h`、界限 `3Fh`、`96h`、B=1；`120h`：基底 `FFFF0h`、界限 `FFFFFh`、`92h`、G=1、B=1；`128h`：基底 `221E0h`、界限 `FFFFFh`、`9Ah`、D=0。`[3F4C]`＝偏移 `0000009Eh`、選擇子 `0128h` | 已證實（probe 傾印） |
| 跳躍前的堆疊 | `SS=110h`（B=0），推入 `18h`、`5DA6h`（16 位元幀）；常式以 16 位元 `retf` 回 `18h:5DA6`（`SP_return_point`），之後 `5DE6` 的 `popad`、`iretd` 回 V86 | 已證實（靜態） |
| `CX≠0` 的分支 | 推入 32 位元幀 `[00005DA6][00000018]`，目標段 D=1；三個已知呼叫者都不走 | 已證實（靜態） |

所以本案只需要：分段模式下 `66` 前綴的遠轉移以 16:32 遠指標執行。D=1 碼段（JEMM 的 `98h`，`int15` 的 `CU`、`XC`）由 443 處理，本規格的遠轉移可以進出 D=1 碼段（由 443 修訂）。

## 2. 範圍

分段模式（`segmented=true`）下，`66` 前綴改變下列指令的遠指標與堆疊幀寬度（照 DOSBox-X `CPU_JMP`（`cpu/cpu.cpp:1785-1835`）、`CPU_CALL`（`1920-2005`）、
`CPU_RET`（`2210-2315`）的 `use32` 參數；`core_normal/prefix_66.h` 的 `9A`＝415、`CA`＝552、`CB`＝559、`EA`＝638、`FF /3`＝730、`FF /5`＝750 行）：

| 指令 | 行為 |
|---|---|
| `66 EA ptr16:32` | 讀 4 bytes 偏移、2 bytes 選擇子，遠跳 |
| `66 FF /5 m16:32` | 從記憶體讀 4 bytes 偏移、2 bytes 選擇子（段前綴適用），遠跳 |
| `66 9A ptr16:32`、`66 FF /3 m16:32` | 遠呼叫：先推 32 位元 `CS`（高 16 位元 0）再推 32 位元 `EIP`（下一道），完成後 `[SP]=EIP`、`[SP+4]=CS` |
| `66 CB`、`66 CA iw` | 遠返回：彈出 32 位元 `EIP`、32 位元 `CS`（取低 16 位元，高半不檢查），`SP += 8 + iw` |

`66 FF /3`、`/5` 的 `mod=3`（暫存器運算元）失敗即關閉（DOSBox-X `prefix_66.h:732`、`752` 為非法指令）。帶 `67` 時照 434 §4.2 的 32 位元有效位址，
讀指標的 `+4` 不截斷。`66 EA`／`66 9A` 的指標是立即值，`67` 無影響。

範圍外、維持失敗即關閉：`66 FF /2`、`/4`、`66 E8`、`66 E9`、`66 C2`、`66 C3`（仍在 `sizeSensitive`，落到 `execute32` 的 default）。`ops386.go` 中
「32 位元的 call／jmp 未實作」的訊息在 `/3`、`/5` 實作後改為只描述 `/2`、`/4`。

- 段載入一律經 434 §5.1 的 `loadSeg`（`CS` 規則：非一致碼段、RPL ≤ CPL、DPL＝CPL、present；遠返回另驗 RPL＝CPL）。目標可以是 D=1 碼段（由 443 修訂）。
- 目標偏移高 16 位元非 0：失敗即關閉（D=0 碼段的 `IP` 只有 16 位元；真機會在取指令時因超過界限而 `#GP`，DOSBox-X 以 `reg_eip` 存 32 位元。刻意從嚴）。
- 堆疊：`SS` 的 B=0 時推入與彈出以 `SP` 計算，16 位元環繞（434 §4.1）；B=1 照 434 失敗即關閉（443 維持）。遠返回的框架寬度看運算元大小，不看目標段的 D：D=1 碼段內不帶 `66` 的 `CB`／`CA` 也走本節的 32 位元形式（443 §2.6）。
- 失敗時 `Seg[CS]`、`cache[CS]`、`SP` 與堆疊位元組都不改；`IP` 照既有失敗路徑（已越過本道指令），錯誤回報 `opCS:opIP`。現有 `farCall`／`farRet`／`push32`／`pop32`
  做不到這點（先提交 CS 或先改 SP），另寫 32 位元版本，順序固定為：
  - 遠跳：讀完指標、`c.fault` 非 nil 就返回、驗偏移高半、`loadSeg(CS)`、設 `IP`。
  - 遠呼叫：讀完指標並看 fault、驗偏移高半、堆疊預檢（`stackCheck`，並對 `SP-8`–`SP-1` 共 8 個 byte 以 16 位元環繞的偏移做寫入的 `segCheck`）、`loadSeg(CS)`、推入、設 `IP`。`loadSeg` 成功後不得再有可失敗的步驟。
  - 遠返回：`stackCheck`、以 `read32` 讀 `SP` 與 `SP+4` 的兩個 dword 而不動 `SP`、看 fault、驗偏移高半、`retRPLOK`、`loadSeg(CS)`、`SP += 8 + iw`、設 `IP`。
- 已知差異：DOSBox-X 的 `CPU_Pop32` 對跨 `FFFFh` 的 dword 不逐 byte 環繞（`cpu.cpp:683-702`），本規格沿用 434 §4.1 的逐 byte 環繞；JEMM 路徑不會碰到。
- 實模式與 V86 的 `66` 遠轉移維持現狀（`ops386.go` 的 `execute32` 現行行為）。434 §6 表中「`66` 形式的遠轉移與 `66 CB`」一列由本規格取代。

## 3. 驗收

1. profile 關閉時：錨點測試常數與 gob 位元組不變；`internal/cpu386` 無改動；SingleStepTests 315 PASS、0 SKIP，`knownGaps` 不變。
2. §2 表每列至少一例：`66 FF /5` 帶 `36` 段前綴讀遠指標；`66 67 FF /5` 以 `[ebx+disp]` 讀遠指標；`66 9A` 與 `66 FF /3` 推入 8 bytes 且 `[SP]=EIP`、`[SP+4]=CS`；
   `66 CB` 使 `SP += 8`、`66 CA 04 00` 使 `SP += 12`；遠返回彈出的 `CS` 高半非 0 時仍成功。
3. 失敗例：目標偏移高半非 0、目標為資料段、RPL＞CPL、遠返回 RPL≠CPL、`SS` 為 B=1、`66 FF /3`／`/5` 的 `mod=3`、遠呼叫的推入位置超出 `SS` 界限；
   失敗時 `Seg[CS]`、`cache[CS]`、`SP` 與堆疊位元組不變，錯誤回報 `opCS:opIP`。目標為 D=1 碼段改為成功例，驗收在 443 §3（由 443 修訂；
   `internal/machine/seginsn437_test.go` 的三列 D=1 失敗例隨 443 實作改寫）。
4. 混合：`66 FF /5` 跳到 D=0 段後，該段以 16 位元 `retf`（`CB`）返回由 16 位元 `push` 組成的幀（仿 JEMM `5D9B`–`5DA0` 與 MENU 常式）。
5. 突變自檢：`66 FF /5` 只讀 2 bytes 偏移、`66 9A` 推入 4 bytes、`66 CB` 彈出 4 bytes、略過偏移高半檢查、遠返回檢查彈出 `CS` 的高半、拿掉堆疊預檢、
   讀指標後不看 `c.fault`，各至少一個測試失敗。
6. 效能：profile 關閉的 `BenchmarkStepRealMode` 與本規格動工前的 commit 交錯比較（431 §7），照實記錄。
7. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile`：
   - `0018:5DA0` 之後 `CS=0128h`、`IP=009Eh`、`CS` 快取基底 `221E0h`；
   - MENU 的常式以 `retf` 回 `0018:5DA6`，`SP_return_point` 之後經 `5DE6` 回 V86；
   - 埠 `201h` 在 dosgolem 沒有模型，讀回 `FFh`，搖桿四軸逾時、無按鍵；MENU 如何解讀搖桿結果另行記錄，不算本規格的錯誤；
   - 繼續執行，照實記錄最後停點、原因、步數、`EXEC` 紀錄與視訊模式變化。
8. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

## 4. 實作收據（2026-10-16）

commit：`0cd8e55`。

- 第 1–6、8 項：容器內 `gofmt`、`go build`、`go vet` 通過；`go test ./internal/... ./cmd/probe ./session ./oracle` 13 個套件 ok；SingleStepTests 315 PASS、0 SKIP，
  `knownGaps` 不變；`go test ./apps/...` 與改動前相同（唯一 FAIL 為既有的 GLFW）；`internal/cpu386`、`internal/state` 無改動。突變自檢全被抓到
  （「只拿掉 `farCall32` 入口的 fault 檢查」為等價突變：之後的檢查在任何狀態改變前攔下同一錯誤）。效能：profile 關閉，對 `491efbb` 交錯 5 輪，
  load average 約 10，最快值 new 38.03、old 42.71 ns/op。
- 第 7 項觸發案例：第 1971309 步進入 `0128:009E`（MENU 的搖桿讀取常式），第 2200751 步以 `retf` 回 `0018:5DA6`，之後回 V86 繼續。共執行 2333937 道後停止：
  `CS:IP=B002:2937`「跑出可用記憶體」（線性 `B2957h`）。停點前 MENU 段 `158Ch` 的 `0446` 呼叫 `0x3C8` 取得遠指標 `DX:AX=B002:2937`，`0453`–`0455`
  以 `push dx; push ax; retf` 跳過去。這段常式（`158C:0415` 起）使用 `int 15h AX=1209h` 的 `SM`（`BX=534Dh`）與 `sm`（`BX=736Dh`）。遠指標的來源與正確值未知，
  需另行查證（DOSBox-X 交叉對照）。`EXEC` 紀錄仍為 `JEMM.OVL`、`MENU.EXE`。
- 處置：實模式與 V86 的 `66 FF /3`、`/5` 訊息改為「32 位元遠轉移只在分段模式實作」；16 位元 `CA iw` 在分段模式失敗時仍會改 `SP`（434 的既有行為，不在本規格範圍）。

# 197 — `int 27h`（舊式常駐結束）

狀態：**CONFORMED**（三輪審查後 READY；實作與同狀態驗證收據見 §9）
日期：2026-10-03
前置：[`008-tsr-resident.md`](008-tsr-resident.md) §3（`AH=31h`）、[`009-exec.md`](009-exec.md) §3（`AH=4Dh`）與 §4（監督佇列）、[`006-layering.md`](006-layering.md) §2

---

## 1. 為什麼需要

一款 DOS 遊戲的啟動腳本依序執行三支很小的 `.COM`，每一支做完事都以 `int 27h` 常駐，
後面的主程式再用 `AH=35h` 讀它們留在中斷向量表裡的位址。`int 27h` 是舊式的 TSR 呼叫，
與 `AH=31h` 並存。現行服務層沒有這個分支，落進 `handle` 的 `default`，只記一筆「未實作」並清 CF 返回。

沒有它的後果，逐條標等級（證據見 §5）：

1. 第一支單跑時沒有常駐：`int 27h` 返回後程式繼續執行自己尾巴之後的零位元組，繞過 64 KB 後回到進入點，
   走到「已安裝」的分支才用 `int 20h` 結束（已證實，`-log-calls` 與指令數的獨立重算）。
2. 串跑時，第二、三支也沒有常駐。它們載入到與第一支相同的 PSP，只覆寫自己的位元組，`int 27h` 返回後執行的是
   殘留的第一支映像尾端，把向量佔位值寫回並再次 `int 27h`（已證實，向量表寫入監看）。
3. 後面的程式落在原本要保留的區塊上。主程式把資料檔讀進向量所指的緩衝區，目的位址與主程式自己的 PSP 重合，
   覆蓋它的 PSP 與映像開頭，之後執行流進入垃圾位址（觀察已證實；因果為強推論，補上 `int 27h` 之前沒有反事實可驗）。
4. 啟動序列因此無法等價重現，其後的 overlay 位址、視訊寫入與畫面都量不到（已證實，與收據的「未量到」一致）。

判準（[`006-layering`](006-layering.md) §2）：「換一支 binary 之後這段程式碼還成立嗎？」`int 27h` 是 DOS 本身的服務，成立，收在 `internal/dos`。

## 2. 語意

輸入：`DX` ＝ 從 PSP 起要保留的**位元組數**（含 PSP 那 256 bytes）。`AL`、`AH` 不使用。

行為：等同 `AH=31h`、`AL=00h`，`DX` 換成段數 `K`：

```text
K = (DX + 15) / 16        ; 向上取整，以 32 位元運算，DX=0FFFFh 時 K=1000h
terminate(離開碼 0, TSR, 保留 K 段)
```

- 離開碼固定 0，`lastExit` 被設為 0。`AH=4Dh` 取回 `AX=0000h`（`AL` 的呼叫端殘值不被採用）。
- 常駐區的記錄、`freeSeg` 的推進、行程疊的彈回、監督佇列的接續、子行程已開的檔不關、
  向量還原（`savedVectors`）、`ExecLog` 的 `TSR`／`Keep`，**全部由既有的 `terminate()` 決定，與 `AH=31h` 相同**，本規格不重述也不改。
- 控制權不會回到 `int 27h` 的下一道指令：行程疊不空就回父行程（並清 CF，`terminate()` 彈回分支的行為），
  空了就照監督佇列接下一支或停機。
- 使用「目前行程」的 PSP（`d.curPSP`），**不檢查 `CS` 等於 PSP**。這與 DOSBox-X 一致。
- 不加 `fixStackedCF`：直接的 `CD 27` 由 `doInt` 攔截，CPU 沒有先推旗標框；經由 stub 鏈接進來時，
  `terminate()` 整個換掉 CPU 上下文，堆疊上的框用不到。父行程若經 trampoline EXEC，子行程以 `int 27h` 結束後回到
  stub 的 `IRET`，堆疊框的 CF 不會被修，這是 `AH=4Ch`、`int 20h` 共有的既有限制，本規格不處理。

## 3. 與 DOSBox-X 的對照

來源：DOSBox-X 的 `src/dos/dos.cpp`，函式 `DOS_27Handler`（`AH=31h` 在同檔 `case 0x31`）。核對所用的原始碼樹是 `github.com/pmanyeh/dosbox-x`（DOSBox-X 的 fork）的 commit `e0b4287`，函式本體未與上游逐行比對。以下是改寫，不是逐字引用：
`DX` 除以 16 向上取整為段數；以目前 PSP 把該 PSP 的記憶體區塊縮成這個段數；縮成功才以 TSR 方式結束，離開碼 0；縮失敗則直接返回呼叫端。

| 項目 | DOSBox-X | 本規格 |
|---|---|---|
| 段數 | `ceil(DX/16)` | 相同 |
| PSP | 目前行程的 PSP | `d.curPSP` |
| 離開碼 | 0 | 0 |
| `AH=4Dh` 回傳的 `AH`（結束型態） | TSR（3） | 0。dosgolem 的 `getExitCode` 一律 `AH=0`（`009-exec` §3，有程式比對整個 `AX`），與 `AH=31h` 相同。已知差異 |
| 區塊縮減失敗 | 不結束，返回呼叫端 | **不同**：`terminate()` 不驗區塊能否切，`int 27h` 沿用 |
| 父行程是自己（疊底行程） | `DOS_Terminate` 在 `dos_execute.cpp` 開頭的檢查直接返回 | **不同**：dosgolem 的疊底行程交給監督佇列接手，刻意走另一條（`009-exec` §4） |

## 4. 範圍外

- 不改 `AH=31h`、`terminate()` 與 bump 配置器的任何行為。
- 不做 MCB 鏈走查（`008-tsr-resident` §5）。`terminate()` 只改 `freeSeg`，不改 `d.arena`，也不呼叫 `syncMCB`；
  spawn 時寫的 MCB 大小是 `EndSeg − freeSeg`，TSR 後不改成 `K`。`int 27h` 前 `arena` 若已建立（有 `AH=48h`／`49h`／`4Ah`），
  `freeSeg` 的推進不會反映在 arena 的自由區塊，`AH=31h` 有同樣行為。
- 不檢查 `CS`、不驗 `DX` 是否超過區塊、不夾到 `MemTop`（見 §8）。

## 5. 量測證據（基底 `2f44a68`，`cmd/probe`，2026-10-03）

證據原始輸出保存在私有研究紀錄，不在本 repo。

| 觀察 | 結果 | 等級 |
|---|---|---|
| 三支 `.COM` 的結尾 | 皆為 `mov dx,imm16; int 27h`，`DX` 依序為 `8000h`、`84D8h`、`84D8h`。三支的指令全部手工解讀過，只含 `int 20h` 與 `int 27h`，沒有 `int 21h` | 位元組已證實；第一支的 `DX` 有軌跡佐證，後兩支的 `DX` 僅由位元組推得 |
| 對應的 `K` | `8000h` → `0800h`；`84D8h` → `084Eh`（`84D8h / 16 = 84Dh` 餘 `8`，進位） | 算術 |
| 現況：未實作 | `-log-calls` 的「沒實作的服務」只列出 `int 27h` | 已證實（軌跡） |
| 第一支單跑 | 共 32,754 道指令（含 `int 27h` 之前的 17 道）。`int 27h` 返回後，執行自己映像結尾之後的零位元組（`add [bx+si],al`）繞過 64 KB、穿越 PSP，回到 `0100:0100`，第二輪向量比對命中，以 `int 20h` 結束。指令數可由位元組與載入器獨立重算 | 已證實 |
| 串跑：第二、三支 | 載入到與第一支相同的 PSP，只覆寫自己的位元組；`int 27h` 返回後執行殘留的第一支映像尾端，把佔位值寫回向量表並再次 `int 27h`。第三支重複 6 輪（共 12 次）。`-watch` 向量表寫入的 CS:IP 與步數可驗 | 已證實 |
| 串跑：`EXEC 紀錄` | 兩支佇列 `.COM` 為 `TSR=false`、`keep=0000`。疊底那支（`-exe` 指定的第一支）由載入器載入，不在 `EXEC 紀錄` 內 | 已證實 |
| 串跑：資料檔讀入的目的位址 | 主程式把一個資料檔讀進向量所指緩衝區，目的段是第一支的 PSP，與串跑中主程式自己的 PSP 重合（兩者都是 `0100`） | 觀察已證實；覆蓋後執行流進入垃圾位址的因果為強推論 |

### 5.1 bump 配置器下的預期段值（預期，待 §7.3 量測）

出處：`internal/dos/exec.go` 的 `terminate()`（TSR 分支 `d.freeSeg = max(d.freeSeg, d.curPSP + keep)`，行 305 至 311、327 至 330）。
這與 `008-tsr-resident` §2.1「TSR 例外：只收回 `PSP + DX` 之後的部分」的字面不同（見 §8），本表依程式行為。
頂層 `.COM` 名義上擁有整個 64 KB 段（`LoadCOM`：`FreeSeg = PSPSeg + 0x1000`），
佇列載入的 `.COM`，`enterProgram` 把 `freeSeg` 設成它的 `EndSeg`（`loadSeg + ceil(len/16) + 1`，`loadSeg = PSP + 0x10`）。
因此保留量小於這個值時**不縮回**，這是 bump 模型的既有限制，不是本規格引入。

前提：三支 `.COM` 在 `int 27h` 之前不呼叫 `AH=48h`／`49h`／`4Ah`（§5 第一列：它們不呼叫 `int 21h`）；後兩支的映像各佔 3 個段（`ceil(len/16) = 3`）。
若某支呼叫 `AH=4Ah` 且區塊等於 `curPSP`，`setPSPBlock` 會改 `freeSeg`，下表不成立。

以頂層 `.COM`（PSP `0100h`）加兩支佇列 `.COM`（映像各佔 3 個段）、`K` 依 §5 的算術，預期值：

| 事件 | 計算 | 結果 |
|---|---|---|
| 頂層 TSR（`K=0800h`） | `max(1100h, 0100h+0800h=0900h)` | `freeSeg = 1100h` |
| 第一支佇列程式 | PSP = `freeSeg+1`；`EndSeg = 1101h+10h+3+1` | PSP `1101h`，`freeSeg = 1115h` |
| 第一支 TSR（`K=084Eh`） | `max(1115h, 1101h+084Eh=194Fh)` | `freeSeg = 194Fh` |
| 第二支佇列程式 | PSP = `194Fh+1` | PSP `1950h`，`freeSeg = 1964h` |
| 第二支 TSR（`K=084Eh`） | `max(1964h, 1950h+084Eh=219Eh)` | `freeSeg = 219Eh` |
| 下一支程式 | PSP = `219Eh+1` | PSP `219Fh` |

這裡指程式的**載入位置**：保留區為 `[0100h, 0900h)`（名義上整段到 `1100h`）、`[1101h, 194Fh)`、`[1950h, 219Eh)`，彼此不重疊，後續程式從 `219Fh` 起。
遊戲對向量所指緩衝區的寫入不在此限，且是預期行為：主程式仍把資料檔讀進第一支的 `0100:0000`，蓋掉第一支自己的 PSP 與程式碼，那些內容在第一支結束後不會再執行。
這組值是 dosgolem 模型的預期，不等於真 DOS 的 MCB 位址。

## 6. 實作影響

- `internal/dos/dos.go` 的 `handle`：新增 `case 0x27:`，呼叫 `d.tsr27(c)`。
- `internal/dos/exec.go`：新增

```go
// tsr27 是 `int 27h`：舊式常駐結束（`docs/spec/197-int27-terminate-and-stay-resident`）。DX 是位元組數，向上取整成段數。
func (d *DOS) tsr27(c *cpu.CPU) {
	d.terminate(c, 0, true, uint16((uint32(c.R[cpu.DX])+15)>>4))
}
```

- `docs/spec/000-index.md` 已加清單一列、份數 17 與尾端說明列；升 READY 與 CONFORMED 時更新尾端說明列的狀態字樣。
- `handle` 在 `switch` 之前統一記 `Call{Int: n, AH: AX>>8}`；`int 27h` 沒有功能號，這個 `AH` 是呼叫端殘值，`-log-calls` 會把同一個服務拆成多筆。不改這個行為。
- 不改既有測試。`TestTSRKeepsMemory`（`internal/dos/exec_test.go`）的期望值維持字面值不變。
- 公開 repo：規格、程式、測試與 commit 不含原版檔名、檔案內容、檔案大小或畫面；指令層級的立即值（本規格的 `DX`）可入規格。

## 7. 驗收

### 7.1 單元測試（`internal/dos/exec_test.go`，期望值一律字面值）

測試 1 至 4（子程式類）：用 `writeChild` 把自編的 `.COM` 寫進 `d.Root`、`execChild` 載入、迴圈 `m.Step()` 直到 `len(d.procStack) == 0`（同 `TestTSRKeepsMemory`），**迴圈之後**先斷言 `len(d.procStack) == 0`（子程式確實結束），再斷言其餘。
`newTest` 的 `freeSeg` 為 `2000h`，子 PSP 為 `2001h`；TSR 彈回後 `d.curPSP` 已是父行程，所以期望值用字面值，不在迴圈之後用 `d.curPSP` 推算。
`DX` 的設定：子程式內用 `mov dx,imm16`；疊底類測試（5 至 7）以 `m.CPU.R[cpu.DX] = ...` 設定後 `call(m, d, 0x27, 0)`。
自編的 `.COM` 位元組：`mov dx,imm16` 是 `BA lo hi`，`mov ax,imm16` 是 `B8 lo hi`，`int 27h` 是 `CD 27`。

| 測試 | 設定 | 斷言 |
|---|---|---|
| `TestInt27RoundsKeepUp` | 子程式 `BA 01 04 CD 27`（`mov dx,0401h; int 27h`） | `freeSeg == 0x2042`；`ExecLog[0].TSR`、`Keep == 0x41` |
| `TestInt27ExactMultipleNotRoundedUp` | `BA 00 04 CD 27`（`mov dx,0400h; int 27h`） | `Keep == 0x40` |
| `TestInt27MaxDX` | `BA FF FF CD 27`（`mov dx,0FFFFh; int 27h`） | `Keep == 0x1000`，`freeSeg == 0x3001` |
| `TestInt27ExitCodeIsZero` | `B8 34 12 BA 00 04 CD 27`（`mov ax,1234h; mov dx,0400h; int 27h`），之後 `call(m, d, 0x21, 0x4D00)` | `ExecLog[0].Exit == 0`（初值 `0xFF`，未結束時失敗）；`AX == 0x0000` |
| `TestInt27RootThenQueuedProgramLandsAboveResident` | 疊底：`newTest` 後 `curPSP = 0100h`、行程疊空；`d.freeSeg = 0x1100`；`m.CPU.R[cpu.DX] = 0x8000`；`writeChild` 寫一支 40 bytes 的自編 `.COM`（`B8 00 4C CD 21` 後補 `90` 到 40 bytes，開頭不是 `MZ`）並 `d.Enqueue(name, "")`（同 `TestSupervisorQueueRunsNextProgram`）；`call(m, d, 0x27, 0)` | `d.curPSP == 0x1101`（`enterProgram` 設定）；`d.freeSeg == 0x1115`（§5.1 第 1、2 列的字面對照） |
| `TestInt27RootResidentAdvancesFreeSeg` | 同上但 `d.freeSeg = 0x0200` | `d.curPSP == 0x0901`；`d.freeSeg == 0x0915`。`K` 取 0 或 `max` 不推進時失敗 |
| `TestInt27RootResidentRoundsUp` | 同上但 `d.freeSeg = 0x0200`、`m.CPU.R[cpu.DX] = 0x8001` | `d.curPSP == 0x0902`；`d.freeSeg == 0x0916`。截斷時失敗 |
| `TestInt27NotReportedUnimplemented` | 呼叫一次 `call(m, d, 0x27, 0)` | `for k := range d.Unimplemented { if k.Int == 0x27 { t.Fatal(...) } }`（鍵型別是 `Call{Int, AH, AL uint8}`，含 `AH`、`AL` 殘值，不能用固定鍵查） |

### 7.2 負對照

- 移除 `case 0x27`（或改成 `d.note`）：`TestInt27RoundsKeepUp`、`ExactMultipleNotRoundedUp`、`MaxDX`、`ExitCodeIsZero`、`RootThenQueuedProgramLandsAboveResident`（佇列沒被啟動）、`RootResidentAdvancesFreeSeg`、`RootResidentRoundsUp`、`NotReportedUnimplemented` 都必須失敗。
- `DX>>4`（截斷）：`RoundsKeepUp`、`MaxDX`、`RootResidentRoundsUp` 必須失敗。
- `(DX>>4)+1`（永遠進位）：`ExactMultipleNotRoundedUp` 必須失敗。
- `(DX+15)>>4` 於 `uint16`（回繞）：`MaxDX` 必須失敗。
- `tsr27` 改傳 `al(c)` 當離開碼：`ExitCodeIsZero` 必須失敗。
- `K` 取 0 或 `terminate` 傳 `tsr=false`：`RootResidentAdvancesFreeSeg`、`RoundsKeepUp` 必須失敗。

### 7.3 同狀態

對象：§5 的三支 `.COM`（`cmd/probe -exe <第一支> -queue <第二支>,<第三支>,<主程式>`）。缺原版時 SKIP，並說明不算驗收。

- 「沒實作的服務」不再列出 `int 27h`。
- 向量表 `0000:0180` 至 `018F` 的寫入以 `-watch 180-18F` 觀測：寫入者的 CS 依序是三支各自的 PSP 段（`0100h`、`1101h`、`1950h`），每個位元組的值變化各只出現一次；修前同一位址被寫 12 次以上，不應再出現。三支 `.COM` 各呼叫 1 次 `int 27h`；之後若有其他 `int 27h` 呼叫，記為新發現。`-log-calls` 的「服務呼叫」只印次數最多的前 25 列，且鍵含 `AH` 殘值，不用它計次。
- 串跑結束後的向量最終值（`-dump-mem 0-3FF:<路徑>` 或 `-watch` 的最後一筆）：向量 61h、62h、63h 的段依序為 `0100h`、`1101h`、`1950h`，偏移為 0（三支程式以各自的 `CS`（等於 PSP）寫入）。若實測與此不符，記為新發現，不回頭改預期值。
- 兩支佇列 `.COM` 的 `EXEC 紀錄` 為 `TSR=true`，`keep` 等於 §5 的 `K`（`084Eh`）。
- 後續程式的 PSP 與 §5.1 的預期相同。
- 疊底那支的 `K`（`0800h`）在同狀態量測中不可觀測：bump 配置器的 `max(1100h, 0100h+K)` 對所有 `K ≤ 1000h` 都得 `1100h`，後續程式的 PSP 與 `K` 無關。它只由 §7.1 的 `TestInt27RootResidentAdvancesFreeSeg` 與 `TestInt27RootResidentRoundsUp` 覆蓋。
- 資料檔讀入緩衝區的範圍（probe 的檔案讀取紀錄：目的 `seg:off`、實得位元組數、線性範圍）：由向量 61h 取得的段為第一支的 PSP，`[seg, seg + ceil(實得位元組數 / 16))` 落在第一支的保留區 `0100h` 至 `08FFh` 內，並且與後續程式的 PSP 區間（`1101h` 起）不相交。
- `-seg-log` 顯示串跑中沒有跳進映像之外的位址（修前的症狀是進入 `3500:xxxx`）。這條同時是 §1 第 3 條因果的反事實驗證：補上 `int 27h` 之後症狀消失，因果才從強推論升為已證實。
- 同一條路徑連跑兩次，傾印雜湊相同。

## 8. 開放項目

- `008-tsr-resident` §2.1 寫「TSR 例外：只收回 `PSP + DX` 之後的部分」，字面意思是 `freeSeg` 縮到 `PSP + DX`；程式是 `max(freeSeg, PSP + DX)`，不縮回。`terminate()` 的註解與 `exec_test.go` 的註解也引同一條，屬既有的錯引。本規格的預期值依程式。
- `008-tsr-resident` §3 寫「`DX == 0` 與 `AH=4Ch` 相同，全部回收」，但 `terminate()` 在 `tsr` 且 `keep == 0` 時只做 `max(freeSeg, curPSP)`，不回收。`int 27h` 的 `DX` 為 0 時 `K=0`，繼承同一行為。
- `008-tsr-resident` §3 寫「`PSP + DX` 超出 `MemTop` 時夾到 `MemTop`」，程式沒有夾制（`d.curPSP + keep` 為 16 位元加法）。`PSP+K > 9FFFh` 時 `freeSeg` 會超過 `MemTop`，之後 `spawn` 的可用段數在 16 位元下回繞，EXEC 可能把子行程載到 `A000h` 以上。要達到需要 PSP 約 `9000h` 以上加大 `DX`，本案三支 `.COM` 碰不到。
- `009-exec` §3 寫 `AH=4Dh`「可重複讀（不清掉）」，程式（`getExitCode`）讀過就清。同一個測試讀兩次 `AH=4Dh`，第二次會是 0；§7.1 只讀一次。
- 以上四項屬 `008-tsr-resident`、`009-exec`，另案處理，不在本規格範圍。
- bump 模型使 `0900h` 至 `10FFh` 成為不可用的間隙，真 DOS 的下一支會緊接在 `0900h` 之後。沒有證據顯示遊戲不依賴三塊緩衝區的相對位置，待 §7.3 同狀態量測後確認。
- 區塊縮減失敗時是否不結束（§3 表的差異）：沒有程式需要，未量到。
- `TestInt27ExitCodeIsZero` 的「`Exit` 初值 `0xFF`」依賴基底的哨兵設計（`ExecRecord.Exit`）。其他分支若改用獨立的 `Ended` 欄位，日後合併時這條斷言要改成斷言 `Ended`。

## 9. 收據（CONFORMED）

日期：2026-10-03。分支 `phantasie-cht-overlay`，基底為 `origin/buck-rogers-cht-output-overlay`（`beca734`），實作 commit 在規格 commit 之後。

### 9.1 單元測試

§7.1 的八個測試全數通過。`internal/...`、`oracle/...`、`xlate/...`、`session/...` 全套通過（離線模組快取，映像含 ebiten 相依）。`gofmt -l`、`go vet ./internal/dos` 無輸出。

### 9.2 負對照（§7.2，逐一套用、逐一還原）

| 突變 | 失敗的測試 |
|---|---|
| 移除 `case 0x27`（改成 `d.note`） | 八個全部 |
| `DX>>4` 截斷 | `RoundsKeepUp`、`MaxDX`、`RootResidentRoundsUp` |
| `(DX>>4)+1` 永遠進位 | `ExactMultipleNotRoundedUp`、`RootResidentAdvancesFreeSeg` |
| `uint16` 回繞 | `MaxDX` |
| 離開碼取 `al(c)` | `ExitCodeIsZero` |
| `K` 取 0 | `RoundsKeepUp`、`ExactMultipleNotRoundedUp`、`MaxDX`、`RootResidentAdvancesFreeSeg`、`RootResidentRoundsUp` |
| `tsr=false` | `RoundsKeepUp`、`MaxDX`、`RootThenQueuedProgramLandsAboveResident`、`RootResidentAdvancesFreeSeg`、`RootResidentRoundsUp` |

規格列出的每一條都由指名的測試殺死（`RootResidentAdvancesFreeSeg` 在 `K` 取 0 與 `tsr=false` 下都失敗；`RoundsKeepUp` 同）。

### 9.3 同狀態（§7.3，`cmd/probe -queue`，三支 `.COM` 串跑）

| 項目 | 預期 | 實測 |
|---|---|---|
| 「沒實作的服務」 | 不列 `int 27h` | 0 種 |
| 向量表 `0180` 至 `018F` 的寫入（`-watch 180-18F`） | 寫入者 CS 依序為三支各自的 PSP，每個位元組只寫一次 | 24 個位元組，寫入者 `0100`（16 個）、`1101`（4 個）、`1950`（4 個），各一次，沒有第二輪重寫 |
| 向量 61h、62h、63h 的最終值 | 段 `0100h`、`1101h`、`1950h`，偏移 0 | 由上列寫入值得 `0100:0000`、`1101:0000`、`1950:0000` |
| 兩支佇列程式的 `EXEC 紀錄` | `TSR=true`，`keep=084Eh` | 兩支都是 `TSR=true keep=084E` |
| 後續程式的 PSP | `1101h`、`1950h`、`219Fh` | `PSP=1101`、`1950`、`219F` |
| 資料檔讀入緩衝區 | 落在 `0100h` 至 `08FFh` 內，不與 `1101h` 起相交 | 讀入 `0100:0000`，5,273 bytes，範圍 `[0100h, 024Ah)`，不與 `1101h` 起相交 |
| `-seg-log` | 不進入映像之外的位址（修前進入 `3500:xxxx`） | 段轉移依序為 `1101`、`1950`、各程式的載入 stub 段、主程式映像 `21AF`、堆疊上的中斷小常式 `2E4E`；沒有 `3500` |
| 連跑兩次的傾印雜湊 | 相同 | `B8000` 起 16 KB 的傾印三次執行雜湊相同 |
| §1 第 3 條的因果 | 補上後症狀消失 | 主程式讀到 overlay 載入器（`ov1.ovr` 載入 `21AF:53EA` 與 `2E4E:B8F0`），視訊記憶體有寫入；§1 第 3 條由強推論升為已證實 |

`int 27h` 三次以 `-watch` 的寫入者推得，沒有用 `-log-calls` 計次（見 §7.3）。

### 9.4 實作期間的處置

- 實作先在 `2f44a68` 基底完成並驗收，之後為了重用覆繪框架（`xlate`、`session`、`frontend`），以 cherry-pick 搬到 `origin/buck-rogers-cht-output-overlay` 之上，只有 `000-index.md` 有衝突，保留框架分支的內容並加入本規格的列。兩個基底上的結果逐項相同。
- 框架分支沒有 `cmd/probe -program-path`（屬 `194-stubseg-font-collision-and-program-path`）。本驗收不用它，結果與使用它時相同。
- `-watch` 與 `-watch-video` 共用同一個掛鉤，不能同時使用；同時使用時 `-watch` 沒有輸出。向量寫入與視訊統計因此分兩次執行。

### 9.5 未量到

- 區塊縮減失敗時是否不結束（§3 表、§8）：沒有程式需要。
- 疊底行程的 `K`：同狀態不可觀測，只由單元測試覆蓋（§7.3）。
- 經 stub 鏈接進來的 `int 27h`、父行程經 trampoline EXEC 的 CF（§2）：沿用 `AH=4Ch` 的既有行為，未量。

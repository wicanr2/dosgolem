# 430 — EXEC 子行程的環境區塊（複製加程式路徑）

狀態：**READY**
日期：2026-10-17
前置：[`009-exec`](009-exec.md)（AH=4Bh AL=00h）、[`009-memory-allocator`](009-memory-allocator.md)、
[`194-stubseg-font-collision-and-program-path`](194-stubseg-font-collision-and-program-path.md)（`ProgramPath`、`probe -program-path`）
案例：三國志V（san5）`SAN5.COM` → `MAIN.EXE`

---

## 1. 為什麼要做

`009-exec` §2 把子行程的 `PSP+2Ch` 直接設成參數區塊給的段，給 0 就抄父行程的。
兩種情況子行程都看到**別人的**環境區塊，尾端的程式路徑是父行程（或第一支程式）
的，不是自己的。

Microsoft C 的 overlay manager 從環境尾端取自己的路徑，再開檔讀 overlay。
三國志V 的 `SAN5.COM` 以環境段 0 EXEC `MAIN.EXE`，`MAIN.EXE` 因此開的是
`C:\SAN5.COM`（28 bytes 標頭讀不到），印 `run-time error R6806 - cannot read file`
後 exit 255，一道遊戲碼都還沒跑（量測：`wicanr2/san5-remake` 的
`docs/re/010-dosgolem-coverage.md`，實跑 `san5c`，dosgolem `81c85be`）。

真 DOS（與 DOSBox-X）每次 EXEC 都替子行程配一塊新的環境區塊。

## 2. 語意

依據 DOSBox-X `src/dos/dos_execute.cpp` 的 `MakeEnv`（`:182`）、呼叫處（`:398-400`）、
`SetupPSP`（`:266`），commit `262a304f768d5d8e93ef4abcc479ea7175aef183`。

AH=4Bh AL=00h 在配子行程的 PSP 之前：

1. 來源段：參數區塊 +0 的段；為 0 時用父行程 `PSP+2Ch`。
2. 來源段為 0（父行程也沒有環境）：字串區只有一個 `00`。
3. 否則從來源段開頭逐條讀 ASCIZ，讀到**空字串**為止，複製到這個空字串的結尾
   （含）。來源尾端原有的 `01 00 <路徑>` **不複製**。
4. 新區塊內容 = 字串區 + word `0001` + 子行程路徑的 ASCIZ。
   路徑是 EXEC 的檔名經 `AH=60h` 同一套規則攤平（`canonical`：大寫、
   補磁碟機與目前目錄、去掉 `.` 與 `..`）。
5. 大小 = 向上取整到段（`字串區長度 + 83` bytes）；83 是 DOSBox-X 的
   `ENV_KEEPFREE` 預設值，保證路徑放得下。最小 1 段。
6. 配置與一般 `AH=48h` 相同（arena 走配置策略；arena 還沒建時走 `freeSeg`
   遞增），而且**在子行程的程式區塊之前**，所以子行程 PSP 比沒有這塊時高。
7. 這塊的 MCB 擁有者是子行程 PSP（`SetupPSP` 把兩塊 MCB 都設成子行程）。
   `PSP+2Ch` 指向新區塊。
8. 子行程非 TSR 結束時，這塊跟著它名下的記憶體一起回收；TSR 結束時保留
   （常駐程式還指著它）。
9. 配不出環境區塊：`AX=8`、`BX` = 最大可用段數、設 CF，行程疊不變。
   環境配到了但程式區塊配不出來：先還環境區塊再回錯誤。程式區塊配到了但載入失敗
   （`LoadProgramAt` 回錯誤）：程式區塊與環境區塊都還回去（以前這條路漏還程式區塊）。

字串掃描的上限是 32,768 bytes（DOSBox-X `MAXENV`）。超過時回 `AX=0Ah`
（environment invalid）、設 CF。

### 2.1 範圍外

- 監督佇列推出的下一支（`009-exec` §4，`spawnQueued`）不是 EXEC，不在本規格。
  它仍沿用第一支程式的 `EnvSeg`。
- 第一支程式的環境（`machine.initPSPAt` 寫在固定的 `EnvSeg`）不變。
- AL=03h（overlay）不建 PSP，也不建環境（DOSBox-X 同樣只在 `flags!=OVERLAY` 時呼叫 `MakeEnv`）。

## 3. `probe -program-path`

`194-stubseg-font-collision-and-program-path` 讓 `-program-path` 指定第一支程式的
argv[0]，但 `cmd/probe/main.go` 在讀完旗標之後無條件把 `m.ProgramPath` 改成
`C:\<-exe 的檔名>`，旗標從來沒生效。改為：旗標空白時才用 `C:\<檔名>`。
沒給旗標的既有呼叫結果不變。

## 4. 已知差異

| 項目 | 本實作 | 來源 | 分類 |
|---|---|---|---|
| 字串區結尾的判定 | 逐條 ASCIZ 讀到空字串 | DOSBox-X 找第一個 word 0；對 `00 01 00 <路徑>` 這種空環境（`-empty-env`）會讀進路徑 | 刻意不照做：兩者在非空環境下結果相同，空環境時本實作得到正確的 1 byte |
| 環境字串內容 | 照抄父行程（第一支程式只有 `COMSPEC=`） | 真 DOS 有 `PATH`、`PROMPT` 等 | 沿用 `003-machine-and-loader` §4 的未定案 |
| 子行程 PSP 位置 | 比改版前高「環境段數 + 1」段 | 真 DOS 同此 | 照參考實作。既有收據若記了子行程的段位址，數值會變（見 §5 第 4 點） |

## 5. 驗收

`internal/dos/exec_env_contract_test.go`：

1. 環境段 0：子行程 `PSP+2Ch` 不等於父行程的；新區塊的字串區等於父行程的字串區，
   尾端是 `01 00` 加子行程的完整路徑。
2. 環境段非 0：字串取自給的段，不取父行程的。
3. 新區塊的 MCB 擁有者是子行程 PSP；子行程非 TSR 結束後，那一段回到自由。
4. `-empty-env` 形狀（`00 01 00 <路徑>`）的父環境：子行程的字串區是單一 `00`。
5. 負對照：把複製長度改成「含父行程尾端路徑」，第 1 條必須失敗。

整體回歸：`tools/ci.sh` 與改版前的基線比較，新增的失敗逐條列出原因
（子行程段位址改變而失效的收據不在本次重錄範圍，回報後再決定）。

上層收據：三國志V `SAN5.COM` 鏈中 `MAIN.EXE` 開的是自己（`C:\MAIN.EXE`），
不再印 R6806。

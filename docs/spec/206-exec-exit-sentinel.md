# 206 — ExecRecord 用 Ended 區分「還沒結束」（Exit 不再當哨兵）

狀態：**READY**（只定欄位與三處用法；觸發案例見 §3；實作另開條目）
日期：2026-09-25
前置：無（`ExecRecord` 定義在 `internal/dos/exec.go`）

---

## 1. 規則

`ExecRecord.Exit` 的 `0xFF` 同時是「退出碼 255」與「還沒結束」——WCG
乾淨退出碼正好是 255（`pop` 回傳值慣用法），撞碼後：

- `terminate` 配對（`exec.go`）靠 `Exit == 0xFF` 認活塊（實務被「由新到舊」
  順序救了，但 PSP 重用＋255 退出理論上會誤配舊紀錄）；
- `cmd/run -trace-after-exit`（`main.go`）判 `ExecLog[0].Exit != 0xFF`，
  對這類跑永不觸發；
- `memops`／`probe` 把乾淨退出的子印成 `exit=FF`，與沒結束長得一樣。

修法：`ExecRecord` 加 `Ended bool`（初值 false；`Exit` 初值照舊 `0xFF`，
只放碼、不再當哨兵）。

- 產生處（spawn／overlay／queue 三處 append）：不用改（零值即 false）。
- `terminate` 配對改 `PSP == curPSP && !Ended`；命中設 `Exit＝code`＋
  `Ended＝true`（TSR／Keep 照舊）。
- `-trace-after-exit` 改判 `ExecLog[0].Ended`。
- `memops` 與 `probe` 的 EXEC 印表：`!Ended` 加印 `未結束`
  （`exit=%02X` 本體不動；overlay 紀錄永遠未結束，照印）。

## 2. 驗收

- 新測試：子 `exit(255)`（`mov ax,4CFFh; int 21h`）→ 紀錄 `Ended 且
  Exit＝0xFF`；再跑一支（PSP 重用情境）仍正確配對，不誤寫舊紀錄。
- 全套件綠；WCC 一支重跑行為不變（只差顯示與 flag 觸發）。

## 3. 觸發案例（`retro-runtime-study-private#47`，接 #32 R56）

chained `WCC EMPTY.C` 修好後 `memops` 印 `WCG.EXE PSP=2BD9 exit=FF`——
不是沒結束，是乾淨退出碼 255（全程 trace 步 122531–122532 實證）。

## 4. 範圍外

- 改退出碼語意、`AH=4Dh`（讀 `lastExit`，不受影響）、快照（`ExecLog`
  非快照內容）、WCC 側。

## 5. 實作（2026-09-25）

照 §1 落地（`Ended` 欄＋配對／flag／兩處顯示），測試
`TestExit255IsRecordedAsEnded` 全綠，全套件綠。chained `WCC EMPTY.C`
重跑：`memops` 印 `exit=FF` 不帶「未結束」，`-trace-after-exit` 正常觸發
（以前永不觸發），編譯結果不變（回傳碼 0，`Code size: 11`）。

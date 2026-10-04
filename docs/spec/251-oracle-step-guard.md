# 251 Oracle 指令前等待閘門

狀態：CONFORMED
日期：2026-10-04

## 契約

既有 BIOS 空佇列非阻塞行為保持原樣。程式層可選用 `Oracle.SetStepGuard(func(*Oracle) error)`，判定何時需要停止執行。通用層不包含遊戲位址或簽章。

1. 零值與 nil 關閉 guard，既有 RunUntil 行為不變。設定新 guard 替換舊 guard；一台 Oracle 一個擁有者，同執行緒使用。
2. 順序為條件成立檢查、結束與 HLT 及位址護欄、guard、OnCall、stub、Machine.Step。條件已成立時回 nil，無預算時仍 BudgetError。原有預算溢位判斷不變。
3. guard 回錯時原樣返回，該次不執行 hook、stub、指令、tick；Oracle 的執行迴圈不寫原版狀態。guard 自身不得直接改 CPU、記憶體、步數或時鐘來跳過程式。一般輸入 API 是明示例外：SendKeys 等可透過既有 BIOS 佇列寫入 BDA，這次合法送鍵不適用「停留時記憶體不變」要求。沒有送鍵而回 InputWaitError 時，原版狀態必須完全不變。
4. `InputWaitError` 是可辨識的 typed error，含 Stopped Addr，Error 說明正在等待鍵盤。它與 BudgetError、ExitError 不同；Run 不得將等待當成已完成 n 步。
5. 外部可收到按鍵後再次呼叫 Run；guard 重查並回 nil 即繼續。沒有排程執行緒或自動輸入。此契約只保證決定性停止，不保證平台硬體時鐘一致。

## 測試與範圍

以合成 COM 驗證預設行為、條件優先、guard 拒絕時 CPU／1 MiB 記憶體／步數／tick 不變、hook 與 stub 未觸發、重複停止、允許後續跑、設定 nil 後移除、一般錯誤原樣傳回與預算錯誤維持。測試不依賴原版素材。

程式：`oracle/oracle.go` 加私有 guard 欄位；`oracle/run.go` 加 setter、InputWaitError 與指令前判定；新增 `oracle/step_guard_test.go`。不改 CPU、DOS、BIOS、Machine 或其他 apps。

審查：2026-10-04 兩種唯讀審查報告在專案本機 `workplace/review/wait-contract.md`、`wait-evidence.md`。主代理核對唯一阻擋並明確列出一般輸入 API 的 BDA 副作用例外後 READY。驗收見下節。

## 驗收收據

2026-10-04，Go 1.24.13，Docker `psychicwar-go-ebiten:latest`，`go test -v ./oracle` 通過。三個 StepGuard 測試實際 PASS，涵蓋整份 Machine Snapshot 不變、hook/stub 零觸發、條件與預算優先、一般錯誤原樣傳回、重複停止、恢復、替換與移除。其餘合成 Oracle 回歸通過；未提供其他遊戲輸入而 SKIP 的測試不算原版驗收。遊戲層另外依自己的規格驗收。

# 457：executeCore 由布林 switch 改為值 switch（已撤回）

狀態：**撤回**（第 2 版）。實作 commit `1669fe4` 的程式與測試已還原為 `a3cfff5`（`internal/cpu/ops.go`；刪除 `opsref_test.go`、`opsref_diff_test.go`），
由主代理以新 commit 記錄。本檔保留供紀錄。
日期：2026-10-22
前置：[`455-fetch-page-cache`](455-fetch-page-cache.md)（同一輪效能工作）、[`002-cpu-8086`](002-cpu-8086.md) §5（CPU 驗收：語料全數通過）

推論等級：已證實、強推論、假說、未知。

## 1. 做過什麼

`executeCore`（`a3cfff5` 的 `internal/cpu/ops.go` 第 134–605 行）寫成 `switch { case <布林式>: … }`，Go 只能依序比較。改寫成 `switch op`：
範圍條件展開成常數列表，兩處機型相依的重疊（60–6F 在 80186 以上走 `op186_6x`；C0、C1、C8、C9 在 80186 以上走移位 imm8、ENTER、LEAVE）
改在 case 開頭判斷，各 case 本體逐字保留。

等價證據（已證實）：

- SingleStepTests 315 PASS、0 FAIL，已知差距 F6.7 6218、F7.7 6371，與改動前相同。
- `TestExecuteCoreDifferential`：256 opcode × 8086／80186／80386 × 每格 60 組隨機狀態，對照逐字保留的舊函式 `executeCoreRef`，比錯誤、panic、CPU 結構、
  匯流排寫入與 OUT；負對照（C1 的機型條件改成 ≥80386）失敗。審查 A 另以 go/ast 解析新舊函式，768 格 0 不符，`executeCoreRef` 與 `07cabfa` 原函式逐字相同。
- 裸 RunUntil（s760、s800 × 觀察器關／開）、截短 Q on23、長測試 Q、C11、存讀檔 F–K 的雜湊與收據全部相同。

## 2. 撤回理由

- **收益未證實**：裸 RunUntil 交錯 8 輪的最小 CPU 時間，四個場景平均約 −2.6%（兩個場景新版較慢），低於事先訂的 5% 門檻；中位數與次小值雖都偏向新版，
  但同設定的 CPU 時間在 2.3–9.7 s 間波動，不足以區分（審查 B 應改 C）。
- **測試成本**：`TestExecuteCoreDifferential` 讓 `internal/cpu` 的測試由不到 1 s 變成約 43 s。
- 等價性沒有問題；若之後在穩定的量測環境（例如固定頻率、無其他負載的主機）證實收益，可以照 §1 的做法與測試重做。

## 3. 未知

- 值 switch 的實際收益：未知（雜訊之下）。
- 前綴表（`CPU.Step` 的前綴迴圈）：只評估未實作，預估 1–3%。

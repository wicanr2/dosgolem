# 443 — 《大航海時代 II》DOS 版的對拍轉接層

狀態：**READY**
日期：2026-11-04
案例：`wicanr2/koukai2`（private）

---

## 1. 範圍

在分支 `koukai2` 上讓 dosgolem 跑 `KOUKAI2.COM`，從片頭走到主迴圈並留收據。不含：遊戲規則的語意（koukai2 專案的 `docs/spec/`）、原版檔案（玩家自備，缺檔就 skip）。

## 2. 觀測層的變更

`oracle.Options` 多一個 `COM bool`。

| 值 | 行為 |
|---|---|
| `false`（零值） | 與舊版相同：只收 MZ，其他檔案回 `machine: 不是 MZ 執行檔` |
| `true` | 用 `Machine.LoadCOM` 載入 |

`cmd/probe` 看 magic 判斷格式，那是探針的便利行為。`oracle.Load` 維持不猜格式，所以這裡要呼叫端明講。既有呼叫端不受影響。

## 3. 程式層

`apps/koukai2` 依 `006-layering` 的判準放這一支的知識：

| 名稱 | 內容 |
|---|---|
| `Load` | `Adlib`、`MouseXScale=1`、`COM` |
| `Bind`、`Current` | 從 EXEC 紀錄算被 EXEC 程式的基底。IDA 線性 0x10000 對應 PSP＋10h |
| `RNGTrace`、`WatchRNG` | 逐次比對 `14D92` 的 LCG，統計 `rand_n` 的參數 |
| `WatchSeedWriters` | 記下 `14DAD`、`14DB0` 以外對種子的寫入 |
| `Capture` | 比對點：步數、IRQ0 次數、模式、畫面、DAC、1 MB 記憶體的雜湊 |
| `BootPath`、`ToMain`、`ToMainLoop` | 冷啟動到主迴圈的固定輸入 |

`cmd/k2boot` 產收據 JSON，`cmd/k2step` 逐步探索。

`o.IDA` 不能用於 `MAIN.EXE`：它以最外層 `KOUKAI2.COM` 的載入段換算，讀到的是另一塊記憶體，而且不會報錯。

## 4. 已量到的行為

| 項目 | 結果 |
|---|---|
| 未實作的服務 | 0 種 |
| 缺的檔案 | 0 個 |
| 機器層補強 | 不需要 |
| 計時器 | `FMDRV.COM` 把 PIT 分頻設成 4096，`TimerHz` 291.30 |

### `OnCall` 在中斷返回時重複觸發

hook 在每道指令執行前檢查。IRQ 剛好落在 hook 位址那一道時，返回位址就是該位址，`iret` 回來後 hook 再跑一次。

開機路徑上，`14D92` 與 `14DC4` 各重複 24 次，1,033,068 次呼叫中出現。症狀是「實際種子的下一個才是預期值」，看起來像種子被別的程式碼改過。`RNGTrace` 以種子與 SP 都相同判定重複，記在 `Repeats`。

這是 `OnCall` 既有的語意，本規格不改。要精確計數的 hook 要自己去重。

IRQ 派送與 ISR 第一道指令在同一個 `Machine.Step` 內完成，所以掛在 ISR 第一道指令的 `OnCall` 不會觸發。

## 5. 測試

- `oracle.TestLoadWithCOMOption`：不需原版。`COM=false` 拿 .COM 回錯；`COM=true` 可載入並跑到結束。
- `apps/koukai2.TestBootDeterministic`：需要原版，環境變數 `DOSGOLEM_KOUKAI2_ROOT`，缺檔就 skip。冷啟動跑到主迴圈兩次，比對點逐項相同；種子首值 0、LCG 不符 0、沒有其他寫入端、`DS:0CA4..8A18` 在標題畫面等於檔案初值、第一天時鐘 `15 04 10 18`。

## 6. 收據

收據放在 koukai2 專案的 `docs/playtest/`，記錄原版雜湊、本分支 HEAD、輸入序列與比較結果。動到執行器行為後要重跑。

## 7. 權利邊界

本儲存庫不含原版檔案。位址與雜湊可進文件。

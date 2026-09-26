# 237 — 暫存層決定性：檔案時間與大小寫

狀態：**CONFORMED**（2026-09-26；驗收見 Buck `docs/re/phase-249-*`）
日期：2026-09-26
前置：[`009-scratch-writes`](009-scratch-writes.md)

---

## 1. 為什麼要做

Buck Rogers 存檔後回主選單、加入隊伍的路徑，同一個 state、同一組按鍵，
兩次 control 間隔 3 秒，`memory_sha256` 就不同；事件、FileOps、畫面都相同。

來源是 `internal/dos/find.go` 的 `dosDateTime`：它把主機檔案的 `ModTime()`
打包成 DOS 日期時間，經 `AH=4Eh／4Fh` 的 DTA 與 `AH=57h` 交給程式。暫存層在
執行中新建或複製的檔案，mtime 是主機建檔時刻，於是程式記憶體隨牆上時鐘變動。
`AH=2Ah` 日期固定 1993-01-01、`AH=2Ch` 時間掛在 PIT tick 上，都是決定性的；
檔案時間是唯一的外洩點。

第二個問題在同一條路徑上看到：暫存層同時出現 `CHARS.DAX` 與 `CHARS.dax`、
`BUCK.who` 與 `BUCK.WHO`。`scratchCopy` 與 `create` 用程式給的大小寫組目標路徑，
只做精確大小寫的 `os.Stat`。暫存層已經有同名、不同大小寫的檔時，會再造一份。
之後 `resolve` 走 `lookupDOS` 大小寫不分地找，可能拿到沒被寫過的那份。
DOS 檔名不分大小寫，一個名字只能對應一個檔。

證據（Buck 專案，私有收據不入版控）：`docs/re/phase-248-*`。

## 2. 契約

1. **檔案時間**：暫存層裡由本次執行建立、寫時複製或寫入的檔案，把主機 mtime
   設成當下的虛擬時刻：日期取 `AH=2Ah` 的回傳值，時間取 `d.clock()`，秒數取偶數
   （DOS 時間解析度 2 秒）。涵蓋的寫入點：
   - `create`（`AH=3Ch`，`5Ah`／`5Bh` 經由它）建立或截斷後；
   - `scratchCopy`（`AH=3Dh` 寫入模式的寫時複製）複製後；
   - `renameFile`（`AH=56h`）從 `Root` 複製進暫存層的分支；
   - `write`（`AH=40h`）對暫存層 handle 每次成功寫入後。
   以 `time.Local` 組時間，讓 `dosDateTime` 讀回相同欄位。前提是寫入與讀回在同一個
   程序、同一份時區資料；日期固定 1993-01-01，沒有日光節約邊界。只需要整秒精度，
   一般檔案系統都能保存。
   選擇寫主機 mtime、而不是在 DOS 狀態內另記每檔時間：savestate 不含暫存層內容，
   從 checkpoint 接續時，暫存層是另外複製的目錄；時間寫在 mtime 上，只要複製時
   保留 mtime（`cp -p`），接續後讀到的時間就一致。記憶體內的表在接續時會遺失。
2. 原版 `Root` 的檔案與執行前就放在暫存層的檔案，時間不變，仍讀主機 mtime。
   呼叫端要自行讓這些輸入的 mtime 固定（例如複製時保留 mtime）。
3. **大小寫**：`scratchCopy` 與 `create` 先用 `lookupDOS(d.Scratch, base)`
   找既有檔；找到就用它的實際路徑，找不到才用程式給的名字建立。
4. `Scratch` 為空時行為完全不變。
5. **補充（2026-09-26）**：`bootroot.Prepare` 把原版樹複製成存檔樹時，檔案與目錄
   都保留來源 mtime（目錄在整棵樹複製完後回填）。sealed session 的 DOS `Root`
   就是這棵存檔樹；不保留的話，同一組按鍵的冷開機，記憶體雜湊隨複製當下的主機
   時間變動（Buck `docs/re/phase-251-*`）。

## 3. 不做什麼

- 不改 `Root` 檔案的時間來源，也不處理主機時區差異。
- 不實作 `AH=57h AL=01`（設定檔案時間）的落地；現況只記帳。
- 不處理目錄名稱的大小寫。
- `rename`、`delete` 的大小寫收斂不在此次範圍。
- [`009-scratch-writes`](009-scratch-writes.md) §3 仍寫「不做時間戳、rename」，但
  `AH=56h`、`AH=57h` 已實作；009 的文字與現況不同步，另案訂正。

## 4. 驗收

1. 單元測試：暫存層新建檔的 `dosDateTime` 等於虛擬時刻；大小寫不同的既有檔被重用，
   目錄裡只有一份。
2. Buck 存檔後加入隊伍路徑：兩次 control 間隔超過 2 秒，`memory_sha256` 相同；
   暫存層沒有大小寫重複檔。

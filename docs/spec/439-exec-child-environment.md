# 439 — EXEC 為子行程建立環境副本（附程式路徑）

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`009`](009-dos-exec.md)（EXEC 與 PSP，若檔名不同以 `000-index.md` 為準）、[`438-probe-guard-physical-pc`](438-probe-guard-physical-pc.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

《Strike Commander》由 `SCCD.EXE` 以 `int 21h AX=4B00h` 依序執行 `menu.exe`、`opttest.exe`、`mkterr.exe`、`strike.exe`（`strike_commander` 的 `docs/re/002`）。
dosgolem 在不開音效的設定下跑到 MENU 選單並點選後（冷開機，第 3403993729 步起），`OPTTEST.EXE` 只執行約 340 萬道就以結束碼 3 結束，主控台出現
「Abnormal program termination」。這段期間 OPTTEST 唯一開過的檔是 `C:\SCCD.EXE`：讀 20 bytes 的檔頭，跳到位移 9232（即 `SCCD.EXE` 的檔尾）讀 16 bytes 得 0。
這是執行期以 argv[0] 開自己的執行檔找附加資料（overlay）的典型行為（強推論）；它開到的是父程式。

原因（已證實，程式碼）：`internal/dos/exec.go` 的 `spawn` 在參數區塊的環境段為 0 時，把子行程 PSP:2Ch 直接設成父行程的環境段（約 211–215 行），子行程與父行程
共用同一塊環境，環境尾端的程式路徑仍是父程式的 `C:\SCCD.EXE`。

真 DOS 與 DOSBox-X 在每次 EXEC（`AL=00h`、`01h`）都替子行程配一塊新環境（DOSBox-X `dos/dos_execute.cpp` 的 `MakeEnv`，182–238 行；呼叫處 395–405 行）：

1. 來源：參數區塊的環境段；為 0 時用父行程 PSP:2Ch 的環境。
2. 長度：來源環境的字串到雙 0 結尾為止，加 2；再加保留 83 bytes（`ENV_KEEPFREE`），換算成節（paragraph），至少 1 節。
3. 在載入程式之前以一般配置（`DOS_AllocateMemory`）配這一塊，所以子程式的 PSP 位於環境塊之後。
4. 複製字串與雙 0，接著寫 word `0001h`，再寫子程式的正規化完整路徑（`DOS_Canonicalize`，例如 `C:\OPTTEST.EXE`）與結尾 0。
5. 子行程 PSP:2Ch 指向新塊；這一塊屬於子行程，子行程結束時與程式塊一起釋放。

`AL=03h`（載入 overlay）不建環境。

## 2. 規則

- 新增 `dos.DOS` 的選項 `ExecEnvCopy bool`，預設 false（行為與現行完全相同）；`cmd/probe` 加旗標 `-exec-env`。預設關閉的理由：這是多個專案共用的 DOS 層，
  開啟後子程式的 PSP 位置會往上移，既有收據（例如以 PSP 或位址比對的測試）可能改變；要不要改為預設開啟，另由使用者決定。
- `ExecEnvCopy` 開啟時，`AL=00h`／`01h` 依 §1 第 1–5 項建環境塊：
  - 配置走 dosgolem 現行的配置路徑（與 `int 21h AH=48h` 相同的 arena 與 MCB 規則），MCB 的擁有者是子行程 PSP；空間不足時 EXEC 回錯誤 8（記憶體不足），與真 DOS 同。
  - 程式塊的配置照現行 `placeChild`，位置在環境塊之後。
  - 正規化路徑：以 EXEC 的檔名（`DS:DX`）依目前磁碟與目錄展開成 `X:\…` 的完整路徑，字母轉大寫，與 `int 21h AH=60h` 的結果一致（若 dosgolem 尚無 `AH=60h`，
    以服務層解析檔案時實際使用的路徑轉成 DOS 形式）。
  - 子行程結束（`AH=4Ch`、`int 20h`、`AH=00h`）時，連同環境塊釋放；駐留（`AH=31h`）時環境塊保留（真 DOS 同）。
- `-pm-profile` 不自動開啟本選項；本專案的執行指令同時給 `-exec-env`。

## 3. 驗收

1. 選項關閉時：`go test ./internal/... ./cmd/probe ./session ./oracle` 與 `go test ./apps/...` 的結果與改動前逐行相同。
2. 選項開啟的單元測試：父程式以環境段 0 執行子程式，子程式 PSP:2Ch 指向新塊；新塊內容為父環境字串、雙 0、`01 00`、`C:\CHILD.EXE`、0；新塊的 MCB 擁有者為子 PSP；
   子程式 PSP 在環境塊之後；參數區塊給非 0 環境段時從該段複製；子程式結束後兩塊都釋放（`AH=48h` 能再配到同一位置）；駐留時環境塊保留；記憶體不足時回錯誤 8 且
   不留下半配置的塊；`AL=03h` 不建環境。
3. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile -exec-env`（`workplace/root-nosound/`，第 3400000000 步點 (160,45)）：
   OPTTEST 不再開 `C:\SCCD.EXE`；照實記錄 OPTTEST 的行為、後續 `EXEC` 紀錄、最後停點與原因。
4. `go build ./...`、`go vet ./internal/... ./cmd/probe` 全綠。

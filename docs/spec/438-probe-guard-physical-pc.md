# 438 — probe 的視訊區護欄在轉換開啟時改看實體位址

狀態：**READY**（第 2 版，依審查與實測修訂）
日期：2026-10-16
前置：[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md) §7、
[`437-segmented-32bit-far-transfers`](437-segmented-32bit-far-transfers.md)

---

## 1. 為什麼要做

`cmd/probe` 每步檢查「下一道指令的線性位址落在 `A0000h`–`MemSize`」就停（`cmd/probe/main.go` 約 762–770 行，434 §7 改用 `PCLinear()`）。理由寫在該處
註解：沒有分頁時那段是視訊記憶體與 BIOS，程式跑進去代表飛掉，`00 00` 會一路解下去而不報錯。

開啟分頁後這個前提不成立。437 的觸發案例在第 2333937 步停在 `B002:2937`（線性 `B2957h`）：MENU 段 `158Ch` 的 `0x3C8` 查表常式從 `cs:[12Eh]` 的表取得
`B002:0039`，比對鍵值 `64h` 後回傳處理位址 `B002:2937`，再以 `retf` 跳過去。停點前一步（第 2333935 步）的頁表（已證實，probe 傾印）：`CR3=0011C000h`，PDE[0]＝`00118027h`，線性頁 `B0h`–`B7h` 的 PTE 為 `00126067h`–`0012D007h`，
即線性 `B0000h`–`B7FFFh` 對到 XMS 區的實體 `126000h`–`12DFFFh`（`B0h`–`B4h` 已被寫過，D 位元為 1）；`B002:2937` 的實體位址為 `128937h`，是一般 RAM。
（`docs/re/003` §3 的映射狀態表記載 `SM` 狀態下 `B0000`–`B7FFF` 指向實體 `B8000`，與本次實測不同；何時套用哪一種映射另行查證。）護欄以線性位址判斷，
把合法執行誤判為飛掉。

## 2. 規則

- `internal/machine` 新增匯出方法 `PCPhys() (phys uint32, ok bool)`，與既有觀測路徑 `peekLin`（`internal/machine/peek.go`）同法：
  - `!m.xlat`：回 `PCLinear()`、true（與現行逐位元組相同）；
  - `m.xlat && !m.paging`：回 `a20Mask(PCLinear())`、true；
  - `m.paging`：`peekXlatPage(PCLinear())`，頁不存在回 (0, false)，否則回 `a20Mask(phys)`。
  無副作用：不填 TLB、不寫 accessed、不留錯誤紀錄、不觸發讀取監看。
- probe 護欄改為：`PCPhys()` 的 `ok` 為 false 時照常執行：該步取指令讀到 `FFh` 並記分頁紀錄，先以 `FFh` 位元組解碼執行，結束時回 `*PageFaultError`
  （432 §5.3、§10 第 1 項），probe 因錯誤停止，不由護欄攔；`ok` 為 true 且實體位址落在
  `A0000h`–`MemSize` 時停，訊息加上實體位址。
- 轉換關閉時行為與現行逐位元組相同（`PCPhys()`＝`PCLinear()`）。
- `oracle/run.go` 約 122 行、`apps/rich2/cmd/parity`、`cmd/scenetime/main.go` 約 80、292 行的同類護欄不在 `-pm-profile` 下使用，不改。
- 不在範圍：中斷點與 `Coverage` 維持線性語意（`PCPhys` 只供護欄）；PTE 指到沒有後備記憶體的實體頁時讀回 `FFh` 而不報錯（`physical.go` 的 `readPhysRaw`），
  屬既有行為，另行處理。

## 3. 驗收

1. profile 關閉時：既有 probe 測試與 `go test ./apps/...` 結果不變；錨點不變。
2. 單元測試：轉換關閉時 `PCPhys()` 等於 `PCLinear()`；轉換開啟、`PG=0` 且 `CR3` 非零時不走頁表；A20 關閉時線性 `100000h` 以上被遮罩；分頁開啟且線性頁 `B0h`
   映到實體頁 `200h` 時回 `200xxxh`；頁不存在時回 false；呼叫前後 TLB、PTE 的 accessed 位元與分頁錯誤紀錄不變。probe 層：線性頁對到 RAM 時護欄不停；
   對到 `A0000h`–`MemSize` 時停；頁不存在時以 `PageFaultError` 結束而不是護欄訊息。
3. 觸發案例（需原版，另附 probe 收據）：`B002:2937` 不再被護欄攔下（實體 `128937h`）；照實記錄繼續執行後的最後停點、原因、步數、`EXEC` 紀錄與視訊模式變化。
4. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

## 4. 實作收據（2026-10-16）

commit：`6cddfed`。`Machine.PCPhys` 與 `cmd/probe/guard.go` 的 `pcGuard`；測試 `internal/machine/pcphys_test.go`、`cmd/probe/guard_test.go`。

- 第 1、2、4 項：`go test ./internal/... ./cmd/probe ./session ./oracle` 全 ok；SingleStepTests 315 PASS；`go test ./apps/...` 只有既有的 GLFW 失敗。
- 第 3 項觸發案例：`B002:2937` 不再被攔。共執行 4300124 道後停在 `0070:519E` 的 `mov cr0,eax`（清 PE，保護模式回實模式，S9）。途中 `MENU.EXE` 以結束碼 1
  結束（沒有送任何輸入），第 4276660 步視訊模式回到 `03h`，之後 SCCD 經 `int 15h` 的 `UN` 讓 JEMM 卸載（`docs/re/004` §4 的 S9 路徑 `583C`→`5187`）。

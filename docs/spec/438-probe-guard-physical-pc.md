# 438 — probe 的視訊區護欄在轉換開啟時改看實體位址

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md) §7、
[`437-segmented-32bit-far-transfers`](437-segmented-32bit-far-transfers.md)

---

## 1. 為什麼要做

`cmd/probe` 每步檢查「下一道指令的線性位址落在 `A0000h`–`MemSize`」就停（`cmd/probe/main.go` 約 762–770 行，434 §7 改用 `PCLinear()`）。理由寫在該處
註解：沒有分頁時那段是視訊記憶體與 BIOS，程式跑進去代表飛掉，`00 00` 會一路解下去而不報錯。

開啟分頁後這個前提不成立。437 的觸發案例在第 2333937 步停在 `B002:2937`（線性 `B2957h`）：MENU 段 `158Ch` 的 `0x3C8` 查表常式從 `cs:[12Eh]` 的表取得
`B002:0039`，比對鍵值 `64h` 後回傳處理位址 `B002:2937`，再以 `retf` 跳過去。這段程式前後以 `int 15h AX=1209h` 的 `SM`（`BX=534Dh`）與 `sm`（`BX=736Dh`）
切換 JEMM 的頁表（`docs/re/003` 的 `SM`：PDE[0] 換成遊戲頁表），所以線性 `B0000h` 一帶可能對應到一般 RAM（強推論，以本規格的收據證實）。護欄以線性位址判斷，
把合法執行誤判為飛掉。

## 2. 規則

- `internal/machine` 新增匯出方法 `PCPhys() (phys uint32, ok bool)`：轉換關閉時回 `PCLinear()` 與 true；轉換開啟時以 `peekXlatPage(PCLinear())` 轉成實體位址
  （無副作用，不填 TLB、不寫 accessed、不留錯誤紀錄），頁不存在時回 false。
- probe 護欄改為：`PCPhys()` 的 `ok` 為 false 時照常執行（下一步取指令會依 432 的分頁錯誤規則回報，不由護欄攔）；`ok` 為 true 且實體位址落在
  `A0000h`–`MemSize` 時停，訊息加上實體位址。
- 轉換關閉時行為與現行逐位元組相同（`PCPhys()`＝`PCLinear()`）。
- `oracle/run.go` 約 122 行與 `apps/rich2/cmd/parity` 的同類護欄不在 `-pm-profile` 下使用，不改。

## 3. 驗收

1. profile 關閉時：既有 probe 測試與 `go test ./apps/...` 結果不變；錨點不變。
2. 單元測試：轉換關閉時 `PCPhys()` 等於 `PCLinear()`；轉換開啟且線性頁 `B0h` 映到實體頁 `200h` 時，`PCPhys()` 回 `200xxxh`；頁不存在時回 false；呼叫前後 TLB、
   PTE 的 accessed 位元與分頁錯誤紀錄不變。
3. 觸發案例（需原版，另附 probe 收據）：記錄停點前一步 `B002:2937` 的實體位址；照實記錄繼續執行後的最後停點、原因、步數、`EXEC` 紀錄與視訊模式變化。
4. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

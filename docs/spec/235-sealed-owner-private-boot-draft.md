# 235 — Sealed Owner 私有開機（READY，限本契約範圍）

狀態：**READY。DRAFT 首輪審查退回兩項 P0（別名活門、TOCTOU＋symlink），
已修；針對性複審五項全落實、無新 P0，判可升 READY。實作
`session/boot_original.go`＋合成測試已落地，production 獨立審查
「可進 production（附 3 非阻擋殘留，均已補）」，`go test -race
./session/` 全綠。升 READY 不授權 observer、多層聚合、存讀檔或視窗。**
日期：2026-09-25

## 缺口與邊界

`session.New` 明寫 narrow owner 尚無原版開機：Booting 不安裝 DOS、
不許 step，「A future private boot must LoadEXE, then Install, then
enter Running」。Linux 可玩前端（Buck #16）的 composition root
缺的就是這一步；它不是 observer 安裝（`Watcher.Install` 介面尚無，
仍屬 DRAFT，見 Buck 規格 019）、不是多層聚合、不是存讀檔。

本規格只收束**私有開機**：已封存 owner 在 Booting 從呼叫端給的
EXE bytes 做 LoadEXE→Install→Running，失敗即關閉。原版樹的
讀取、版本／雜湊核對、唯讀 original 與可寫 save root 的分離與複製，
屬於 composition root／CLI 的 `bootpreflight` 契約，不在本規格；
owner 只核對到手的 bytes 與 save root 本身。

現行程式事實（fork `71f4763`；行號經獨立審查核對）：
`machine.LoadEXE(data []byte)` 在呼叫期間借用、返回後不保留別名
（`loader.go:80` 深複製映像、`207`／`234` 經 `WriteBytes` 拷貝進
`Mem`，`machine.go:1108` 逐 byte 拷貝）；`dos.Install()` 無回錯值、
必須在 LoadEXE 後（`dos.go:645`，`freeSeg` 取自 LoadEXE 已設的
`M.FreeSeg`）；`dos.New(m, root)` 的 `Root` 是公開欄位。
`New` 以 `dos.New(m, "")` 建機，mouse 持 `MouseOutput` 介面
（內裝同一 `*DOS` 指標，無 `Root` 快取，`mouse_bridge.go:100-114`），
keyboard 持 `*dos.DOS`＋`*machine.Machine`（`keyboard.go:19-51`）；
`resolve` 每次即時讀 `d.Root`（`files.go:48-60`），`Install` 不讀
`Root`。故開機不重建 machine／DOS／bridge，只填入 save root、
LoadEXE、Install、轉 Running；事後填入 `d.Root` 安全，無需重綁。

## 候選契約

```go
type BootInput struct {
    EXE            []byte
    ExpectedEXESHA256 [32]byte
    SaveRoot       string
}
type BootReceipt struct {
    Phase   Phase
    EXESHA256 [32]byte
}

func (o *Owner) BootOriginal(input BootInput) (BootReceipt, error)
```

1. `o == nil`、phase 非 Booting、重複開機一律拒絕；Running／Stopped／
   Failed／Closed 不可回頭。**第 1–3 條的驗證拒絕一律不呼叫 `fail()`**，
   phase 不變、不裝 DOS、不計步。
2. `len(input.EXE) == 0`、超過上限（`4<<20`；START.EXE 實測 67,619
   bytes；**上限僅為損毀輸入護欄，非版本斷言，可載入性以
   `LoadEXE`／MZ bounds 為準**）、`ExpectedEXESHA256` 全零拒絕。
3. `SaveRoot` 為空、經 `Lstat` 為 symlink（含指到目錄的連結，一律拒絕）、
   非目錄、含 NUL 等不可表示字元（`open` 回錯映射為拒絕，不 panic）、
   `unix.Access(path, W_OK|X_OK)` 失敗，一律拒絕。**禁止以
   `O_CREATE|O_EXCL` 建檔＋刪除探測**（污染 save root mtime、crash 留
   孤兒檔、語意不等於目錄寫權）。owner 不比較 original root（它不知
   original 路徑）；分離是 composition root 的責任，見邊界段。
4. **僅步驟 4**（設 `d.Root`→`m.LoadEXE`→`d.Install`）任一步錯：
   phase 轉 Failed、鎖存 firstFault（`fail` 語意：已 Failed 不覆寫、
   `Closed` 不改 phase；`owner.go:705-715`）、回錯；已設的 Root 不回滾
   亦不重用，後續 `Deliver`／`Advance` 零新步（`225-247`／`485-504`），
   `Close` 副作用一次（`578-594`）。`LoadEXE` 失敗在首次 `WriteBytes`
   前返回（`loader.go:201-206`），機器無半套映像；`Install` 現行簽名無
   回錯值，其錯分支為不可達防禦，須明寫。不得留下「裝了一半的 DOS
   可被 step」的狀態。
5. 全過：phase 轉 Running，`BootReceipt{Running, sha256(私有複本)}`。
   開機本身零 instruction step（`Machine.Steps` 不變）；epoch 語意不變，
   仍由 `acceptTurn` 推進。
6. 本方法與既有 `Deliver`／`Advance`／`Close` 同屬「唯一前端 goroutine」
   契約，不另加鎖；跨 goroutine 呼叫不在本規格驗收。

`BootInput` 按值傳但 `EXE` 是 slice 頭，有「先算 hash、後載入被改過
bytes」的 TOCTOU。**無條件禁止別名**：`BootOriginal` 必須先將
`input.EXE` 複製為私有複本，對複本算 SHA-256（比 `ExpectedEXESHA256`）
並以複本 `LoadEXE`；成功／失敗皆不保留呼叫端 slice，不設「以文件註記
代替不別名」的活門。呼叫端在返回前經任何別名修改 `EXE` 屬違約，
實作以「先複製」消除該類 TOCTOU；負例驗收須含 `-race`＋呼叫端在返回後
改寫原 slice、斷言機器映像不受影響。

## 失敗矩陣（實作審查用）

nil owner、非 Booting 二次開機、空 EXE、超大 EXE、零 hash、錯 hash、
空 SaveRoot、SaveRoot 是檔案、SaveRoot 是 symlink、SaveRoot 不存在、
SaveRoot 不可寫（含 NUL 路徑）、LoadEXE 壞 bytes、成功後再開機：
全部回錯；除步驟 4 的 Failed 轉換外 phase 不變且不經 `fail()`；
成功案 machine steps 零增加、phase Running、receipt hash 等於複本實算。
跨子測試以不同 Owner 隔離，不共用 machine。

## READY 前置

- 針對性複審確認本輪 5 處修改（別名無條件禁止、`Lstat`＋`Access`
  探測、`fail()` 僅步驟 4、mouse／keyboard 持有限定、`4MiB` 護欄註記）
  已落地，無新 P0。
- 合成測試（無原版 EXE：以偽 MZ／隨機 bytes 走全部拒絕路徑；成功路徑
  以最小可載入 EXE fixture 或延至原版驗收，審查時決定）。
- 本規格升 READY 不授權 observer 安裝、多層聚合、存讀檔或 Linux 視窗；
  那些仍是 Buck #16／#18 的 DRAFT。

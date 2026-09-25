# 236 — BootRoot composition 前檢（READY，限本契約範圍）

狀態：**READY。首輪審查四項必改＋production 審查殘留（`..` 逃逸真 bug、
兩處漏清、測試缺口：反向巢狀、別名、同根併發、端到端）全修並終審通過；
實作 `bootroot` 套件＋合成測試（含 Prepare→BootOriginal 端到端）
`go test -race -count=2` 全綠。升 READY 不授權 launcher、owner 接線、
存讀檔或視窗。**
日期：2026-09-25

## 缺口與邊界

規格 235 的 `session.BootOriginal` 只收 EXE bytes＋SHA＋save root；
「原版樹讀取、版本／雜湊核對、唯讀 original 與可寫 save root 的分離
與複製」留給 composition root。本規格收束的就是這塊：
`bootroot.Prepare` 把已存在的 original 目錄驗成可用的 save 目錄。
它不碰 machine／DOS／owner／視窗，不讀譯文／字型，不開網路；
Buck 檔名與期望雜湊由呼叫端（未來 launcher／CLI）以參數釘住，
本套件不內建任何遊戲版本斷言。

## 候選契約

```go
type RequiredFile struct {
    Name   string
    SHA256 [32]byte
}
type BootRootInput struct {
    OriginalRoot string
    SaveRoot     string
    Required     []RequiredFile
}
type BootRootOutput struct {
    SaveRoot string
    Verified map[string][32]byte
}

func Prepare(input BootRootInput) (BootRootOutput, error)
```

檔名規則與執行期對齊：`Required[].Name` 須是單一 basename——空、
`.`、`..`、含 `\/:` 任一者拒絕（同 `DOS.AllowFileWrites` 的
`dos.go:485` 規則；執行期 `baseName` 以 `` `\/:` `` 切段、`lookup`
大小寫不分，驗證層採嚴格名是刻意的 fail-closed）。

已知差異（非缺口）：驗證層大小寫嚴格，執行層 `lookup`
（`files.go:73`）大小寫不分＋8.3 截斷；original 樹內若有 fold 後
碰撞，驗證仍逐筆嚴格通過，執行期命中第一個——呼叫端應避免此類樹，
本規格不替執行期做碰撞檢查。

0. 路徑正規化：`OriginalRoot`／`SaveRoot` 先 `filepath.Abs`＋`Clean`；
   失敗即拒。輸出 `SaveRoot` 為正規化後者（執行期 `resolve` 每次重拼
   路徑，launcher 不可在 `Prepare` 後 `chdir`）。
1. `OriginalRoot` 經 `Lstat`：不存在／非目錄／為 symlink，一律拒絕。
2. `Required` 不可空，同 `Name`（位元組完全相等，大小寫敏感）重複即拒；每筆以
   上述 basename 規則檢查；目標須經 `Lstat` 為常規檔（含 NUL 等不可
   表示字元：底層回錯即拒絕）；整檔讀入算 SHA-256，不等即拒絕。
   單檔超過 `64<<20` 拒絕（原版樹實測最大檔約數百 KB；護欄防損毀樹，
   與 235 的 4MiB EXE 護欄層級不同，互不取代）。
3. 路徑互斥先於一切副作用：正規化後 `Rel(original, save)` 與
   `Rel(save, original)` 各判一次，任一不以 `..` 起頭即拒（save 不得
   位於 original 之內，反之亦然；此檢查不碰檔案系統，故巢狀 save
   不會先被建出來污染 original）。隨後 `SaveRoot` 經 `Lstat`：
   不存在→以 `Mkdir(saveRoot, 0700)` 建（僅建一層；父層缺失即錯，
   不遞迴；建目錄是本函式唯一的建立副作用）；存在則須為目錄且非
   symlink。再分別 `Lstat` 兩根取 `FileInfo`，`os.SameFile(FileInfo, FileInfo)`
   為真拒絕（含同目錄、symlink 別名、bind mount 同 inode）；
   `unix.Access(W_OK|X_OK)` 失敗拒絕；頂層 `ReadDir`
   非空（dotfile 計入）拒絕。
4. 全樹複製 original→save：只處理目錄與常規檔；樹內遇 symlink／fifo／
   socket／device 等非常規節點即停回錯（hardlink 以常規檔看待，複製時
   展開為獨立 bytes）；setuid／setgid／sticky 位一律丟棄（固定目錄
   `0700`、檔案 `0600`，`Mkdir`／`OpenFile` 後 `Chmod` 補回，不受 umask
   影響）；新檔一律 `O_CREATE|O_EXCL`（已存在即錯，不覆寫）；複製後逐
   檔重讀兩端比 hash。
5. 複製後對 save 內每筆 `Required` 重算 hash，比對期望值。全過回
   `BootRootOutput{正規化 SaveRoot, 新建 Verified map}`。
   `Verified` 是執行期腐敗檢出的診斷收據（非 EXE 檔執行期不驗 hash）；
   **EXE 交接另行**：launcher 另行指定 EXE 檔名並從 save 重讀 bytes
   餵給 235（235 會對該 bytes 重算 SHA），本規格不指定哪筆是 EXE。
6. 複製中途失敗：按「目錄樹深先」盡力刪除本次新建（含第 3 條所建的
   save 根，`RemoveAll` 語意但逐項忽略回錯），回原錯；殘留（刪不掉）
   即廢棄該目錄，GC 屬 launcher（重試一律用新目錄，不重用殘留）。
   複製是「全有或全無」的嘗試，不是交易保證。

前提與接受風險（明寫）：launcher 與遊戲同 uid（`0600`／`0700` 語意
所依）；`Lstat`→複製之間被換檔屬接受風險（本地非特權路徑，不承諾
對抗惡意併發寫者；併發塞檔只會轉成 `O_EXCL` 複製失敗，fail-closed）。

`Prepare` 是純函式：除上述檔案系統作用外無全域狀態，不開 goroutine；
同一行程併發呼叫不同 save root 須安全（`-race` 驗）。

## 失敗矩陣（實作審查用）

original 缺失／是檔案／是 symlink、Required 空／同名重複／壞 basename
（含 `.`、`:`、`..`、分隔符）／缺檔／錯 hash／超大檔／樹內 symlink、
save 缺失（建目錄；父層缺失即錯）／是檔案／是 symlink／與 original
同 `SameFile`（含經 symlink 指回）／互為巢狀／非空／不可寫、複製中途
IO 錯（至少驗新建內容被清掉且回錯）：全部回錯，Verified 為空；
除第 6 條盡力清理外不刪使用者既有檔案。成功案 Verified 與兩端重算
一致，save 樹與 original 逐檔同 bytes，輸出為正規化 SaveRoot。

## READY 前置

- 針對性複審確認本輪修改（Verified 錯句、EXE 交接、巢狀拒絕、
  SameFile 簽名、mkdir／GC 策略、同名拒絕、絕對化）已落地。
- 合成測試（無原版遊戲：全用 `t.TempDir` 小樹；成功路徑不依賴任何
  真實 EXE）。
- 本規格升 READY 不授權 launcher、owner 接線、存讀檔或視窗；那些仍是
  Buck #16 的後續切片。

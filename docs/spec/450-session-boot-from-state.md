# 450：session 從 probe 狀態檔啟動

狀態：**READY**（第 4 版），已實作（§5）
日期：2026-10-18
前置：[`235-sealed-owner-private-boot-draft`](235-sealed-owner-private-boot-draft.md)（`BootOriginal` 與 save root）、[`447-bios-int09-int15-keyboard-intercept`](447-bios-int09-int15-keyboard-intercept.md)、
[`448-session-config-legacy-pm-profile`](448-session-config-legacy-pm-profile.md)、[`449-session-irq1-scan-channel`](449-session-irq1-scan-channel.md)

dosgolem 的程式位置以 commit `4276829` 為準。推論等級：已證實、強推論、假說、未知。

---

## 1. 為什麼要做

`session.Owner` 只能以 `BootOriginal` 冷開機（`session/boot_original.go` 37 到 87 行）。《Strike Commander》的中文化覆繪要在基地、停機坪等畫面驗收，這些畫面在冷開機第 44 億步以後，中間要點擊與輸入姓名。`cmd/probe` 早已存好這些畫面的狀態檔（`strike_commander` 的 `workplace/scratch/p485/s449.state` 等），session 讀不了，驗收只能從冷開機重放輸入，一次十幾分鐘，路徑也脆弱。449 §2.7 把 session 讀檔列為範圍外，本規格補上。

## 2. 規則

### 2.1 入口

```go
type StateInput struct {
	State               []byte   // probe -save-state 的整個檔案內容
	ExpectedStateSHA256 [32]byte
	SaveRoot            string   // 同 BootInput.SaveRoot
	// RebaseFrom 是狀態檔裡開檔路徑的舊前綴（probe 的 -scratch 等），讀檔時改寫到 SaveRoot 底下。
	// 狀態檔內記的 Root 一律包含在內，不必重複給。
	RebaseFrom []string
}

func (o *Owner) BootFromState(input StateInput) (BootReceipt, error)
```

- 不收 EXE。probe 的 `-load-state` 路徑不呼叫 `LoadEXE`（`cmd/probe/main.go` 370 到 385 行都在 `if *loadState == ""` 內），程式映像、`FreeSeg`、arena 等全由狀態檔還原。狀態檔不記 EXE 身分，收 EXE 雜湊也證明不了配對。
- `BootReceipt` 另加 `StateSHA256 [32]byte`、`Steps uint64`（讀檔後的 `m.Steps`）、`LegacyState bool`（`m.LegacyState`，`internal/machine/machine.go` 533 行）。`BootFromState` 回傳的 `EXESHA256` 為零值；`BootOriginal` 回傳的三個新欄位為零值。
- `State` 照 235 的別名規則先複製成私有複本，對複本算雜湊、以複本讀檔，不保留呼叫端的 slice。「先複製」與 235 相同由 code review 關閉（`session/boot_original_test.go` 157 到 160 行的做法）。長度上限 64 MiB（壓縮後；`s449.state` 是 3.6 MB）。
- `internal/state` 新增 `LoadFrom(r io.Reader, m, d)`，`Load(path, …)` 改為開檔後呼叫它。`LoadFrom` 在 `gzip.NewReader` 失敗時回包著新 sentinel `state.ErrNotGzip` 的錯誤；`Load` 以 `fmt.Errorf("state: %s 不是 gzip：%w", path, err)` 包上路徑。沒有既有測試比對這段訊息。
- SaveRoot 的檢查抽成 `BootOriginal` 與 `BootFromState` 共用的 `checkSaveRoot`：拒絕空字串與含 NUL，再 `Lstat`、symlink、目錄、可寫（`boot_original.go` 54 到 69 行）。不做 `filepath.Clean`：檢查與使用的是同一個字串，`RebasePath` 的 `Join` 會自己整理結果。

### 2.2 開檔路徑改寫

`DOS.LoadState` 對每個還原的檔案代號直接 `os.Open(hs.Path)`（`internal/dos/state.go` 285 到 296 行），路徑是存檔時的絕對路徑。`s449.state` 有 4 個：`/orig/OPTTEST.EXE`、`/orig/LILTRE.TRE`、`/orig/BIGTRE.TRE`、`/orig/VOCLIST.TRE`（以 `zcat | strings` 核對）。session 不得開原版樹（235 的邊界），改寫在 DOS 層、開檔之前做。

**純函式** `dos.RebasePath(path string, prefixes []string, to string) (string, error)`：

1. `path` 與每個前綴先 `filepath.Clean`；空字串與 `.` 前綴略過。
2. 對每個前綴算 `rel, err := filepath.Rel(前綴, path)`；`err == nil`、`rel != "."`、`rel != ".."` 且 `rel` 不以 `".." + 分隔字元` 開頭時（`..x` 這類檔名照常接受），這個前綴符合（嚴格位於前綴底下；等於前綴不算，避免把目錄開成檔案代號；前綴為 `/` 時照樣成立）。
3. 取符合的前綴中最長的一個，回 `filepath.Join(to, rel)`。
4. 沒有前綴符合就回錯。

`Join` 的結果一定在 `to` 底下，不需要再做越界檢查。

**`DOS.LoadState`**：

- `dos.DOS` 新增欄位 `RebaseTo string`、`RebaseFrom []string`。
- `RebaseTo` 非空時，以前綴集合 `s.Root`（狀態檔的值）加 `RebaseFrom`，對每個 `hs.Path` 算出改寫後的路徑，並以 `os.Stat` 確認它是一般檔案；`hs.Size` 非 0 時大小也要相等（`create` 建的檔案代號與舊版狀態檔的 `Size` 可能是 0，這時不比）。這些全是讀取，放在 `checkXMSState` 之後、關閉舊檔案代號的迴圈（`internal/dos/state.go` 253 到 255 行）之前，也就在 `d.Drive, d.Dir, d.Now` 的賦值（256 行）之前；任何一項失敗就回錯，DOS 維持原狀。
- 開檔改用改寫後的路徑，`handle.path` 存改寫後的路徑，`d.Root` 設為 `RebaseTo`，不取 `s.Root`。
- `RebaseTo`、`RebaseFrom` 在 `LoadState` 返回前（成功或失敗）清空。`Machine.LoadState` 先失敗時 `DOS.LoadState` 不會被呼叫，所以 `BootFromState` 在 `LoadFrom` 返回後也清一次。
- `RebaseTo` 為空時行為與現在相同（probe、oracle 不受影響）。

存檔那次執行寫進 scratch 層的檔案不在 SaveRoot 裡，要由呼叫端事先複製進 SaveRoot（§4）。

### 2.3 順序

照 `cmd/probe` 的 `-load-state` 路徑（`main.go` 502 行 `d.Install()`、504 行 `state.Load`、532 行 `configureKbdInt15`）：

1. 前置檢查，不改 phase：nil owner、phase 非 Booting、`generation` 溢位、`State` 為空或超過上限、`ExpectedStateSHA256` 全零、複本雜湊不符、`checkSaveRoot`、`Config.ClockPercent` 非零（§2.4）。
2. `o.dos.Root = o.dos.Scratch = SaveRoot`，`o.dos.RebaseTo = SaveRoot`，`o.dos.RebaseFrom` 設為 `input.RebaseFrom` 的複本。
3. `o.dos.Install()`。`state.Load` 的前提是 Install 已跑過（`internal/state/state.go` 74 到 77 行註解）；讀檔後再跑 Install 會重設 `freeSeg`、arena、`curPSP`（`internal/dos/dos.go` 726 到 733 行），所以不能用 `startLoadedMachine`。
4. `state.LoadFrom(bytes.NewReader(複本), o.machine, o.dos)`。`DOS.LoadState` 不改 `d.Scratch`，第 2 步的值保留。
5. 讀檔後 `o.dos.Mouse.Buttons != 0` 就回錯：狀態檔存於按鍵按住時，`host.MouseBridge` 剛建好、不知道有鍵按著，放開永遠不會送出。
6. 鍵盤攔截：`m.KbdIntercept`（`New` 依 `Config.KbdIntercept` 設定，`LoadState` 不改它）為真時再呼叫一次 `m.EnableKbdIntercept()`（狀態檔覆蓋了 stub 區與 `KbdInterceptOff` 的客體碼，447 §2.1）。為假時呼叫新的 `m.CheckKbdInterceptAbsent()`：向量 9 的 stub 是 `KbdInterceptStub9()` 就回錯。這個檢查從 `cmd/probe/kbdint15.go` 的 `configureKbdInt15` 關閉分支搬進 `machine`，probe 改呼叫它。
7. `phase = Running`、`generation++`。

第 2 到 6 步任一失敗走 `o.fail`，owner 進 `PhaseFailed`（`fail` 會關閉 DOS 與已開的檔案代號），呼叫端要建新的 owner 重來。

`BootFromState` 在第 1 步做與 `startLoadedMachine` 相同的三項護欄（nil、phase、`generation` 溢位，`session/owner.go` 840 到 848 行）；三項都只回錯、不呼叫 `fail`，與 235 第 1 點「前置檢查不改 phase」一致（`startLoadedMachine` 對後兩項會 `fail`，那是它被呼叫時已過前置檢查的語意）。`startLoadedMachine` 不動。

235 規定開機本身零 instruction step；`BootFromState` 完成時 `m.Steps` 是狀態檔的步數，這是本入口的明文例外，由 `BootReceipt.Steps` 記下。

### 2.4 讀檔後的選項

| 項目 | 處理 |
|---|---|
| 延伸記憶體大小、`NoEMS`、PM profile、`SBDMA8` | `Machine.LoadState` 比對，不符回錯（`internal/machine/state.go` 266 到 299 行） |
| `SoundBlaster`（單獨）、`AdLib`、`Mode13Phase`、`ExecEnvCopy`、`ExecReclaim` | 不驗也不重套；呼叫端負責與存檔時一致 |
| `DOSFiles` | 只在程式載入時寫 SFT（`internal/machine/sft.go` 44 到 70 行），讀檔後沒有作用；SFT 內容來自狀態檔。448 §3.2 的 `DOSFiles: 30` 照給，與 probe543 讀檔時沒給 `-dos-files` 等價 |
| `ProgramPath` | 讀檔後由狀態檔的 `m.ProgramPath`、`d.Dir`、`d.RootDir` 取代 |
| `ClockPercent` | `Owner` 新增私有欄位記下 `Config.ClockPercent` 原值；非零時第 1 步就拒絕，包括明示的 100。讀檔會依存檔版本改寫 `VGAFrameEvery`、`KeyEvery` 並重設時脈比例（369 到 380 行），與 `New` 套的值互相覆蓋，語意未定。`m.ClockPercent()` 對 0 與 100 都回 100（`clock_scale.go` 51 到 56 行），不能拿來判斷 |
| 滑鼠欄位（含 `XScale`） | 由狀態檔決定（`internal/dos/state.go` 265 到 275 行） |
| 按住狀態 | `Machine.LoadState` 在提交點清空（449 §2.2） |

### 2.5 範圍外

- 存狀態檔（`SaveState`）。
- 狀態檔的版本相容性，沿用 `Machine.LoadState`、`DOS.LoadState` 現有的檢查。
- 寫入型檔案代號的還原（§4）。

## 3. 驗收

### 3.1 `internal/dos` 套件測試

1. `RebasePath` 表驅動：
   - 正常改寫；子目錄；最長前綴（`/a` 與 `/a/save` 同時給，`/a/save/X` 取後者）。
   - 兄弟目錄 `/orig2/X` 不符合 `/orig`。
   - `path` 等於前綴回錯。
   - `path` 含 `..`（`/orig/../etc/x`）先 Clean 再比對，不符合 `/orig`。
   - 重複分隔字元、尾斜線、前綴為 `/`、空字串與 `.` 前綴略過。
   - 沒有前綴符合回錯。
2. `LoadState` 改寫成功：存檔端 `DOS` 的 Root 是暫存目錄 A、Scratch 是 B，A 下開 `X`、B 下開 `Z`，各 seek 到中段後 `SaveState`。讀檔端是另一個 `DOS`，`RebaseTo` 是 C（C 內放 `X`、`Z` 的複本，內容與 A、B 的原檔不同、大小相同）、`RebaseFrom` 是 `[B]`。讀檔後兩個 `handle.path` 都在 C 下，從檔案代號讀到的是 C 內複本同位置之後的位元組，`d.Root == C`，`RebaseTo` 與 `RebaseFrom` 已清空。
3. `LoadState` 改寫失敗時維持原狀：讀檔端事先開著一個檔、`Drive`、`Dir`、`Root` 都與狀態檔不同。分別餵「`RebaseFrom` 漏給 B」「C 內 `X` 的大小不同」「C 內沒有 `Z`」三種情形，各自回錯；之後舊檔案代號仍可 seek 與讀，`SaveState` 經 `StateDigest` 與呼叫前相同，`RebaseTo`、`RebaseFrom` 已清空。
4. `RebaseTo` 為空時，既有的 `LoadState` 測試結果不變。

### 3.2 `session` 套件測試

存檔端用 `machine.New`、`dos.New` 載入合成程式，`m.RunUntil(nil, N)` 後 `state.Save`。合成程式依序做前半、執行一個計數 K 次的空轉迴圈、再做後半；N 選在迴圈中段，讀檔端接著跑就會離開迴圈做後半。合成程式把每次 DOS 呼叫的結果（`AX` 與進位旗標）寫進固定位址，測試以 `o.machine.Peek8` 讀回。`Digest` 抽成 `digestOf(m, d)`，`Owner.Digest` 與存檔端共用。讀檔端用 `session.New`（`Config` 帶 `InitialLayout`，`Epoch` 非 0）加 `BootFromState`，每次 `Advance` 之前 `View`、`Deliver` 空 update。

1. 前置檢查：`State` 為空、超過上限、`ExpectedStateSHA256` 全零、雜湊不符、`ClockPercent` 為 50 與 100、非 Booting 的 owner，以及 SaveRoot 為空、含 NUL、不存在、是 symlink、是檔案、不可寫，各自回錯，phase 不變。雜湊不符的案例用合法的狀態檔配錯的期望值，並斷言機器未被改動（`o.machine.Steps` 與記憶體雜湊與呼叫前相同）。
2. 讀檔失敗：合法 gzip 但內容截斷；延伸記憶體、`NoEMS`、PM profile、`SBDMA8` 各與存檔時不同；各自回錯，`Status().Phase == PhaseFailed`。
3. 往返與收據（存檔端的 Root、Scratch 與讀檔端的 SaveRoot 是同一個暫存目錄，所以兩邊的 `dos` 狀態可以逐欄比較）：`BootReceipt.Steps == N`、`StateSHA256` 等於檔案雜湊、`EXESHA256` 為零值、`LegacyState` 為假；觀察器第一次 `BeforeStep` 的 `Steps == N`；兩邊再各跑 M 步後 `digestOf` 的 Memory、CPU、Indexed、Palette 全同，`dos` 的 `StateDigest` 也相同；`BootOriginal` 收據的三個新欄位為零值。
4. Root 與 SaveRoot（另一組 fixture，不比 `StateDigest`）：存檔時 Root 是 A；A 底下另有一個存檔前沒開過、也不複製進 SaveRoot 的 `V`。讀檔後合成程式以檔名開 `V` 必須失敗（`AX = 2`）；開只存在 SaveRoot 的 `W` 必須成功。
5. 滑鼠：存檔時 `Mouse.Buttons != 0` 的狀態檔回錯。
6. 鍵盤攔截：`KbdIntercept` 開，讀入選項關閉時存的狀態檔，讀檔後向量 9 的 stub（3 bytes）與 `KbdInterceptOff` 的客體碼（14 bytes）都等於 `EnableKbdIntercept` 寫的內容；`KbdIntercept` 關，讀入選項開啟時存的狀態檔，回錯。probe 的 `-kbd-int15` 既有測試（`cmd/probe/kbdint15_test.go`）在改呼叫 `CheckKbdInterceptAbsent` 後結果不變。
7. `BootOriginal` 的既有測試、235 的失敗矩陣與 448 的零值錨點不變。

### 3.3 負對照

各關一次，對應測試必須失敗。負對照列由實作子代理逐列執行；執行後證明是等價突變的列，在 §5 記錄理由並刪除或改寫，不因此退回 DRAFT（同 449 §5.2 的處置）。

| 關掉 | 必須失敗 |
|---|---|
| `RebasePath` 不先 Clean | §3.1 第 1 項的 `..` 案例 |
| `RebasePath` 改用字串前綴（不經 `Rel`） | §3.1 第 1 項的兄弟目錄 |
| `RebasePath` 取第一個符合而非最長 | §3.1 第 1 項的最長前綴 |
| `RebasePath` 接受等於前綴 | §3.1 第 1 項 |
| `LoadState` 不改寫、開原路徑 | §3.1 第 2 項（讀到 A、B 原檔的內容） |
| `handle.path` 存原路徑 | §3.1 第 2 項 |
| 驗證移到 `d.Drive, d.Dir, d.Now` 賦值之後 | §3.1 第 3 項 |
| 不比對檔案大小 | §3.1 第 3 項的大小不同 |
| 讀檔後 `d.Root` 取狀態檔的值 | §3.2 第 4 項的 `V` |
| 不檢查 `Mouse.Buttons` | §3.2 第 5 項 |
| 不做第 6 步的重寫 | §3.2 第 6 項的開啟情形 |
| 不呼叫 `CheckKbdInterceptAbsent` | §3.2 第 6 項的關閉情形 |
| 雜湊在 `LoadFrom` 之後才驗 | §3.2 第 1 項的機器未改動斷言 |
| 不檢查 `ClockPercent` | §3.2 第 1 項 |

### 3.4 觸發案例（主代理執行，結果寫進 §5）

**輸入**：

- `Config` 同 448 §3.2：`KbdIntercept` 為真，`InitialScale`、`InitialLayout`（`Epoch` 非 0）照 448 給。
- `State` 是 `strike_commander` 的 `workplace/scratch/p485/s449.state`（SHA-256 `7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0`，第 4490000000 步，基地畫面，熱點文字 "Talk to Gwen"）。
- `SaveRoot` 同 448 的複製樹；`RebaseFrom: []string{"/scratch"}`（狀態檔的 Root 是 `/orig`，已自動包含）。狀態檔的 4 個檔案代號都在 `/orig` 下，`/scratch` 前綴在這份資料上用不到，收據要註明，並記 SaveRoot 內這 4 個檔的 SHA-256。

**步驟**：

1. `BootFromState`，斷言 `BootReceipt.Steps == 4490000000`。
2. `View()` → `Deliver(空 update)` → `Advance(400000)`，斷言 `MachineStepsAfter == 4490400000`，取 `Digest()`。
3. 再 `View()` → `Deliver(空 update)` → `Advance(1)`，以 448 的觀察器記 `Steps == 4490400000` 的暫存器。

空 update 的寫法同 448 §3.2。

**期望值**（`cmd/probe` 跑同一段，`probe543.log`）：

```text
psc.sh p530 probe543 -kbd-int15 -load-state /scratch/s449.state -steps 4490400000 -shots 4490399999:/scratch/g.png
```

| 斷言 | 期望 |
|---|---|
| 第 4490400000 步 `IndexedSHA256` | `b26228b4a0ade66ba4e2d9ccd368d0c2add297653396bf3341c5ea3fcbe13883`（`p530/f.png`，副檔名是 png，內容是 64000 bytes 的原始色號，即 `m.Indexed()`；`probe533` 在 `m.Steps == 4490400000` 時存；`probe543` 在第 4490399999 步存的 `g.png` 雜湊相同，畫面靜止） |
| 第 4490400000 步 `PaletteSHA256` | `f7816e01a78377ade14d76015e51cb1d081a12158d69fd979fc55cc7ba3a2db0`（`f.pal`，與 `g.pal` 相同） |
| 第 4490400000 步暫存器 | CS:IP `0CF2:0C07`，AX、BX、CX、DX 皆 `0000`，DS `4E66`，ES `4E66`，SS:SP `4E66:FC46`（`probe543.log` 終態） |

`probe543.log` 的「開過的檔 0 個」只算這次執行期間新開的檔。狀態檔還原的 4 個檔案代號在讀檔時以 §2.2 改寫後開啟，改寫或大小比對錯誤會讓 `BootFromState` 失敗。

**負對照**：不做第 6 步跑一次，只記錄斷言是否改變。`probe543.log` 顯示這段 `KeyIRQs=0`、`KbdIntercepts=0`，預期不變，第 6 步的鑑別由 §3.2 第 6 項負責。

**執行環境**：長測試，以環境變數 `DOSGOLEM_SC_ROOT` 指向 448 的 SB16 設定樹、`DOSGOLEM_SC_STATE` 指向狀態檔時才跑，否則 `t.Skip`。測試把設定樹實體複製到 `t.TempDir()` 當 SaveRoot。在 Docker 內執行（`--rm`、`--memory`、`--cpus`、`--network none`），設定樹與狀態檔唯讀掛載。

## 4. 已知限制

- probe 與 session 讀同一個狀態檔後的主迴圈差異同 448 §4。
- 存檔那次執行寫進 scratch 層的檔案要由呼叫端複製進 SaveRoot；大小比對只擋得住大小不同的複本。
- 改寫結果恆以 SaveRoot 為根（`Join`），呼叫端給 `RebaseFrom: ["/"]` 也不會逸出；SaveRoot 內的 symlink 不在檢查範圍，與 235 相同。
- 狀態檔的 Root 是相對路徑（`.` 或空字串）時前綴被略過，這類狀態檔要靠 `RebaseFrom` 給絕對前綴，否則讀不了。
- 寫入型檔案代號：存檔時以 `O_RDWR` 開著的檔（`files.go` 262 到 273 行），`DOS.LoadState` 以 `os.Open` 唯讀重開且 `writable` 為假（`state.go` 287、295 行），之後的寫入只記帳不落地（`files.go` 643 到 655 行）。這是 probe 與 oracle 共有的現行行為，本規格不修；狀態檔要避免在遊戲開檔寫入期間存。
- `SoundBlaster`、`AdLib`、`Mode13Phase`、`ExecEnvCopy`、`ExecReclaim` 與存檔時不同不會被偵測（§2.4）。
- `LegacyState` 為真的路徑沒有單元測試（需要舊版 v2 狀態檔）。
- 觀察器只看得到讀檔之後的事件：讀檔當下畫面上已有的內容不會以 `VideoWrite` 重播，OPL 暫存器不在狀態檔裡（讀檔後音源從空白開始）。靠印字事件辨識畫面文字的覆繪要等遊戲重畫；OPTTEST 每畫格重畫熱點標籤，不受影響。
- 待送的滑鼠回呼（`QueueCallback`）不在狀態檔裡，是 probe 就有的現況。
- 狀態檔含原版的整份記憶體，`State` 位元組與存檔都不得散布（同 `oracle.LoadStateFile` 的註記）。

## 5. 實作收據（2026-10-18）

實作 commit 見本收據的上一個 commit。實作由子代理完成，測試由主代理在 Docker 內重跑。

### 5.1 單元測試與回歸

- `go vet ./...` 通過；`session`、`internal/dos`、`internal/state`、`internal/machine`、`cmd/probe`、`oracle` 的 `go test -count=1` 全過（主代理重跑）。
- 測試：`internal/dos` 的 `TestRebasePath`（19 列）、`TestLoadStateRebasesHandles`、`TestLoadStateRebaseFailureKeepsDOS`、`TestLoadStateWithoutRebaseKeepsPaths`；`internal/state` 的 `TestLoadFromMatchesLoad`、`TestNotGzipSentinel`；`session` 的 `TestBootFromStateRejectsWithoutFailing`、`TestBootFromStateLoadFailureFailsOwner`、`TestBootFromStateRoundTrip`、`TestBootFromStateRootIsSaveRoot`、`TestBootFromStateRejectsHeldMouseButton`、`TestBootFromStateKbdIntercept`。
- 回歸：改動前（`git archive 319739f` 的乾淨樹）與改動後的 `go test -count=1 ./...` 套件結果相同（42 ok、3 FAIL，3 個是容器沒有顯示器的 ebiten 套件，改動前就失敗）。

### 5.2 負對照

§3.3 的 14 列由子代理逐列執行（輸出在 `strike_commander` 的 `workplace/i450/neg/`），13 列照預期使對應測試失敗。處置：

- 「`RebasePath` 不先 Clean」對 `..` 案例是等價突變：`filepath.Rel` 內部會先 Clean 兩個參數，`/orig/../etc/x` 有沒有先 Clean 都不符合 `/orig`。這一列改由同一張表的兩個案例鑑別，突變時都失敗：前綴含重複分隔字元時的最長前綴比較、空字串前綴的略過。
- 「驗證移到賦值之後」使 §3.1 第 3 項失敗的機制是舊檔案代號已被關閉、`SaveState` 回錯，不是摘要值不同；仍是同一項失敗。

主代理另在複製樹上把 `BootFromState` 的 `RebaseTo` 設為空字串，§3.4 的 `Base` 失敗於「還原時開不了 /orig/OPTTEST.EXE」。

### 5.3 觸發案例

`TestBootFromStateTrigger/Base` 通過（0.9 秒）。SaveRoot 由 `strike_commander` 的 `workplace/root` 實體複製，4 個還原檔案的 SHA-256：`OPTTEST.EXE` `e8c47cfe…a1f1`、`LILTRE.TRE` `ac1ae00c…78de`、`BIGTRE.TRE` `96c2ba71…0594`、`VOCLIST.TRE` `7301c330…2785`。`RebaseFrom: ["/scratch"]` 在這份資料上沒有用到（4 個檔案代號都在 `/orig`）。

| 斷言 | 結果 |
|---|---|
| `BootReceipt.Steps` | 4490000000，`LegacyState` 假，`StateSHA256` `7dff6dd2…47a0` |
| 第 4490400000 步 Indexed、Palette | `b26228b4…3883`、`f7816e01…2db0`，相同 |
| 第 4490400000 步暫存器 | `0CF2:0C07`，AX、BX、CX、DX `0000`，DS、ES `4E66`，SS:SP `4E66:FC46`，相同 |

負對照 `NoKbdRewrite`：步數、Indexed、Palette、暫存器都未改變，與 §3.4 的預期相同（這段 `KeyIRQs=0`）。

### 5.4 實作期間的處置

- `Load` 的錯誤訊息照規格包上路徑，字面會重複「不是 gzip」（sentinel 本身含這四個字）。
- 長測試負對照的開關是未匯出欄位 `Owner.noKbdRewrite`，沒有新增公開 API。

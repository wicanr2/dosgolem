# 241 — buckrogers-play 的輸入：遊戲鍵、滑鼠與前端功能鍵

狀態：**READY**（2026-09-27，兩輪獨立審查後）
日期：2026-09-27
前置：[`008-bios-keyboard-injection`](008-bios-keyboard-injection.md)（BIOS 按鍵字組）、
[`185-keyboard-trace-and-keypad-names`](185-keyboard-trace-and-keypad-names.md)（數字鍵盤名稱）、
[`228-host-mouse-bridge-ready-candidate`](228-host-mouse-bridge-ready-candidate.md)（主機滑鼠橋）、
[`238-session-step-observer`](238-session-step-observer-draft.md)、
[`240-opl-audio-output`](240-opl-audio-output-draft.md)（靜音開關作用的對象）。

## 1. 為什麼要做

`cmd/buckrogers-play` 目前只送可列印字元與 15 個具名鍵，每次按下只送一次，沒有功能鍵、Ctrl 組合鍵、
連發，也完全沒有接滑鼠。發行版要讓玩家用鍵盤與滑鼠正常遊玩，並有說明、倍率、靜音、全螢幕、截圖等
前端功能。

## 2. 證據與未知

- 前端現況（已證實，程式）：`hostKeys` 用 `ebiten.AppendInputChars` 取字元、`inpututil.AppendJustPressedKeys`
  取具名鍵；F2 由前端消耗切換 2×／3×；PgUp／PgDn 在手札面板開啟時由前端消耗（Buck 規格 030）。
  `session.Config.InitialLayout` 固定為 2×、上方留 36 像素 chrome，但視窗 `Layout` 回 `320×s, 200×s`，
  沒有 chrome；`CapturedUpdate` 從不帶 `PointerDown`／`PointerUp`。
- session 的滑鼠路由（已證實，程式）：只接受左鍵 Down／Up 與 FocusLost；`host.PlanMouseRoute` 把事件座標換成
  DOS 座標 `(X/scale, (Y-chrome)/scale)`；panel 關閉時右鍵以 `non-left-rejected` 拒絕（panel 開啟時為 `panel-open-consumed`；本前端不開 session panel）；沒有「只移動」事件。
- BIOS 掃描碼（已證實，IBM PC/AT 技術參考，set 1 經 BIOS 轉譯）：F1–F10 為 3Bh–44h、ASCII 0；
  Ctrl＋字母為字母掃描碼、ASCII 為字母碼 & 1Fh；Alt＋字母為字母掃描碼、ASCII 0；
  數字鍵盤在 NumLock 關閉時為 47h–53h、ASCII 0（`dos.namedKeys` 的 `KP*` 已有）。
- 原版會用哪些鍵：**未知**。本規格不依原版挑鍵，而是把標準 BIOS 字組如實送進去；原版不認得的鍵由原版自己忽略。
  F1–F3 由前端保留（§3.3），原版若使用它們，玩家無法觸發：列為已知取捨，於說明頁載明。
- 原版是否呼叫 `int 33h`、在哪些畫面接受滑鼠：**未量**。驗收（§4）以冷開機量測並記錄。
- Shift／Ctrl／Alt 旗標（已證實，程式）：`int 16h AH=02h／12h` 一律回 `AL=0`（`internal/dos/bios.go` 取旗標狀態分支），
  BDA `0040:0017` 不模擬，屬規格 008 的既定範圍。原版若靠旗標判斷「Ctrl 按著」，§3.1 的 Ctrl 字組不會生效：**未知**，
  驗收如實記錄。
- 游標（已證實，程式）：dosgolem 的 `int 33h AX=1／2` 只收下、不畫游標。原版冷開機路徑呼叫 `int 33h` 的
  `0000、0001、0002、0003、0004、0009`（Buck repo `docs/re/phase-1-input-and-startup.md`，已證實，量測）：
  它請驅動顯示並設定圖形游標形狀（強推論：依賴驅動畫游標），因此畫面上不會有遊戲游標。
- 本規格是規格 228 主機滑鼠橋第一次接上真實 ebiten 前端（228 的收據來自獨立 harness，且明文不授權接正式前端），
  屬新的驗證範圍。
- F3 靜音作用在規格 240 §3.5 的前端播放；該播放與本規格同一批實作（`cmd/buckrogers-play` 的 `mixer.Ring` 與
  ebiten `audio.Player`）。

## 3. 契約

### 3.1 遊戲鍵（送進原版）

- 名稱表（`internal/dos/scancode.go`）新增：`F1`–`F10`（3Bh–44h, 0）。另新增函式
  `KeyCtrl(letter byte) (Key, bool)`（字母掃描碼, letter&1Fh）與 `KeyAlt(letter byte) (Key, bool)`（字母掃描碼, 0），
  只接受 A–Z；其他回 false。不新增猜測的鍵。
- 前端對映：
  - 可列印字元：沿用 `AppendInputChars` → `KeyForRune`（字元已含作業系統的連發）。
  - 具名鍵：沿用現有 15 個；加上 F4–F10、數字鍵盤 0–9 與小數點（一律送 `KP*` 方向字組，不論主機 NumLock），
    數字鍵盤 Enter 送 `Enter`。按下數字鍵盤鍵的同一畫格，主機產生的數字與 `.` 字元丟棄，避免一鍵兩送。
  - Ctrl＋字母送 `KeyCtrl`，Alt＋字母送 `KeyAlt`；此時同一畫格的主機字元丟棄。
- 連發：具名鍵與組合鍵按住 30 畫格（0.5 秒）後，每 6 畫格（約 10 次／秒）再送一次，接近 BIOS 預設
  typematic（500 ms、10.9 cps）。可列印字元不另做連發（作業系統已連發）。
- 同一畫格送出的順序：先字元、後具名鍵，與現況一致。

### 3.2 滑鼠（送進原版）

- 前端把視窗游標座標（`ebiten.CursorPosition`，已是 `Layout` 的邏輯座標，含全螢幕縮放）除以目前顯示倍率，
  得到 DOS 座標 `(dx, dy)`；範圍外視為 `MouseTargetOutside`。
- 送進 session 時換成 session 固定版面的座標：`X = dx×2`、`Y = 36 + dy×2`（`InitialLayout` 是 2×、chrome 36），
  Target 為 `MouseTargetCanvas`。顯示倍率（F2）只影響前端換算，不改 session 版面。
- 左鍵按下的畫格送 `PointerDown`、放開的畫格送 `PointerUp`；同一畫格兩者都有時依序送。
  視窗失焦（`ebiten.IsFocused` 由真變假）送 `FocusLost`。
- 右鍵、滾輪與「只移動」：session 不支援，不送（已知取捨）。DOS 游標位置依規格 228：按下時 Move→Press；
  在畫布內放開時 Move→Release；放開在畫布外、chrome 或失焦時只 Release、不移動。
- 系統游標：依上項證據保留系統游標（它是玩家唯一看得到的游標），不隱藏。端到端截圖若發現原版自畫游標，
  改為在畫布上隱藏系統游標並記入 Buck repo `docs/re/`。
- 說明頁開啟時不送滑鼠與遊戲鍵。

### 3.3 前端功能鍵（原版看不到）

| 鍵 | 功能 |
|---|---|
| F1 | 開關說明頁 |
| F2 | 2× ↔ 3×（現有） |
| F3 | 靜音開關（規格 240 的播放音量 0／1；不影響機器；控制點是 `game` 持有的 `audio.Player`） |
| F11 | 全螢幕開關（`ebiten.SetFullscreen`） |
| F12 | 截圖：把目前合成畫面存成 PNG |

- 衝突時前端優先：F1–F3、F11、F12 一律不送進原版；PgUp／PgDn 維持 Buck 規格 030 的規則。
- 說明頁：開啟時暫停。暫停的畫格 **`owner.View`、`Deliver`、`Advance` 三者一起跳過**（session 規定 `Deliver` 之後
  必須 `Advance` 才能再 `Deliver`，只跳過一邊會變成前端故障）；音訊不產生樣本（播放端補 0）。畫面為目前合成畫面上覆一層半透明底與說明文字；
  關閉後恢復。說明文字列出上表、遊戲鍵說明（方向鍵與數字鍵盤、Enter、Esc、字母快捷鍵）、
  滑鼠只用左鍵，以及「F1–F3、F11、F12 由前端使用，不會送進遊戲」。
- 說明頁文字屬玩家可見譯文：放在 Buck repo 的 `text/host-ui.zh-TW.tsv`（新 key，`source` 欄為
  `frontend-help`），字型子集照常由正式譯文重建。`LoadLiveRuntime` 讀取並驗證所有字元有字模；
  缺字即載入失敗。
- 截圖存到 `-shot-dir`（預設為存檔目錄的上一層下的 `screenshots/`），檔名 `buckrogers-YYYYMMDD-HHMMSS-NNN.png`；
  目錄不存在就建立；寫檔失敗只在視窗標題列顯示失敗，不中止遊戲。
- 自動模式（`-frames` > 0）不處理主機輸入，行為不變。

### 3.4 自動模式腳本

- 既有語法 `畫格:鍵[,…]`，鍵名新增 `F1`…`F10`（直接送進原版，不經前端保留規則）、`Ctrl+X`、`Alt+X`（X 為 A–Z）。
- 新增 `畫格:click@x;y`：x、y 為 DOS 座標（0–319、0–199，十進位），以分號分隔（逗號已用來分隔腳本項目）。在該畫格送 `PointerDown`，第 `畫格+6` 格送
  `PointerUp`，兩者座標相同、Target 為 Canvas，座標依 §3.2 換成 session 版面。按住的 6 格讓原版輪詢看得到按下。
  同一個按下尚未放開時再出現 click 為腳本錯誤。
- 新增 `畫格:press@x;y` 與 `畫格:release@x;y`：分開送按下與放開，x、y 可超出 0–319／0–199（超出即 Target
  Outside），用來驗證畫布外放開。
- 新增 `畫格:blur`：送 `FocusLost`。
- 新增 `畫格:help`：切換說明頁，用來在自動模式驗證 §3.3 的暫停。

## 4. 驗收

1. 單元測試：`F1`–`F10`、`KeyCtrl`、`KeyAlt` 的字組；前端對映函式（把「這一畫格按下／按住的鍵與字元」轉成
   BIOS 鍵序列）對：數字鍵盤丟重複字元、Ctrl 丟字元、連發時點（第 30、36、42 畫格）、前端保留鍵不送、
   說明頁開啟時全部不送；滑鼠座標換算（2×、3× 的四角、邊界內外一像素、負座標），與按下在畫布內、放開在畫布外，
   以及按下後失焦的事件序列。對映函式不依賴 ebiten，可在無頭測試。腳本解析（§3.4）的正反例。
2. 端到端：以自動模式腳本從冷開機跑到可見主選單，以 `click@x;y` 點選一個選項並截圖確認進入下一畫面；
   量測期間 `int 33h` 各功能的呼叫數（含 AX=1）並記錄於 Buck repo 的 `docs/re/`。若原版在該畫面不接受滑鼠，
   記錄並改測一個接受滑鼠的畫面；兩者都無則如實記錄。另跑兩個邊界情境：畫布內按下、畫布外放開（`press`／`release`）；
   畫布內按下後 `blur`。兩者之後原版不再看到左鍵按住（`int 33h AX=3` 的 BX 回 0），session 仍在 Running。另以 `help` 開兩個以上畫格再關閉，確認 session 仍在
   Running、步數在暫停期間不變。
3. 互動（使用者實機確認）：F1 說明頁、F3 靜音、F11 全螢幕、F12 截圖；Ctrl＋字母、Alt＋字母、數字鍵盤去重與連發；
   滑鼠點選與拖出視窗後放開。
4. 既有 receipt 與端到端路徑（phase-278）不受影響：`hostKeys` 以外的路徑不改。

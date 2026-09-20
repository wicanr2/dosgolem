# Buck Rogers 選定種族後的文字事件收據

狀態：**CONFORMED**

本規格核准以既有 `cmd/buckrogers-text-receipt` 從固定 `PICK RACE` state 排入兩次正常
BIOS Enter，建立選定預設種族後下一畫面的 content-safe 文字事件與 indexed framebuffer
收據。它另核准一個通用 `-receipt-out PATH` 選項，使命令在所有驗收閘門通過後，把與 stdout
逐 byte 相同的 JSON 寫到明示路徑。它不翻譯、不繪中文、不改原版狀態或角色建立規則。

## 輸入、版本與位址空間

- state：`after-bios-space-100m.state`，SHA-256
  `cfe15d3c66c9fe3c2e684815740a0cc0165e59d08ab5866370608d49f8a8e164`，起點
  #99,999,999。
- `START.EXE`／`GAME.OVR` SHA-256：
  `58a34a38b1db455202d2d30daa82915982d7d905932b46bdc7371cb466226cf1`／
  `3a4ad4856c08fe5973179f1d907feed1d870af99d08abd1cb884b316324f3cc0`。
- 工具基線為 dosgolem commit `64b15779edc9d5be35e1acba0f854ba022008511`。
- dispatcher `0763:0424`、caller、`SS:SP` 與 guarded return 均採 dosgolem runtime
  `segment:offset`；不是 IDA 線性位址或檔案偏移。
- 原版檔案與 state 唯讀；輸出只有 content-safe JSON 與 64,000-byte indexed framebuffer，
  均留在呼叫端被忽略的 `workplace/`。

## 固定排程與事件契約

- #100,010,000 排入第一個 BIOS Enter，正常進入 `PICK RACE`。
- #100,240,000 排入第二個 BIOS Enter，選定當下預設種族。
- 精確停止於 #101,000,000；兩筆鍵都必須成功排入。
- 收據恰有 14 個完成事件、零 pending、零 drop。前九筆必須保持既有功能選單→種族選單
  identity；第十筆是選定前 normal redraw；後四筆是下一畫面。

後四筆已由一次可丟棄 probe 直接量得，規格固定完整 identity：

| entry → post-call | caller | len／SHA-256 | bg/fg | row,col | 已證實內容角色 |
| --- | --- | --- | --- | --- | --- |
| 100250787 → 100259380 | `37F1:158C` | 11／`d6359f6f…6a91e` | 0/13 | 2,1 | 畫面提示 |
| 100277753 → 100282556 | `37F1:15BD` | 6／`b261705c…8116d` | 0/10 | 3,1 | normal 第一選項 |
| 100304303 → 100310629 | `37F1:15BD` | 8／`8e30eb57…08183` | 0/10 | 4,1 | normal 第二選項 |
| 100331347 → 100334602 | `37F1:175D` | 4／`03f8c127…f73a1` | 15/0 | 3,3 | selected 第一選項 |

probe 以原版 bytes 與候選雜湊核對四筆分別為性別選擇提示、含兩格縮排的第一／第二選項，
以及 selected 第一選項；repo 與正式收據不保存這四段原文全文。上述畫面角色與 identity
是**已證實**；其後性別選擇生命週期、不同種族分支及再下一畫面仍是**未知**。

## `-receipt-out` 契約

- 空值維持既有 stdout-only 行為；非空時在事件數、請求數、鍵排程、pending／drop 等既有
  閘門全數通過後才寫檔。
- JSON 只 encode 一次並以單一換行結尾；stdout 與檔案必須使用同一 byte slice。
- 建立／截斷／寫入／關閉任一步失敗都必須使命令非零退出；不得留下成功宣稱。
- 選項不得改 JSON schema、事件順序、framebuffer 或 stdout bytes。

## 驗收與停止線

- unit test 覆蓋 stdout／檔案同 bytes 的 encoder helper，以及無效輸出路徑失敗。
- 固定排程完整重播兩次；兩份 JSON 與兩份 framebuffer 各自逐 byte 相同。
- 外部 verifier 固定 state／原版／工具 hash、兩鍵排程、14 筆 exact identity、嚴格遞增時序、
  第 11–14 筆新畫面事件與終點 framebuffer hash；命令成功退出另證明執行期零
  pending／drop，不把 JSON 未保存的內部計數偽稱為 verifier 欄位。
- 全部正式 packages test／vet 與 `apps/buckrogers`、receipt command race detector 通過。

符合後可標為 CONFORMED。這只證明單一預設種族的下一畫面文字路徑；不授權翻譯 catalog、
text-safe rectangle、renderer、倍率或 production runtime 接線，也不得外推完整角色建立流程。

## 符合性紀錄（2026-09-21）

- `cmd/buckrogers-text-receipt/main.go`／測試 SHA-256：
  `8ce3a79a791659f72f4fa4190274155462ce701431bf11c8d7da7ee8ae291506`／
  `aed9c7beade76d2b6d08bc3917869edee7f2c385b1a62e2c0f07b356282609b0`。
- 固定兩鍵排程完整重播兩次，兩份 JSON 逐 byte 相同，SHA-256 均為
  `0c24a9fa56f5815bfed35b9ebff1ca219d10931beafaed16ed57a99eabbb7dab`；命令成功退出，
  恰有 14 事件且零 pending／drop。
- 兩份終點 64,000-byte indexed framebuffer 逐 byte 相同，SHA-256 均為
  `dcf947d18c85b051ec85e1bc968f30cec5c315a9158d598055d14ebfe82a715c`。
- 後四筆事件完整 identity 與本規格逐欄相同；#100,334,602 後到 #101,000,000 沒有新增
  dispatcher 完成事件。這只證明該固定窗口穩定，不外推後續輸入或其他分支。
- `-receipt-out` 單元測試證實 stdout／檔案同 bytes，且輸出指向目錄時失敗。
- 排除既有非正式 `workplace/` 後，全部正式 packages test／vet，以及
  `apps/buckrogers`／`cmd/buckrogers-text-receipt` race detector 全數通過。

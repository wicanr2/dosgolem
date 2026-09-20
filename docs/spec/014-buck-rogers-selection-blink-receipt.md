# Buck Rogers 種族選取列連續取樣收據

狀態：**CONFORMED**

本規格只核准一個 content-safe 診斷命令，從既有 dosgolem state 走正常 Enter 或
Enter→Down 路徑，以固定絕對 step 取樣 row 3／4 indexed pixels 與 palette。目的在區分
selected row 的 pixel redraw、palette-only 變化及穩定不可見狀態；不繪中文、不送範圍外
輸入、不改原版狀態，也不修改 VGA／PIT 實作。

## 固定輸入與位址空間

- state SHA-256：`cfe15d3c66c9fe3c2e684815740a0cc0165e59d08ab5866370608d49f8a8e164`，
  起點 #99,999,999。
- 正式 `menu-events.tsv` SHA-256：
  `973a6a1e247e7d9e16518a1a66266f340666d32e785f3f6f6652a890e830da2e`；命令只固定其
  雜湊，event identity 由外部 verifier 逐筆交叉驗證。
- dispatcher 位址 `0763:0424`、caller、`SS:SP` 與 `RETF 0Ch` guard 皆是 dosgolem runtime
  `segment:offset` 契約，沿用 specs 009–012，不是 IDA 線性位址。
- 原版檔案根目錄唯讀；state 與事件 TSV 也唯讀。輸出僅 JSON，不含英文／中文全文。

## 排程與取樣

命令參數必須可表達：state、正式事件檔、停止 step、Enter step、可選 Down step、第一個
sample step 與固定 interval。正式收據固定：

- Enter #100,010,000；steady 不送 Down，Down 路徑於 #100,240,000 送 BIOS Down；
- #100,220,000 起每 10,000 steps 取樣，停止於 #110,000,000；
- 恰有 978 點，最後一點 #109,990,000；
- steady 恰有九個 guarded post-call，Down 恰有十一個，零 pending／drop。

每點保存：絕對 step、完整 palette SHA-256、色號 0／10／13／15 的 8-bit RGB、色號 0 與
15 是否具 contrast、row 3 `[24,72)×[24,32)` 與 row 4 `[24,80)×[32,40)` 的 SHA-256／
色號計數，以及當時完成事件數。

## 收據 schema 與失敗模式

頂層另保存工具識別、state／事件檔 SHA-256、排程、samples、content-free events、pending
與 drops。任一必要路徑缺失、排程無效、BIOS buffer full、state 載入／CPU step 失敗、未達
精確停止點或 JSON 寫出失敗皆須非零退出。

正式 verifier 必須失敗即關閉：

- 兩次重播不逐 byte 相同；schema、輸入 hash、step 序列或樣本數不符；
- palette 在窗口內出現第二個 hash，色號 0／15 不再同為黑色，或 contrast 變 true；
- steady 自 #100,230,000 後不是 selected Terran／normal Martian 的固定區域 hash；
- Down 自 #100,260,000 後不是 normal Terran／selected Martian 的固定區域 hash；
- event 未逐筆精確對應正式 inventory、時序不遞增，或 pending／drop 非零。

## 證據界線與驗收

本收據只能證明 #100,220,000–#109,990,000 的玩家可見結果；不能外推整場遊戲永久不閃爍，
也不追求 VGA DAC／PIT 逐週期考古。若窗口內 palette 唯一、完成重畫後 region hash 穩定，
可排除該窗口內的 palette-only blink 與週期性 pixel redraw，並把 selected row 分類為原版
穩定黑底黑字狀態。

- 命令 unit test 覆蓋 region hash／counts、palette contrast 與參數拒絕；
- steady／Down 各重播兩次並由專案 verifier 驗證；
- 全部正式 packages test／vet 與相關 race detector 通過。

符合後可標為 CONFORMED。這項結果只限制未來 renderer 必須忠實處理 selection style；
不授權自訂高亮色、不決定 2×／3×，也不授權接入 production renderer。

## 符合性紀錄（2026-09-21）

- 命令原始碼 SHA-256：`e7661440add6b151925355a45bf8b983b414c6cb3ad97f586a5c9ebc8b9787d4`；
  測試檔 SHA-256：`cb3646ff29ae6eee4849ac9a4cff65d853917ffd0f6341702637dcac41140b95`。
- steady／Down 各重播兩次，pair 內逐 byte 相同；權威收據 SHA-256 分別為
  `4bacd03889f182070a826880be353cb998ffcb7c17219eeb7d33f92d43bef83f` 與
  `339b6920ac26428514b4ca2eb4ac09769cac08d8a4bbfa707f15a1fbd8e67569`。
- 每條路徑恰有 978 點；完整 palette 在所有點只有一個 SHA-256，色號 0／15 全程皆為
  `(0,0,0)`，`selected_contrast=false` 共 978／978。
- steady 自 #100,230,000 起 row 3 固定 selected Terran hash、row 4 固定 normal Martian
  hash；Down 自 #100,260,000 起 row 3 固定 normal Terran、row 4 固定 selected Martian，
  直到 #109,990,000 都沒有第二個 region hash 或新文字事件。
- 專案 verifier 與 44 項 Python 測試通過；dosgolem 全部正式 packages test／vet、
  `apps/buckrogers` 與本命令 race detector 通過。
- 因此已證實取樣窗口內沒有 palette-only blink 或週期性 pixel redraw；selected row 是
  index 15 背景加 index 0 glyph，而兩色 RGB 同為黑色的穩定不可見狀態。窗口外仍不外推。

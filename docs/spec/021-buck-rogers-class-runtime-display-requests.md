# Buck Rogers 職業選擇執行期繁中顯示請求

狀態：**CONFORMED**

本規格核准將正式 `class-events.tsv`／`class.zh-TW.tsv` 接到既有 exact catalog 與同一個
`MenuRequestWatcher`。只產生 content-safe `DisplayRequest`，不載入字型、不繪圖、不清除
英文、不改原版狀態，也不選定 2×／3×。

## 證據與譯詞

- identity 來源為專案第三十八、三十九階段的 post-gender 與 class lifecycle 收據。
- 中文說明書 `SCAN0352_007.jpg` 至 `SCAN0352_009.jpg` 的「C.職業」逐項並列：太空船駕駛員
  （ROCKETJOCK）、戰士（WARRIOR）、工程師（ENGINEER）、流浪漢（ROGUES）、醫生
  （MEDICS）。提示採原書「職業」與既有介面動詞組成「選擇職業」。
- runtime 位址是 dosgolem `segment:offset`，不是檔案偏移或 IDA 線性位址。

## catalog 與命令契約

- class inventory 沿用既有十欄 schema，恰有十個唯一完整 identity；翻譯表恰有六鍵，兩表
  雙向完整，UTF-8 無 BOM，任何衝突失敗即關閉。
- 新增 `LoadClassCatalog` 只包裝既有共用 parser，不複製 Resolve 或 watcher。
- receipt command 新增成對 `-class-events`／`-class-translations`；任一缺件必須拒絕，與 menu、
  gender catalog 合併遇 identity 衝突必須拒絕。
- request 只含 event key、text key、譯文字數；未知事件只增加 miss，不得沿用舊請求。

## 真實路徑驗收

- steady：三次 Enter，22 events／22 requests／0 misses，停止 #101,000,000。
- Down→Up：26 events／26 requests／0 misses，停止 #102,000,000。
- Escape：30 events；前 23 筆（含職業 normal 取消列）命中。其後返回選單七筆與初始 menu
  identity 不同，必須得到 23 requests／7 misses，且不得模糊合併或沿用請求；停止
  #102,000,000。
- 三路徑各重播兩次，完整 JSON 逐 byte 相同；專案 verifier 固定輸入、命令、排程、event、
  request 次序與 SHA-256。

## 停止線

全部正式套件測試、`go vet`、相關 race detector 與專案真實收據通過後才能升為 CONFORMED。
本規格不授權 renderer、文字安全矩形、倍率預設或尚未量得的角色分支。

## 符合性紀錄（2026-09-21）

- `class-events.tsv`／`class.zh-TW.tsv` SHA-256：`1eca7113…4de`／`4d4c83dc…3794`。
- 共用 catalog 核心／測試 SHA-256：`fc371e08…bea1`／`e7b520fc…c144`。
- receipt command／測試 SHA-256：`363fc804…231`／`65b4717b…e161`。
- steady、Down→Up、Escape 各重播兩次且 pair 內逐 byte 相同；SHA-256 分別為
  `e2ea55e8…c37e0`、`3f542218…1399`、`16033cc1…1f66`。
- 專案 65 項測試與正式 verifier 通過；全部正式套件測試、`go vet`、相關 race detector 通過。

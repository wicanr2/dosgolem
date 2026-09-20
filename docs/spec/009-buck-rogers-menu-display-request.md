# Buck Rogers 選單顯示請求核心

狀態：**READY**

本規格只定義《Buck Rogers: Countdown to Doomsday》固定版本中，已完成的
`0763:0424` 文字輸出事件如何以內容無關中繼資料精確解析成繁體中文
`DisplayRequest`。它不繪圖、不決定 2×／3×、不攔截輸入，也不修改遊戲狀態。

## 證據與固定輸入

- 事件表：`text/menu-events.tsv`，SHA-256
  `973a6a1e247e7d9e16518a1a66266f340666d32e785f3f6f6652a890e830da2e`。原九筆版本
  `38bc0fa6…370e9` 已由 spec 012 的 selection 證據擴充為 12 個唯一 identity。
- 翻譯表：`text/menu.zh-TW.tsv`，SHA-256
  `ca3319830adb7b34a048b498d8d0466b38b8fb6f418e5244a3a467e77ea68077`
- 執行期位址一律是 DOS `segment:offset`；`caller` 不是檔案偏移。
- 事件由 `docs/spec/008-buck-rogers-manual-runtime-watcher.md` 的同一個
  far-return／SS／SP 完成閘門產生。只有 `PostCallStep > EntryStep` 的完整事件
  可以送入解析器。

## 輸入契約

事件表標頭必須逐欄等於：

```text
event_key	sequence	text_key	original_length	original_sha256	caller	background	foreground	row	column
```

翻譯表標頭必須逐欄等於：

```text
key	translation	source
```

兩份檔案都必須是無 BOM 的有效 UTF-8 TSV；不得有空白列、額外欄位或重複
標頭。`sequence` 必須是從 1 開始、按檔案順序連續的 ASCII 十進位數。
`original_length` 與四個畫面欄位必須是 0–255 的 ASCII 十進位數；
`original_sha256` 必須是 64 個小寫十六進位字元；`caller` 必須是大寫
`XXXX:XXXX`。

每個 `event_key` 與下列事件 identity 都必須唯一：

```text
original_length + original_sha256 + caller + background + foreground + row + column
```

`sequence`、`EntryStep`、`PostCallStep` 不屬於 identity。步數只證明呼叫已完成，
不得讓同一事件因執行時序不同而失配。

每個事件的 `text_key` 必須存在於翻譯表；翻譯表不得有孤兒鍵或重複鍵。
多個事件可以共用同一個 `text_key`，例如 Terran 選項與標題共用翻譯。

## 解析結果

解析器只接受 `TextEvent`，先驗證完成步數，再以完整 identity 精確比對。
命中時回傳既有的 `DisplayRequest`：

- `Generation = 0`：選單事件沒有手冊題目的 generation；零值是明示的
  非手冊請求，不得拿來做生命週期判斷。
- `EventKey`：事件表中的穩定事件鍵。
- `TextKey`：翻譯鍵。
- `Translation`：繁體中文輸出文字。

沒有完全命中、事件未完成或任何輸入驗證失敗時一律失敗即關閉
（fail-closed），不得回退到模糊字串、步數範圍、單獨 caller 或座標比對。
解析器不得保存或輸出英文原文；原文只以長度與 SHA-256 參與 identity。

## 驗收

- 正式兩份 TSV 可通過嚴格載入，並確認固定 SHA-256。
- 固定狀態收據的九個完整事件依序解析成九個 `DisplayRequest`。
- 測試覆蓋：重複事件鍵、重複 identity、跳號、錯誤大小寫雜湊／位址、
  數值越界、缺少翻譯、孤兒翻譯、允許共用文字鍵，以及任一 identity 欄位
  不同時拒絕命中。
- 核心套件測試與 race detector 通過；本階段不宣稱畫面已繪出中文。

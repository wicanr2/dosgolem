# Buck Rogers 角色資料靜態文字執行期顯示請求

狀態：CONFORMED

## 目的與邊界

本規格只把《Buck Rogers: Countdown to Doomsday》角色資料／重擲畫面的 35 筆已證實靜態
文字事件解析為繁體中文 `DisplayRequest`。原版先完成繪製；resolver 不改原文、能力值、技能值、
亂數、輸入、記憶體、存檔或控制流。本規格不接 renderer，也不選定 2×／3×。

正式輸入由專案 `text/character-sheet-events.tsv` 與 `text/character-sheet.zh-TW.tsv` 提供。事件
identity 固定為原文長度、SHA-256、caller、背景／前景色、row 與 column；譯文不作比對鍵。

## 已核准事件

- 19 筆角色資料固定欄位／區塊標題。
- 7 筆能力名稱。
- 8 筆太空船駕駛員專業技能名稱。
- 1 筆重擲提示。

空白姓名、種族／性別／職業值、生命值等摘要值、七項能力數值、八項技能數值，以及重擲後的
所有動態重畫都不得進入 catalog。相同靜態 identity 在 `Y`／`N` 分支重畫時仍由同一筆解析，
不得為終態畫面另造模糊規則。

## 實作契約

1. `LoadCharacterSheetCatalog` 沿用 exact catalog parser；schema、UTF-8、連續 sequence、唯一事件、
   唯一 identity、譯文雙向完整與孤兒 key 均失敗即關閉。
2. `buckrogers-text-receipt` 的 `-character-sheet-events` 與
   `-character-sheet-translations` 必須成對提供，並與其他 catalog 合併交給既有
   `MenuRequestWatcher`。
3. 每個完成事件最多產生一筆 request；不命中只計 miss。request 僅含事件鍵、文字鍵與譯文。
4. 正常四次 Enter 到重擲畫面，以及 `Y` 或 `N` 分支，必須保存決定性 request 收據；加入 watcher
   前後的原版 event 與 64,000-byte framebuffer 必須逐 byte 相同。

## 證據

- 專案 `text/post-class-events.tsv` 的 96 筆完成事件。
- 專案 `text/reroll-yes-events.tsv` 與 `text/reroll-no-events.tsv` 的兩條正常 BIOS 分支。
- 中文說明書 `SCAN0352_007.jpg` 至 `SCAN0352_010.jpg` 的屬性、職業與專業技能術語；沒有直接
  手冊對應的畫面欄位標為 `runtime-interface`。

已以四次正常 Enter 的 118-event 基線及其後 `Y` 重擲的 149-event 分支各獨立重跑兩次；
JSON 與 framebuffer 各自逐 byte 相同，並由專案正式 verifier 核對 43／52 筆 request 的 exact
identity 順序。原版 framebuffer 雜湊仍分別等於第四十二、四十三階段基準，因此升為
CONFORMED。

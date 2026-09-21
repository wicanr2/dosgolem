# 208 — 執行期覆繪空終態輸出

狀態：CONFORMED

## 問題與證據

姓名提示 Enter 正常路徑會先建立 stamp，再由原版 `026F:029C` 清除矩形使其在技能配置終態
失效。`RuntimeMenuOverlay.Draw` 正確回傳 `drew=false`、零缺字，但
`buckrogers-text-receipt` 把任何 `drew=false` 當成錯誤，因而無法輸出「曾有覆繪、終態已清空」
的正式收據。這是收據工具的狀態表達缺口，不是遊戲或 renderer 失敗。

## 契約

1. 有 presenter 時永遠輸出 `overlay_drew` 布林值；無 presenter 的 control 不輸出該欄。
2. `drew=true` 當且僅當終態 active overlay keys 非空；`drew=false` 當且僅當 keys 為空。
3. 缺字永遠失敗；`drew=false` 不得掩蓋非空 active keys、renderer 錯誤或未套用的 request。
4. 空終態仍輸出與同 frame／palette baseline 相同尺寸的 RGBA；不得修改 indexed framebuffer。
5. `overlay_actions` 保留本次執行曾成功建立的 request 歷史，不因終態失效而刪除。

## 驗收

- 單元測試涵蓋非空／空 active keys 與 drew 一致性。
- 姓名 Enter 路徑須有一筆 action、空 active keys、`overlay_drew=false`，且 RGBA 等於 baseline。
- Escape 重印路徑須有兩筆 actions、終態一個 active key、`overlay_drew=true`。
- 正式測試、vet、race detector 與同狀態收據通過後升為 CONFORMED。

## READY 審查

現有 layer 已以 active stamp 決定 `Draw` 回值；原版清除 hook 與姓名轉場均已有正常路徑證據。
本規格只讓收據忠實表達既有空終態，不新增遊戲特例或改變覆繪失效規則，可以進入實作。

## CONFORMED 收據

- 收據以可省略的布林指標區分「沒有 presenter」與「presenter 終態 drew=false」；既有
  drew=true JSON 契約不變。
- Enter 終態一筆 action、零 active keys、`overlay_drew=false`，2×／3× RGBA 都逐位元等於
  baseline；raw framebuffer 等於 control。
- Escape 原地重印終態兩筆 actions、恰一個 active key、`overlay_drew=true`；同 key replace
  沒有堆疊第二份 stamp。
- 單元測試拒絕非空 keys／drew=false、空 keys／drew=true 與任何缺字；正式 test、vet、race
  detector 皆通過。

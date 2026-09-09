# 規格012：EOB1第一角色種族選擇checkpoint

狀態：**CONFORMED**

## 證據與契約

在CONFORMED建角入口直接按Enter，原版選取左上第一角色槽並顯示`SELECT RACE:`，其後列出
Human、Elf、Half-Elf、Dwarf、Gnome與Halfling的male／female選項。此路徑沒有座標注入，也不
依賴尚未實作的`int 33h AX=0Ch`滑鼠callback。

左側角色槽仍有循環效果；右側矩形`(138,60) 170×130`完成繪製後，其色號SHA-256固定為
`34cd6ac1502742323eb299439fe6066154336a9f42f372457eec4d8b5e3b13f6`。首次過渡幀的不同雜湊
不得當成checkpoint。

`apps/eob1.ToFirstCharacterRace`由冷啟動走既有`ToNewPartyCreation`，送一個`KeyEnter`，再以
有界預算等待上述安全矩形。真實資料測試須核對矩形雜湊，以及四鍵Escape／Down／Enter／Enter
共八次port `60h`讀取。

## 限制與停止線

本checkpoint只證實第一角色種族選擇頁，不代表選定種族、職業、陣營、屬性、肖像或姓名。
原版資料與畫面只作本機研究，不提交版本庫；真實資料測試、八次port `60h`讀取及人眼畫面
均已通過，本規格升為CONFORMED。

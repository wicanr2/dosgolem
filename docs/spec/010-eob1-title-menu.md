# 規格010：EOB1主選單checkpoint

狀態：**CONFORMED**

## 契約

`apps/eob1.ToTitleMenu`由冷啟動呼叫既有`ToWestwoodLogo`，送出一次
`PressKey(KeyEscape)`，等待首次開啟`EOBDATA3.PAK`及連續1,000,000道指令畫面不變。
這是正常`START1.EXE → 44Y`路徑，沒有座標注入或改寫遊戲狀態。

## 真實資料驗收

- 主選單在7,240,174道指令保持穩定。
- 色號SHA-256：`caa3082b3e8cb5ee15547555669eb82e954982fe919674d5481271e06a253dc0`。
- palette SHA-256：`bf08a3424f429a6cb5400a8ddef50f8ee35ed03deb0de51a307799bbc6b9687f`。
- port `60h`恰讀取兩次，對應Escape make／break。
- 人眼檢視可見原版標題及三項選單；不把此checkpoint外推為建角或遊戲流程完成。

## 停止線與權利

主選單已成為可重播具名checkpoint，本切片停止。下一切片若操作「START A NEW PARTY」，
須另立輸入、狀態轉移與建角畫面的證據／規格。原版檔案與畫面不提交版本庫。

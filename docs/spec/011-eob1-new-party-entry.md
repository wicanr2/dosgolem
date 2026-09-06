# 規格011：EOB1新隊伍建角入口checkpoint

狀態：**CONFORMED**

## 範圍與證據

由CONFORMED主選單狀態按一次向下鍵，原版把反白項目由`LOAD GAME IN PROGRESS`移到
`START A NEW PARTY`；再按Enter，畫面轉為`Character Generation`。方向鍵與Enter分別使
port `60h`多兩次讀取，證實make／break皆由原版ISR消費。

建角畫面的左側角色槽有循環色號效果，全畫面SHA-256會持續變化，因此不得使用全畫面
`ScreenIdle`。原版完成繪製後，下列兩個不含循環區的安全矩形保持固定：

- 標題區 `(32,8) 256×40`，色號SHA-256
  `2496edf386e334b90af5de1a670f171e21dd1591b0b574be84847d69177d85f0`。
- 右側說明區 `(130,55) 180×140`，色號SHA-256
  `39cb98087f15b0604262c4767b6a7df50593a4d29e604b4a778db58a2df3e3f6`。

## 契約

`apps/eob1.ToNewPartyCreation`必須：

1. 由冷啟動走`ToTitleMenu`，不直接寫入選單或建角狀態。
2. 送`KeyDown`後等待完整主選單等於已觀察的選取幀，避免Enter與方向鍵在遊戲主迴圈讀取前互相覆蓋。
3. 送`KeyEnter`後，以有界預算等待上述兩個安全矩形同時符合雜湊。
4. 任一路標未出現、程式退出或CPU錯誤都回錯，不以固定總步數冒充成功。

## 驗收、限制與停止線

- 真實資料測試核對兩個安全矩形及port `60h`累計六次讀取（Escape、Down、Enter各make／break）。
- 人眼檢視必須看到原版`Character Generation`標題、四個角色槽與右側建立／檢視角色說明。
- 本checkpoint只證實建角入口，不代表角色建立步驟、姓名輸入或隊伍完成。
- 原版檔案與畫面只作本機研究，不提交版本庫。真實資料回歸、六次port `60h`讀取及人眼畫面
  檢視均已通過，本規格升為**CONFORMED**。

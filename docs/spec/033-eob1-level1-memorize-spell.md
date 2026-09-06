# 規格033：EOB1 LEVEL1 CAMP記憶第一個法術

狀態：**CONFORMED**

## 正常玩家路徑與畫面契約

由規格032的CAMP第二列Memorize Spells送Set 1 `Enter=1Ch`：

- 新隊伍只有ALFA可施法，因此直接進入其法術頁，不先顯示角色選擇對話框；
- ALFA肖像以黃色框高亮；左側標題為`Spells Available:`，有法術等級1～5頁籤；
- 一級頁顯示`2 of 2 Remaining.`、四項法術及Clear／Exit；
- 完整320×200色號SHA-256為
  `1c430348d1f4d21309d1b553fb3f21597baa97c6682563c9d599ad9b9bf6c00a`，port `60h`88次。

在預設第一項再送Enter：

- remaining由2減為1，畫面新增第一個待記憶法術標記；
- 完整色號SHA-256為`2d78cd6c8a1886cb8d06b14a708d70ffc6fd9529422c0ade2eff36b2df386da8`；
- port `60h`90次，恰增加一組make／break。

再送`Escape=01h`返回CAMP根選單，Memorize Spells仍是紅字目前列；完整色號SHA-256為
`8bd3e9866787810347ed38d1929b8dbb1a12683359037056f5fc369df3b2962c`，port `60h`92次。

## 證據等級與邊界

上述presentation、單次選取交易與返回均由同版本DOS bytes以dosgolem正常路徑重播，等級為
已證實。沒有使用存檔、direct-entry、角色／法術槽注入或記憶體修改。

本規格不把畫面短字反推為正式法術ID，也不外推多施法者角色選擇、其他法術等級、Clear、
重複選取、休息後真正記憶或施放結果；這些由各自來源／remake垂直鏈負責。原版截圖只供本機
人工檢視，不提交或散布。

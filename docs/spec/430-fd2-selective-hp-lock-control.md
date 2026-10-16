# 430 FD2 選擇性血量鎖定控制

狀態：READY。日期：2026-10-16。

使用者明示要求再次抽樣第27章，保護索爾與敵方隊長直到增援抵達。這是測試注入，
不改原版EXE、CPU、硬體或正式遊戲規則，不作傷害、存活或一般玩家路徑證據。
固定FD2.EXE SHA256：222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
原始80-byte record的HP在+0x40，camp在+6、identity在+8；沿012及既有lock-ally-hp契約。

## 控制契約

唯一oracle新增選用旗標`-allow-hp-lock-control`。未啟用時拒收`hp_lock`。
不得同時啟用舊的`-lock-ally-hp`。一般執行不改變。

控制JSON新增`hp_lock`陣列，最多4筆，空陣列解除全部。每筆明示整數`index`、`camp`、
`identity`；camp只准0或2。選用`hp`只准1..32767，未指定則捕獲當前正HP；相同
綁定的既有鎖保持其捕獲值。所有target先核對來源與範圍，原子發布，錯誤零寫入。
拒收重複index、缺欄位、未知欄位、inactive或HP0，不能藉此復活。

每條guest指令前，重新由LE線性0x53A45／0x53BEB取得陣列；確認80-byte邊界、
所有綁定與活性後只寫+0x40。合法擴充／搬動陣列仍按index／camp／identity重新定位，
不持有過時記憶體指標。綁定不符即停止，不改+5、MaxHP、其他record或事件狀態。

`force_enemy_clear`在有敵方鎖時須明示`preserve_hp_locks=true`，清除其他camp0的HP，
跳過鎖定index。未明示則拒收；舊無鎖清場保持。控制歷史保留原JSON，checkpoint與
final記錄選用旗標、活動綁定／捕獲HP、更新數、逐index寫入數與證據限制。

## 第27章採樣順序

正常LOAD與選人後鎖record0／camp2／identity0及records24..26／camp0／identity116。
正常移動觸發62→63，確認groups1／2已到齊，再移開鏡頭、清除56個非隊長敵軍。
前兩名隊長HP設1後解鎖，正常攻擊依序擊倒；保留第三名直到event64的groups3／4／5
全部登場。狀態16依死亡次數0→1→2；第二次死亡才播放idx1與三次35822，不能用清敵
冒充該入口。重製側採相同階段與注入，驗正式死亡派送、對白及增援。

## 驗收

- 未啟用、無效來源、缺／錯欄位、重複與越界皆拒收且零寫入。
- 綁定與HP捕獲、選用HP1、解除、陣列搬動及全體先驗證皆有測試。
- 保護清場只清非鎖定敵軍；舊無鎖清場行為保持。
- 新能力停用時，原有FD2啟動／正常鍵盤前綴保持；來源由commit與檔案SHA記錄。
- 原版及重製實際到達event63兩組及event64三組，不只檢查工具旗標或資料存在。

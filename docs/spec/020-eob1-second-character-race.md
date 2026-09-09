# 規格020：EOB1第二角色種族選擇checkpoint

狀態：**CONFORMED**

## 證據與契約

第一角色ALFA完成後，四槽總覽可由滑鼠callback點擊第二槽中心`(98,82)`。原版把游標移到第二槽，
進入`SELECT RACE:`並在此座標狀態反白`HUMAN MALE`。右側矩形`(138,60) 170×130`穩定色號
SHA-256為`0ac2799d93a1c51adcd884eeefa770b5dc01fbfba36311000a8b5f9d586fc2b3`。

此雜湊不同於第一角色鍵盤Enter進入種族頁的`34cd…`，證實清單反白受滑鼠／hover狀態影響；
不得把清單第一項或先前checkpoint稱為全域預設。

`apps/eob1.ToSecondCharacterRace`由冷啟動完成ALFA，再以正常Click選第二槽並等待安全矩形。
真實資料測試核對雜湊；Click無回應或路標不到均回錯。

## 限制與停止線

本checkpoint只證實第二槽與目前Human Male反白，不代表已選定種族或其後角色完成。真實資料與
人眼檢視均已通過，本規格升CONFORMED。

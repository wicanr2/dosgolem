# 規格021：EOB1第二角色種族至檢視頁

狀態：**CONFORMED**

## 證據與契約

由CONFORMED第二角色種族頁依序按四次Enter，原版正常經過職業、陣營、屬性／肖像與檢視頁。
此槽位／hover狀態的下游屬性consumer明確顯示`HUMAN MALE / FIGHTER`，不同於第一角色的
Elf Male Mage；因此只重用導航結構，不共用選項身分或畫面雜湊。

各頁右側`(130,55) 180×140`安全矩形依序為：

- 職業：`31954c4eabf3fe236734a89b17447907ac3c54448a962b4ffd0432a29e2777eb`
- 陣營：`b701f7d8e0738aaf110f702f34a53d3ab9f0c01b60fd0f7e514bdd773a7bafbc`
- 屬性：`b0794062e92812ef3321b3e3184427a3b2440d6671f6ffd3dffe0fb71e0aa49d`
- 檢視：`46c7b7647dd6ae8102a8b970315b5597c422a0690308b05668d548d6bdc40759`

`apps/eob1.ToSecondCharacterReview`逐頁送Enter並等待各自路標；任一頁未達不得繼續。真實資料
測試核對最終矩形及累計十七鍵make／break、三十四次port `60h`讀取。

## 限制與停止線

此規格不宣稱陣營身分，也不代表KEEP、姓名或第二角色完成。原版資料與畫面只作本機研究；
真實資料、三十四次port `60h`讀取與四頁人眼檢視均已通過，本規格升CONFORMED。

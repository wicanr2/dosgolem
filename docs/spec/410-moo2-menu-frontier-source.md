# 410：GAME 原版末態、較低返回 caller 與最小 helper 邊界

狀態：**CONFORMED，限定原來源與既有409收據**
日期：2026-10-05

接續[409](409-moo2-game-outer-frame-continue.md)。官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA9.4 linear EA、runtime＝EA+F0000h與原file offset分列。兩份窄查詢678列／609個EA／89列原bytes與重定位bytes不同；原2object／365page／51363fixup records獨立核對。本篇沒有新Go或guest，不把畫面helper列成玩法完成分母。

## 較低框架的實際返回

已證實：409在226846742的runtime173D05、ESP2BD38C讀到真SS return174BC9；唯一下一Step226846743的ESP2BD390與CS／SS握手通過。原84BC4 CALL87BAE與84BC9 pop esi已核對。當次RET值與這個靜態CALL的返回定位相符；87BAE當次入口及其內部路徑未取樣，不能由共同尾段推成86188的外層返回。原名與原定位保持。

## 230M末態的最小分類

已證實：末態runtime21F7C1對應原12F7C1，位於sub_12F578；IDA函式180指令、49個direct caller，只保存最小入口、末態窗口、RET與caller邊界，沒有完整解碼格式。原12F7A3..12F7C1比較[ebp-2Ch]與[ebp-28h]，讀一byte、寫一byte，增加[ebp-38h]與[ebp-2Ch]再回比較。這是原byte-copy循環定位，不給欄位新增推測名稱；當次bound與實際剩餘迭代數未知。

原12F587將ESP置入EBP，12F589減ACh。末態EBP2BD934、ESP2BD888吻合。原12F7DE..12F7E5恢復ESP、pop EBP／EDI／ESI／ECX／EBX及C3，故當次真RET槽應為EBP＋20＝2BD948。該槽實際dword、當次caller及自然返回尚未讀取，不能預填16EE5E。

已證實的靜態選單依賴鏈是原80211 CALL7EDF2、7EE59 CALL12F578。sub_12F578還有其他caller，這條靜態鏈不能單獨證明230M當次就是7EE59的呼叫。409已到8012F，但尚未到8028F控件建立。原PNG仍是星圖；沒有CPU錯誤或選單正常輸入證據。

## 下一個最小行動與停止線

[411](411-moo2-game-frontier-continue.md)凍結完整409末態與15個phase後，只讀當次RET槽與原target code，再沿唯一Step自然續行。按當次SP分開helper返回，接回已知控件建立、case0與正常reader契約。沒有新裝置事件，不改原RAM、mode或CPU。不為這個helper展開解碼格式、畫面規則、標準函式庫或driver內部。

本機忽略入口workplace/new-game-410-ida-run.sh、new-game-410-epilog-ida-run.sh、new-game-410-byte-verify.py、new-game-410-source-verify.py。原JSON／LOG／byte index及私有腳本不公開。IDA9.4 locked-v1、Go1.24.13／Python3.11、UID/GID1000、network none、patch唯讀；IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU。兩殼層exit0／idat_exit1、非空JSON與5365函式、固定hash及UID通過。

回填：[408](408-moo2-star-map-frame-source.md)的較低target已由409讀到174BC9；[405](405-moo2-star-map-game-source.md)的原mode8／1004BC／8012F已動態觀察。409仍DRAFT，正常選單、存讀與remake同狀態尚未驗。

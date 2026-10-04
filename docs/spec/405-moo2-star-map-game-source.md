# 405：原星圖 GAME 綁定與選項入口來源

狀態：**CONFORMED，限定原RE來源與404既有末態**
日期：2026-10-04

接續[404正常列表RETURN](404-moo2-colonies-list-return-input.md)。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。IDA Pro9.4 linear EA、runtime＝EA+F0000h與原file offset分列。四份窄查詢597列／475個EA／187列原bytes與重定位bytes不同；原2object／365page／51363fixup records獨立重建核對。保留原函式名、bytes、xref與推論等級。沒有新guest或裝置輸入。

## GAME producer與consumer

已證實：sub_81D73的81E31設EDX＝5、81E36設EAX＝249；81E4F CALL1151B0回傳AX，81E6B寫入word_19417C。呼叫與回傳之間只有push／mov，不改AX。控件建立內部不追。404星圖原23×55byte表raw6／kind0矩形249,5..307,21及GUI280,13的九個first-hit案例通過；GAME文字與矩形的關係由404原PNG人工核對，當次19417C值仍待406讀取，不能從幾何單獨當作綁定已驗。

sub_86188原86516 CALL1171AB，8651B保存輸入值；86BB6把選取值與word_19417C比較，86BBF再比較word_191976＝0。通過後86BC9把191830設0、86BD2把191A08設8、86BDB把191A10設0。主分派器104A6 CALL8012F，104AB把last byte191F19設8；星圖原caller是104B7 CALL86188。原選取、分支與writer須由正常輸入另驗，不注入mode8或ID。

## 真正的星圖返回邊界

sub_86188末端8763F是JMP83D00，原bytes e9bcc6ffff。87643位於這條指令內，不是RET。共享尾段83D00..83D04依序pop EDI／ESI／EDX／ECX／EBX，83D05原C3才是near RET。[408](408-moo2-star-map-frame-source.md)已補原ENTER6CC／EBP減82的outer slot；173D05須同ESP作用域才辨識外層RET。104B7只提供靜態1004BC caller來源，當次值由[409](409-moo2-game-outer-frame-continue.md)只讀取得，不預填。

首份producer查詢因把87643當指令起點而失敗，沒有guest；failed1-405四份原產物與manifest保留。修正查詢使用IDA原item head並另存requested address，後續窄查詢補齊共享尾段。四份非空JSON、5365函式、固定原hash、UID/GID1000通過；殼層exit0，idat退出碼逐檔保留。這是定位腳本問題，沒有CPU缺陷證據。

## 選項控件建立與停止線

完整sub_8012F的146條指令只保留玩家入口與分派邊界。191830限制0..3；8028F CALL7D061，下一原位址80294讀取該word。case0的802AE CALL7DD41、case1的802B8 CALL7E00F、case2的802C2 CALL7DA76、case3的802CC CALL7E154；存讀只定位原call，未驗正常功能。[407](407-moo2-menu-control-input-source.md)已補證7D061的控件建立用途；首個mode0正常輸入在7DD77 CALL1171AB，實際進入仍待觀察。

406先凍結完整404，正常GAME press／release後觀察mode writer、真RET、原8012F入口及8028F→7D061控件建立真CALL；到7D061第一條指令即停止，畫面判讀另存。真正的選單1171AB輸入、後續操作、正式存讀與remake同狀態仍待驗。主庫RE-first及Go玩法保持。

本機忽略入口workplace/new-game-405-ida-run.sh、new-game-405-menu-ida-run.sh、new-game-405-producer-ida-run.sh、new-game-405-epilog-ida-run.sh、new-game-405-byte-verify.py、new-game-405-source-verify.py。Go1.24.13／Python3.11／IDA9.4 locked-v1，UID/GID1000、network none、patch唯讀；IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU。原EXE／JSON／LOG／私有腳本不公開。

## 406首輪同前置原值補證

406首輪在完整404凍結後的225305800只讀驗出19417C／191976／191830＝6／0／34；沒有送出新的GAME輸入。來源405的86BC9是原GAME分支將191830設0，不能把writer輸出當成分支前置。34只標原word實測值，不推定星圖中的語意。first-hit6、callback22／22、IF與裝置buttons0均保持。原405四查詢及byte索引不覆寫，失敗收據見[406](406-moo2-star-map-game-input.md)。

409動態回填：完整406七phase與先前正常玩家前置保持，227146859原first-input返回EAX6、原mode8與1004BC outer真RET、8012F入口已驗。230M尚未到控件建立或真正正常reader，409仍DRAFT；見[409](409-moo2-game-outer-frame-continue.md)。末態最小來源由[410](410-moo2-menu-frontier-source.md)保存，下一私有只讀續行依[411](411-moo2-game-frontier-continue.md) READY，不新增裝置輸入。原版正常存讀與remake同狀態仍未驗。

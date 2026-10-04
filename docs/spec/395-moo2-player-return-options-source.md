# 395：正常 RETURN 與存讀選單來源

狀態：**CONFORMED，限定 RE 來源及393既有控件表**
日期：2026-10-04

接續[394人口職務與產出](394-moo2-pop-slot-consumers.md)。沿官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、IDA Pro9.4 linear EA；dosgolem_high_le runtime投影另加F0000h，原file offset／bytes與LE重定位分列。本輪來源查詢及邊界核對沒有新guest、輸入或Go改動，主庫RE-first保持。

## 已證實的原玩家來源

原BF0B7傳EDX=1CBh、BF0C3傳EAX=22Ch，BF0C8呼叫1151B0，BF0CD將返回AX存word_17AAD1。C0897／C08BD以此word比較輸入結果；C08CA進清理，C0960跳共用BAD99..BAD9F的原leave／pop／near RET。COLONIES caller包括104EA及FE0DE，不猜目前是哪一個，正常返回必須讀當次真SS槽。

不可變393 place-terminal SHA-256 `a92979e8b9b9badf8e8b95cbb3d472d18726eca3442c116f05a28306b0664ef0`，212590909原1B0845、36表／1980bytes、table298848／bias0。右下RETURN為index18、kind0、signed矩形556,459..628,478，原PNG `e7398617ced7c1af2bcf063e9d67c196aeaea159e8878e159a5dd2ce18972b75` 可見RETURN。中心GUI590,468對應裝置1180,468；四角及中心均first-match18，四個相鄰界外點均first-match35。不能拿底部左側index4當返回鈕。

原8012F是146指令的共用選單輸入owner。802A3的原4-entry跳表位於8011F，逐dword解碼為802AB／802B5／802BF／802C9；raw mode2經802C2呼叫7DA76讀檔，mode3經802CC呼叫7E154存檔。此兩個靜態分支由強推論升為已證實，不代表正常存讀已驗。

原1049B的106A7使用103EB的43-entry跳表，raw1到104EA呼叫C058A、raw8到104A6呼叫8012F。只保留原表／caller與入口，不直接派送這些值。返回後的實際玩家畫面與控件表仍由原執行器取得，不能以靜態caller預設已到星圖。

## 驗證、工具與邊界

兩次窄IDA共436列／358EA／117筆原LE重定位差異；原MZ／LE／2object／365page／51363fixup records獨立核對。首輪後處理列印器以raw target必有meta誤拒KeyError，保留new-game-395-ida-validation-rejected.txt；原JSON已寫完，改列印器並核對既有schema／固定hash／5365函式／4context／UID1000，不重跑IDA。第二查詢殼層exit0、idat_exit1保留；原bytes及9個熱區邊界、4／43-entry原跳表通過。

本機忽略入口：workplace/new-game-395-ida-run.sh、new-game-395-switch-ida-run.sh、new-game-395-byte-verify.py、new-game-395-source-verify.py；結果new-game-395-result.json。既有IDA locked-v1／Go1.24.13容器，UID/GID1000、network none；IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU，來源核對30s／512MiB／1CPU／64pids。原patch唯讀，EXE／JSON／PNG／LOG／IDA與private scripts不入Git。

下一步見[396正常RETURN輸入](396-moo2-colony-return-input.md)：保持完整393原結果後才送一次正常按下／放開，最多原215M，不預設返回後畫面。正常存讀、跨殖民地與remake同狀態未驗；主庫玩法／公開CPU／DOS保持。

## 解決回鏈

| 不可變原始定位 | 已證實語意 | 新證據 | 受影響的舊規格 | 必要回填 |
| --- | --- | --- | --- | --- |
| DOS官方1.31，固定上述EXE SHA，IDA linear EA 8011F／802C2／802CC | 四項原跳表的mode2讀檔／mode3存檔分支 | 本篇395 | [394](394-moo2-pop-slot-consumers.md) | 395 原存讀跳表補證 |

同固定SHA／位址引用核對只命中本篇及394，workplace/new-game-395-document-gate.py檢查原定位、較早規格的補證標記與雙向連結。其他正常GUI輸入未知保持，不能由靜態分支推定存讀成功。

### 397實際RETURN caller補證

[397正常RETURN](397-moo2-colony-return-deferred-release.md)由當次真SS驗出runtime返回0x1004ef，與本篇原caller相符；C0960後才接受共享BAD9F，原C3及ESP+4／同SS通過。原RETURN18／flags與正常放開已驗；正式存讀仍未驗。396候選維持DRAFT，不能把held結果重寫成自然返回。

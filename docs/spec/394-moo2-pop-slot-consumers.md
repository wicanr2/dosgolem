# 394：人口槽位職務與產出讀取端

狀態：**CONFORMED，限定 RE 來源與既有正常玩家收據**
日期：2026-10-04

接續[393正常放開與配置返回](393-moo2-colonies-pop-release.md)。本輪沒有新 guest、裝置輸入或 Go 修改，只核對原 BA5DA 寫入與 DE280 讀取端，再重讀不可變390／393收據。主庫 RE-first 保持；本文不是 remake 玩法實作規格。

## 輸入、工具與位址

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。IDA Pro 9.4，既有 locked-v1 image `6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780`；原始函式名與運算元保留。下列程式位址皆為 IDA linear EA，dosgolem_high_le runtime code／data 投影另加F0000h。LE file offset與原bytes／重定位bytes分別登錄，不能混稱同一地址。

主庫既有殖民地產出研究使用另一執行檔SHA-256 `7ae2ac2e5904ca330009af2827279d889906b0b9b7a8854c38eb707a56e955b5`。本輪重新查證官方EXE，僅沿用逐指令相符的16個錨點；不以相同EA證明兩版整個函式或遊戲相同。

## 已證實：寫入與讀取

`sub_BA5DA @ BA5DA..BA6FB` 的輸入由BA5E1／BA5E3／BA5E6保存：EAX的colony index到EDI、EDX的slot index到var_C、EBX的職務值到ESI。實際寫入按169h record stride及4-byte slot stride定位 `record+0Ch+4×slot`。

| IDA EA | 原運算 | 已證實的局部用途 |
| --- | --- | --- |
| BA6DC | `and edx,3` | 職務只取兩位 |
| BA6DF | `and word ptr [eax+0Ch],0FE7Fh` | 清除word第7、8位 |
| BA6E5／BA6E8 | `shl edx,7`、`or [eax+0Ch],edx` | 回寫職務值 |
| BA6EF | `or byte ptr [eax+0Dh],2` | 恢復word第9位，即mask200h |
| DE376／DE37A／DE390 | 讀`[esi+0Ah]`、取`[esi+0Ch]`、每筆退4 | 以record內slot count掃描人口槽 |
| DE393 | `test byte ptr [edi+1],2` | 第9位為0時略過該人口的產出 |
| DE39F／DE3A6／DE3A9 | 左移17h、右移1Eh、比較職務 | 等價於`(slot_dword>>7)&3`，只計指定職務 |
| DE3F2..DE3FA | 綁定職務、slot及record，呼叫DE22C | 同一筆人口進入職務基礎產出 |
| DE61E／DE621 | 比較首槽地址、回DE390 | 有界逐槽讀取，不以啟用人數改slot count |

三個直接caller重新核對：DE6F8以EDX=0後於DE6FA呼叫DE280；DEE97以EDX=1後於DEEA5呼叫；DFFE1以EDX=2後於DFFE8呼叫。它們分別對應既有食物／工業／研究產出鏈，job 0／1／2為農夫／工人／科學家。DE22C由當筆slot的低nibble取來源族群，再依職務dispatch。只查參數、職務篩選與實際consumer，不重新挖完整產出公式或平台helper。

word第9位的確切證明限於「是否計入這條產出鏈」及原選取／放置寫入。其他bit、job 3行為與所有拒絕文案沒有本輪證據，不能猜補。

## 已證實：原正常配置結果

不可變390 pop-terminal SHA-256 `8510fe25f71add56aee28ea6621f3475d40fb68470c343a1ad9d140e4e49f996`；393 place-terminal `a92979e8b9b9badf8e8b95cbb3d472d18726eca3442c116f05a28306b0664ef0`。393正常輸入、真SS返回及下一輸入點的時序沿393，沒有重新執行或注入RAM。

record位於runtime 5B25E8，slot count的+0Ah保持08。四個原農夫slot在390選取時由0200h變0000h，職務仍為0，但第9位被清除。393實際BA6E8各使0000h變0080h，BA6EF再變0280h，且EDX=80h／SI=1。八次實際byte變更按槽0、0、1、1、2、2、3、3重建；其他槽及每筆其餘位元保持。

| 正常原版階段 | 計入產出的農夫 | 工人 | 科學家 | slot count |
| --- | ---: | ---: | ---: | ---: |
| 390選取前 | 4 | 2 | 2 | 8 |
| 390選取後、393按下前 | 0 | 2 | 2 | 8 |
| 393配置返回及下一輸入 | 0 | 6 | 2 | 8 |

因此選取中的4000k顯示不證明刪除人口；此次原record保留八槽，四人暫停計入產出後改派工人。+0Bh、+C8h等其他raw欄位不在本輪定名。跨殖民地與remake同狀態尚未驗證。

## 正常存讀的來源邊界

已證實官方EXE的802CC直接呼叫原儲存入口7E154；同一caller sub_8012F的802C2呼叫7DA76讀檔入口。主選單81136另呼叫804B7。強推論：IDA switch註記case2／3對應遊戲中讀檔／存檔分支；此註記尚未以正常選單輸入與原jump-table逐項值驗收，不能直接派送case ID。

下一個最小工作是由393下一正常輸入點，先核對COLONIES返回星圖與原options入口，再確認讀存控件及安全輸入條件。已存在的SAVE10／MOX副本保持不等於正常存讀通過。未達有證據的輸入契約前，不送存檔事件、不改存檔內容或延長guest cap。

## 驗證與重生

兩次窄IDA查詢，BA5DA只為此次正常配置所需的88指令邊界；DE280僅取slot篩選、輸入與迴圈片段，三產出函式只取caller鄰近片段。合計300列／294個EA／9筆LE重定位差異。原MZ／LE／2object／365page／51363fixup records獨立核對。兩次殼層exit0、idat_exit1均保存；非空schema、固定EXE hash、UID1000及全部原bytes通過，不改寫內層退出狀態。

本機忽略工作區入口：

```text
bash /out/new-game-394-ida-run.sh
bash /out/new-game-394-binding-ida-run.sh
python3 workplace/new-game-394-byte-verify.py
python3 workplace/dosgolem/workplace/new-game-394-verify.py
```

前三項在工具副本根目錄與既有IDA／Go容器執行，最後一項在moo2主庫根目錄的Go容器執行。原patch唯讀，UID/GID1000，network none；IDA120s／2GiB／2CPU／128pids，bytes核對90s／2GiB／1CPU，收據核對30s／512MiB／1CPU／64pids。沿既有image，未建新image。

結果 `new-game-394-result.json`：8個實際writer、三階段slot數／職務分布、50個變更重建與16個別版相符錨點通過。原EXE／IDA／JSON／PNG／LOG及private scripts維持本機忽略；公開只提交自撰來源記錄、索引及歷史追加回填。

### 395 原存讀跳表補證

[395原RETURN與存讀選單來源](395-moo2-player-return-options-source.md)逐原bytes解碼8011F的四個跳表項，raw mode2到802BF／802C2讀檔，mode3到802C9／802CC存檔。此兩個靜態分支由強推論升為已證實，保留本篇原caller定位；正常GUI存讀仍未驗，不直接派送ID。

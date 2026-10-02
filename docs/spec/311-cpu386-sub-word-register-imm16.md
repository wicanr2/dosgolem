# 311：word 暫存器與完整立即值的 SUB

狀態：**CONFORMED**
日期：2026-10-03
範圍：CPU386 16位operand的81 /5 iw、目的為暫存器；不修改主庫玩法。

## 原始定位與公開契約

固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、工具基線f6bf96a976fb31e19b19545ae438b3abcb2006fa。原版309在高位LE0x240A32的CD21窗口已有後續0x240A34的66 81 E9 6C 07，即SUB CX,076Ch。310以明示1996-01-01平台日曆，日期服務／原caller待自然驗收；自製同形consumer已得到CX07CCh後在81 word形狀拒絕，不把自製樣本當原版收據。

公開來源：[Intel 80386 Programmer’s Reference，SUB](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SUB.htm)：81 /5 iw是word立即數減法，目的減來源，更新CF／PF／AF／ZF／SF／OF。iw為完整16位，不按83的imm8符號延伸。沿既有sub16算術helper與81 prefix／fetch框架，只加word暫存器group5分支。高16位與其他R／段／FPU／非算術flags保持；EIP完整讀5byte後續行。memory SUB、其他group／位寬／prefix不擴張。

## READY候選與驗收

- 運算輸入為uint16目的／立即值；寫回僅目的低16位。借位、低半byte借位、結果byte偶同位、有號16bit差值是否越界各自決定六旗標。
- prefix沿既有81範圍，segment／REP／REPNZ與既有未支援形狀拒絕；未完整讀iw不得寫回R／flags。不同失敗時既有fetch的EIP行為明示，不假稱通用交易回滾。
- 八目的、邊界與全部65536個低word對固定原版1900立即值，以獨立借位／popcount／有號範圍驗旗標；真實指令及MOV CH,AL／後續shift／word MOV consumer、截短／拒絕／既有word ADD／OR／AND／83 SUB與dword SUB回歸。全部CPU與固定EXE全套、兩同48M Esc／明示日曆／50M自然完整SUB與後續原始寫回後才限定CONFORMED。

## 邊界與回填

只補原版日期caller的CPU缺件，日曆值為受控平台初態，不宣稱原版當時日期或RNGseed。310日期輸入／近似與主選單、255／299、玩家路徑及整款remake未知保持。原始EXE、RAM、終端與PNG只留本機。不可變鍵固定EXE＋高位LE0x240A34＋66 81 E9 6C 07；閉合後須回填310及startup_probe_131.py護欄，309 AH2A屬不同鍵由310處理。

## 原版診斷與READY審查

310兩正常固定48M Esc／明示epoch診斷已真正AH2A返回：第48796894步原caller完整R的AL01／CX07CC／DX0101，其他R／六段／flags216h保持；下一第48796895步高位LE0x240A34、66 81 E9 6C 07拒絕，完整R／段／flags保持，EIP到240A37只是fetch，非運算成功。兩gzip e7afe56b8d82ca4bdecc8a5a943b94880d6dd12a05b3ac8ebf0f0acdfc794254／b8e0f1bf7c46256b316edd9a8f83eba3f404cc577d5916dbbc605d2eee4e2b42，仍是全套未通過前的有界診斷，不冒稱正式驗收。來源與公開算術足以READY，只補既有81的word暫存器group5、完整iw讀取後沿sub16；不改日曆或原版資料來避開指令。

## 實作與原版完整 consumer

限定CONFORMED：沿既有81的operand16解析，只容許暫存器group5，完整iw讀成功才用既有sub16與低word寫回。其餘字寬、memory、group與prefix邊界保持。三個CPU測試驗八目的／全部65536低word對1900、49組word邊界、完整iw、六旗標以獨立無號／低半byte借位、結果byte popcount與有號差值範圍推算；高word／其他R／段／非算術旗標、截短／prefix／memory／未知group拒絕與FPU保持、word ADD／OR／AND／83 SUB及dword SUB回歸、真正後三consumer通過。310日期三測試不改預期即可通過。

CPU SHA-256 cfe5bc387aee00907acd897c3c6b77a5e4186828250e53fbc0513d0e40b504e1；internal/cpu386/sub_word_imm16_test.go SHA-256 9214d2e870df40feec41e64a5dec9b481a365877137e497f0de9b62044825587。其餘日曆／探針來源、Go1.24.13與固定工具映像、全部ZIP／EXE／MOX.SET、600s／UID1000／network none／2GiB／2CPU／128pids、六測試／固定EXE全套／兩正常明示epoch與未設定第三流程的完整命令與SHA-256集中[310-moo2-dos-calendar-date.md](310-moo2-dos-calendar-date.md)，同次收據，不另外重做或冒稱硬體語料。

已證實，兩設定日期的原版正常流程各有兩筆真正SUB：

| 步數／dosgolem高位LE | 真正前後值／flags／保持 |
| --- | --- |
| 48796895／0x240A34→0x240A39 | 66 81 E9 6C 07，ECX000007CC→00000060，flags216h→206h |
| 48796931／0x240A98→0x240A9D | 相同SUB bytes，ECX000007CC→00000060，flags246h→206h |

兩筆六算術旗標為CF0／PF1／AF0／ZF0／SF0／OF0，六段及所有其他R保持，目的高16位保持0。高16非零由八目的全word測試補足，不稱原版當次非零高word樣本。每筆再實際MOV CH,AL把ECX變00000160、SHL ECX,16變01600000、MOV CX,DX變01600101，flags的shift未定義項僅按既有近似記錄。第一筆高位LE0x240A41的89 4C 24 08原始dword寫回SS:002BDB90；第二筆0x240AA5的89 4C 24 04寫回SS:002BDB8C。兩32byte前後窗口只有對應四bytes由原值改為01 01 60 01，完整R／段／flags保持。這些是原版CPU真正寫入，沒有以測試代寫日期結果或套用推測欄位名。

日期前完整309基線、兩次公開服務返回、兩筆SUB的全部R／六旗標／六段、16條下游caller與兩次真正堆疊寫回已獨立審查；兩設定排程的全部受驗狀態一致。未設定第三流程完整維持原309拒絕，不把初態缺件填為正式日期。全部Go CPU／固定EXE全套通過SHA-256 ee661ea0e8fb6e464337402fbeb5a15a009a5a9c4c664abc8d313550b12978b2；兩自然gzip 950f0e6690aad7f7546e2dcdcfb8ddd6aeeb4b398aa001c14a483b359d5ba1f8／09b176610ba23b5f6b221564659a3666ed7bd8d2589b2f984a86e3e66ab4bc79。

新停點第48797763步高位LE0x210C7E、66 03 05 A4 BE 29 00的word ADD記憶體來源；兩時計58554306，IRQ0完成8022、IRQ7完成304。影像仍與已檢視黑色過場逐位元相同，主選單未見。原版實測限於這條明示日期初態，255／299／完整鍵盤、AH2C時間／RNG、玩家流程與整款remake仍未知；311不宣稱完整CPU或未測形狀。下一步公開ADD契約與該來源／下一66 A3寫回，不分析整個runtime helper。

## 解析回填

word SUB完整立即值停點已由規格 311 接通。310必須保留此標記與本檔連結；它的初始拒絕收據保留，現行依以上有限CPU／正常consumer範圍。不可變鍵固定EXE＋高位LE0x240A34＋66 81 E9 6C 07，另保留同bytes第二caller0x240A98；309的AH2A是不同鍵由310處理，其他word SUB／dword或runtime用途不擴張。驗證入口apps/moo2/tools/startup_probe_131.py --check-sub-word-imm16-spec-backlinks；缺定位、兩筆SUB／六旗標／consumer／原始寫回／收據或310回填必拒絕。

同次53個回填函式／45新增負例與兩CLI通過，所有受驗來源與兩設定／一未設定正常收據保持；索引、所有修改與輸出UID:GID1000:1000，工具工作樹root-owned／.md目錄自檢空，Go容器已退出移除。

## 2026-10-03 ADD來源停點勘誤

word ADD記憶體來源停點已由規格 312 接通，見[312-cpu386-add-word-memory-source.md](312-cpu386-add-word-memory-source.md)。同一固定EXE／高位LE0x210C7E，來源0002h＋AX000Fh、flags216h及真正DS0188:0029BEA2兩byte寫回已驗。原拒絕收據保留；兩明示日期正常流程現在停0x14E3DE word CMP完整立即值，已見原版Loading畫面，其他日期／時間／RNG、音訊、玩家流程與本規格範圍未擴張。

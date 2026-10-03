# 348：配置母星進度函式返回與上層原分支

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03

沿[347原進度文字](347-moo2-home-worlds-text-source.md)，主庫c6b990d5246414cf895422b8df14cd30cc03a5db／工具53243f6d5633380f456a9c27faedf908cbade675。維持原正常輸入、160M預算與固定日期，固定日期不是RNG seed。主庫RE-first保持，不猜欄位用途或修改玩法。

## 固定來源與最小問題

官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30、正式probe ca5643c8d598cb705914cbbcfdc79ef571c6d603960b9395f552c7defe52c546不改。347的10530原列／36PNG是歷史基準。

已證實：原dosgolem_high_le:16C78E prolog五PUSH再ENTER，347於152598618的SS188:EBP2BDB5C＋24保存16B98A，實際caller16B985。這只證明返回位址，最終RET未捕捉。IDA linear EA caller7B985返回7B98A後CMP word[EBP-8],0及JNZ，欄位用途未知。

## 蒐證契約

1. 固定1.31原EXE建立一次性IDA9.4 DB；正式.i64唯讀。只匯出sub_7C78E邊界、共享退出尾端與caller後續分支，保留原名、線性EA、file offset、bytes、operand及工具版本。
2. private可丟棄probe只使用既有activationPeek／描述符界限檢查，讀原RAM／核心／Bus／FPU／VBE；不注入guest、不換Bus／hooks、不改CPU。原caller／入口／原copy完成後首上層返回／Jcc各只取首例；等待超過160M就明示未返回，不提高cap。
3. 捕捉實際RET時，必須驗原stack、指令bytes、執行前後ESP／EIP與正常caller返回；只讀保存位址不能當作RET。
4. 全部10530原347列／36PNG與160M終態以既有mtime／DTA／每次RAM規則比較。固定1996-01-01及原99M press／99084355 release不變。
5. 尚未READY前只跑private蒐證，不接正式診斷。若仍未返回，保存未知與最小下一步，不用helper拆解取代完整開局。
6. 原EXE／LBX／RAM／LOG／PNG／IDA DB與私有腳本留既有忽略workplace，公開只含定位、證據等級、雜湊與回填。新文件同時列入000-index。

Go1.24.13、既有IDA9.4 locked-v1沿347已驗image／UID1000組合；Docker network none／2GiB／2CPU／128pids／outer timeout，原ZIP／patch／正式DB唯讀，輸出只寫workplace／tmp。收尾驗擁有權與容器清理。完整生成／開局、母星配置規則、正式writer、RNG及remake同狀態仍未知。

## READY 證據審查

private原160M正常流程通過：153214295原dosgolem_high_le:16BD86 C3正常RET，153214296到16B98A；原ESP2BDB74→2BDB78、EBP恢復2BDBA0、stack原8AB91600。153214297的16B98F原JNZ因caller的SS188:2BDB98 word0000／ZF1而不跳，153214298到16B995。callee local EBP-8的7779與caller word0000屬不同框架，不混用。

固定EXE的IDA linear EA sub_7C78E有103原指令；7C8E1 E9 71 F6 FF FF→7BF57 C9，再7BF58 E9 24 FE FF FF→7BD81，依序5F 5E 5A 59 5B／7BD86 C3。只附語意，不改原名稱／位址／bytes／operand／file offset。正式DB保持唯讀，兩次一次性DB輸出schema1／固定input SHA／UID1000通過，idat exit1不是失敗判準。

全部10530原347列／36PNG與160M終態保持。11個新事件核心／Bus／FPU／VBE及完整RAM readonly。證據已足夠把兩有界觀察區塊接到正式probe；只能改348標記的private名稱，執行語句必須逐byte等於private。universe160關閉即返回，defer也不輸出；原CPU／平台／輸入／日期／cap不改。驗證private160M、正式建置／舊CLI、正式關閉8M、來源逆轉與回填後，只將本觀察契約列為CONFORMED。母星配置規則、完整生成／開局與remake同狀態仍未驗。

## 限定驗收：原進度函式RET與上層零分支

348限定CONFORMED。只閉合原進度函式返回與一個原caller分支，不代表母星配置完成。完整生成／開局與remake同狀態仍未驗。

| 事件 | 原步 | dosgolem_high_le EIP | 原ESP／EBP |
| --- | --- | --- | --- |
| 原caller CALL | 149825343 | 16B985 | 2BDB78／2BDBA0 |
| 原callee入口 | 149825344 | 16C78E | 2BDB74／2BDBA0 |
| 原NUL複製後 | 152598741 | 16C8B6 | 2BDB48／2BDB5C |
| 跳共享退出 | 153214287 | 16C8E1 | 2BDB4C／2BDB5C |
| 原LEAVE | 153214288 | 16BF57 | 2BDB4C／2BDB5C |
| 五POP開始 | 153214290 | 16BD81 | 2BDB60／2BDBA0 |
| 原C3 RET | 153214295 | 16BD86 | 2BDB74／2BDBA0 |
| 原caller CMP | 153214296 | 16B98A | 2BDB78／2BDBA0 |
| 原JNZ | 153214297 | 16B98F | 2BDB78／2BDBA0 |
| 原不跳分支 | 153214298 | 16B995 | 2BDB78／2BDBA0 |

**已證實**：固定原CALL E8 04 0E 00 00後一原步進16C78E；entry原SS:ESP存8AB91600。原五POP恢復EDI／ESI／EDX／ECX／EBX，RET C3真正執行後ESP＋4且返回16B98A，EBP恢復原caller。caller word0000位於SS188:2BDB98，CMP 66 83 7D F8 00→flags246h／ZF1，JNZ 0F 85 77 01 00 00不跳，下一原步16B995。callee local7779位於另一框架的EBP-8，不能當caller的條件值。欄位用途仍未知，只證明raw讀值與原分支。

**已證實，原定位**：IDA linear EA的7BD81..7BD86為5F／5E／5A／59／5B／C3；file offset1053653..1053658。原caller7B985／7B98A／7B98F／7B995的file offset分別1052633／1052638／1052643／1052649。已逐項與dosgolem_high_le CALL／共享退出／RET／CMP／JNZ bytes核對，各工具基準分開標明；不外推其他版本。原sub_7C78E邊界7C78E..7C8E6／103指令，保留原名及operands。

**強推論，下一個呼叫**：160M原caller返回槽2BDB74已變成E3BA1600。既有固定IDA caller窗口的7BADE E8 30 F2 FF FF呼叫sub_7AD13並返回7BAE3，支持當時後續dosgolem_high_le:16BADE→16AD13尚在執行；本輪未捕捉該CALL入口，保存位址不能當實際命中或返回完成。只以此縮小下一個觀察，不猜其規則。

## 保持、正式觀察與可重生入口

全部10530原347列／36PNG保持，以既有mtime／DTA／每次RAM規則正規化。11事件及終態核心／Bus／FPU／VBE、RAM before／after一致。終態160000000／17FD04／unique_sites39434，無新CPU拒絕；原PNG仍Placing home worlds...。原160M正常輸入不重送、99M press／99084355 release保持，固定日期不是seed；probe exit0只是上限。

DRAFT可丟棄觀察取得證據、READY審查後才接正式兩區塊。正式與已驗private只更名348標記，執行語句相同；兩正式區塊及新增空行逆轉逐byte保持347。private SHA-256 9a5b920cb3c167eff8d2e472ce780fab37ccd56bc9b88fd9595078e23c8be8a7，正式2aa4d46019d0c532fef169ba686f48aaadc347ccdb9b4b49928b8054a2dc8571；CPU／平台不改。universe160關閉立即返回，defer不輸出。正式關閉8M啟動1693原列／PNG保持，沒有home_return_或progress_text_觀察；它不取代正常160M。舊338的17／339的22／346的29 CLI負例及100M／120M／160M參數正對照保持。

容器內入口：bash workplace/new-game-348-run.sh；正式go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；bash workplace/new-game-348-off-run.sh。python3 workplace/new-game-348-verify.py／new-game-348-source-verify.py均PASS。IDA原EXE另複製至tmp建一次性DB，正式.i64唯讀；兩非空JSON／schema1／input SHA／UID1000通過。不重跑120M或無關CPU全套，既有346／344回歸只稱歷史已驗。

原160M收據SHA-256 33623a56218c0c0684b9513704266928c47ed49488ae880b5a55eb9181c73efa；原final PNG c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca／RGB4a1d9efc9d15da575e330128f22d27e97f6d6616ac1b4184ec94efc2bd27f2c9。只沿347原圖byte核算，不冒稱新玩家頁。

來源逆轉的初次核算漏移除新增空行而拒絕；diff只有兩個空行，修正核算腳本後逐byte通過。原private與正式執行語句未因此修改，不當作CPU或產品缺陷。

## 不可變鍵與回填

| 不可變鍵 | 已證實／未知 | 較早規格 | 必須勘誤 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:16B985→16C78E→16BD86→16B98A／16B98F | RET／raw零值／JNZ不跳已證實，欄位用途未知 | 347 | 原進度函式正常RET與上層分支已由規格348驗證 |

原後續CALL入口參數及外層等待已由規格349驗證，見[349](349-moo2-generation-entry.md)。本348只憑保存slot的強推論已由153878499的原CALL接通。下一步維持160M，觀察外層迭代／實際上限與16AE07重繪，再決定續行預算，不逐行翻譯runtime或盲提高cap。母星配置規則、生成完成／完整開局、正式writer、RNG、人耳與remake同狀態未知；主庫RE-first保持。

85項規格回填、新348的27缺證據／狀態／較早回填／索引負例及既有負例通過。公開Python語法、diff與擁有權通過。

## 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-348-ida-home-return.py | 1fdd50368529695e682b09cad75f02f999a4c66908697dc884ddb36ddfbcf73b |
| workplace/moo2-348-ida-home-return.json | 2038716b21fe3332fb8044cf01d914637eceb7cbebe55c0650b4762e2784b0aa |
| workplace/moo2-348-ida-home-epilog.py | f944158607c92a47af670c271bdc6ddc7711f164f0880d594036f67c7f72a21b |
| workplace/moo2-348-ida-home-epilog.json | a0c6edcee841854579115800ff5ef0edeec7c0df07ad5d9320c351682fe0993d |
| workplace/moo2-348-home-return.go | 9a5b920cb3c167eff8d2e472ce780fab37ccd56bc9b88fd9595078e23c8be8a7 |
| workplace/moo2-probe-348-home-return.txt.gz | 33623a56218c0c0684b9513704266928c47ed49488ae880b5a55eb9181c73efa |
| workplace/moo2-vbe-348-home-return.png | c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca |
| workplace/new-game-348-run.sh | 82af3816ef9756888432bd3457fb9d8d93892f28964799890e642d129f0ab2f6 |
| workplace/new-game-348-verify.py | 185f38302d53008c56e18bab8cf8a12cf2bc94167de1a67605ca7678d71a5386 |
| workplace/new-game-348-tests.txt | f66285c1243f3493b6bd37ed875a82c9f057405fdb09366ff72b2e1a5af39c22 |
| workplace/new-game-348-source-verify.py | 382e38868bc40b06b7a556fdc1e126715da14de308795c0dc12b88901bf76c28 |
| workplace/new-game-348-source-tests.txt | c029e9776e92d9798835896002d816c2248fbb0c4f745e608980d1b29f01bc94 |
| workplace/new-game-348-off-run.sh | 48569ef0b1b10f22da4a688daf431647567900710ce996e4c51e847d919b78f5 |
| workplace/new-game-348-off-cli-tests.txt | 7d720ece028de31c0f318e14ac3aa0eaf3a62300093ab5410538b8b4007df7f1 |
| workplace/moo2-probe-348-off-old.txt | 46bf56e6965b90579fb24fba65ae70f5e5707110de40149bd0ebd43d75ce121c |
| workplace/moo2-probe-348-off-new.txt | 8ba812f8773be1e48cff2b5146929359e7bbabea8cf8acfbe511c672b3bf4ac5 |
| workplace/moo2-vbe-348-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| workplace/moo2-vbe-348-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| workplace/new-game-348-backlink-verify.py | fcc64554ff1312fadd0d28e67866fdac0784379ea0b36d32be950cc1db3fd2bd |
| workplace/new-game-348-backlink-tests.txt | f9d2f9a204f756743bc680bd15400a4d7e685bad3b553f39e9288d8b60215d08 |
| workplace/new-game-348-frames.json | 79a5115a26fafec11dd48bcf5c89b62ed8a86e6f0596cde819545dda104b2bcd |

公開回填入口：python3 apps/moo2/tools/startup_probe_131.py --check-home-return-spec-backlinks，只核對348原RET／零分支／347回填，不啟動DOSBox-X或修改玩法。

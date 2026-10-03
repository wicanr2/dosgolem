# 349：後續母星生成呼叫的原入口與等待狀態

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03

沿[348原進度函式返回](348-moo2-home-worlds-return.md)。起點主庫ff2c377b2e3a08faba7f3d600a0e547759c4bcdc／工具9afe6570dc3e49e354b9a9ec07360125682b3d67。正常160M預算、原99M press／99084355 release及固定1996-01-01保持；固定日期不是seed。主庫RE-first保持。

## 來源與問題

官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30；probe 2aa4d46019d0c532fef169ba686f48aaadc347ccdb9b4b49928b8054a2dc8571。348原10542列／36PNG為待核對基準。

已證實：153214295原16BD86 C3 RET，原caller的word0000／ZF1使153214298到16B995；160M仍17FD04生成迴圈。強推論：160M保存返回槽16BAE3與固定IDA的7BADE→sub_7AD13支持後續呼叫，但未捕捉實際入口。不能將保存位址當CALL已命中。

## 最小蒐證

1. 固定1.31原EXE重建一次性IDA9.4 DB，只看sub_7AD13邊界、直接玩家生成callee與caller，原名／EA／file offset／operand／bytes／工具版本保持，正式.i64唯讀。
2. private唯讀觀察原16BADE CALL、16AD13入口、該框架與正常返回；只取首例，原參數／stack窗口各有界。若生成callee連到17FC82，僅保存邊界與參數，不猜內部規則。
3. 160M終態保存等待與原框架、已知caller返回槽。未返回即明示未知，不跳過／代寫／重送，不提高cap。
4. 每事件沿既有activationPeek核對核心／Bus／FPU／VBE與RAM before／after，檢查描述符／RAM邊界；不裝CPU hook、不換Bus或修改guest狀態。
5. 全部10542原348列／36PNG及終態依原mtime／DTA／每次RAM規則核對。private不直接進production；READY審查後才可接有界診斷，CPU／平台／主庫玩法不改。
6. 原LOG／RAM／PNG／EXE／LBX／私有IDAPython留本機忽略workplace，公開只存定位、等級、雜湊與回填；同次列入000-index。

沿既有Go1.24.13及IDA9.4 locked-v1已驗image／UID1000，Docker --rm／network none／2GiB／2CPU／128pids及外層timeout；原ZIP／patch／正式DB唯讀，輸出只寫現有workplace／tmp。寫入前／後驗UID1000及root-owned／.md目錄，收尾清理專案容器。完整母星配置／開局、正式writer、RNG、人耳及remake同狀態仍未知。

## READY 證據審查

private160M正常流程已捕捉153878499的dosgolem_high_le:16BADE原CALL、153878500的16AD13入口及153878505的四PUSH／ENTER 0BF8h後框架。原EAX2BDB78=caller EBP2BDBA0-28h，原EDX147D9保持；caller ESP2BDB78、callee EBP2BDB60／ESP2BCF68、保存16BAE3。三個直接子呼叫都已返回，首次16AE7F→16B0B4以AX1／DX3輸入，在29原步後16AE84返回EAX1CC4D。原外層16B01F RET／16BAE3返回未見，160M仍17FD04。

獨立核對全部10542原348列／36PNG與終態保持，13事件readonly。固定IDA sub_7AD13有216指令，原sub_7B0B4有28指令，不是29；原第三子呼叫sub_8EFE1末尾8F052 C2 18 00清理24byte，與實際ESP2BCF48→2BCF60一致。核算初版猜錯指令數、預設每個CALL不清理參數，已拒絕並保留；正式實作前按原bytes修正驗收，CPU／private執行語句不改。

證據足以批准兩有界readonly診斷區塊，正式與private只能更名349標記、執行語句相同。universe160關閉立即返回且defer不輸出；沿既有描述符檢查、activationPeek／完整RAM readonly，原CPU／平台／輸入／160M cap不改。正式建置、關閉8M／舊CLI、來源逆轉、回填與擁有權通過後，限定CONFORMED。外層未返回／完整配置及開局仍未知，不用子呼叫已返回冒稱外層完成。

## 限定驗收：原入口、三直接返回與外層等待

349限定CONFORMED，只閉合原CALL／入口／框架／三直接子呼叫返回／首次距離計算與160M外層pending，不代表母星配置完成。完整生成／開局與remake同狀態仍未驗。

| 原事件 | 原步 | dosgolem_high_le EIP | 原結果 |
| --- | --- | --- | --- |
| 唯一caller CALL | 153878499 | 16BADE | EAX2BDB78／EDX147D9 |
| 原callee入口 | 153878500 | 16AD13 | ESP2BDB74，原slot16BAE3 |
| 四PUSH／ENTER後 | 153878505 | 16AD1B | EBP2BDB60／ESP2BCF68 |
| 第一直接CALL／返回 | 153878512／153878623 | 16AD34／16AD39 | 17E5C5，111原步 |
| 第二直接CALL／返回 | 153878632／153878743 | 16AD56／16AD5B | 17E5C5，111原步 |
| 第三直接CALL／返回 | 153881393／153979600 | 16ADFB／16AE00 | 17EFE1，98207原步 |
| 第一配置段起點 | 153993574 | 16AE0C | 原框架保持 |
| 首次距離CALL／返回 | 153994840／153994869 | 16AE7F／16AE84 | AX1／DX3→EAX1CC4D，29原步 |
| 固定終態 | 160000000 | 17FD04 | 外層RET／16BAE3返回未見 |

**已證實，原入口與參數**：原CALL E8 30 F2 FF FF→16AD13，次原步進入；EAX2BDB78=原caller EBP2BDBA0-28h，EDX147D9=83929。四PUSH EBX／ECX／ESI／EDI後ENTER 0BF8h，callee EBP=caller ESP-24=2BDB60、ESP2BCF68；原SS188的caller ESP-4保存E3BA1600。原參數含義未知，沒有命名或猜補用途。

**已證實，直接返回**：前兩原CALL 17E5C5各111步且ESP2BCF60保持。第三CALL 17EFE1以原ESP2BCF48進入，返回16AE00時ESP2BCF60；固定IDA linear EA的8F052 C2 18 00清理24byte，file offset1132198。這是原正常參數清理，不能預設所有CALL返回都保持ESP。

**已證實，首次距離callee**：16AE7F E8 30 02 00 00→16B0B4，AX1／DX3；153994869到16AE84，EAX1CC4D=117837。原IDA sub_7B0B4有28指令，先以71h stride讀兩records的signed word +0Fh／+11h，再各做差、平方、相加；原字節／operand保留。只證實距離平方形式與本次原返回，座標單位、records正式名稱、上下界、完整母星配置規則與remake對齊未驗。此callee無call，不把它誤稱為進入17FC82的等待來源。

**已證實，工具基準**：固定IDA linear EA sub_7AD13邊界7AD13..7B020／216指令，唯一direct caller7BADE file offset1052978 E8 30 F2 FF FF；入口file offset1049447。原nested sub_7B0B4範圍7B0B4..7B0FB／28指令，原RET7B0FA C3 file offset1050446。逐項與dosgolem_high_le原CALL／PUSH／ENTER／直接返回核對，保留原名、位址、file offset、bytes與operand，不外推其他版本。

**已證實，等待邊界**：13事件readonly，seen的0..11與16為true，12..15為false，returned=false。160M原保存外層slot16BAE3與框架仍在；沒有原16B01F RET／16BAE3返回收據。只稱觀察範圍內pending，不將保存返回位址冒稱完成，也不因17FD04仍在就判CPU故障。

**未知**：外層全部迭代與剩餘工作、後續重繪分支16AE07、正式持久writer、原frame欄位語意、母星配置完成／完整開局、存檔、RNG、人耳及remake同狀態。首四原迭代與該次實際比較界限36已由350驗證。

## 保持、接線與重生

全部10542原348列／36PNG及160M終態保持，沿既有mtime／DTA／每次RAM規則正規化；原home_return_observation的RAM before／after各次都相等，不要求跨次RAM SHA相同。新13事件核心／Bus／FPU／VBE、完整RAM before／after一致，不代寫guest、不換Bus、hooks或原輸入。原final PNG c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca／RGB4a1d9efc9d15da575e330128f22d27e97f6d6616ac1b4184ec94efc2bd27f2c9仍配置母星。終態160000000／17FD04／unique_sites39434／無新CPU拒絕；probe exit0只代表cap。

READY審查後接正式兩區塊，只更名349標記、執行語句與已驗private相同；逆轉後逐byte保持348，CPU／平台不改。private fe1b7a036bcedfd0bc26705cb86b1fcd26973ae28fbac7058f5a1d64eac8f9dc，正式 f2826d243f01ff4b6659aa45d4faae635d218dc779e8357b486f04b5870471be。旗標關閉不讀guest、不新增defer；正式8M關閉1693原列／PNG保持，沒有generation_entry_／home_return_／progress_text_新增觀察。68舊CLI負例與100M／120M／160M參數正對照保持。8M不取代正常160M，本輪不重跑120M或無關CPU全套。

原160M收據SHA-256 c47496ca8f677245371ca239a5a945baf7f56747e0cbd7e2471d81c55e24126d。Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac、IDA9.4 image sha256:6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780、network none／UID1000／2GiB／2CPU／128pids，原版timeout450s、正式8M／CLI180s、IDA90s。原ZIP／patch／正式DB唯讀，官方EXE只複製tmp建一次性DB；三非空JSON／schema1／input SHA／UID1000通過，idat exit1不當失敗。

容器入口：bash workplace/new-game-349-run.sh／new-game-349-off-run.sh。正式go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；python3 workplace/new-game-349-verify.py／new-game-349-source-verify.py PASS。原EXE／LBX／RAM／LOG／PNG／私有腳本仍在忽略workplace，不入Git。

初版驗證把28指令猜為29而拒絕，原腳本及stderr已保留並重現exit1；另外原第三RET會清理24byte，按IDA原C21800修正核算。CPU／private執行語句未改，不列CPU或產品缺陷。不要從probe用到29原步反推函式有29指令，CALL本身也是一步。

## 不可變鍵與較早回填

| 不可變鍵 | 已證實／未知 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:16BADE→16AD13／16AE7F→16B0B4 | 原入口及首次距離返回已證實，外層160M pending | 348 | 原後續CALL入口參數及外層等待已由規格349驗證 |

下一步依350核對原JL不跳出口與後段／RET，維持160M；不由四次樣本推算整段完成時間、不逐行翻譯繪圖helper或猜欄位用途。主庫RE-first保持，整款remake／中文化仍未完成。

86項回填正對照／新349的32缺證據、狀態、較早回填與索引負例／既有負例通過。公開入口：python3 apps/moo2/tools/startup_probe_131.py --check-generation-entry-spec-backlinks，只驗原入口／直接返回／pending及回填，不啟動DOSBox-X。

## 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-349-ida-generation-entry.py | c4f6fd6abb0fd757447cc4d33450057b3901ffc3c53305a169477e59e7b93a42 |
| workplace/moo2-349-ida-generation-entry.json | 46f87208bd5896c95af4598c1e1cfbea6fe23114841eb9852462b0c29b153428 |
| workplace/moo2-349-ida-generation-nested.py | 603f5a87dbeee3203ae3c0e866f82f1998fee198fb31e0b1b95d532f2b3d3f22 |
| workplace/moo2-349-ida-generation-nested.json | 07944763f6098d52665442faae7729a3b1cf1811cb4a98980c79100bdcfc8a6a |
| workplace/moo2-349-ida-ui-ret.py | f8b9754475957a5c9ca09ce98161546efc72ee4b48a38205e87a78f4c35943e7 |
| workplace/moo2-349-ida-ui-ret.json | 0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec |
| workplace/moo2-349-generation-entry.go | fe1b7a036bcedfd0bc26705cb86b1fcd26973ae28fbac7058f5a1d64eac8f9dc |
| workplace/moo2-probe-349-generation-entry.txt.gz | c47496ca8f677245371ca239a5a945baf7f56747e0cbd7e2471d81c55e24126d |
| workplace/moo2-vbe-349-generation-entry.png | c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca |
| workplace/new-game-349-run.sh | 573f9a810a93136e7a5a78dfdb8d29242c3c481c98ac7d46007db3e5b981a97a |
| workplace/new-game-349-verify.py | 16d3e5bc7a4d83a015953b819ca4b5727b017887e30b0350e52c0952f932341c |
| workplace/new-game-349-tests.txt | a7e3f2e45420a231c00270496b8b4b3f6dbc19c455edde78c5281d360184ceca |
| workplace/new-game-349-attempt1-verify.py | 92335a08ba904116c43ace623338adae1decb5872986db2ee62a4e631feab304 |
| workplace/new-game-349-attempt1-tests.txt | 5915c4337c93e40c5141b6768754cc4313c1fb1fd50f533cb193e5e2a51bb5d3 |
| workplace/new-game-349-attempt1-verify-output.txt | 4c5c68a11c199e993db2d08da691f6ef2ebe4225d2f771ed694ca70509e91139 |
| workplace/new-game-349-source-verify.py | 1c291c9852cee4cd85d767f47dfc13fd33a4166b1ebb76c1a1e945bc9719faf2 |
| workplace/new-game-349-source-tests.txt | 8cc211bb15558b6950cfc0a183b951fdd8238a2860ef46fb09846281d43e4399 |
| workplace/new-game-349-off-run.sh | fb90870320e62dcc800601f43bd201be8c7259ba187d01e42ac5d2847544a43c |
| workplace/new-game-349-off-cli-tests.txt | 7d720ece028de31c0f318e14ac3aa0eaf3a62300093ab5410538b8b4007df7f1 |
| workplace/moo2-probe-349-off-old.txt | faeb64440db8b4a958ea16263211a19f5a0723f5a64dbaef12f52886901812f9 |
| workplace/moo2-probe-349-off-new.txt | 0e6bfc4639a8292e1b66db5fdd325f8ea73165ff50306c380907946bbb301d68 |
| workplace/moo2-vbe-349-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| workplace/moo2-vbe-349-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| workplace/new-game-349-backlink-verify.py | 22d2e06a78f4530bc63ad6d01537ba1c2c1d228dadc8e44e692b6e466f8cfc1c |
| workplace/new-game-349-backlink-tests.txt | 7091c1cf8e1e8435873cfa8ec2baa81b78ad531d9432fd5bbfad253badee99e9 |
| workplace/new-game-349-frames.json | 661ebbf70b113592d12355fa40b2632acbfdc68feea7116b2ce315b96584d098 |

## 350後續回填

原首四迭代與實際比較界限已由規格350驗證，見[350原迭代與界限](350-moo2-generation-iteration-bound.md)。原SI1..4後CMP2..5／signed36，首四組重繪未見，第5索引使觀察飽和。全部出口／RET／剩餘工作與完整開局仍未知。本文最初的未知是349快照，不重新開啟已驗首四迭代及該次bound。

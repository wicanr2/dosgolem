# 306：AL 與立即 byte 的 XOR

狀態：**CONFORMED**
日期：2026-10-03
範圍：cpu386裸34 ib；不改玩法或放寬其他指令形式。

## 原始輸入與契約

工具基線dcf764ed14d1b141948968fd8ebc596ee3dec851。[305](305-moo2-irq7-real-mode-passdown.md)保存固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、417原檔／MOX.SET553bytes及兩自然收據。第42488059步，dosgolem高位LE線性0x247BE1，原始bytes34 01 C3，停止於未支援34。解碼失敗後EIP0x247BE2；其他CPU狀態尚未提交。完整R=1 57F 0 1 2BDB6C 2BDBB4 3259A0 3D6978、段8 188 188 0 20 188、flags297h。此定位／bytes與狀態已證實，不命名未知函式用途。

[Intel80386 XOR契約鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XOR.htm)定義34 ib只將AL與imm8逐bit不同結果寫AL，EAX其餘24位保持。CF／OF清零，SF／ZF／PF取八位結果；AF未定義。沿既有setLogicFlags8清AF模型，明標工具近似，不稱硬體未定義值exact。正常1 XOR1預期AL0、flags246h，其餘R／段／FPU／記憶體及非算術旗標保持，EIP到0x247BE3。

## READY 最小契約與驗收

- 只接無66／67／segment／REP／LOCK的34 ib。完整取得立即byte才提交AL與旗標；截短／前綴拒絕不發布寄存器或旗標，解碼EIP可前進，錯誤保留原始起點。
- 256×256×兩種初始旗標以既有獨立逐bit比較／bit計數驗結果與定義五旗標；AF清除另驗模型。核對完整外層狀態、記憶體不寫、全部前綴／截短拒絕、原始1／297h與EAX高24位保持。
- CPU全套／固定EXE全套後，兩自然依305相同50M、分離DOS、固定輸入與toolchain重生完整XOR前後態、真正C3近返回／caller消費及下一停點，不注入CPU／遊戲資料／亂數或提高上限。有限唯讀探針只觀察原指令。
- 垂直鏈為原始bytes與AL → typed byte解碼 → XOR與五定義旗標 → 原版返回／caller正常續行。不改主庫資料、UI或存檔，不當完整玩家路徑完成證據。

證據足夠建立窄契約。診斷停點及公開XOR定義待審查READY後才修改CPU。原版資料、gzip／PNG留本機。索引見[000-index.md](000-index.md)。305平台近似／255／299／鍵盤IRQ1／人耳／主選單／受控亂數與整款remake限制保持。

## READY 審查

305兩自然完整CPU停態、原始34 01 C3及公開XOR契約已核對；裸34立即byte完整取得後提交，五定義旗標與AF模型／高24位保持及拒絕邊界足夠，審查READY後才修改CPU。自然RET／caller真正消費與下一停點仍待驗，其他指令及玩法不擴張。

## 三組自然結果、返回消費與 CONFORMED 範圍

審查READY後才接裸34 ib。完整imm8取得後以既有byte目的寫回與setLogicFlags8提交，其他形式與前綴不放寬。256×256×兩初始旗標、全部高位／外層／FPU／記憶體保持、11種前綴與截短拒絕、原始1／297h及高24位非零案例均通過；結果由獨立逐bit比較與bit計數產生。AF清除另驗工具模型。固定EXE全部CPU與機器層通過。

兩自然沿305固定417原檔／MOX.SET／EXE／Go1.24.13映像、600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none。命令只換306輸出名，不注入CPU／遊戲資料／鍵盤／時計或提高50M上限。原305首XOR以前完整收據逐列保持，只排除解壓mtime與DTA四byte。新增探針每次最多三組XOR與後面三條原指令，只讀完整CPU／原始bytes／SS:ESP四bytes，不改hook結果或堆疊。

| outer_step | AL前→後 | flags前→後 | 實際消費 |
| --- | --- | --- | --- |
| 42488059 | 1→0 | 297h→246h | RET後MOV ESI,EAX得到0 |
| 42596057 | 0→1 | 202h→202h | RET後MOV ESI,EAX得到1 |
| 43335828 | 1→0 | 297h→246h | RET後MOV ESI,EAX得到0 |

三組XOR皆高位LE0x247BE1→0x247BE3，AL以外R／全部段保持。下一條原始C3近返回從SS:ESP真正讀DF 1A 23 00，即0x231ADF，ESP加4、EAX與旗標保持。caller高位LE0x231ADF的83 C4 04將ESP再加4，旗標到206h；0x231AE2的89 C6真正把完整EAX寫ESI，前兩組得到0／1，第三組0，其他寄存器／段及flags206h保持。沒有以孤立helper或人工跳轉取代RET／caller鏈。

兩自然都到50M上限而無未知CPU或平台停止，高位LE0x21588F、完整R=FFFFFFFF 108 178 15A 2BDB70 2BDB90 46 260C2B、段8 188 188 0 20 188、flags246h。IRQ0 started8411／completed8411／failed=false；IRQ7 started386／completed386、最後73步EOI／22E／IRET成功，IRQ7Deliveries387包含較早實模式1次。兩時計61913470，DMACompletions386、block剩1959、current4059h／countFA6h／信用391200。從C6返回信用926100加(61913470−43985659)×44100獨立整除為790617個傳輸sample，餘391200；對應386個2048byte block與4096byte ring重載。PCM只收錄前65536byte，不能把記錄容量稱傳輸總量。

兩PNG SHA-256 535e27c45ba579132ef36398335f0c773aa4889473d8336f7180184290e7e975，實際檢視為星空片段，主選單未見。VBE indexed SHA-256 507767e686cc6586b1c54b9ec5f87e1e6a7aa6c81e23c89bb5e2658d77b6a601、RGB SHA-256 4bd77ff30725abb3740f1cedf085d9e2fdea350214ef650d30885ed854cf41d4。全部CPU／裝置／影像終態相同，受控滑鼠回呼0／1另列；unique_sites17591／17621含回呼，不當玩法差異。正常鍵盤尚未安裝，BDA隊列空、60／61／64埠讀取零。原版AH2509的保護模式IRQ1 8:21C4D8已由303保存，下一步依正常輸入／IRQ1公開契約補有界平台入口，不用BIOS入隊猜補。

限定CONFORMED：裸34 ib的已定義byte結果與旗標、真實RET／caller消費。AF近似／硬體時間、255完整游標、299自然OF=1、後續鍵盤／音訊／主選單／正常玩家操作、受控亂數、人耳與整款remake未完成。主庫玩法RE閘門不變，不因50M無拒絕稱原版完整同狀態。

| 自製來源或本機收據 | SHA-256 |
| --- | --- |
| internal/cpu386/cpu.go | cc9317a6be192c44e16ac1ce4553caa7a7a18a6d926ea431e3863fe3de00dd4c |
| internal/cpu386/xor_al_imm8_test.go | 97210f062fd96ccf6c53e32e92727398f8244e76bae0b107a4a2ae4946fddcaf |
| workplace/moo2-probe/main.go | e0b9695a6cf5817442f29c032c3f69a341b50d85a111edef7992de4d8a6a4f44 |
| workplace/moo2-306-xor-al-tests.txt | 2837250b74140692761843480185d09089176fe8ff440341294537b7c1fc1001 |
| workplace/full-test-306.txt | 43bf923f2d3c8d7d01a9d231fd5f2cc273c9157c945108c197fac3c15bf32bf8 |
| workplace/moo2-probe-306-full-game.txt.gz | 7a984c68b925623589aa6dae45973ef8431b98b72df65523dbed65eaeb485720 |
| workplace/moo2-probe-306-mouse-event.txt.gz | d4526b29dc51b057566604e4538c0adb4c379b06ad005a589e38d59a2a84506e |

實際驗證命令：GOMAXPROCS=2 GOCACHE=/tmp/go-cache go test -p 2 -buildvcs=false ./internal/cpu386 -run TestXORALImmediate -count=1 -v；DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1。兩自然沿305完整命令只換306輸出名。305平台四來源未變，CPU僅接34。

## 解析回填

不可變鍵：固定1.31 EXE雜湊＋dosgolem高位LE0x247BE1＋34 01 C3。305舊停點同次追加「XOR AL立即值停點已由規格 306 接通」及本檔連結，保留原始IRQ7／CPU停點收據。回填入口apps/moo2/tools/startup_probe_131.py --check-xor-al-immediate-spec-backlinks；缺裸34定位、完整AL／五旗標與AF邊界、0x231ADF／0x231AE2真實返回／MOV消費、兩收據或舊標記必須拒絕。其他XOR寬度／形狀與既有音訊回填不受影響。

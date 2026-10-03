# 331：非零事件分支的按鈕判定與正常CALL返回

狀態：**CONFORMED**（僅96 caller步、五自然返回與第一筆跳過）
日期：2026-10-03
範圍：原版probe唯讀觀察；不改CPU、平台、CLI、正式輸入與remake玩法。

## 已有證據

工具34d758498f931d9dc155c4ca93309dd98646328e；固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。沿[330](330-moo2-event-return-caller.md)原417根檔／MOX.SET、44M Esc／1996-01-01、47850592按下與47851578移動放開及原50M／100M模式。固定日期不是RNG seed。

330已證實47863859 C3返回20DB69／EAX1；47863860 TEST後flags202h、47863861 JNE跳20DB87。下一步追這條非零臂的原始判定與指令消費，尚不能命名26C4xx欄位或宣稱NEW GAME被激活。

## 唯讀契約與停止線

只在明示phasePrefix、正常點擊已放開、callback非活動時，於第一次20DB87開始一組。最多96個實際caller Step收據，從起點最多8192個外層Step，任一上限只停觀察、不停原程式。保存每步完整R／六段／EIP／flags／16指令bytes、callback前後計數，直接有界RAM peek存DS:2A121A的16bytes、DS:2A11EC四bytes、DS:26C480的192bytes、DS:29BE0E的16bytes、當步DS:[EAX]的8bytes、固定初始SS:EBP-160的320bytes與當步前後SS:ESP四bytes；另存當步DS:[EBX]64bytes的原定位與可讀旗標，只作未知原值，不從暫存器名推定結構。peek前後核對核心／FPU／Bus／VBE保持。

遇到真正CALL E8只記caller當步及原新return，不觀察callee內部。實際target／ESP-4／新SS:[ESP]=下一EIP吻合才等正常返回；等待期間原程式照常Step，不跳指令、不改hook、不呼叫或代寫。只在實際EIP=原return、SS／ESP均回到CALL之前且callback非活動時續觀察，記resumed=true與被省略步數。8192上限內沒返回，原值記未知，不加預算。無法驗證或未支援的CALL形式停止觀察，不能當一般caller步。

首個實際caller RET C3，以SS:[ESP]／ESP+4／真正EIP核對後停止。錯誤、callback／IRQ轉向、兩預算到期也明確停止。未知指令保存bytes，未獨立核對者不標已證實；不追compiler helper、callee或整個renderer。

## 驗收與回填

先READY才改probe。Docker固定Go1.24.13映像、600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔。go build後同330兩50M與獨立100M正常流程各跑一次。扣除331新列後全部330舊列，除既定mtime、DTA四byte、PNG路徑與每次RAM雜湊外保持；72既有PNG逐位元保持。連續caller步核對完整核心／視窗接續，CALL／resume段明示空白、不假裝逐步連續。正常RET、CALL返回值及已獨立驗的CMP／Jcc以原始bytes核算；高層NEW GAME命令語意依實際結果保留未知。

CPU／平台／CLI保持，325固定EXE全套與329 CLI仍有效。新規格同次索引000-index，限定CONFORMED後回填330與守衛。原版LOG／PNG／RAM留忽略workplace，不公開素材。主庫RE-first、正常開局與remake同狀態保持未知；不更改點擊時長，不追加輸入，不提高原流程cap。

READY審查：開始位址與非零分支由330真實RET／TEST／JNE證實；新功能只保存原始狀態，不命名未知欄位。E8 CALL以原opcode、rel32、ESP-4與新return核對，callee不採指令列；以實際return EIP／SS／ESP續觀察，記明省略步數。沿既有IRQ0State唯讀快照識別轉向，callback／IRQ介入原始caller步則明確停止並留原值，不能套用一般CALL／RET。96收據／8192外層步雙上限只停觀察；所有舊流程與72PNG保持才限定驗收。直接RAM peek不新增讀hook或Bus請求，足以READY。

## 原始來源窗口補充

初次96步取得兩次座標返回500／229，並見DS:26C480來源指標、DS:29BE0E計數與DS:[EAX]的word欄位；原窗口未覆蓋這些bytes，不能把返回值單獨稱已驗來源。保持96／8192界限，globals窗口改為DS:26C480的192bytes，另存DS:29BE0E的16bytes與當步DS:[EAX]的8bytes，各步前後與可讀旗標俱全。EBX窗口仍保留原未知語意。初次三收據留331-initial，正式流程同容器命令乾淨重跑，不改原輸入。

來源窗口READY審查：實際331 bytes已定位26C480／29BE0E與DS:[EAX]讀取端。新增peek皆有descriptor／長度界限，保存frozen當步EAX偏移前後，不經CPU讀hook或Bus；只擴只讀收據，不改96／8192界限或正常輸入。足以READY獨立核對已見CMP／Jcc的原bytes來源，不預設完整命中結果。

## 正式收據與限定結論

Docker固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP／patch唯讀重建417檔，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿330兩50M與獨立100M正式環境換331輸出名。同命令乾淨重跑後，python3 workplace/new-game-331-verify.py PASS：全部3847／4414／6354舊列除既定正規化保持330，72PNG逐位元保持；兩預算的完整新96列相同。CPU／平台／CLI保持，325固定EXE全套與329 CLI有效。

**已證實，僅固定1.31正常單次輸入的caller樣本**：47863862..47864195範圍含96實際caller步與238省略callee步，sample_budget停止，未達caller RET。全部96 caller步獨立原bytes／來源窗口／R／六段／EIP／定義flags／記憶體寫回核算PASS；沒有新CPU拒絕、readonly=true，callback2／2，所錄caller步IRQ沒有介入。IMUL只驗已定義CF／OF，SAR16不把未定義AF／OF列為契約，不以CPU自行清旗標猜原版語意。

五次自然CALL返回都實際EIP／SS／ESP吻合，省略步數[32,32,32,104,38]，返回EAX依序[1,500,229,260001h,0]。前3次從20DB87／20DBF0／20DBF8呼叫213C1B／213BC1／213BEE，於20DB8C／20DBF5／20DBFD續觀察，caller保存事件1／x500／y229到初始SS188:EBP2BDB40的-30h／-38h／-34h。事件CMP1與2為不等，20DB96 JNE到20DBF0；不預設事件值的所有高層分類。

DS188:26C480的原dword指標298848，DS:29BE0E word9、DS:29BE12 dword0。caller從index1開始，IMUL37h得到55 byte stride，實際讀DS:29887F的八bytes 0A00140019002300，四word10／20／25／35；index2讀DS:2988B6的14001E0023002D00，四word20／30／35／45。這是目前判定消費的原資料，不推定每筆對應哪個可見按鈕。

第一筆實際x500與左界10比較後JL不跳；47864164於20DCE5 CMP EDX500,EAX25令flags216h，47864165的20DCE7 JLE不跳、47864166 E9到20DDAE，再INC初始SS:EBP-44h的index1→2。**僅第一筆因x500>25被跳過已證實**；其餘項目與最終命中仍未知。sample96停在index2讀取四word之後，不能稱所有範圍不命中、NEW GAME指令被丟棄或資料錯誤。第一筆尚未走y邊界比較。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-331-baseline.txt.gz | a74ea3d6d2d0af74126ba3df8a8b9e5617464883fc3617d82e5e21ba8ce0b87b |
| workplace/moo2-probe-331-click.txt.gz | 7ec5479ec14b80b6790b08b495e7d1ad0cd6953aa2f857e3b5b135af543a9a88 |
| workplace/moo2-probe-331-extended.txt.gz | ec2a35b53c8e20fa1dbecf205b5f12d241c0fe69a907d233fce8895e5d1265c8 |
| workplace/new-game-331-verify.py | 58094ab282f5e2e0c35212a3500b801f83c658d492fe16c7449f40dce821bbf9 |
| workplace/new-game-331-parity-tests.txt | 2b224220f0a6d7d5c08b30905c95ed37a229ba8cfa08222891608b38075fa251 |

probe SHA-256 05592edc1f377a163687a09f82ef2d1293e94d3577b8bf26ee5c41d2c80b3e23；CPU 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，startup／provider／matcher與330保持。原版素材、LOG／PNG／RAM留本機忽略，不公開。初次缺來源窗口的331-initial僅保存定位線索，不當正式來源驗收。

限定CONFORMED只含96步／五自然返回／原來源與第一筆跳過／舊基線保持；**正常開局／NEW GAME指令仍未知**。下一步同輪核對index2..8與最終caller返回、命中項及目前主選單的關係，優先追玩家阻塞；不提高原流程cap、不改點擊時長、不重點、不深入callee或整個renderer。主庫RE-first、255完整游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG／人耳與remake同狀態保持未知。

68回填函式、既有32／49／25／27／27／34／31／36／31與新增37缺證據負例、兩CLI PASS。workplace/new-game-331-backlink-tests.txt SHA-256 e1ceaca0b3593d9ae9d8b7daeee07b2b42cbe91ca7ab10e8261620885881bb15。原ZIP／patch／EXE／MOX.SET／417檔、CPU／平台來源保持、gofmt／Git差異與擁有權核對通過，工具root-owned／誤建.md目錄自檢空。

## 2026-10-03：後續範圍回填

同輪完整範圍命中與後續CALL已由規格332接通，見[332](332-moo2-button-tail-return.md)。原96步限定結論與收據保留；後續證實index8全畫面範圍命中，NEW GAME仍未知。

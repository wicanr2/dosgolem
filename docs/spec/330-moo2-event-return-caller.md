# 330：正常滑鼠事件返回與首個上層判定

狀態：**CONFORMED**（僅兩組事件返回與首Jcc／CALL邊界）
日期：2026-10-03
範圍：探針唯讀觀察，不改CPU、平台、輸入與remake玩法。

## 來源與目的

工具2bfb2db0f860d115cb0e96e9d1e5938a89a25c23。官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；位址皆dosgolem高位LE。沿[329](329-moo2-bounded-new-game-continuation.md)原417根檔、44M Esc、1996-01-01、單次正常按下47850592／放開47851578及50M／100M流程。固定日期不是RNG seed。

329實際事件讀取47863846於213C60讀DS:2A1228=1；47864834於213A7C讀DS:2A1226=1，均晚於放開，已知按下事件沒有在放開時消失。329時尚未取得這兩函式實際RET及首個上層條件分支，不能推定NEW GAME指令已激活或短按被丟棄。

## 有界契約

只在明示phasePrefix、正常按下已放開、滑鼠callback非活動時，分別於213C60且DS:2A1228=1、213A7C且DS:2A1226=1開始一組，每組只採一次、最多48個真正CPU.Step。停止條件：錯誤、48步用盡，或實際RET C3後首個Jcc／CALL。RET以SS:[ESP]四byte、ESP增加4且實際EIP吻合驗證；若介入IRQ或callback，記錄實際狀態，不能把轉向當RET。未知解碼保留bytes，不補步、不呼叫原函式、不改輸入、不改hook或Bus。

各步保存完整R、六段、EIP、flags、16指令bytes、callback計數；直接有界RAM peek保存DS:2A121A的16bytes、DS:2A11EC四bytes、DS:26C518四bytes與固定初始SS:ESP-16的64byte堆疊，RET另保存當步SS:ESP四byte。所有視窗保存可讀旗標及前後原值。直接peek不計入CPU Bus讀取，不寫guest狀態。快照前後核對R、段、EIP、flags、FPU、Bus身分與VBE狀態保持；不宣稱跨次完整RAM相等。

## 驗收

不改CPU／平台／CLI或正式Go玩法，沿325固定EXE全套PASS。Docker固定Go1.24.13映像、600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP與patch唯讀，乾淨417檔重建。build後50M未點擊、50M正常點擊、100M正常點擊各跑一次。扣除330新觀察列後，全部329舊列除既定mtime、DTA四byte、PNG路徑、每次RAM雜湊外保持；所有舊PNG逐位元保持。新組須逐步連續核對；RET與首Jcc獨立核算，不把未驗指令稱已證實。

新規格同次索引000-index；結果限定CONFORMED並回填329入口與守衛。原版LOG／PNG／RAM留忽略workplace。主庫RE-first保持；正常開局、完整renderer及remake同狀態仍未知。下一步由實際上層結果決定，不增加預算或追完整helper。

READY審查：只用既有descriptor與RAM peek，沒有ReadSegment／Bus／CPUhook新增；48步上限只控制觀察，不控制原程式。每組保留固定初始堆疊及當步RET指標，RET同時核對opcode、ESP、實際EIP與callback／IRQ轉向；首個上層Jcc只聲明原始分支，不命名高層功能。快照讀前後核對核心、FPU、Bus與VBE，真正Step前後則完整列出，不要求狀態相同。50M及100M只增加文字收據，所有329舊列與PNG必須保持。沿已確認平台規格與公開x86 RET／Jcc契約，足以READY；未取得的命令語意仍未知。

## 初次觀察與契約修訂

初次330觀察group0實際RET到20DB69，TEST EAX為1後JNE跳20DB87；group1實際RET到209197，再RET到20DDF2，隨後CALL209325。第一次探針將較後callee內Jcc標成caller_jcc，該標記不成立，不採作正式上層分支收據。最小充分觀察改為實際RET後首個Jcc或CALL邊界；遇到CALL立即停止，不追被呼叫helper。追加保存每步後SS:ESP四byte，獨立核對CALL target、ESP-4及stack return。正式收據須用修訂後同容器命令乾淨重跑三流程，初次列留本機330-initial，不用來宣稱READY驗收。

修訂READY審查：RET後CALL E8只核對rel32目的、ESP-4及新堆疊return，不進入callee。上層無Jcc時以caller_call停止且保留未知。未改原函式或輸入，正式觀測固定最多48步，沿同命令乾淨重跑，初次誤標記不採正式結論。

## 正式驗證與限定結論

固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac、600s／2GiB／2CPU／128pids／UID1000／network none。唯讀原ZIP／patch重建417根檔，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿329兩50M與獨立100M正常環境換330輸出名。CPU／平台／CLI逐位元保持，325固定EXE全套PASS沿用，329 CLI閘門未改。

python3 workplace/new-game-330-verify.py PASS：扣除330新觀察列，baseline全部3847列／click全部4382列／extended全部6322列除既定正規化保持329；全部既有PNG72逐位元保持。每次RAM雜湊僅作各次readonly／前後相同檢查，不宣稱跨次完整RAM一致。50M／100M的完整新32列也相同；未點擊流程沒有新事件組。

**已證實，僅固定1.31正常單次輸入的實際32步與最小消費**：

| 組別 | 原事件 | 實際返回 | 首個上層邊界 |
| --- | --- | --- | --- |
| group0，47863846..47863861，共16步 | DS188:2A1228 word=1，讀後清0 | 47863859，213C83 C3，SS188:ESP2BDAD4的69DB2000，返回20DB69／ESP2BDAD8／EAX1 | 47863860 TEST EAX,EAX令flags206h→202h；47863861 JNE751Ah跳20DB87，ZF=0 |
| group1，47864834..47864849，共16步 | DS188:2A1226 word=1，寫入DS:26C518 word1 | 47864842，213A8E C3返回209197；47864848，20919D C3返回20DDF2；兩次EAX1、ESP增加4 | 47864849，20DDF2 CALL E82EB5FFFF到209325，ESP2BDAD8→2BDAD4，新stack return20DDF7 |

所有32步獨立MOV／POP／RET／TEST／JNE／CALL核算PASS；MOV記憶體前後、三RET指標／ESP、CALL rel32／新return／ESP-4、TEST的已定義flags與JNE分支獨立核對。TEST的AF未定義，不用它當平台契約。每組連續outer_step與完整R／六段／EIP／flags／四視窗保持接續。快照readonly=true，callback前後都非活動，started／completed2，沒有新CPU拒絕。不採第一次誤標的callee分支；正式探針遇首CALL停止，無callee探勘。

**未知**：20DB87非零分支尚未連到實際NEW GAME按鈕命中／指令消費；209325的玩家可見作用未驗。原100M仍主選單與致謝變化，正常開局／remake同狀態未完成。已證實上層讀到保留按下事件，不支持因短按丟棄而延長點擊的修法。下一步只追20DB87之後的按鈕命中與指令值，不增加預算，不重點，不追整個renderer或compiler helper。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-330-baseline.txt.gz | 2e3944aef1e541d94231d0b737afd11041bee204bd376cfa5856a6a2ea824a0d |
| workplace/moo2-probe-330-click.txt.gz | a08c6b3c18fc1a4d89dcaedf69666d0b180e612ade7bff59bc742028025c5cdd |
| workplace/moo2-probe-330-extended.txt.gz | efa03837fe9dcac0be033eab345efc59cfdaa330a67030b5f82204d156ab130a |
| workplace/new-game-330-verify.py | a9bf44ae5cbd49df4aebba97dfd204243bc375a660065516409f739a6b262e2b |
| workplace/new-game-330-parity-tests.txt | d8c3fe1d846d3d0a41195c65ee38d05af7cafa3e2f471b878b11666a59c62d48 |

probe SHA-256 931bb9d364a144460f2b358543f36e110f07e732020b80f69cadc5a6dbcdbdc1；CPU 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，startup／provider／matcher／VBE與329保持。原始輸入、LOG／PNG／RAM留本機忽略，不公開。限定CONFORMED只含兩組事件返回／首Jcc或CALL與舊基線保持；主庫RE-first、255完整游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG／人耳仍未知。

67回填函式與既有32／49／25／27／27／34／31／36、新增31缺證據負例、兩CLI PASS；workplace/new-game-330-backlink-tests.txt SHA-256 4423e974f97d1852c72ccd57080070b6fb6dbfdfa07aaafacffc7c6714b00584。原ZIP／patch／EXE／MOX.SET／417檔與來源保持再核對、gofmt、擁有權及工具root-owned／誤建.md目錄自檢通過。

非零臂caller與原範圍來源已由規格331接通：[331-moo2-button-branch-call-return](331-moo2-button-branch-call-return.md)。正常返回x500／y229，index1因x500>25跳過、續查index2；仍未達完整caller RET，NEW GAME指令與最終命中未知。

### 2026-10-03 selector註記勘誤

332續觀察前核對原330三RET收據，return_selector皆188h。cpu386/cpu.go的段陣列順序是CS／DS／ES／FS／GS／SS，before_seg=[8 188 188 0 20 188]中的20h屬GS，SS是末槽188h。原表的SS20h是文件誤標，已修正為SS188h；原始位址、bytes、ESP、返回目標與收據雜湊完全保持。

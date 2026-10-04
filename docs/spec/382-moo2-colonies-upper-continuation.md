# 382：原上層返回與殖民地畫面建立的有界觀察

狀態：**CONFORMED，限定原自然返回與可見畫面觀察**
日期：2026-10-04

## 玩家範圍及固定來源

承接[381](381-moo2-colonies-frame-source.md)，只追相同Sol II正常行選取後的畫面建立。先核對sub_133237的直接上層及退出邊界，再從原200M前置觀察自然返回及畫面；不修改主庫玩法或輸入、不追palette計算與DAC／PIT內部。

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，工具基線 `a03c322d28002bb0e11d5d5109d5833a17114d04`。381原journal SHA-256 `1bcb2e8890eff4e9fb7b8cb9acf7a3c7c65ce58f76263fdc92c7238ffc0eb51d`，原200M完整核心／FPU／clock及table／DAC／PNG仍作前置，不以新末態冒稱同狀態。

IDA Pro9.4、IDA linear EA；原執行器另標dosgolem_high_le。本固定LE加F0000h映射須由原bytes與file offset核對。工具及容器版本沿381；原ZIP唯讀、/tmp一次性DB、UID/GID1000，私有匯出與收據不入Git。

## 最小靜態及既有動態證據

已證實：IDA sub_133237範圍133237..1334BB、184項。133277比較EBP-20h與100h，133288遞增；133294比較EBP-1Ch與10h，1332A2遞增。兩層有界，原退出到1334B2，mov ESP,EBP、6pop後1334BA C3。不解讀palette演算法內部。

381原child frame保存父EBP2BDB9C，frame後12bytes對應父EBP-20h／-1Ch／-18h，原值117／1／0。這些是raw區域counter，沒有命名正式玩法欄位。父框架按原6push／6pop，返回槽為SS:父EBP+18h，即2BDBB4；原值尚未取樣，不能用COLONY.LBX字串挑caller。

IDA有26個直接CALL；保留全部原EA／bytes／file offset／xref及最小上層脈絡。多個不同UI共用該函式，資產名稱僅導覽，不當本次玩家路徑證據。當前caller鏈仍須讀原返回槽及自然RET才能驗證。

## READY後的private觀察契約

新private mode DOSGOLEM_MOO2_COLONIES_UPPER_CONTINUE只接受1，要求完整ROW_CLICK／restore／COLONIES／HOME／BANNER、原MAX_STEPS185M及可寫state。mode off保持381的200M與全部參數守衛。mode on明示200M至210M的一次10M觀察窗口；此預算用於已定位的有界退出與其後UI，不承諾窗口內完成，也不反覆加cap求過。不送新玩家輸入。

200M首個loop邊界保存upper-source200完整frame、原32bytechild frame，另只讀父EBP的32bytes框架及EBP-20h的32bytes區域空間。原完整core／FPU／clock guard不符即拒絕；既有code／table／DAC／PNG與381原final逐項核對，只依既有契約排除每輪RAMhash。新父return槽在+18h獨立解碼，僅與固定26原CALL比對，不注入或修正其值。

200M後只觀察第一個dosgolem_high_le2234BA C3原RET，保存前後全部R／段／flags／FPU、code16、真SS:ESP stack16、clock及RAM只讀護欄。原RET應按真stack轉移且ESP+4；輸出與原父槽核對，未命中就如實未知。原指令效果與observer只讀性分開，不跳helper、不強制return。

另外只讀記錄200M後第一個原DAC非0及其VBE畫面、實際終點與PNG；首非0不當可見UI。完整DAC序列獨立重播到真末態，PNG獨立解碼、palette映色與親看；新UI、table、state依實際結果寫入，不預填殖民地完成。CPU stop／DOSexit／210M step_limit都記實際狀態，probe exit0不是guest完成。

## 垂直鏈、驗收與停止線

原行選取13→既有200M來源→原父框架／自然RET→畫面／物件表→state為本次限定鏈。正式人口調整、存讀内容、完整開局、RNG與remake同狀態仍未知，固定日期不是seed。主庫RE-first保持，不做新玩法規格或Go實作。

private patches可逆回381，公開internal／CPU／DOS／原probe保持。舊34CLI逐byte保持；新增mode錯值／缺row／缺restore／缺state／錯baseline在EXE前拒絕，mode on／off缺EXE正對照分開。原guest只跑一次，既有全部來源、195M前置／原輸入、完整200M收據與沿途frame先核對，再檢查新窗口的RET／DAC／UI／state。

private入口workplace/new-game-382-ready-review.py、moo2-colonies-upper-382.go、new-game-382-run.sh及new-game-382-verify.py。native一次性Docker為network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP與patch唯讀，550s owned PID監測／trap。驗證90s／1536MiB／1CPU／128pids，完成後檢查容器與擁有權。

公開只保存本契約、索引及381回填；原EXE、RAM／LOG／journal／PNG／state與private probe留在忽略workplace，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。沒有自然UI時只報實際邊界，回到玩家路徑所需的最小RE，不深入helper或以高cap掩蓋未知。

READY審查：固定26個E8、256x16有界退出、381原200M的117／1與父框架位置、單次210M及只讀RET／DAC契約已通過，才修改private probe。
## 實際驗收與證據等級

**已證實**：原200M父框架2BDB9C的32bytes為`C8DB2B0000002B00576AE730FF00000000000000000400008E071B0000000000`，原SS188:2BDBB4讀得1B078E。唯一吻合IDA linear EA C0789／file offset1334749／E8A92A0700→sub_133237，下一C078E映射原1B078E；直接上層為sub_C058A，C058A..C0965／215項。以原返回槽選caller，沒有用資產字串選定。

**已證實**：203011820／463667422µs原2234BA C3自然執行，真stack為8E071B00000000000000000000FFFFFF，返回1B078E／ESP2BDBB4→2BDBB8。其餘R、段、flags246h、FPU127F／status0／depth0／8個bits保持，clock增1µs，RET不改RAM，callback18／18、IRQ54032／54032均不活躍。observer只讀及原指令效果分別核對；未觀察當前child的每一次RET或把所有26上層命名成殖民地。

**已證實**：200M後首非0DAC在205281523／222F4F，raw非0為1、RGB仍0，PNG仍黑。實際210M step_limit／228DF6／483821442µs，沒有CPU拒絕或DOSexit；終圖親看可見「Colony of Sol II」、人口8,000k、職業列與殖民地地景。raw DAC756非0、RGB681553非0，RGB SHA-256 `1135b9bc7686966d5fdf0992aa02e35cf73cc0a82bc20ce660d864eac78f7364`、PNG `1a0e7551173173b43897be18ccf5295755ca80b222be7dc6dbca0b1d986c8c03`。DS188:298848 count36／stride55完整1980bytes取得，SHA-256 `5a6102f64d586c1978a37d8c5b034caf99783c44ba8ca5eb96896780c8b334db`；表內識別仍raw，畫面出現不證明所有操作已可用。

九patch可逆回381，46CLI通過，public internal／CPU／DOS／原probe保持a03c322。原200M完整核心／FPU／clock、35876 DAC事件與所有舊PNG保持；12962列至收尾前逐項相同，封裝hash只依既有獨立journal核對正規化。整段DAC寫入獨立重播，PNG filter／CRC／palette／histogram與末態核對，原418來源、SAVE10.GAM／MOX.SET／sound.lbx及同guest副本保持、UID/GID1000。

原guest僅一次。驗證器初次錯找不存在的200M color_source標記，改用原restore_terminal；第二次漏套既有row journal封裝hash正規化，獨立原195M核心／FPU／clock／table／DAC／PNG已通過後才沿381既有規則修正。兩者是驗證腳本問題，沒有改原收據、guest、輸入或cap。主庫玩法仍受RE-first；人口調整、正式存讀語意、完整開局、RNG與remake同狀態未知，固定日期不是seed。

下一步保持本次210M來源，先核對36筆原熱區與人口職業列的輸入消費端及安全輸入前置。來源充分後才訂正常操作觀察契約，不盲增cap或深入renderer／palette helper。

## 383 職業列來源回填

固定官方1.31 EXE／原210M的36表由[383](383-moo2-colonies-job-control-source.md)核對。index1／2／3均kind6；原+18h僅word值，+20h才是pointer。IDA linear EA 11B0B8比較樹到11C2C2，11C2CF→1156E2，115988間接寫原pointer暫存；不同kind不共用pointer語意。source file bytes及IDA已套fixup bytes分別核對，保留原relocation record。

本382的210M原收據及可見畫面保持。383沒有新guest或操作，正式人口變更仍未知；下一步維持210M補只讀pointer值、current colony／pool／record與scene callback，不直接由熱區推定安全點選或拖曳契約。

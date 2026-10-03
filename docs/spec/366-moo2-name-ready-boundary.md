# 366：可寫名稱頁的首個可接受輸入狀態

狀態：**CONFORMED，限定首個平台readiness只讀快照；正常名稱確認DRAFT**
日期：2026-10-04

## 問題與來源

[365](365-moo2-overlay-race-humans.md)已驗正常Humans及95M名稱頁，但原95000000為EIP22F263／R=[20 2BD8C8 2A38E0 2A38E0 2BD8C8 2BD8E4 33 28439D]／flags12h，IF關閉；callback mask1／pending0／非活動／8／8，IRQ22338／22338非活動且非failed。這不是原338的95M READY初態，不能只換165byte表hash或放寬guard送輸入。原名字Strader與ACCEPT畫面相同，原完整165bytes SHA-256 db67c471ba6a34a66146e083662f4c134c192e2371a807ec9647327558a7fd1b；index2+24候選28439D、32byte已只讀取得，持久名稱writer仍未知。

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；原365收據89f17f037e6b8d840ba082e11bbb7da96992c9ded103b09b9c7be43990e83133。原417 ZIP根檔另加官方EXE共418檔、MOX.SET／1996-01-01／180M參數／44M Esc與已驗正常輸入保持；1996不是seed。位址為dosgolem_high_le，沿365私有試作，可寫source隔離／原版資料唯讀，主庫RE-first保持。

## 只讀契約

私有DOSGOLEM_MOO2_NAME_READY_OBSERVE=1要求完整365 profile與名稱旗標依賴；其他值／缺依賴在讀EXE前拒絕。未啟用保持365在95M只讀停止。啟用時保留95M原名稱前置與候選，只略過正式名稱按鈕分支，原CPU.Step與其餘正常診斷繼續；不直接continue而跳過guest指令，不改CPU／RAM／平台／輸入。

從95000000起最多4096原指令，以第一個符合下列既有平台安全條件的原狀態作只讀快照，不送事件：IF開、callback target8:2136D1／mask1或2B／pending0／非活動、IRQ非活動且非failed。此時保存完整R／六段／EIP／flags／FPU與位元、VBE／RGB、原globals192byte／header16byte／實際count≤64的55byte表、index2+24原32byte候選、整RAM前後保持及只讀。最多64筆readiness邊界列；未達條件於4096上界明示停止，不能挑選後續碰巧可通過時點或增加上界。

既有InjectMouseEvent原契約見internal/machine/le_mouse_callback.go:123：位置變化flags1、左鍵按下flags2、放開flags4；flags&mask非零排回呼，裝置狀態仍更新。原名稱放開mask1已由338驗，按下mask1的原玩家結果仍未知，366不送輸入，也不把平台可排回呼當名稱確認成功。callback dispatcher在IF關閉時不派發，恢復IF與readiness只觀察外部邊界，不深入critical section helper。

## 驗收與停止線

365所有95M前共通原列與29PNG保持，既有mtime／DTA／每輪只讀RAMhash正規化沿352；95M原前置／候選亦保持。新增最小bounded診斷與明示停止，可逆為365，公開CPU／DOS／provider／probe及全部internal不改。原418檔前後SHA-256保持，state清單與365核對；私有資料／PNG／LOG／RAM不公開。

原來源→只讀表與平台readiness→下一輪輸入契約；本輪沒有名稱press／release或旗色輸入，不稱存檔／完整開局／RNG或remake同狀態。足夠界定首個可接受狀態後才另立名稱確認DRAFT，保留原338唯讀guard；未知按下mask1與持久名稱仍單獨記錄。

Docker沿Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。私有入口workplace/moo2-name-ready-366.go／new-game-366-run.sh，規格同次掛000-index；收據掛主庫研究入口docs/re/dosgolem-moo2-intake-20260930.md。

## 限定CONFORMED：首個平台readiness只讀快照

**已證實**：原95000000 flags12h／IF關閉，不送名稱輸入。probe每個原指令重算readiness，首個true即停；實際95000065為EIP234A49／R=[3 1 210160 2BD8C8 2BD8DC 2BD8E4 2843A5 28439D]／段=[8 188 188 0 20 188]／flags216h，IF已自然開啟，FPU127F／status0／depth0／八stack bits0。此刻target8:2136D1、mask1或2B、無pending／活動callback及IRQ非活動／非failed由ready=true與受審查判定共同核對；同guest只讀defer終態另證mask1／pending0／非活動／callback8／8，沒有名稱press／release。終點精確IRQ計數未直接列出，95M初態與64個已打印邊界均為22338／22338，不將其外推成終點直接收據。

首64筆逐指令邊界95000000–95000063均ready=false；offset64因64筆上限未打印，沒有假造該列。其後判定仍每指令執行，真正first-stop在65，四替換可逆及實際first-stop收據核對。完整globals192bytes／header16bytes／165bytes原表／32byte候選與RGB逐byte或hash保持365的95M初態；table SHA-256 db67c471ba6a34a66146e083662f4c134c192e2371a807ec9647327558a7fd1b，candidate28439D原Strader補零，RGB2646a7ef25939869ede60aa35bcd97d649b906fd2cc287d08deb6c2d8058cc12，VBE Active／Bank4／StartY0／DisplaySets48保持。CPU／FPU／VBE／RAM只讀保持，不修改IF或跳過CPU.Step。

**驗證**：95M前7662原365共通列依352既有mtime／DTA／每輪自身只讀診斷RAMhash正規化、29PNG逐byte保持，原95M前置及候選保持；原418檔guest前後SHA-256保持，state同365僅sound.lbx4250888bytes／原ZIP同bytes／UID與GID1000。公開CPU／DOS／provider／probe與全部internal保持，沒有名稱輸入／旗色／正式存檔。

首次CLI核對把READY中的read子字串誤當讀檔訊息，原exit2拒絕正確、原guest尚未啟動；核對檔案缺失後修正同一腳本，原guest僅首次執行一次。四無效值／缺依賴在讀EXE前拒絕與合法值越過閘門後缺EXE正對照通過。64筆邊界上限按實際收據處理，讀同guest及其defer終態完成驗證，沒有重跑挑結果。

### 命令與下一步

```text
bash workplace/new-game-366-run.sh
python3 workplace/new-game-366-verify.py
```

既有Docker執行並通過，11份私有來源／收據雜湊同次保存主庫研究入口。原LOG／PNG／資產窗口不公開。下一步另立名稱確認DRAFT：使用原95000065的完整165bytes／globals／header／CPU／FPU／RGB／32byte候選與同target／IF／callback／IRQ安全條件；按下mask1的flags3可排既有平台回呼，但原正常確認結果仍未驗，先審查才送一次press／release。原338唯讀guard保持，不靠單純換hash繞過95M IF關閉，也不深入helper。正式名稱writer／旗色／存檔／完整開局／RNG及remake同狀態仍未知，主庫RE-first保持。

## 367可寫名稱正常確認回填

[367](367-moo2-overlay-ruler-accept.md)已核對原95000065／dosgolem_high_le:234A49的mask1正常名稱ACCEPT。原95008897於20DDDB實際六byte66A3A6C42600寫DS188:26C4A6 word0000→0100，正常一次press／release後99M進入SELECT BANNER COLOR，未送旗色輸入。338唯讀未命中共享store的收據仍有效，可寫367才命中；365、366各輪未送名稱輸入的歷史保持，後續正常確認由367限定接通。原338與339唯讀guard保留，正式名稱持久writer／存檔／完整開局未驗。

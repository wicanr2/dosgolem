# 463：session唯讀原版根與存檔根

狀態：**READY**（第1版，2026-10-25），未實作。明示opt-in的雙根啟動與必要檔案保護；原單根不變。

前置：[006分層](006-layering.md)、009、[235私有開機](235-sealed-owner-private-boot-draft.md)、[450狀態啟動](450-session-boot-from-state.md)。需求：Strike Commander規格016§3.2、6.1、10 R4；通用規則不含遊戲名稱、檔名或位址。

證據基準：fork `2b3da1610fe58c339e2c3ab946006f13dd74ddc9`。[事實與定位](../../../strike_commander/workplace/impl016/session-r4/FACTS-001.md)，SHA `adf1e1939fa350e57bd835bed3703562157b926eefc7df4768534c6f35bf005a`；六個合成DOS探針與唯讀state metadata由同目錄README進入，無guest指令。本文新增行為是工程契約，不宣稱原版新語意。

草稿001 SHA `9f54c21c9a6bc482de298a8030b6c5f5a89e15b8a33d6cac7b01c33ad544f826`。[獨立審查甲](../../../strike_commander/workplace/m6model/impl-3b-material/r4-review-a-001.md) SHA `eb2bc04c8a291fcfb77a03c667e049b198bc31a211e6b268ef66bbae8b04a9a4`；[獨立審查乙](../../../strike_commander/workplace/impl016/r4-review-b-001.md) SHA `26f116f93ddadd544708be603495a4433191557a6531fd3b5c491a12c38782b7`。兩名互不見報告。甲的中間元件硬錯及預開handle、乙的先驗雙根再驗可寫順序與精確拼字反例均採納。state casefold多候選固定拒絕。


## 1. 入口及相容

BootInput與StateInput各新增可選OriginalRoot string，原SaveRoot及StateInput.RebaseFrom保留。名稱是候選，需有等價單一契約。

- OriginalRoot為空：完整沿現235／450單根模式，Root=Scratch=SaveRoot、原RebaseTo、JSON／狀態格式、讀寫及錯誤行為不變；不默默啟用新guard。
- OriginalRoot非空：明示分根。DOS.Root=OriginalRoot、DOS.Scratch=SaveRoot；唯讀找SaveRoot再OriginalRoot，允許落地的變更僅在SaveRoot。不得複製整棵原版樹當SaveRoot冒成功。
- BootOriginal仍接caller傳入的EXE bytes與期望SHA，不自行猜EXE路徑；原EXE上限／私有複本／零instruction開機順序不變。
- 這是235「Owner不接original路徑」邊界的明示opt-in擴充；舊模式保持。通用session／DOS負責路由與保護，前端仍負責來源檔SHA、33檔同名偵測及遊戲資源清單。

## 2. 根與安全mutation邊界

所有新模式根檢查在LoadEXE／Install／LoadFrom之前；失敗不改Booting phase。OriginalRoot拒絕空／NUL／symlink／非目錄，但不要求可寫。先完成兩根不寫入的Lstat、實體正規化、同根／巢狀／別名檢查；全部通過後才對SaveRoot呼叫checkSaveRoot的可寫檢查。不得先讓非Unix的可寫探針碰到OriginalRoot或其子目錄。檢查OriginalRoot不得建立permission-probe檔。

分根模式將兩根正規化成受控的實體絕對路徑，驗證並使用同一組路徑；原模式不加Clean。根本身不收symlink；祖先實體路徑用於同目錄／巢狀／別名判斷。兩根不得相同、互為巢狀或指到相同實體目錄；不能用字串HasPrefix代替路徑元件包含關係。

新模式需要以下一般DOS保護，僅加Root欄位不足：
1. 取得可寫host handle、create/truncate、rename/delete、目錄及實際mtime變更之前，目標必須是SaveRoot嚴格後代。從OriginalRoot rename採既有複製語意，不能因兄弟名字有共同字串前綴而移走原檔。
2. 不跟隨資料路徑中通往根外或另一根的symlink；可寫目標及其父路徑的symlink均拒絕。來源read／state reopen同樣不得用symlink逃出所選根。
3. 現存SaveRoot可寫目標不得是多硬連結檔；本切片固定採拒絕，不另做硬連結COW或全VFS。無法證實一般檔／安全單連結時拒絕，不假定安全。取得writable handle後沿該已驗證所有權，合法SaveRoot內rename不猜成OriginalRoot來源。
4. unsafe根在boot回明確錯；runtime unsafe mutation在任何host變更前回DOS AccessDenied(5)，不照舊記帳冒落地成功。新建立的一般檔均由SaveRoot建立。上述保護只在明示新模式啟用，不重寫原模式或新增AllowFileWrites直寫原版的分支。

這不是全DOS檔案系統翻修：繼承009的Scratch遮蔽、COW及無whiteout語意，刪／改名Scratch副本後原版同名檔可再出現。暫不增刪檔白化、磁碟機、路徑猜測或時鐘功能。caller在Owner生命週期內不得從外部替換根／祖先／連結或同時修改其檔案；敵意host並行替換不是本切片聲明。

## 3. BootFromState分根重定位

只在新模式使用新的雙根rebase計畫，原RebaseTo單根路徑不變。私有型別可等價，不新增snapshot欄位／版本。

1. prefix集合仍為狀態內s.Root加caller RebaseFrom。沿450先Clean、嚴格後代、最長前綴；沒命中／等於根／越界回錯。舊Scratch未存於snapshot，仍由caller提供其已知舊前綴，不能按副檔名／Name／RootDir推測。
2. 只取得命中的相對路徑，不先分類「原版檔／存檔」。同一相對路徑先找SaveRoot，缺檔才找OriginalRoot。只有真正缺少某個路徑元件才算缺檔。SaveRoot任一中間元件已存在但不是目錄、無讀取權、為不安全symlink或casefold有歧義，以及最終候選不安全、非一般檔、不可讀或大小不符，均整體拒絕，不退到OriginalRoot。不得用把所有錯壓成空字串的舊lookupPath作此判定。
3. 路徑逐元件沿DOS既有不分大小寫的名稱匹配；實際拼字優先，否則不分大小寫必須唯一。不得basename攤平、8.3截斷或套RootDir重猜host路徑。這個匹配／歧義規則需純測後固定，不能以os.Stat失敗就當另一根成功。
4. 每個mapped handle先完成根內、一般檔、可讀及hs.Size非0時的大小檢查，預開全部唯讀handle並seek到hs.Off，全部成功才進DOS提交點。預開或seek任一失敗關閉所有暫存handle並保留原DOS及原handle；不能只Stat後先關舊handle，再試os.Open。不得先開舊絕對路徑。hs.Off／Name／PSP保留；Root固定為新OriginalRoot，Scratch固定為SaveRoot，不被s.Root覆寫。DOS邏輯Dir／RootDir照450，不把其字串當host根分類。
5. 所有重定位暫值在成功／失敗後清空；Machine.LoadState先失敗時session也清。phase、generation、mouse／keyboard及ClockPercent限制均沿450，boot不額外Advance。

所有還原handle仍以os.Open唯讀，writable=false。舊snapshot沒有writable／原open mode／dup alias來源，不能靠path、Name或檔案類型補回。維持450§4的只記帳寫入限制；snapshot來源應避開必須接續寫回的開檔期間。本切片不修writable handle還原或dup alias，不改state bytes。

StateSHA仍hash原始私有State複本。物理根路徑改變會改DOS摘要的host metadata；不能把不同根的舊DOS digest硬稱逐byte同。比較兩runner時使用同一對根及同檔內容／mtime；原零值金樣保持原bytes。

## 4. 最小驗收

- 原模式235／450測試、原零值金樣與已有收據不變。
- 新模式合成DOS：Save優先／大小寫、Root只讀、COW、create／write／delete／rename／find union；不要求可寫的OriginalRoot。以host可寫的合成Original檔驗應用層不改原版，不能只靠ro mount掩蓋。
- 兩入口根為symlink／檔案／相同／巢狀／實體別名時boot拒絕；兄弟字串前綴、可寫leaf／父目錄symlink、多硬連結及unsafe state候選各驗相應守門，變更前無原版污染。同根／巢狀拒絕須證可寫檢查尚未呼叫，沒有permission-probe與Original目錄mtime變更。拿掉每個guard的真正負控必敗。
- 狀態同時有兩舊前綴、同名Save覆蓋、只在Original、Save壞候選不fallback、offset／size／最長前綴／case歧義／缺前綴。Save完全拼字unsafe而另有安全fold項目仍拒絕；Save中間為file／unsafe symlink／casefold多目錄且Original有合法同名檔仍拒絕。預開／seek失敗須保留原DOS摘要及可讀舊handle。驗全部readonly及暫值清空，不聲稱恢復原write mode。
- 一個唯讀原版根＋空SaveRoot的冷開機／狀態接續及016 A1仍須之後同狀態驗收。不得用可寫整副本代驗。

## 5. 已知限制與實作後閘門

- Linux純探針已實證prefix rename、symlink與hardlink三個現況洞；新模式的portable單連結／實體根驗證機制尚未實作或跨平台驗證，不能因其未知而默認安全。
- 新雙根state router尚未實作；第3節規則已固定，casefold歧義拒絕。已有450單根rebase不能拿來聲稱已分根。
- 已有c800 snapshot只離線解出3個host handle，未含其原mode；不依TRE名字斷言readonly。原格式不能證實writable，這是沿450的界線，不是本切片待猜補。
- READY只授權本切片實作。實作／負控／兩入口／三平台編譯與可用平台純測，以及016同狀態正常玩家驗收分開留收據；未齊不CONFORMED。

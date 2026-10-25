# 464：session首次輸入前的有界PSP相對讀取

狀態：**READY**（第1版，2026-10-25），通用入口已實作，app／正常玩家路徑未驗收。前置235私有開機、450狀態啟動、454唯讀語意；不含遊戲名、檔名、載入器辨識或渲染規則。

本版只澄清重疊Dst順序與ReadBootPSP錯誤原子性。001原SHA `c640f1b5e6b985f7f0d4791f9f93b9232897cb6d4eabd54ec4f20bdc1ae7018c`及其所有凍結產物保持不變；範圍、數字與app分類規則不變。

## 依據與最小範圍

016 R8需要從BootFromState成功到首次Deliver之前判定MENU期間。現Owner只有callback內StepView記憶體讀取；靜態ProgramPath不隨EXEC變更，ExecLog不進state。原DOS CurPSP與physical Mem仍保存。

[固定六案離線核對](../../../strike_commander/workplace/impl016/session-r8/raw-mem-result-001.json)以CurPSP+10h的映像位置、原MZ重定位後256-byte prefix及64-byte entry逐byte核對。c700/c800/c900唯一對MENU，s449對OPTTEST，s760/s800對STRIKE，靜態ProgramPath六案都仍是SCCD。這只證明這六案的定位，不宣稱所有任意snapshot都可辨識。

通用層只提供一次、caller-owned、有界physical讀取；app負責合法本機EXE內容鍵、MZ期望與唯一分類。沒有新snapshot欄位，不新增current-program字串或按guest檔名猜身分。

## 介面

```go
type BootPSPRange struct {
    Offset uint32 // 從CurPSP*16起算，明示physical RAM，不經當下CR3／CS
    Dst []byte    // caller-owned，方法不保留
}
type BootPSPReceipt struct { PSP uint16 }
func (o *Owner) ReadBootPSP(ranges []BootPSPRange) (BootPSPReceipt, error)
```

- 只允許BootOriginal／BootFromState已成功、PhaseRunning，且從未嘗試Deliver／Advance、未使用過本入口。Status／Digest／MachineDigest／View等既有讀取不關閉此窗口。只提供一次同步資料批次，不回傳live view、Machine、DOS或內部slice。
- 首次嘗試Deliver或Advance即永久關閉新窗口，包括參數錯誤；此新限制只影響新API，不改既有回傳、phase或機器語意。需要兩個private owner旗標或等價邏輯，零值不影響舊流程；不進snapshot／既有摘要／收據JSON。
- PSP由DOS實際curPSP值取得；0拒絕，不改成固定PSPSeg。1至32個range，每個1至4096 bytes，總數不超16KiB。Offset及PSP*16相加以uint64先驗，不能overflow、wrap、超過physical Mem長度。所有range檢查完成後才複製，不允許部分成功。
- ReadBootPSP所有錯誤一律回零值BootPSPReceipt；不呼叫o.fail、不latch fault，也不改phase、firstFault、generation、Dst、機器或兩個新旗標。前置拒絕不消耗一次讀取機會；先前Deliver／Advance嘗試已永久關閉的窗口仍保持關閉，不由讀取錯誤重開。
- 全部合法後才記已使用，依ranges索引遞增順序複製原Mem到caller Dst並回PSP。不要求Dst彼此不重疊；重疊位置由後面的range覆寫，但方法不保留它們。Caller須按既有Owner序列化契約使用，不新增並行讀寫保證。
- 沒有Read8／Read16／任意執行期inspect、callback、寄存器、寫入、guest步數、補Observe、顯示事件、音訊或檔案事件。預設未呼叫時的既有235／450／454零值與舊bytes不變。

## app採用條件

這是明示呼叫的新入口，不改既有啟動流程。Strike Commander的app依016 R8先用已驗本機六份MZ各自的prefix／entry offset組一次batch，再以回傳PSP計重定位期望，與讀回bytes唯一匹配。只在唯一已驗候選下設定MENU期間；零個／多個候選、未知格式、範圍無法表示均明確拒絕該headless狀態啟動，不偷偷推進或猜布林。

六案raw Mem證據不等於新API已完成；正式採用還需實際Owner批次與offline原raw期望相同、原memory／CPU／indexed／palette不變，以及原cold boot與正常玩家路徑驗收。Normal冷開機的既定SCCD起始狀態沿016，不由本API擴範圍。

## 最小驗收

- 兩種Boot入口各驗成功後、任何View／Digest之前或之後仍可讀一次；與原Mem獨立期望一致，原全摘要及observer／送達計數不變。
- nil／非Running／PSP0／空批次／range數、單range／總量上限／Offset溢位或越界／第二次／首次Deliver或Advance嘗試之後，各驗零值receipt及上述錯誤原子性。失敗先驗後允許合法一次；成功後修改caller Dst不改原Mem。重疊Dst驗ranges正序且後者覆寫。
- 真負對照：PSP換常數、Offset當linear經頁表、先拷貝第一段再驗第二段、漏掉總額／單次／窗口守門，對應測例必失敗。原模式回歸及snapshot bytes不變。
- 固定六案以真Owner啟動、原狀態及等價輸入驗R8；不以offline decoder或synthetic mock取代正式入口。本版已通過下列兩份獨立審查，正式Owner及正常玩家路徑驗收仍未完成。


## 既有probe來源及重跑界線

[來源sidecar002](../../../strike_commander/workplace/impl016/session-r8/probe-provenance-002.json)逐一保存六份現存EXE的實際路徑、大小及與原raw結果相同的SHA，也保留原結果記錄的六份state順序／SHA。`workplace/root/`六檔全部符合；其他數個根也全部符合，不能由hash相等反推歷史argv。已安裝目錄的STRIKE.EXE不符合，明列排除。

現有目錄未保存完整原argv／EXE根參數／Docker掛載命令，原run log為0 bytes，因此這些歷史欄位固定標unknown。sidecar中的SuggestedReplay僅是以目前已核實副本組成的可重跑參數，不是原執行命令，本次未執行。沒有重建歷史log、重跑probe、建立Machine或執行guest，也沒有新增原版行為證據。

## 採納與追溯

本版是工程契約，不宣稱新的原版行為證實。基準fork `77f6d3a`，Go程式相對 `2b3da16` 不變。原草稿002 SHA `3139a27fbac33f484d5d02cbafdee1b201961226e630d283b2b56335adc417ec`。

[獨立審查C](../../../strike_commander/workplace/impl016/session-r8/review-c-001.md) SHA `8eec03b428fd54d9a06c8e8952d7d9a21f6c82aa28f3981fbf864eac31644cfd`；[獨立審查A](../../../strike_commander/workplace/impl016/session-r8/review-a-002.md) SHA `8584f013f8021fb6fe46cfbe37c8ce914b680ab993d83ee81b9f4b1dc786044a`。兩者未互讀報告；C要求的重疊Dst順序與錯誤原子性已於002補明，A對002無阻擋或應改。歷史argv保持unknown。

這不補463的缺份production審查；463程式仍暫不套用。464的離線事實、工程規格、實作與正常玩家路徑分別驗收。

## 實作與驗收範圍

三個產品檔為`session/boot_psp.go`、`session/owner.go`及`internal/dos/current_psp.go`。採納source004 SHA `fe889a66f69e2893eb2d3f4b0b2354242df2fbed57b77b25c902212bdb716169`；[主代理程式窄審](../../../strike_commander/workplace/impl016/r8-production-parent-review-004.md)及[獨立程式窄審A](../../../strike_commander/workplace/impl016/session-r8/production-review-a-004.md)均無阻擋／應改。

[實作收據](../../../strike_commander/workplace/impl016/session-r8/RECEIPT-003.md)保存完整單測、race、vet與16項有效負控。套用後在actual工作樹重跑session／internal/dos完整單測及六案零指令測例，結果由[parent-adopt-unit-009.log](../../../strike_commander/workplace/impl016/session-r8/parent-adopt-unit-009.log)及[parent-six-state-009.json](../../../strike_commander/workplace/impl016/session-r8/parent-six-state-009.json)進入。六案72範圍皆與獨立raw雜湊相同，讀取前後memory、兩種摘要及callback計數不變。

這只驗通用API及固定六案零指令連續性。app唯一分類、冷開機正常流程、玩家GUI及完整016 R8仍未驗，因此本版不標CONFORMED。

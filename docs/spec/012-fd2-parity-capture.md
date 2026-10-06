# 012 — FD2 原版／重製對拍資料契約

狀態：**READY**
日期：2026-09-06（原版執行器權威順序勘誤）
前置：[`005`](005-oracle-api.md)、[`006`](006-layering.md)

## 1. 目的與邊界

本規格建立《炎龍騎士團 2》原版與重製版的可重播畫面對拍證據。正式對拍的
原版執行器固定為 dosgolem；DOSBox-X 只用來補足、除錯及交叉驗證 dosgolem
尚未具備的功能。DOSBox-X 擷取只能標為 `dosbox-bootstrap`，不可單獨構成
最終對拍、`same-state` 或 dosgolem 玩家可見路徑已完成的證據。

DOSBox-X、Xvfb 與輸入自動化只屬 `apps/fd2`／`tools/fd2`，不可滲入
`internal/cpu`、`internal/dos`、`internal/machine` 或通用 `oracle`。

## 2. 固定輸入身分

每份報告必須記錄：

- `FD2.EXE` 大小、MD5、SHA-256；
- dosgolem Git commit；
- 原版執行器名稱與版本；
- 重製版 Git commit；
- 場景名稱、輸入腳本雜湊與畫面狀態等級。

本專案的已知參考執行檔為 357,074 bytes，MD5
`b97caf2239a27a896069d03549d96e1e`，SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
雜湊不符時失敗即關閉，不沿用位址或對拍結論。

## 3. 狀態等級

- `same-state`：同場景節點、同機制輸入、同游標／選取位置及同動畫相位。
- `near-state`：場景與機制輸入相同，但動畫相位或非決定性視覺仍有已記錄差異。
- `layout-only`：只比較版面，不宣稱規則或動畫狀態相同。

報告不得自行把較低等級提升為 `same-state`。
場景 schema 2 必須以 `original_runner` 明示 `dosgolem` 或
`dosbox-bootstrap`；後者若宣稱 `same-state`，工具必須失敗即關閉。

### 3.1 FD2 機制對拍不依賴 RND()

FD2 對拍的驗收對象是固定輸入後的遊戲機制、狀態轉移、畫面、輸入回應與演出
時序，不是原版與重製版的 `RND()` 呼叫歷史或亂數產生器實作。場景建立器應直接
將雙方必要的角色、座標、回合、事件旗標及其他前置狀態設定到記憶體，再從共同
機制入口開始擷取；不得為了同步 `RND()` 序列而重跑前史、反覆刷關或延後對拍。

每份收據必須列出被直接設定的位址／typed field、設定前後值與設定時機，並標示
這是「機制同狀態注入」，不能外推為未修改的一般玩家路徑證據。只要兩側進入機制
時的可觀察輸入狀態等價，原版此前消耗多少次亂數不影響 `same-state` 判定；
`RND()` 本身不列入 FD2 對拍完成門檻。

## 4. 影像與比較

- canonical 畫布為原版原生 `320×200` RGB。DOSBox 輔助擷取環境產生
  `1024×768` 視窗圖時，擷取器必須先驗證完整視窗尺寸，再明確裁出左上角
  `320×200` 遊戲畫布；其他尺寸一律失敗，不得靜默縮放。重製端若從 `640×400`
  顯示輸出取證，必須另以明示的整數倍率最近鄰正規化步驟產生 `320×200` 證據。
- `apps/fd2/cmd/parity` 只接受原生 `320×200` 重製圖，或精確二倍的
  `640×400` 重製圖。後者固定以每個 `2×2` 區塊左上像素取樣成
  `320×200`，並在報告寫入 `remake_normalization=nearest_2x`；其他尺寸或
  非零起點畫布一律失敗即關閉，不依賴外部 ImageMagick 隱式轉換。
- 報告至少輸出完全相同像素數、總像素數、相符率、RGB 絕對誤差平均值、
  不同像素範圍及差異圖；原版與重製 PNG 的檔名及 SHA-256 也必須由工具
  直接計算並寫入報告，不接受操作者事後手填。
- 第一輪重製允許以近似效果驗收，但相符率不能單獨證明忠實；場景狀態與
  差異範圍必須一起保存。

### 4.1 具名區域比較

場景可在宣告式 JSON 中附加 `regions`，每筆固定保存名稱與
`x/y/width/height`，並必須完全落在 `320×200` 畫布內。工具仍以全畫面為
主結果，再對每個具名區域輸出相同的像素數、相符率、RGB 平均絕對誤差與
絕對座標差異框。區域不可取代全畫面結果，也不可用來遮蔽區域外的差異。

這項契約來自 2026-09-06 王座廳重跑證據：連續五個原版時間點的全畫面
誤差穩定，而差異遮罩集中在上半部地圖與人物，對話框內容區域已高度一致。
因此報告必須將「背景色盤差異」與「對話排版差異」分開呈現，不得將全畫面
百分比誤解為兩個子系統皆未還原。

## 5. 原版擷取工作階段

正式原版擷取由 dosgolem 的可程式化執行器產生，並記錄版本、機制輸入、直接
記憶體設定清單、虛擬時間及擷取點；不要求記錄或同步 `RND()` 狀態。所有擷取固定
採有界 Docker 工作階段：原版資料唯讀掛載、
容器內建立可寫複本、
Xvfb 由同一擁有程序管理並以 trap 清理、輸入腳本具逾時、輸出只寫明確目錄。
未找到視窗、未走到指定狀態、輸入步驟未完成或截圖尺寸不符都必須失敗。

外部擷取產物為 PNG、輸入事件 JSON、工具版本與來源雜湊；不得提交原版執行檔、
記憶體快照或可重組的原版素材。

DOSBox 輸出可協助定位 dosgolem 缺少的 CPU、DOS、顯示、輸入或時序功能；修正
dosgolem 後必須由 dosgolem 重生正式原版畫面。不得把 DOSBox 圖片換個檔名後
登錄為 dosgolem 收據。

## 6. 第一個驗收切片

第一個端到端場景依序為開場、晉見父王母后、第一戰初始配置與第一戰短移動。
每個場景都要用同一份宣告式機制狀態與輸入描述驅動原版與重製執行器，最後由
`apps/fd2/cmd/parity` 產生報告。測試用直接進場只能標為較低證據，不能冒稱
未修改的一般玩家路徑；但只要直接記憶體設定有完整清冊，且兩側從同一機制入口
開始，便可作為正式機制對拍證據，不因略過 `RND()` 前史而降級。


## 7. 唯讀 EIP 追蹤範圍

狀態：**READY**。日期：2026-10-03。
工單：[fd2_re #102](https://github.com/wicanr2/fd2_re/issues/102)。

第十八章同槽正常操作在第 9,341,834,326 步停止於 IDA LE
`0x4E6BD`。既有 `-eip-trace` 在早期字型繪圖已耗盡
200,000 筆上限，無法看到後期素材列寬。此限制來自收據與目前
`apps/fd2/cmd/oracle/main.go` 的先到先記政策，尚未確認 CPU 或記憶體的修法。

原版執行器保留同一程式、輸入、時鐘與遊戲狀態。新增
`-eip-trace-from` 與 `-eip-trace-to`，只控制唯讀記錄：

- 指令範圍含首尾。預設起點 0、終點 0；終點 0 表示不設上界。
- 起點不得為負；非零終點不得小於起點。
- `-eip-trace-max` 必須介於 1 至 200,000。窗口外不消耗筆數。
- 等待邊界、CPU、BIOS 時間、鍵盤及畫面輸出均不因窗口改變。
- 最終報告記錄起點、終點、上限、實際筆數與所選 EIP。
- FD2 包裝入口為 `tools/dosgolem_oracle.sh`。其環境參數
  `FD2_ORACLE_EIP_TRACE_FROM`、`FD2_ORACLE_EIP_TRACE_TO`、
  `FD2_ORACLE_EIP_TRACE_MAX` 對應上述旗標。
- 驗證包含窗口首尾、停用上界、筆數耗盡與非法參數。
  正式原版重跑另須核對相同計畫的狀態、步數與 PNG 雜湊前綴。

這份 READY 只授權觀測工具擴充。它不授權放寬記憶體邊界、
跳過素材解碼，或宣稱第十八章通過。

## 8. 逐格單位原始記錄

狀態：**READY**。日期：2026-10-05。
工單：[fd2_re #171](https://github.com/wicanr2/fd2_re/issues/171)，父項[#166](https://github.com/wicanr2/fd2_re/issues/166)。

物理返回地圖的逐格PNG與checkpoint在不同指令時點，overlay selector已直接
觀察到不同值。不得以後一停點單位／視圖拼成同狀態影格。原始欄位沿用oracle
既有checkpoint讀取端：LE線性53A45指標、53BEB筆數、80-byte stride；固定EXE
身分同本規格第2節，不新增原版欄位語意。

- `-frame-units` 預設停用。啟用需要 `-frame-dir`。
- 只對既有窗口、去重、settle與上限已接受的PNG，在相同指令時點附加
  `units` 及 `frame_units_valid` 至同一筆 `frames.jsonl`。
- 每列沿用checkpoint的原始index、x/y、pose/motion、byte5、camp、fig、
  identity、level、exp、hp及80-byte `raw_hex`。沒有有效欄位語意不另猜名稱。
- 允許0至128筆。全域讀取或完整陣列範圍不合法時輸出空陣列與false；
  消費端必須拒收false。用uint64檢查總範圍，不讀越界。
- 所有讀取皆唯讀，不改CPU、遊戲記憶體、RNG、按鍵、擷取相位或PNG。
  未啟用時維持原metadata及checkpoint契約。
- 包裝入口以 `FD2_ORACLE_FRAME_UNITS=1` 傳遞並記錄來源。
- 驗證原始資料與128筆端點、非法範圍及記憶體不變；固定第十二章計畫
  實跑還須逐byte核對controls、停點、trace與所有PNG，新增欄位之外的
  逐格metadata保持。返回地圖caller取得完整33筆來源才可關閉工具工單。

本READY只授權可重跑的觀測工具，不驗收重製地圖返回或PLAYER-E2，
也不改近堆重用初值的工具政策。

## 9. 同時點地圖狀態

狀態：**READY**。日期：2026-10-05。工單：[fd2_re #173](https://github.com/wicanr2/fd2_re/issues/173)，父項#166。

固定EXE身分同第2節；審查來源為fd2_re的 docs/data/ida/fd2_physical_background_selection_20261004.json，physical_map_runtime_evidence／spec。既有IDA9.4原始定位保持，未重做closed RE。

- -map-state預設停用，需要frame-dir或eip-trace；FD2包裝以FD2_ORACLE_MAP_STATE=1傳遞。非法值或沒有目的拒收。
- 16項LE全域保留address、width_bytes、raw_hex與unsigned value：53C07／53C0B／53C0F／53C1F／539F4／53A40／53A00／53A04／53A08／51A93為dword；046C／60000為word；51AAB／51AAC／60002為byte；51A0C為dword。
- 在既有有界trace或已接受frame同一指令同步讀取globals、view與171完整units。任一越界、units非法或缺mode13 video時map_runtime_valid=false，不發布半份globals／palette。
- 色盤取正式Palette API的256個RGB8，另存768-byte DAC6；驗證RGB8==(DAC6<<2|DAC6>>4)。不從像素推導，不改port counters、guest memory或CPU。
- 停用保持原metadata位元組；不改PNG去重、settle、窗口及上限。
- 驗證寬度／signed bits、端點／越界、缺video／units、palette round-trip、guest memory與ports不變及停用。來源提交後同槽／seed／1539controls重生，四停點、既有trace原欄位及44PNG保持。
- 290C2與11CAC入口觀測arg1；11EED／11D3B觀測copy出口。consumer依前狀態合成、後狀態核對，不由後狀態再推cycle。

只授權觀測工具。raw／palette留本地work，公開庫只保存工具與hash。不提升map parity、原版配置器或PLAYER-E2。

## 10. 有界原始位元組變更觀察

狀態：READY。日期：2026-10-06。
工單：[fd2_re #197](https://github.com/wicanr2/fd2_re/issues/197)，父項[#52](https://github.com/wicanr2/fd2_re/issues/52)。

目前native第十二章固定槽前綴，seq3146在畫面(11,85)仍有原版118對remake116。
EIP trace能確認map work與physical copy plane重用指標，卻沒有目標byte的寫入值。
不以配置位址重用推定最後writer。來源為fd2_re主契約
docs/data/ida/fd2_terrain_mode3_review_20261001.json的native_profile_recheck。

工具只擴充apps/fd2/cmd/oracle/main.go，沿原始FD2.EXE固定身分與既有CPU／平台，
不改CPU、heap、鍵盤、RNG、影片、檔案服務或遊戲資料。

- 新選用旗標-memory-change為逗號分隔的1至16個不同LE線性byte位址，十六進位。
  預設空值停用；需要-run-dir。重複、空項、負值、非法字元與32-bit越界在原版啟動前拒收。
- 觀察窗口與上限重用-eip-trace-from／to／max，但變更收據有獨立筆數，
  不搶EIP trace預算。輸出-run-dir/memory-change.jsonl，檔案存在時拒絕覆寫。
- 每個允許觀察的instruction先讀宣告byte，再成功執行既有CPU.Step，然後讀同一byte。
  第一次先寫baseline；後續只在該instruction前後byte不同時寫change。
  控制邊界的外部狀態注入不冒充instruction寫入。CPU.Step失敗不寫change。
- 每筆保留kind、instruction_step、step、control_seq、eip_before、eip_after、
  address_space及samples。baseline的step是執行前；change的step是instruction_step+1，
  samples保留address、before、after原始byte值，無推測語意或自訂名字。
  一步多個byte改變合併為一筆，最多16個樣本。
- 用uint64與實際Mem長度檢查每個來源；越界輸出memory_valid=false及空samples，
  停止本觀察，不改guest或中止既有遊戲。消費端拒收false，不以0偽裝成功讀取。
- 同值寫入不會出現change，收據只能證明最後改變值的步驟。既有平台adapter的整批寫入
  仍標成當次Step來源，不稱為未觀察到的CPU內部逐byte指令。
- 停用時不新增既有checkpoint或最終metadata欄位，不建立觀察檔、不讀guest。
  啟用只在最終報告增加memory_change_observation的位址、窗口、筆數與上限；
  FD2 wrapper以FD2_ORACLE_MEMORY_CHANGE傳遞並在runner記錄同一設定。
- 最小回歸包含非法參數、首尾窗口與筆數、baseline／改變／同值、同一步多byte、
  memory越界、CPU.Step失敗、記憶體與CPU不變、停用及檔案拒絕覆寫。
  正式來源提交後以固定第三方短槽正常按鍵核對完整checkpoint／PNG／controls不變，
  第十二章既有native同槽計畫再驗目標byte來源。不得用新收據放寬像素比較。

這份規格只授權必要byte的唯讀觀察，不擴展到通用heap逆向，不驗收#52或PLAYER-E2。

審查2026-10-06：已核對oracle控制邊界、nativeHeap.observe及成功CPU.Step的順序。觀察前後限包住同一CPU.Step，不能把控制注入當writer；停用直接維持既有Step，runtime與heap來源不改。READY只允許上述工具契約，最後pixel writer仍未知。

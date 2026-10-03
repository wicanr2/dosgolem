# 346：固定160M的正常宇宙生成續行診斷

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03
範圍：獨立明示診斷分支，沿原正常輸入續行至160M。CPU／平台／主庫玩法與原100M／120M預設契約保持，不代寫資料、不跳過生成、不重送。

## 證據與決定

- 沿[345有界迴圈進度](345-moo2-universe-loop-progress.md)。主庫8f5dbc7b3eb2b3e93f8bd7262e2ff476ea2bd660，工具a666ae584ba4df9468c233af2a07823229b12e09。
- 官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，所有原位址dosgolem_high_le；CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30。
- 已證實：345完整收據c5ffbfda48e4ff7354452ace99bded92c4ccb31f4522e4cdc8534a34d84a4e44，兩次原17FD1E C21400正常返回17F037，第三例120M尚pending；三組576步與全部8503原344列／32PNG保持。
- 160M是一次性觀測預算，不保證生成完成，也不是玩法或正式遊戲設定。若160M仍同頁，先查生成producer／狀態變化，不連續盲加cap。主庫RE-first保持。

## typed契約與失敗邊界

- 新旗標DOSGOLEM_MOO2_UNIVERSE_CONTINUE_160M只接受值1；空值／未設定表示不開啟。
- 開啟時MAX_STEPS必須恰為160000000，且完整BANNER_RED_CLICK依賴鏈、44M Esc、1996-01-01、SEPARATE_DOS=1與frame prefix齊全。非160M、有早期滑鼠、改日期、缺依賴或其他值，在讀EXE前exit2。
- 沒有新旗標，原1..100M／完整旗幟情境的120M仍照345；160M仍拒絕。不接受140M／150M／160000001等替代預算。
- 僅延伸診斷for迴圈的固定上限與checkpoint適用範圍；原CPU.Step、事件、callback、IRQ、Bus、檔案服務、原始資料與兩側正式亂數不變。固定日期不是RNG seed。
- 120M在下一原CPU.Step之前保存完整CPU／FPU／VBE、RAM readonly、callback／IRQ、345第三例pending及640×480原圖；130M／140M／150M／160M只加只讀checkpoint。所有正常輸入時點與內容保持。
- 終態保存實際CPU拒絕／DOS exit或160M上限，不以probe shell exit0當完整開局通過。圖依實際人工檢視；原生成完成／下一頁／持久writer尚未知。

## READY審查與驗收

1. 核對345不可變收據與完整初態、正常RET和來源雜湊，現有cap／輸入／checkpoint守衛可查；資料與診斷契約充分才READY。
2. 只增四個有界區塊及五個已列guard條件；逆轉後source逐byte保持345。新旗標false時額外條件短路，不讀寫guest；CPU／平台不改。
3. 同公開binary重生原120M與新160M各一次。原120M完整345列與全部PNG保持；新160M到120M前的可比原列與原圖保持，只允許配置header的max_steps與新增checkpoint不同，不改造原終態紀錄當新收據。
4. 新120M同狀態點逐欄比對原eip17FCE4、完整R／六段／flags297h／FPU／VBE／virtual time／callback／IRQ／原圖／第三pending。160M以實際終態限定驗收，生成完成不提前列為pass。
5. 新CLI缺旗標／錯值／錯cap／缺依賴／早期輸入等負例與合法新正對照；舊338的17與339的22負例／100M及120M正對照保持。新增較早345回填與索引缺失守衛，其他guard保持。
6. CPU與平台未改，不重跑無關Go全套；固定EXE344全套只稱既有回歸。兩個原版由fresh417根檔／MOX.SET固定重生，原ZIP／patch唯讀。

## 工具與公開邊界

Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，原版外層450s／2GiB／2CPU／128pids／UID1000／network none。沿345低成本抽樣observer，不逐步全RAM雜湊。raw LOG持續寫忽略workplace，結束以gzip保存；原LOG／PNG／frame／正版資產不入Git，公開只提交probe／spec／索引／回填／guard。停前核對擁有權、精確HEAD、Git與Docker清理。

原ZIP3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f、patchZIP908d6b7b37ad580039c5d108bab2c64b28f51ba735485287d284d5f5242b98e5；417根檔／MOX.SET553bytes bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80。完整開局、持久姓名旗色writer、typed種族特性、正式RNG、人耳與remake同狀態未知。

## READY 證據審查

345不可變收據／原完整120M同狀態／兩RET／第三pending與CPU／probe來源核對通過。原輸入契約齊全，新160M只有明示固定診斷預算，無需猜玩法或代寫狀態。批准以上最小診斷與守衛實作；原新終態尚未驗，不提前CONFORMED。

## 限定驗收：固定160M續行、120M同狀態與第三正常返回

346限定CONFORMED，只閉合明示160M診斷分支、原120M同狀態及第三RET；生成完成／完整開局與remake同狀態未驗。CPU／平台完全保持345，主庫RE-first保持。

**已證實，預設保持**：同公開source重生120M與160M各一次，每側從fresh417根檔／官方1.31 EXE／553bytes MOX.SET開始。預設120M全部9085原345列／32PNG保持。明示160M在120M前全部9028可比原列保持，只正規化配置header的max_steps，新增兩個checkpoint／platform紀錄另外驗；沒有把原terminal重新命名作新收據。DTA檔案時間及其RAM雜湊依既有每次執行差異規則處理；每側RAM before／after相等，不宣稱跨次整個RAM逐byte相同。

**已證實，120M同狀態**：eip17FCE4，R=[48 8 1 5 2BD9D0 2BD9EC 2BDA74 F]、seg=[8 188 188 0 20 188]、flags297h；FPU control127F／status0／depth0／stack全0；VBE Active:true Bank:4 StartY:0 BankSets:1685 Writes:36595476 DisplaySets:50；virtual_micros227500108。callback mask2B／pending0／activefalse／started12／completed12，IRQ started28643／completed28643／activefalse／failedfalse；BIOS deliveries28663與IRQ計數分列。gen_waiting=[false false true]／gen_samples=[192 192 192]，保留原120M第三pending。新RAM唯讀、與該次existing checkpoint相同；全部完整platform欄位及31原frame保持。RGB3eeb511abe9d33ff110ce8e478b7d2c5073d36d6775a56622e64651ca8c63fce，PNG d0e387dc900c9955d82dc9c6b21a533d581ac383ba36abca9ce2de28b290f13a。原碼在下一CPU.Step前查詢，不改CPU／FPU／VBE。

**已證實，第三正常RET**：group2於120083995，dosgolem_high_le:17FD1E C2 14 00實際返回17F037。before_R=[57 8 8 F 2BD9F8 2BDA20 2842F0 2BDA74]，after只有ESP為2BDA10；原stack37F01700、pop4+imm20，全部其他R／六段／flags246h保持、readonly／valid真、error nil。從119946572觀察起點到返回137423步，三組waiting=[false false false]。345第三例「120M尚pending」仍是正確歷史；本輪已捕捉其較晚RET，不稱所有生成helper都完成。

**已證實，後段終態與人工圖判讀**：

| 原步 | EIP | 虛擬微秒 | 原圖 RGB SHA-256 | 畫面 |
| --- | --- | --- | --- | --- |
| 130000000 | 2385FB | 256253573 | 952d1290738a02c6ab8913f76fa5a09b367df0641215acb93b66ed2a61b4c775 | Generating Universe... |
| 140000000 | 17FCEC | 288293099 | a8df4696d7f20432e767727a476f6ecd9f4c3b3d8f3e233d70f275f9bf6ea527 | Generating Universe... |
| 150000000 | 2385F9 | 320809509 | eaafce611654316168d85456dae14b8c223460cc8e59786893d38d98eb0dbd14 | Generating Universe... |
| 160000000 | 17FD04 | 349146004 | 4a1d9efc9d15da575e330128f22d27e97f6d6616ac1b4184ec94efc2bd27f2c9 | Placing home worlds... |

四checkpoint皆RAM readonly，FPU control127F／status0／depth0，35個frame加final共36真實PNG；160M final等於同點原frame。150M至160M之間已更換原畫面文字，不能再稱整段停在同一次原生成呼叫；文字producer的原位址及玩家規則仍未知。

終態step_limit=160000000 eip=0x17FD04 unique_sites=39434，R=[6D 0 5 4 2BCEE8 2BCF04 2BDB44 F]／六段保持／flags207h，原bytes66 0F B6 4D 1C 66 39 CA 7C D3 40 66 3B 45 E4 7C。VBE Bank7／StartY512／BankSets2328／Writes46391556／DisplaySets77；沒有新CPU拒絕，probe exit0只是到固定上限。正式writer、typed種族特性、生成完成／完整開局、正式RNG、人耳與remake同狀態未知；固定日期不是RNG seed。

**來源及驗證**：四346有界區塊及五guard逆轉後逐byte保持345，CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30保持，source b389f6c6b661e534b92e0be060c31921c2251735594187c7ed842a2f1f12f7e2。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；新CLI29負例讀EXE前exit2、合法160M正對照與舊338的17／339的22負例、100M／120M正對照通過。CPU／平台未改，本輪不重跑無關Go全套；344固定EXE全套只稱既有回歸。

實際原版命令保持1996-01-01、44M Esc、SEPARATE_DOS=1、NEW_GAME_CLICK_AFTER_DISPLAY40、MENU_READY_CLICK、SETUP_ACCEPT_CLICK、RACE_HUMANS_CLICK、RULER_NAME_ACCEPT_CLICK、BANNER_RED_CLICK及本機frame prefix。第一側MAX_STEPS=120000000，第二側另明示DOSGOLEM_MOO2_UNIVERSE_CONTINUE_160M=1與MAX_STEPS=160000000；原99M press／99084355 release保持。兩個450s容器均完成，未增加cap來掩蓋CPU錯誤。

初次執行腳本以CAP字串代換時誤改HARDWARE_ESCAPE_STEP，兩次都在讀EXE前exit2；原版未啟動。原rejected raw／gzip保留，修正命令名稱後相同容器／公開source乾淨重跑。環境腳本錯誤不列為產品缺陷。

120M收據workplace/moo2-probe-346-120.txt.gz SHA-256 f8a87f0a377ccc16d53526263bbe2751cf90accfa43da9b4e9db0302385209dd；160M收據workplace/moo2-probe-346-160.txt.gz SHA-256 09038decb752495dfc75519e89dc8f0cedec997a42fe02c467085fd130d869af，共10523原列。160M final PNG c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca。全部LOG／PNG／raw／私有驗證腳本留本機忽略workplace，不入Git。

### 回填帳及下一步

| 不可變鍵 | 本輪已證實 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:17FD1E→17F037／SS188:2BD9F8／group2 | 原120M pending之後120083995正常RET；160M進入Placing home worlds...，完整開局未驗 | 345 | 原120M第三例pending的較晚正常返回與固定160M續行已由規格346驗證 |

下一步維持160M固定預算，用150M至160M的既有畫面變化縮小「Placing home worlds...」文字producer及其caller／狀態，建立最小DRAFT觀測，不逐行翻譯helper，也不直接提高cap或重送。此項是原版診斷閉合，非remake玩法驗收；主庫RE-first仍關閉。

83項規格回填正對照、新346的23缺證據／狀態／回填／索引負例與全部舊負例通過。來源與新收據1000:1000，工具樹無root-owned／.md目錄，兩個原版容器已結束並移除。

### 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/new-game-346-ready-verify.py | 3f57e33ac48b2f7c2d50e9f84ca72142617fd642929c6daccb9f232fa53c08a5 |
| workplace/new-game-346-ready-tests.txt | 2dc21fe750aa318c262d3d82286b37f22adf7a3f69b3f6cab7412a3f56e7db48 |
| workplace/new-game-346-source-verify.py | 0b36c270f693cda3e8568a7f545ebec8850ffd98abe6a4dfa780707c069522b7 |
| workplace/new-game-346-source-tests.txt | 3fae2a65f8aa3f5d4578fa6ae19f2c9bf1fb359ba1673638501968f85cbe673f |
| workplace/new-game-346-cli-verify.py | fa18280fad37abc1c8bab98e8d53e1d7c824bbbad640009f78eda64b77c0c79c |
| workplace/new-game-346-cli-tests.txt | 8b1443a99376737876f15ec2b99afee310602a31733b8377000feeac1e44d8f8 |
| workplace/new-game-346-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-346-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |
| workplace/new-game-346-run-120.sh | 67af7bb351387ccc2f58f41f2b14a58db005eafe6a407a0d2e22793d27163e66 |
| workplace/new-game-346-run-160.sh | adeb31446d7297b6c6feeded05b615d0e3dc5bed2f2f7c0a9eb5402af09eba1c |
| workplace/moo2-probe-346-120.raw.txt | 00c7be8ca72851f793879eac0414edb795f98b001aa769d34ee8c0a7c6ad4eb1 |
| workplace/moo2-probe-346-160.raw.txt | 149c5c3f99ef56be28d84dc49e940d36b41c96d4919fc42cd97ea47b9ca657a5 |
| workplace/moo2-probe-346-120.txt.gz | f8a87f0a377ccc16d53526263bbe2751cf90accfa43da9b4e9db0302385209dd |
| workplace/moo2-probe-346-160.txt.gz | 09038decb752495dfc75519e89dc8f0cedec997a42fe02c467085fd130d869af |
| workplace/moo2-vbe-346-120.png | d0e387dc900c9955d82dc9c6b21a533d581ac383ba36abca9ce2de28b290f13a |
| workplace/moo2-vbe-346-160.png | c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca |
| workplace/new-game-346-verify.py | 76cc426525a1a6ce04fb2c6e7ed46c50127ebb7d5285295790eba88c2436c698 |
| workplace/new-game-346-tests.txt | 51a032f667f7fd516274638474f53b01deb5a79d07f37f9f58ac02b8c192eb0b |
| workplace/new-game-346-backlink-verify.py | 1f773cef7c8db3496e749658244ce4eae5a34b21566a275609b9fc5f4f56c7e8 |
| workplace/new-game-346-backlink-tests.txt | 0b7a0ab2dfee09a5829c44e10d215d65e47fe009a910de4b53e23d3b03a2dd4a |
| workplace/moo2-probe-346-cli-rejected-120.raw.txt | 531247bf00fd36b424407cf703a6460b06e5fc7deec8d03f7e096e95cee5f8aa |
| workplace/moo2-probe-346-cli-rejected-160.raw.txt | 108e19d356c0b577c4ebb15ab6ca32aa18917196bd37f0eec2db7b234294e1b3 |
| workplace/moo2-probe-346-cli-rejected-120.txt.gz | 13783d6f70719c0ad0ec999a76cfcd695f0e9768a6cdd10994b55bb1c8643fdd |
| workplace/moo2-probe-346-cli-rejected-160.txt.gz | 56d6959c3ae0202b375c3f764047f1bd436a568ebed6c669f11118a4870ef58f |
| workplace/new-game-346-frames.json | 008b17b9ed7b033fba7a33c77e47964527f9a9ff4bcd7d6eb52d4e035da47359 |

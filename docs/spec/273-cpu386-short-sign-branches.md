# 273 — 短距離負號與非負號分支

狀態：**CONFORMED（有限 CPU 與分支範圍）**
日期：2026-10-01
範圍：32-bit CPU、無前綴的 `78 rel8`（JS）與緊鄰的 `79 rel8`（JNS）；不改 remake 玩法。

## 原始定位與擬議契約

[272-cpu386-inc-word-memory.md](272-cpu386-inc-word-memory.md) 固定 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，無事件／受控事件第 7,684,074／7,684,109 步停於 **dosgolem 高位 LE 線性** `0x21C2D6`，原始 bytes=`78 06 2B C2 79 02 EB EC 5E 61 C3 68 24 00 00 00`；EAX=2、EBX=Eh、ECX=10000h、EDX=1、EFLAGS=202h。JS 與 JNS 的 DOSBox-X 候選 **CS:EIP** `0180:003502D6`／`0180:003502DA` 已由下列有限樣本核對。

[Intel Jcc](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/Jcc.htm) 定義 JS 在 SF=1 取分支，JNS 在 SF=0 取分支；位移是下一指令之後的有號 byte，不改旗標。32 位元 EIP 以模 2^32 加法，其他暫存器、段與記憶體保持。這不是有號大小比較，不取決於 OF／ZF／CF／PF。只增加兩個 opcode；其餘短／近分支與未審查前綴保持。截短位移拒絕且不發布暫存器／旗標／記憶體變更；目前 CPU 取指後不回復 EIP，既有分支不預檢目標 descriptor 或實作 x86 例外，這些模型界線須明示。

## 驗收與停止線

JS／JNS 的 SF 兩值、所有五個其他算術旗標組合、有號位移全 256 值、正負與 EIP 環繞、未取分支也必讀完整位移、全部狀態保持及未知前綴拒絕。以原版實際 SF 與分支後完整狀態驗證，不注入原版旗標；若有不同初始狀態，照實記錄。取得原版有限分支樣本並審查 READY 後才實作；CPU、固定原檔全套與兩條 dosgolem 自然路徑後才限定 CONFORMED。不追該 helper 內部或硬體時鐘，不把 Simtex 圖像寫成已進主選單／最終畫面對拍。255 及音效／受控亂數／Go remake 玩法同狀態仍未完成。

## 原版樣本與 READY 審查

**已證實**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`、`startup_probe_131.py --short-sign-branches`，固定上述 EXE 與 417 根層原檔；MOX.SET=553 bytes、SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`，未注入旗標。

JS 在 **CS:EIP** `0180:003502D6`、16 bytes=`78 06 2B C2 79 02 EB EC 5E 61 C3 68 24 00 00 00`；EAX=1、EBX=Eh、ECX=0、EDX=1、ESI=470h、EDI=3A2090h、EBP=3EBC06h、ESP=3EBB9Ch，DS／ES／SS=188h、FS=0、GS=20h、EFLAGS=202h。LOG 2 與後續 EV 實際確認不取分支至 **CS:EIP** `0180:003502D8`、所有擷取暫存器／旗標保持。

下一 JNS 實際在 **CS:EIP** `0180:003502DA`、bytes=`79 02 EB EC 5E 61 C3 68`；EAX=0、EFLAGS=246h，其餘擷取狀態同上述 JS。LOG 2 與後續 EV 實際確認取分支至 **CS:EIP** `0180:003502DE`，所有擷取暫存器／旗標保持。兩者 SF=0；另一 SF 值以公開標準及受控 CPU 測試驗證，不冒稱為原版該處曾取另一分支。

JSON／JS LOG／JNS LOG／終端 SHA-256 `98e6846cd4ac9647c4c707b03bd7c171c0858f156a1abbceebef5dff326ebf15`／`e3321b3f57f71081c80da868cfe19f6e82bb772ad8e60ba0ce678b1631a3e197`／`471a7afdf0c463e5c1eb72be1e37d64fc4443d2a3361a5bb3ee00a1123547970`／`fe479f44d4dc56c87b5c28cedc02c376972c419c5d0226a6083c422fa4710c29`；完整資料留本機，不進 Git。

dosgolem 起點 `3a0c6b4cb475acc75588badff6436fd3612800d4` 的 JS 初值 EAX=2／ECX=10000h，與 DOSBox-X 的 1／0 不同；實際 SF 與完整 EFLAGS 均 202h，可比較該單指令條件，不宣稱整段時間／資料布局同狀態。已具備公開規則、固定 bytes 與兩個實際分支後結果，DRAFT 審查後轉 READY 才實作。測試範圍涵蓋 SF 兩值、其他旗標干擾、全 rel8、環繞／截短／前綴／保持契約與原版最小樣本；停在有限 CPU／分支，不追 helper 或硬體時鐘。

## CPU 實作與驗收

READY 後只新增 78／79 的無前綴分支，完整取 rel8 後依 SF 決定目的；不寫旗標、暫存器、段或資料記憶體，32 位元 EIP 的有號位移環繞沿用既有分支模型。其他短分支與 0F 88／89 近分支保持；目標 descriptor 例外與截短後 EIP 回復不在目前模型，沒有暗中聲稱硬體例外一致。

`short_sign_branches_test.go` 核對兩 opcode ×六算術旗標全 64 組×全 256 rel8，確認只依 SF、精確兩 bytes 取指與全部狀態保持。另以六個 EIP 起點、SF 兩值及全 rel8 驗環繞；即使不取分支，缺位移也拒絕。全部 segment／operand-size／REP／LOCK／address-size 前綴拒絕，既有近 JS／JNS 抽樣保持。原版最小樣本只重定位代碼，保持 JS／SUB／JNS 順序，原版 EAX=1／202h → JS 不取 → SUB 的 EAX=0／246h → JNS 取分支至 POP 前均通過；這是有限 CPU 控制流樣本，不是整段時間／布局同狀態。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，私有 `workplace/cpu-test-273.txt` SHA-256 `75c97fa0a346ebd8f40805126835938477e3454df7d0bd50bf2a29dc9dfa7237`。

固定 EXE 雜湊＋高位 LE `0x21C2D6`／DOSBox-X CS:EIP `0180:003502D6`＋`78 06` 為回填不可變鍵。272 必須保留「短 JS 停點已由規格 273 接通」與本檔連結，`--check-short-sign-branches-spec-backlinks` 的正例、刪原始定位／舊標記的拒絕例、全部既有回填函式、Python 語法與擁有權通過。正式 dosgolem 重跑仍必須自行重生兩條自然收據，不使用輔助圖片替代。

## 診斷步數界線與驗證腳本訂正

第一輪全套 Go 測試通過，但兩條自然探針均到既有 8,000,000 步上限，正在不同位置的初始化迴圈，未遇 CPU 不支援錯誤。上限路徑原本不輸出 VBE PNG，導致外層最後 sha256sum 找不到檔而退出 1；這是驗證腳本問題，不是產品或 JS／JNS 失敗。首次 Go 全套／無事件／事件 gzip SHA-256 `e35e0e9b43e41ed14fab339bf97bbedc19f093117abb542862cd45417b7ded4e`／`c5895a3604da834fbafd41c03785554322268105efdec9edde8969bc2cc55328`／`b56f97a48e5592e103d9420c2512c6506ef065fc31c9d112c04662f3eed6f248`，私有檔名改留 `273-first-limit`，不覆寫失敗成因。

診斷入口的 `DOSGOLEM_MOO2_MAX_STEPS` 將限制為十進位 1–50,000,000；未設定仍 8,000,000，無效值拒絕。只改觀測上限，不改原版輸入、CPU、虛擬時鐘或亂數。步數上限與 CPU 拒絕共用同一 VBE 擷取，避免未輸出假失敗。本輪使用明示 16,000,000 上限、相同 Docker 映像及全套／兩條原版命令乾淨重跑；不因上限停住就猜補硬體時序或初始化語意。

## 固定原檔全套與 16,000,000 步重跑

修正診斷入口後，以相同 Go 1.24.13 映像及固定原版輸入乾淨重跑，`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過；私有 `workplace/full-test-273.txt` SHA-256 `abf7ee78fdd09afe67ddba3c0a05f4a9a0e0c5fad5301e507b457f407b1f2815`。另以編譯出的診斷入口核對步數 0、-1、50000001、非整數與整數溢位，皆在讀原版前以 exit 2 拒絕，不放任無界執行。

兩條原版命令加 `DOSGOLEM_MOO2_MAX_STEPS=16000000`，各最多保存前三次 JS／JNS 前後狀態。無事件首 JS 第 7,684,074 步至高位 LE `0x21C2D8`，JNS 第 7,684,076 步至 `0x21C2DE`；受控事件相對各 +35 步。JS 的 EAX=2／EFLAGS=202h、JNS 的 EAX=1／EFLAGS=202h，兩個分支均完整保持擷取暫存器、段與旗標；這與 DOSBox-X 的 EAX=1→0／202h→246h 是不同初始狀態，只確認相同 SF=0 的有限控制流，沒有把數值差異藏掉。

兩條均達 16,000,000 步、無 CPU 拒絕，停於不同動畫處理指令：無事件高位 LE `0x228D24`，事件 `0x21A9B8`（精確停點以原始收據的 step_limit 列為準）。gzip SHA-256 `98cb5eda2583cd5637c8dbffd7de575445cf00c960499b5c020b622ab575230c`／`84dfffe57f2a7f02caa90bc7c25f4b51722f0bee3727a03647fb818957d16763`。Active=true、Bank=9、StartY=512、BankSets=260、Writes=3436260、DisplaySets=5；索引 SHA-256 `374d95bd2a61908aff015c243954c365c3e6c1b747a2e1885f185ff195cbb280`、RGB `713d26a2d51028a67d56de17d4bc3c71958bdd0c5c4a8bf187bbd186b06eb5c7`，兩份 PNG SHA-256 均 `98cb13ebdeda137d02d88dc49d15ccbd833cc12b82fb8bbf7fd09166dd56012b`。已實際檢視為黑底、展開中的 MicroProse 啟動動畫，不冒稱主選單或穩定完成畫面。

為確認下一真正缺口，維持輸入與虛擬時間、改用明示 50,000,000 步上限續跑；不重跑已通過的 CPU／全套，只重新觀測兩條正常啟動路徑。

## 50,000,000 步有界觀測與下一實際停點

維持固定原版資料、CPU 及虛擬時間，`DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=<本機輸出> go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。兩條未到上限，無事件第 20,100,561 步、受控事件第 20,100,596 步在 **dosgolem 高位 LE 線性** `0x239A42` 的 `66 31 FF 8E C7 CD 2F 66 89 3D 22 BC 2A 00 66 C7` 拒絕；word XOR 尚未支援，原阻塞已跨過 12,416,487 指令。拒絕後 EIP=239A44h，EAX=F1684h、EBX=5、ECX=239F20h、EDX=1、DS／ES／SS=188h、EFLAGS=246h、DOS 呼叫 6；受控回呼 started=1／completed=1，先前布局差異仍保留。

私有 `workplace/moo2-probe-273-50m-full-game.txt.gz`／`workplace/moo2-probe-273-50m-mouse-event.txt.gz` SHA-256 `5994ce355565faaefe780bac97836cce37dc9ad98f4564d7eb192f7752734a09`／`1dda8342da159dd5720e6d9f8eb81b5fdf02ade47250ff9a8b728d6f907bb48a`。Active=true、Bank=2、StartY=0、BankSets=441、Writes=4046164、DisplaySets=6；索引 SHA-256 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`；兩 PNG SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，與先前已檢視黑圖逐位元相同。此前 Simtex／MicroProse 啟動圖像成立，當前停點仍是黑圖；不以啟動標誌宣稱主選單或最終畫面對拍。

273 僅在短 JS／JNS 的定義條件、保持狀態及有限原版分支範圍 CONFORMED；255 仍 READY，主選單、玩家操作、音效、受控亂數與 Go remake 玩法同狀態未完成。下一步依公開 XOR 契約核對 word 暫存器、實際 DI 初值／高半部、旗標與下一 MOV ES 消費；DOSBox-X 候選 **CS:EIP** `0180:0036DA42` 尚是地址假說。AF 未定義、近／短分支例外模型、IMUL 未定義 ZF、DIV／SAR、DTA 保留區與平台布局限制明示，不追 INT 2F 平台內部或 helper 內部。

## 後續停點解析回填

word XOR 停點已由規格 274 接通：[274-cpu386-xor-word-register.md](274-cpu386-xor-word-register.md) 連接固定 EXE 的高位 LE `0x239A42`／DOSBox-X CS:EIP `0180:0036DA42`／`66 31 FF`，公開 word XOR 契約、原版高半部保持／定義旗標與下一 MOV ES 的 null selector。AF 未定義與資料布局差異保持，後續驗收狀態以 274 為準，不重開 JS／JNS 或宣稱整體玩家路徑完成。

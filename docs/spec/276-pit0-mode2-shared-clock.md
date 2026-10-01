# 276 — PIT 通道 0 的模式 2 設定與共享週期

狀態：**CONFORMED（限定 PIT 設定與共享週期近似）**
日期：2026-10-01
範圍：既有 LEPIT0 接受通道 0 的二進位模式 2、先低後高寫入；以既有共享虛擬時鐘消費合法除數。屬硬體規格近似（hardware-spec approximation），不改 Go remake 玩法、不逆向計時驅動程式、插斷服務常式（ISR）或忙碌等待（busy-wait）。

## 原始定位與平台前提

[275-moo2-protected-vtd-entry-query.md](275-moo2-protected-vtd-entry-query.md) 固定 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，無事件／設定後事件第 20,101,398／20,101,433 步，在 **dosgolem 高位 LE 線性** `0x239AE8` 的 `E6 43 EB 00 88 D8 E6 40 EB 00 88 F8 E6 40 A1 48` 拒絕。AL=34h、EBX／EDX=174Eh、ECX=0、DS／ES／SS=188h、flags=246h；對應 **DOSBox-X CS:EIP** `0180:0036DAE8` 已由下列有限擷取核對，不能以位址差當成同狀態。

[Intel 8254，231164-005（1993-09）](https://www.cs.cmu.edu/~410/doc/8254.pdf) 的頁 6 控制字、頁 10 模式 2、頁 17 圖 22：34h 選通道 0、先低後高、模式 2、二進位；模式 2／3 合法初始除數為 2..65536，編碼 0 表示 65536，1 不合法。模式 2 的重複週期為 N 個輸入時脈，不能因週期相同把輸出波形與模式 3 混稱。PC 埠映射與既有模式 3 見 [186-fd2-platform-gap-continuation.md](186-fd2-platform-gap-continuation.md) 批次 83；共享時鐘公式與差異見 [250-moo2-bios-clock-attach.md](250-moo2-bios-clock-attach.md)。

## 擬議契約與驗收

- 43h 只接受 34h／既有 36h；以型別化 Mode 保存 2／3，清未完成的低高組字與 Loaded，不發布新除數世代。
- 40h 未設定時拒絕；第一筆只記低位元組，第二筆形成 16 位元除數，0 轉 65536，1 拒絕且保持第二筆前的狀態。合法完整寫入後才更新 Reload、Loaded 與 Generation。重寫控制字清半筆；其他通道、BCD、其他模式／讀回／鎖存拒絕。
- 模式 3 的除數 1 先前未拒絕，須依公開最小值訂正；其原版 36h／0／0 收據與合法模式 3 不失效。未知與未執行原版的非法值不可假稱原版實測。
- 沿既有共享 LEBIOSClock 使用除數，保持每步一微秒與分數時基，第二筆完成時重設相位；控制字後未完成首次除數期間暫停計數，重寫已載入除數時先沿用舊值、完整後立即切換屬既有模型近似，與硬體於週期末載入的相位不同。IF／PIC 遮罩、待處理邊緣合併、客製向量拒絕不放寬。不宣稱實機時鐘、脈波、取樣率或音效同狀態。
- 原版有限三筆 OUT 及第一個後續讀取、CPU 暫存器／旗標、原版兩側除數條件須先取得才 READY。以全 16 位元除數、模式／重設／拒絕、共享埠紀錄及實模式／保護模式、既有時鐘週期與拒絕邊界驗收；固定原檔全套與兩條自然路徑後才限定 CONFORMED。
- 工具不新增遊戲存檔／UI 欄位；原檔、完整終端／記憶體及 PNG 只留本機。平台通過不等於正常玩家畫面或受控亂數對拍；255 仍 READY。

## 原版有限擷取與 READY 審查

**已證實的有限原版樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；`startup_probe_131.py --pit-mode2`，固定上述官方 EXE、417 原檔及 MOX.SET（553 bytes、SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`），未注入 CPU／記憶體或時間。

起點 **DOSBox-X CS:EIP** `0180:0036DAE8`，32 bytes=`E6 43 EB 00 88 D8 E6 40 EB 00 88 F8 E6 40 A1 48 F1 39 00 3B 05 48 F1 39 00 75 08 3B 05 48 F1 39`。完整暫存器 EAX=1734h、EBX=174Eh、ECX=0、EDX=174Eh、ESI=C8h、EDI=8、EBP=8、ESP=3EBAD4h；DS／ES／SS=188h、FS=0、GS=20h、EFLAGS=246h。LOG 8 實際執行七條指令，三筆 OUT 為 43h←34h、40h←4Eh、40h←17h，合法除數 5966；到 `0180:0036DAF6` 時 EAX=1717h，其餘擷取欄位保持。LOG 2 執行下一 A1 讀取 **DS offset** 39F148h 的 dword，EAX=0、到 `0180:0036DAFB`，其餘欄位保持。最後一行 LOG 是尚未執行的指令，不聲稱後續比較／等待已完成。

本機 JSON／三筆 OUT LOG／下一讀取 LOG／完整終端 SHA-256：`c522c19a88f1ea333ec2491befcb3e23b1dafab284e98b39e7728bc92f93dd74`／`a135930b7be27ea0b394ace213d093487cf463fa298b7b3a4e51232b0bc9ab70`／`cdbdf8c52ad6818a174d68dc0bd0bf0c04c4c175bc33486450a58446fab277ab`／`609ca9fe3eb4e9b6d5dd394c20061670b1d85399d9a5a0c23e219c4fdedee318`。DOSBox-X 只作輔助有限原版樣本，正式 dosgolem 收據須自行重生。

公開 Intel 合法值／週期與原版三筆寫入相符，證據足夠轉 READY；不再追 driver／ISR／忙碌等待。模式 2／3 除數 1 的拒絕是硬體規格導出的工具邊界，未對原版注入非法值；兩模式相同除數只有週期可共用，Mode 仍須保留。既有每指令一微秒、改除數立即重設相位、半筆期間暫停及 IRQ 合併均保留近似標記，不能冒稱硬體同波形／同時鐘。

## 實作與平台測試

READY 後 LEPIT0 接受 34h／36h，Mode 保存 2／3；合法完整除數才發布世代，非法 1 保持第二筆前狀態。共享 LEBIOSClock 的公式、IF／PIC／向量拒絕及近似不變。既有模式 3 的未知測試改以 BCD 35h 拒絕，並保留原先模式 3 合法值測試。

驗收兩模式全部 65536 個編碼、未完成／重寫／控制字重設、全部其他控制字及其他埠拒絕。真正的實模式 CPU 經 DPMI 匯流排設定模式 3，再由保護模式 CPU 執行原版七條 OUT 設定與重定位後 A1 讀取；兩側共享同一埠、模式與時鐘，完整暫存器／段／旗標、OUT 紀錄與程式 bytes 符合。50,001 微秒的 5966 除數累計 10 次；65536 第一週期 54,926 微秒，遮罩／IF 合併、未載入的暫停與非零客製 IRQ0 拒絕均通過。這些都是公開規格近似及執行器回歸，未量測原版硬體時鐘。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/machine -run 'Test(PIT0|BIOSClock)' -count=1` 通過；`workplace/platform-test-276.txt` SHA-256 `f096373c82ba90e9bdc635964167dbe3d7b657c7cab4da197131350bfcf18512`。正式 dosgolem 自然收據尚待重跑，255 仍 READY。

## 探針尾端容量訂正與自然收據

固定 EXE 全套成功後，前兩次自然探針以非零狀態結束；第二次保存 `signal: killed` 及模式 2 設定後的只讀狀態，`workplace/moo2-probe-276-first-run.txt.gz` SHA-256 `dad7ce3ec4dfe714a1f8d5c77753ad3e90b0ff2555a3dfe95bbb4720865c92c2`。首輪暫存輸出隨容器消失，未保留 cgroup 計數，不能確稱核心 OOM。

**已證實的驗證腳本缺陷**：原先最後 32 筆的 slice 以 `ring=ring[1:]` 移除首元素，同時縮小容量；append 擴容後又繼續累積。100,000 步重現舊 len=99977／cap=122880，新固定移位 len=32／cap=32，最後 32 筆完整保序；`workplace/probe-ring-276.txt` SHA-256 `ba1b2922445398b59c65c500e05e8a09f4cc02b3f13f7ae9a54f674e4c46691e`。只修觀測緩衝，不改 CPU／原版輸入、虛擬時間或等待結果。`signal: killed` 與無界緩衝的關聯為強推論；修正後同一 2 GiB／2 CPU／128 PID 設定乾淨執行，記憶體峰值 775704576 bytes，cgroup oom／oom_kill=0。額外的 shell 引號與 Go 快取權限錯誤發生於驗證控制面，修正後執行；不當產品缺陷。

Go 1.24.13 固定原版全套 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 通過，`workplace/full-test-276.txt` SHA-256 `1604659fa9545f812655752abb417fc2878a79194720dfccc7c9a54b8e923e7d`。之後觀測緩衝及只讀診斷的修正由原版自然執行重編譯驗證，不重做無關測試。

`DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=<本機輸出> go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，事件一路另加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`，均由 dosgolem 重生：
- 無事件第 20,101,398／20,101,399／20,101,402／20,101,405 步，控制前、34h 後、低位後與完整高位後，**高位 LE** `0x239AE8／0x239AEA／0x239AF0／0x239AF6`。事件一路各晚 35 指令。
- EAX=1734h→174Eh→1717h；EBX／EDX=174Eh、ECX=0、ESP=2BDAF4h、EBP=8、ESI=C8h、EDI=8；CS=8、DS／ES／SS=188h、FS=0、GS=20h、flags=246h 保持。Mode 3→2，Loaded true→false→true，完整 Reload=5966、Generation 4→5。低位未發布，控制字與高低寫入同有限原版選擇；兩側布局／stack／完整虛擬時間不同，不宣稱完整同狀態。
- 兩條均到 50M 上限，無事件 EIP=`0x239B03`、事件 EIP=`0x239B09`，末尾交替 CMP／JE 等待；沒有新的未支援 CPU 拒絕。無事件 bytes=`3B 05 48 11 27 00 74 F8 5D 5F 5E C3 00 B8 01 01`，事件由 `74 F8` 起。EAX／ECX=0、EBX／EDX=174Eh、DS／ES／SS=188h、flags=246h。這是尚未退出的等待，不是主選單或成功啟動。
- gzip SHA-256 `1a740e3cb2e56eec53dff999928bc72b969a3d036f83c2a031589a07090e25f7`／`57ce3b43683a2b1d5dacb301f7f6a9f766bae9acec5eb08bd264a12b203bc29e`。

只讀診斷另以相同輸入、有界 20,120,000 步核對：PIT Mode=2／Reload=5966／Generation=5；共享 clock Micros=20121042、Deliveries=1538，較設定前多三次；DPMI 實模式 08h／1Ch 均 0000:0000，絕對 IVT 08h／1Ch 皆零；**高位 LE DS:offset** `0188:00271148` 可讀四 bytes 均零。收據 `workplace/moo2-probe-276-boundary.txt.gz` SHA-256 `dc92747ae37dba5d2124cc8f808e2eff0027f87525ec4873b926bf3f813d0862`，cgroup oom／oom_kill=0。已證實平台預設時鐘有推進，而等待來源未變；等待來源的完整語意及正式遊戲向量派送尚未知，不用猜測的 counter 寫入讓它退出。

三張 PNG 都保持已檢視黑圖 SHA-256 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，Active=true、Bank=2、StartY=0、BankSets=441、Writes=4046164、DisplaySets=6；索引 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`，不重做相同圖像的目視驗收。

276 限定 PIT 模式 2／3 的合法設定與共享週期近似 CONFORMED；原版模式 2 的三筆寫入已自行重生，時鐘測試不代表原版逐週期一致。255 仍 READY；主選單、正常玩家操作、音效、受控亂數及 Go remake 玩法同狀態未完成。下一最小行動只讀核對 DOS 向量服務 `INT 21h/AH=25h／35h` 的實際參數與平台保存／派送接線，既有 s.dosVectors 與實模式 IVT 為不同儲存；尚未證實 08h 已安裝於該表。公開 IRQ 契約足夠即停止，不反組譯 driver／ISR／busy-wait、不追實機時鐘。

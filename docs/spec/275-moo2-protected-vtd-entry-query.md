# 275 — MOO2 未安裝 VTD 的裝置入口查詢

狀態：**CONFORMED（限定未安裝 VTD 的平台查詢）**
日期：2026-10-01
範圍：明示 MOO2 DOS 啟動設定的保護模式 `INT 2Fh/AX=1684h/BX=0005h`，ES:DI 輸入為 0:0；未安裝 Windows／VTD。其他裝置、非零輸入或未審查模式仍拒絕，不建模 VxD／Windows 內部或時鐘。

## 原始定位與平台前提

[274-cpu386-xor-word-register.md](274-cpu386-xor-word-register.md) 固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，無事件／受控事件第 20,100,563／20,100,598 步在 **dosgolem 高位 LE 線性** `0x239A47` 的 `CD 2F 66 89 3D 22 BC 2A 00 66 C7 05 24 BC 2A 00` 拒絕；EAX=F1684h、EBX=5、ECX=239F20h、EDX=1、ES=0、DS／SS=188h、flags=246h。候選 **DOSBox-X CS:EIP** `0180:0036DA47` 已由下列樣本核對，位址空間不可混用。

[RBIL 裝置 API 入口](https://delorie.com/djgpp/doc/rbinter/id/51/45.html) 定義 AX=1684h、BX 裝置 ID、ES:DI=0:0；不可用時返回空指標。ID 5 的 VTD 定位依同頁表。未安裝 Windows 的多工服務不一定接管，此時不應憑名稱主動清除完整 EDI 高半部或 EAX。DOSBox-X 的 [多工服務及未處理入口](https://raw.githubusercontent.com/joncampbell123/dosbox-x/master/src/dos/dos_misc.cpp) 可作輔助契約，原版的完整返回及保存消費尚待有限擷取。

## 擬議契約與驗收

typed 條件只限 moo2Profile、低 AX=1684h、低 BX=5、ES=0、低 DI=0；高半部可能非零，不能當不同 DOS API。此未安裝環境保留完整 R／段／EIP／EFLAGS／記憶體及 DOS 呼叫計數，不虛構入口或時間值。其他形狀保持拒絕且不發布狀態。

需核對原版同次完整暫存器／段／旗標、返回位址、入口 record 的 word 寫入與第一個消費；證據足夠才 READY。八暫存器高半部／各旗標／連續查詢、未知裝置／非零 ES／DI／其他 profile 拒絕、原版最小 consumer、固定原檔全套及兩條自然路徑均納入驗收。工具不保存新的遊戲狀態，不改 UI／存檔／正式亂數；原檔、終端、記憶體及 PNG 只留本機。CONFORMED 只限此平台空入口，不代表主選單／正常玩家玩法同狀態，255 仍 READY。

## 原版返回、保存與 READY 審查

**已證實的有限原版樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --vtd-entry`，固定上述 EXE、417 原檔及 MOX.SET（553 bytes、SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`），沒有注入暫存器或旗標。

實際 **CS:EIP** `0180:0036DA47 → 0180:0036DA49`，前 40 bytes=`CD 2F 66 89 3D 22 9C 3D 00 66 C7 05 24 9C 3D 00 00 00 66 8C 05 26 9C 3D 00 07 8B 0D 22 9C 3D 00 66 8B 15 26 9C 3D 00 85`。EAX=F1684h、EBX=5、ECX=36DF20h、EDX=1、ESI=3EBB20h、EDI=380000h、EBP=0、ESP=3EBAD0h；DS／SS=188h、ES=0、FS=0、GS=20h、EFLAGS=246h。返回時完整擷取欄位保持；ES:DI=0:0，沒有清除 EDI 高半部，也沒有合成 AX=0 的成功值。

入口 record **DOSBox-X DS 線性 offset** `0188:003D9C22`，查詢前後六 bytes 均 0。LOG 8＋EV 實際確認低 word DI 保存於 +0、零 word 寫於 +2、ES 保存於 +4；之後 POP ES 恢復 188h，載入 ECX=0／DX=0、TEST 後到 `0180:0036DA70`、ESP=3EBAD4h、flags=246h，record 六 bytes 仍 0。debugger 的 C7 文字顯示 dword，但原始 66 前綴與立即數長度是 word；以原始 bytes／實際 EIP 為準，不把顯示名稱當證據。尚未實際執行其下一條件分支，不宣稱其分支後結果。

JSON／LOG／終端 SHA-256 `e7068aa1b1d301609c58f2211e37cbbc66bb6b2588a8482f877033853bfc3e90`／`487745aa92e31e78218f9b990f2a12be5bccb6db00bdca3b8f173aa620ae1789`／`201dd5795433bb353b7dcd3a2f0bcf0d69638eb40feb4e96d6f46071ff77760c`。首輪 LOG 2 的最後一筆尚未執行，EV 停在第一個保存後；因此不足以聲稱完整入口消費。原收據保留為 `vtd-entry-first-*`，JSON／LOG／終端 SHA-256 `15a164cd7a3ad8ffc076d3958ffb8930a14e8ea806458764ff71fba4fa4666e7`／`2bcd2d2b21fe600b06b0a9d4777a824631f8b9cde3a1b03275637f46af7d467f`／`b07a5a2a725b655d04596bf2d8cab3e352d3394dcf58a7b244d802262657feda`；同一映像擴大有限觀測，不改原版輸入。

[RBIL 61 的表 02642](https://fd.lod.bz/rbil/interrup/windows/2f1684.html) 明列 ID=0005h 的 VTD。公開契約與原版空入口、完整返回、caller record 寫入／讀取相符，證據足夠轉 READY；只限明示未安裝裝置的 MOO2 平台。dosgolem 起點 `45b46688666ff21d38402870d8f585b563dc2d58` 的高位 LE 與 DOSBox-X CS:EIP、EDI 高半部／布局不同；不能宣稱完整同狀態。最小 CPU 消費樣本只驗既有兩筆 word 保存，下一 selector 保存／完整 caller 由自然路徑分類，不為本平台規格擴張未知 CPU 能力。

## 首輪最小測試環境訂正

查詢狀態保持／拒絕測試均通過；最小 caller 測試未設定 DS=188h 的可寫段描述子，既有 CPU 正確拒絕 MOV word 寫入。這是測試環境缺件，平台契約與 production 不變。依既有段模型補入僅覆蓋測試記憶體的可寫描述子，以同一命令乾淨重跑；首輪收據 `workplace/platform-test-275-first-failure.txt` SHA-256 `571785b82c6b47cd7761709a2124763500625be2f17baeae8bb6afd44226d233` 保留。

## 實作與平台測試

READY 後只在既有 MOO2 保護模式服務層接受低 AX=1684h／BX=5／ES:DI=0:0，完整狀態保持；不增加呼叫計數、裝置入口或虛擬時間，既有 160Ah 查詢保持。FD2 設定、其他裝置／功能／非零 ES 或 DI 均拒絕且保持狀態。

高半部四組×全部六算術旗標 64 組×重複呼叫、完整 R／段／EIP／EFLAGS／記憶體／計數保持，未知形狀／其他設定拒絕及原版 INT→兩筆 word 保存最小樣本通過；相鄰記憶體的非零哨兵保持。最小樣本有明示 DS 描述子與重定位，不能取代自然啟動。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/machine -run 'TestMOO2(AbsentVTDEntry|WindowsVersionAbsent)' -count=1` 同命令乾淨重跑通過，`workplace/platform-test-275.txt` SHA-256 `5b8e964cc09f0722c9a01cad0529acb7b11ff9982cce4c24402892467602b94a`。回填 274→275 的固定 EXE＋高位 LE／DOSBox-X CS:EIP＋CD 2F＋0005h 定位、缺定位／缺舊標記必拒絕與全部既有回填函式通過。待全套及兩條自然路徑後才限定 CONFORMED。

## 固定原版全套及自然收據

同一 Go 映像、417 根層原檔及官方 1.31 EXE，`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-275.txt` SHA-256 `e47cea1afdb99120cff4887a484aa2138901fc60cb98b6126443168893bcba03`。

`DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=<本機輸出> go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，事件一路另加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`，兩條均由 dosgolem 自行重生。查詢前／後的完整只讀觀測：
- 無事件第 20,100,563／20,100,564 步、事件第 20,100,598／20,100,599 步；高位 LE `0x239A47 → 0x239A49`。
- 兩次 R 均為 EAX=F1684h、ECX=239F20h、EDX=1、EBX=5、ESP=2BDAF0h、EBP=0、ESI=2BDB40h、EDI=260000h；CS=8、DS／SS=188h、ES=0、FS=0、GS=20h、flags=246h，完整狀態保持。
- 此與 DOSBox-X 的 EDI=380000h／資料布局不同，只確認可比較的空 ES:DI 與狀態保持，不宣稱完整同狀態／時鐘／布局。

兩條較原查詢停點前進 835 指令，無事件第 20,101,398 步、事件第 20,101,433 步，停止於 **高位 LE 線性** `0x239AE8` 的 `E6 43 EB 00 88 D8 E6 40 EB 00 88 F8 E6 40 A1 48`，error=OUT port 43 未處理，拒絕後 EIP=239AEAh。EAX=1734h、EBX=174Eh、ECX=0、EDX=174Eh、DS／ES／SS=188h、flags=246h、DOS 呼叫 6，事件回呼 started=1／completed=1。自然路徑已越過原 caller，但沒有額外逐位元擷取其 record，不能用步進成功代替完整 consumer 的同狀態驗收。

gzip SHA-256 `53e73b6fd668062d43eaa21292f1ea4b248c565ad405d4dc4b79fdbb41c526f4`／`0a116f56852f469c095ac08c7ca9ba78a77cb5de8bbf3421c55fe8f66631c554`。VBE Active=true、Bank=2、StartY=0、BankSets=441、Writes=4046164、DisplaySets=6，索引 SHA-256 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`，兩 PNG 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`；與先前已檢視黑圖逐位元相同，不重做目視驗收。

275 限定為未安裝 VTD 的平台查詢 CONFORMED；255 仍 READY。主選單、正常玩家輸入、音效、受控亂數、Go remake 玩法同狀態仍未完成，不能因空入口返回而宣稱 Windows／VTD／硬體時間已建模。下一步依公開硬體契約只核對 43h／34h 呼叫及既有共享平台埠的缺口；不逆向 timer driver、ISR 或 busy-wait，不追逐週期硬體時鐘。

## 後續平台接通

PIT 模式 2 停點已由規格 276 接通，見 [276-pit0-mode2-shared-clock.md](276-pit0-mode2-shared-clock.md)。保留本次 OUT 43h 的原始停點與收據；正式後續停點以 276 重生收據為準，不把平台設定提升為玩法或硬體時鐘對拍。

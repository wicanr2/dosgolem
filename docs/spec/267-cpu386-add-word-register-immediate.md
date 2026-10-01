# 267 — 16 位元暫存器的帶符號立即值加法

狀態：**CONFORMED（有限 CPU 與首個消費範圍）**
日期：2026-10-01
範圍：32-bit CPU 的 `66 83 /0 ib`，目的為暫存器；不修改 remake 玩法。

## 證據

- **已證實，自生停點**：dosgolem `6da3a10b07d8b272eb55baa52ef8b52d3704ae03`、Go 1.24.13，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP 根層 417 檔與固定 MOX.SET。規格 [266-moo2-vbe-display-start.md](266-moo2-vbe-display-start.md) 的兩條自然路徑越過 Y=512，在無事件第 6,738,873 步、受控事件第 6,738,908 步於 **dosgolem 高位 LE 線性** `0x234B10` 的 `66 83 C3 18 81 FB E0 01 00 00 7C 13 33 DB 66 BB` 拒絕；EAX=0、EBX=`F0h`、ECX=EDX=0、DS／ES／SS=`188h`、EFLAGS=`246h`。完整輸入與診斷雜湊見 266。
- **公開 CPU 契約**：[Intel 80386 ADD](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/ADD.htm)：`83 /0` 的 byte 立即值帶符號延伸至操作寬度，再加到目的值；更新 CF／PF／AF／ZF／SF／OF。此處 `66` 改為 word，低 16 位寫回，高半部保存。
- **現有實作**：`cpu386.add16` 已由其他 word ADD 使用；`83` 的 word 暫存器 SUB／CMP／OR／AND 已接受同類無 segment／REP 前綴的暫存器形狀。ADD 目前缺解碼分支，不另寫一套算術旗標。
- **已證實，同次輔助結果**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --add-word-register` 命中 **CS:EIP** `0180:00368B10 → 0180:00368B14`，16 bytes 與上述 dosgolem 窗口完全相同。EAX=ECX=EDX=ESI=0、EBX=`F0h`、EDI=`3A2090h`、EBP=`3EBBA8h`、ESP=`3EBB74h`、DS／ES／SS=`188h`、FS=0、GS=`20h`、EFLAGS=`246h`；返回 EBX=`108h`、EFLAGS=`202h`，其他擷取欄位保持。下一 CMP／JL 比較完整 EBX 與 480，未追 helper 用途或內部。
- 私有 JSON／第一個消費端 LOG／完整終端 SHA-256 `6cd17791b1d45eb584a30d10705cc56bbd0da7b0834f54036912cf5becf2ae70`／`435c7e65e91ebfa7ce1c5f103aaf9bf6f424c669f545f6e582f842b8e461fe82`／`02aae464be90675d5a66390683a120dade776c3c8e09a9e138d9c342fb0b4d07`。最小 bytes／架構樣本可留測試，完整原版輸入、記憶體與終端留本機。

## 擬議契約

`operand16=true`、opcode 83、group=0、ModRM `mod=3`、無 segment／REP 前綴時，接受八種 word 暫存器。先完整讀取 imm8，再將 `int8` 帶符號延伸至 word；結果以 16 位環繞寫入目的低半部，目的高半部、其他暫存器、段、記憶體與非算術旗標保持。EIP 前進四個 bytes，旗標由既有 `add16` 生成。

截短、未列前綴／記憶體形狀保留明確 CPU Error，資料與旗標不發布半次結果；不要求錯誤時回復已取指的 EIP。既有無前綴 dword ADD 與 word SUB／CMP／OR／AND 保持各自規格，不放寬未知 group 或加入未審查的其他 word 指令。

## 驗收

原版輔助樣本核對當次 low BX=`00F0h + 18h`、完整 EBX、所有擷取暫存器／段／旗標與下一 CMP／JL。CPU 測試覆蓋八目的、正／負立即值、0／127／-128／-1、進位／輔助進位／正負溢位／零／符號／低 byte 同位、目的高位與其他狀態、未知／截短拒絕及既有操作回歸。算術預期以有號範圍、無號進位與 半位元組（nibble）進位獨立計算，不只重複實作公式。

加入實際最小架構樣本後，固定原檔全套測試與兩條自然重跑，再記錄下一結果及有效頁快照；先前黑圖不能當作正常玩家畫面。規格 266 回填，索引／正負護欄／擁有權與 Docker 清理一起核對。CPU 切片不代表正常 UI、音效、受控亂數或 Go remake 玩法同狀態，255 維持獨立閘門；不追 runtime／圖形 helper 內部。

## READY 審查

Intel 已定義立即值帶符號延伸、word 環繞、算術旗標與高半部保持；固定原檔形狀及同次輔助結果已證明這個解碼缺口。既有 add16 與其他 word 暫存器分支可直接沿用，只在完整立即值取得後寫回。八目的、全部 imm8 與旗標／別名／拒絕／消費端驗收已列明；不放寬 segment／REP／記憶體形式。DRAFT 審查後轉 READY，再實作。

## 實作與驗收

`internal/cpu386/cpu.go` 在 READY 後加入 word `83 /0` 暫存器分支，沿用 `add16`，完整讀立即值後才寫目的低半部。`add_word_register_immediate_test.go` 覆蓋八目的、16 個邊界值與全部 256 個 imm8，以 無號進位、有號範圍與 半位元組（nibble）進位獨立計算旗標；另驗原始碼／資料哨兵、高半部、段與非算術旗標保持、截短／前綴／記憶體／未知 group 拒絕、既有 dword ADD 與 word SUB／CMP／OR／AND、原版架構樣本及第一個 CMP／JL。原版樣本 ADD EFLAGS=`202h`，CMP EFLAGS=`287h`；同一架構樣本的 dosgolem JL 測試選至相對窗口 `+1Fh`（對應 DOSBox-X 候選 `0180:00368B2F`，原版當次 LOG 2 尚未執行 JL）。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，私有 CPU 測試輸出 SHA-256 `ed4bf0766d93be14ad0b08dba6abe6d95902783cba3534ef0a51b287bf2d4623`。加入原版樣本後，`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-267.txt` SHA-256 `251f0d83baf901593ff0ec2ec8e9cbe75b8c143df7cb19cc7afdf849fcac26a9`。

**已證實，兩條自生下一停點**：同一固定輸入／分離 DOS arena，`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。無事件第 6,738,883 步、設定後事件第 6,738,918 步，自行越過 ADD／CMP／JL，在 **dosgolem 高位 LE 線性** `0x234B43` 的 `66 F7 EB A3 7A 0D 27 00 33 DB 33 C9 33 C0 33 D2` 拒絕。EAX=1、EBX=5、ECX=EDX=0、DS／ES／SS=`188h`、EFLAGS=`246h`、DOS 呼叫 6；拒絕後 EIP=`0x234B46`，受控回呼 started=1／completed=1。F7 `/5` 的 word 形式由 [Intel IMUL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IMUL.htm) 定義，下一步只核對有號乘積、CF／OF、未定義旗標策略與固定使用點，不追 helper 內部。

兩份 gzip SHA-256 `ea179f8ff25efa3f3ed64dd6f42ffda01194fa01120bd4d6c920caafcf663c9f`／`99c0094f44fb0b53cab736fa1e699b1be629f65a94874e1095061676748434df`。當次 Active=true、Bank=9、StartY=512、Writes=307200；有效頁索引 SHA-256 仍 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`，RGB 仍 `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`。兩條命令另設 `DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-267-full-game.png`／`moo2-vbe-267-mouse-event.png`；PNG SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，與上輪已檢視的黑圖逐位元相同，不重做無必要的目視檢查，也不能升格為正常玩家畫面。

解析回填不可變鍵：固定 EXE 雜湊＋DOSBox-X `0180:00368B10`／dosgolem 高位 LE `0x234B10`＋`66 83 C3 18`。266 必須保留「word ADD 停點已由規格 267 接通」及本檔連結，`--check-add-word-register-spec-backlinks` 自動核對；缺定位或舊標記必拒絕。Python 語法、Go 格式、索引、全部既有回填函式與正負護欄通過。

267 僅在 word 暫存器帶符號立即值加法、定義旗標與第一個消費端 CONFORMED；其他平台、SAR 的 AF、DIV 的 ZF／除法例外、DTA 保留區差異仍保留。255 仍 READY，正常玩家畫面、音效、受控亂數及 Go remake 玩法同狀態未完成。


## 後續停點解析回填

**word IMUL 停點已由規格 268 接通**：[268-cpu386-imul-word-register.md](268-cpu386-imul-word-register.md) 已依公開 CPU 契約與同次輔助樣本審查 READY。保留本檔原始停點與收據，不重寫歷史；268 自然重跑結果為下一個現況入口。限定有號 word 暫存器乘法，不表示其他 F7 形狀、未定義旗標或正常玩家路徑已完成。


## 分支證據範圍勘誤

270 的 LOG 1／LOG 2 實驗確認，LOG 2 直接保存第一個指令執行後、第二個指令執行前的狀態。本檔原版 LOG 的 CMP 已執行、JL 尚未執行；先前把 `0180:00368B2F` 寫成原版實際分支結果超出了收據。該 target 是 Intel 條件與相同架構的 dosgolem 測試所支持，原版動態分支後定位未擷取；已修正現行敘述，不重開已驗 CPU ADD／旗標與第一個 CMP。保留原 JSON／LOG 雜湊與原始位址供回查。

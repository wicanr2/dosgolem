# 284 — byte 記憶體目的與 imm8 的 SUB

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 無前綴的 80／5、32 位元有效地址、記憶體 byte 目的；沿既有平坦段／descriptor／bus 與八位減法旗標模型，不建立完整權限／精確硬體 exception。

## 原始停點與公開 CPU 契約

工具基線 c58709c5ffc8841a22112ad1ea8016890f87d9e5，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、417 原檔與 MOX.SET、Go 1.24.13／映像沿 [283-cpu386-in-al-imm8.md](283-cpu386-in-al-imm8.md)。兩個自然排程第 20,637,037 步停 **dosgolem 高位 LE 線性** 0x254249，bytes 80 2D C0 26 27 00 08 C1 ED 08 8A C3 EB 26 8A 0D；ModRM 2D 為 /5、mod=00、r/m=101，DS:offset 002726C0 byte 減 08h。這是原始 CPU 運算元，不推定遊戲欄位用途，不分析所在 helper／driver／ISR。只讀自然初值與實作後態見下節，不以測試任選值冒稱原版。

[原廠 Intel 80386 SUB 契約鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SUB.htm)：80 /5 ib 為 r/m8−imm8，結果寫目的，更新 CF／PF／AF／ZF／SF／OF；非算術旗標保持。讀寫地址須有效且目的段可寫，例外不產生成功結果。既有 SUB byte 暫存器與 CMP byte 已用 sub8，本項補記憶體消費端，不改既有形狀。

## 候選型別、轉移與拒絕邊界

- typed input 為 mod≠11 的 ModRM /5、既有 decodeAddress32 的段／offset、imm8、可讀寫 byte 及初始旗標。支援既有 32 位 ModRM／SIB／disp8／disp32、DS 與 EBP／ESP 預設 SS、32 位有效地址繞回；不依固定遊戲地址特判。
- 只接無 66／67／segment／REP／LOCK 的 80 /5 記憶體。其他 group、既有 SUB byte 暫存器及其他指令不因此放寬；沿現有工具模型，不新增完整 VM／DPL／頁表或精確 #GP／#SS。
- 全部位址／立即值解碼成功，來源 byte 可讀後，計算八位結果並嘗試寫回同一目的 byte；只在寫入成功後更新六算術旗標，所有暫存器／段／FPU／其他記憶體與非算術旗標保持。讀取沿既有 SegmentRead8／descriptor／bus 優先序，寫入沿既有可寫 descriptor＋byte bus。
- 缺位址／立即值、未知／越界／不可寫段、bus 讀寫拒絕、前綴或未知形狀失敗即關閉，不部分更新六旗標。解碼 EIP 可前進；bus／外部回呼已發生的副作用不倒帶，沿既有 CPU 契約，不宣稱任意客製 bus 的交易回滾。
- 垂直鏈為原版 bytes／原始 DS byte → typed CPU 解碼 → 減法與可寫提交 → 原版黑箱自然繼續；不改 Go remake 玩法／UI／存檔或替該欄位命名。

## 驗收與停止線

只讀擷取原版停點運算元／完整 CPU 狀態及公開契約審查後 READY，才實作。測全部 256×256 byte 算術／獨立六旗標、ModRM／SIB／DS／SS 分離、鄰 byte 保持／非算術旗標與 FPU、立即值／來源／目的失敗、唯讀／未知段／前綴拒絕及既有暫存器 SUB。全部 CPU／固定 EXE 全套與 dosgolem 兩個自然排程自行重生後才限定 CONFORMED。

只保存下一標準平台停點原始地址空間／bytes，不推定遊戲資料語意、不翻譯整段函式或重開 timer driver／ISR／busy-wait。正常玩家路徑／主選單、255 完整座標／游標、音效／受控亂數及整款 remake 另行驗收；單元測試不代替跨原版完整同狀態。


## 只讀原版初態與 READY 審查

已證實：同固定官方 EXE／417 原檔／MOX.SET，未修改 CPU 的自然命令 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game，外層 240 秒、既有 Go 映像／2 GiB／2 CPU／UID/GID 1000:1000／network none。第 20,637,037 步 **高位 LE 線性** 0x254249，DS:002726C0 可讀 byte=16h、imm8=08h、flags=212h；R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：80000000 6BBC7C 272610 31488 2BDA8C 31488 6BCC64 6BBC80，段 CS DS ES FS GS SS：8 188 188 0 20 188。原始 opcode／ModRM／disp／imm 與 283 收據一致，不是任選測試初值。

本機 workplace/moo2-probe-284-input.txt.gz SHA-256 5ceb69559923fee3fda23986f3ec082bae6df1ddb7c9e31de6a024b44976d925；只讀有限探針納入版控，不注入 byte 或追 helper 內部。公開 SUB 推導結果為 0Eh，六旗標中僅 AF=1，其他非算術位保持，故此初態結果 flags 仍212h；這是 CPU 規格預期，實作前不當作原版後態已實測。

原始輸入已具備，與公開 byte／旗標、既有 DS／SS 解碼及可寫 byte 提交契約審查一致，284 轉 READY。寫入成功前不改六旗標，CPU 全套／原版自然後態與下一停點仍待實作驗證；不分析該 byte 的遊戲語意或其所在函式。


## 受限實作與原版自然後態

限定 CONFORMED：裸 80 /5 記憶體 byte，沿既有 32 位 ModRM／SIB 與 DS／SS 解碼。byte 寫回成功後才發布六算術旗標；不放寬前綴、其他 group 或未知段，不推定原版欄位用途。全部 256×256×兩種初始旗標、全部 ModRM／SIB 消費形狀、DS／SS 分離、32 位地址繞回、鄰 byte／FPU 保持及失敗拒絕通過。前綴負例改用所有段均可寫的樣本，避免未知段掩蓋錯誤接受。既有暫存器 SUB 保持。

Go 1.24.13／映像沿 283，GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過；moo2-284-cpu-tests.txt SHA-256 3e507d508afaf2049fa06cff5e2eb88019155f99dfaddc79a78ab54371877d5e。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-284.txt SHA-256 92fc976dd96d5d1f659b4b26e7d620e7740f6d3abb0c41f0980c17488a077fe1。

沿 283 自然命令、50M／分離 DOS／417 原檔／固定 MOX.SET 及有無受控滑鼠事件兩條件，600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。兩條第 20,637,037 步高位 LE 0x254249 的 DS:002726C0 byte=16h，下一步 0x254250 byte=0Eh；完整 R／段保持，flags=212h。第二次自然 SUB 第 20,637,061→20,637,062 步 byte=0Dh→05h、flags=202h→206h，完整 R／段保持。沒有注入目的 byte、計時或亂數。這是本工具原版自行消費的有限 CPU 收據，不是跨執行器完整同狀態。

兩條第 20,637,097 步轉停高位 LE 0x25425F，bytes 80 05 C0 26 27 00 18 8A C3 8B 1E FE C9 8B EB D3；ModRM 05 為 byte 記憶體 ADD／imm8 的標準 CPU 缺件，decode EIP=254261h。EAX=8000008Ah、EBX=0、ECX=6BBC03h、EDX=10h、DS／ES／SS=188h、flags=297h。下一步僅按公開 ADD 契約與只讀原始 byte 建窄規格，不追所在函式、runtime、timer driver／ISR／busy-wait。

IRQ0 started=completed=1595、active=false／failed=false，等待來源 DS:00271148=01 00 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false、Micros=21094685、Deliveries=1615、Pending=false／InService=false。事件條件 requested=true／injected=true，第 1,612,067 步 x=657／y=189，仍不是完整座標／游標或玩家操作驗收。

自然 gzip SHA-256 01efe7de89d0f2968720aa52ad742093d3fc5864624b19d782b326cc91703b06／ceb730b6fb5002e0e32f02a868e6363f05a20148834399cf2582cf402783486f。兩 PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，indexed 7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf、RGB 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366，同已檢視黑圖。原版素材與完整終端／記憶體／gzip／PNG 留本機；主選單、正常玩家路徑、音效／受控亂數及整款 remake 未完成，255 READY。

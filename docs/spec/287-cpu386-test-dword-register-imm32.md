# 287 — dword 暫存器與 imm32 的 TEST

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 F7 /0、mod=11 的八個 32 位元暫存器與立即遮罩；沿既有邏輯旗標模型。

## 原始定位與公開 CPU 契約

工具基線 30faf6eb5643aac18d197c64804f81c18293876b，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417 原檔／MOX.SET、Go 映像與自然命令沿 [286-cpu386-xor-byte-register-imm8.md](286-cpu386-xor-byte-register-imm8.md)。兩個原版自然排程第 20,651,437 步停 **dosgolem 高位 LE 線性** 0x254499，bytes F7 C1 00 00 00 80 75 2A FE 0D C0 26 27 00 75 0C。C1 是 /0、mod=11、r/m=001；ECX=400h 與 imm32=80000000h 的 TEST 缺件，flags=213h；完整 R／段與自然後態見下節。下一 JNZ 75 2A 位於 0x25449F，非跳轉續點為 0x2544A1、目標 0x2544CB。

[原廠 Intel 80386 TEST](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/TEST.htm) 定義 F7 /0 id 對兩個 dword 做逐 bit AND，結果不寫回，只更新旗標。CF／OF 清零，ZF／SF／PF 依完整結果與低 byte。[Intel 旗標附錄](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm) 將 AF 列為未定義；沿既有 setLogicFlags 的清 AF 工具模型，不作硬體未定義值對齊聲明。

## 候選轉移與拒絕邊界

- 裸 F7 /0 的 mod=11，以 r/m 讀八個 R 之一，完整取得 little-endian imm32 後才發布定義五旗標；全部 R／段／FPU／記憶體與非算術旗標保持，不寫回 AND 結果。
- 全部截短立即值／ModRM 與 67／segment／REP／LOCK 前綴拒絕，不發布旗標，解碼 EIP 可前進。66 F7 word TEST 是既有合法路徑，保持其低 word 與 imm16 寬度；不把它錯列為未知前綴。
- 既有 F7 記憶體 TEST、word TEST、MUL／IMUL／DIV／IDIV／NOT／NEG 與其他拒絕形狀保持；不新增權限／頁表或精確硬體 exception。
- 垂直鏈為原版 bytes／完整 ECX → typed 遮罩 → 定義旗標 → 下一 JNZ 自然消費；不猜所在函式語意，不改主庫玩法。

## 審查、驗收與停止線

唯讀完整初態與公開 CPU／AF 近似審查 READY 後才實作。八暫存器×32 位邊界／全部 bit／全部低 byte 遮罩組合／兩種初始旗標，以逐 bit AND、dword 符號／零與低 byte 同位獨立驗旗標；AF 模型另驗。全部狀態保持、截短／前綴／未知 group 拒絕、既有 word／記憶體／其他 group 回歸與兩個 JNZ 方向。全部 CPU、固定 EXE 全套與兩自然排程 TEST 後態／第一分支後才限定 CONFORMED。

只保存下一標準平台停點原始定位／地址空間／bytes，不解 helper／runtime／timer driver／ISR／busy-wait。255 完整座標／游標、主選單／正常玩家路徑、音效／受控亂數與整款 remake 另行驗收。


## 唯讀初態與 READY 審查

已證實：未改 TEST CPU 的自然命令沿 286，50M／分離 DOS／固定 EXE／417 原檔／MOX.SET。第 20,651,437 步高位 LE 0x254499，完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：80000929 400 6BBC60 3FF09 2BDA88 0 6BCE48 609070；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=213h。有限 dword_test_state 探針只讀，不注入資料／旗標；本機 moo2-probe-287-input.txt.gz SHA-256 e0437c65e125aad6f927ce4ee59e129fb1e75907188d6f8d041d1b888bb9d604。

原始 ECX／遮罩、公開 TEST 與既有 AF 清除模型審查足夠，預期完整交集為零、定義旗標 ZF=1／PF=1／SF=0／CF=0／OF=0、模型 flags=246h；下一 JNZ 75 2A 不跳轉到 0x2544CB，正常續點為 0x2544A1。AF 原本1、工具將清除，但不把這個預期當硬體未定義值。287 轉 READY 才補暫存器形式；CPU／固定 EXE／兩自然後態及第一分支仍待驗證。


## 受限實作、回歸與自然分支

限定 CONFORMED：裸 F7 /0 的八個 dword 暫存器，只完整取得 imm32 後發布定義五旗標，AND 結果不寫回。AF 清除沿既有工具近似；非算術旗標、全部 R／段／FPU／記憶體保持。前綴與截短拒絕，word／記憶體 TEST 及其他 group 保持。

Go 1.24.13／既有映像，GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過；八暫存器、所有單 bit／補集／符號／高 word 邊界、全部低 byte 遮罩配對與兩初始旗標，逐 bit AND／低 byte 同位獨立驗定義旗標，AF 模型另驗。立即值全部截短、10 種未知前綴與未知 group 拒絕、word／記憶體／NOT 及 JNZ 兩方向保持已驗；其他 F7 group 由 CPU 全套回歸。moo2-287-cpu-tests.txt SHA-256 e3dabc645fc4da8b16e174461054dfd82f1d0610a21b21ee1cca33a15c5aa4d0。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-287.txt SHA-256 e517ff40941fbd770a536caf2348751aa874f42cbb806c8a48935a3a9bb34b52。

沿既有 50M／分離 DOS／417 原檔／固定 MOX.SET 兩自然命令，另一條件加 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1，600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。第 20,651,437 步高位 LE 0x254499 TEST 初態完整 R／段同唯讀樣本、flags=213h；第 20,651,438 步 0x25449F flags=246h、全部 R／段保持；第 20,651,439 步第一 JNZ 不跳，抵達 0x2544A1，全部 R／段與 flags=246h 保持。後來第 20,651,512 步自然抵達 0x2544CB、ECX=80000002h／flags=286h；這是不同時間的後續狀態，不當作第一 JNZ 跳轉或完整獨立前後樣本。AF 213h 的1轉0僅屬工具近似，不冒稱原版硬體未定義值。

兩條第 20,651,583 步轉停高位 LE 0x254510，bytes C1 C0 08 66 39 05 E0 27 27 00 75 07 33 C0 89 7A；C1 /0 的 dword EAX 左循環移位／imm8 缺件，EAX=02000000h、EBX=3FF09h、ECX=60906Ch、EDX=272610h、DS／ES／SS=188h、flags=286h。下一步只按公開 ROL 的計數／CF／OF 定義與未定義邊界建窄規格，不追圖形 helper／runtime／timer driver／ISR／busy-wait。

IRQ0 started=completed=1598、active=false／failed=false，等待來源 DS:00271148=04 00 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false、Micros=21109931、Deliveries=1618、Pending=false／InService=false。事件條件已注入，255 完整座標／游標仍 READY；主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。

自然 gzip SHA-256 b3341128e31fbed5f8fd3f191a7de03967a537cb870f50377dbf59d6caf6b004／c146fc6aae759c1e114fbcd237a9c460571f9e36cfb6072b7b45c95592ed8fa5。兩 PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，同已檢視黑圖。原版素材與完整終端／記憶體／gzip／PNG 留本機，未注入遊戲資料／計時／亂數。


## 後續停點回填

dword ROL 停點已由規格 288 接通：[288-cpu386-rol-dword-register-imm8.md](288-cpu386-rol-dword-register-imm8.md)。兩自然排程自行重生三筆資料／CF 與完整目的外狀態，未定義 OF 保留明列工具近似；下一 byte NEG 停點與原始定位集中於 288。這不改 287 第一 JNZ 不跳的既有證據，也不代表主選單或正常玩家路徑完成。

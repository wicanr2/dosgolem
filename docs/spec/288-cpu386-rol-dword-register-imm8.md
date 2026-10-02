# 288：dword 暫存器的立即數左循環移位

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 C1 /0、mod=11，八個 32 位元暫存器與全部 imm8 計數。

## 原始定位與 CPU 契約

工具基線 4d7ae6c2176feda4c61e47ee2db41695a25c2208。固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417 原檔、MOX.SET、映像與自然命令沿 [287-cpu386-test-dword-register-imm32.md](287-cpu386-test-dword-register-imm32.md)。兩自然排程第 20,651,583 步停 dosgolem 高位 LE 線性 0x254510，bytes C1 C0 08 66 39 05 E0 27 27 00 75 07 33 C0 89 7A。C0 為 /0、mod=11、EAX，EAX=02000000h、flags=286h；完整 R／段另以唯讀探針保存。

[原廠 Intel 80386 Rotate](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/RCL.htm) 定義 C1 /0 ib 的 dword ROL，計數只取低五位。最高 bit 回到最低 bit，不將舊 CF 輸入資料。[Intel SDM Volume 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 253667-060US 的 RCL／RCR／ROL／ROR，印刷頁 4-524 運算與 4-525 Flags Affected：遮罩後計數0不改資料與全部旗標；非零 CF 為結果最低 bit；計數1的 OF 為結果最高 bit XOR CF；計數大於1的 OF 未定義。SF／ZF／AF／PF 始終保持。

多位循環移位保留原 OF，沿既有 word ROL／dword ROR 工具近似，不宣稱原版硬體未定義值一致。其他 R／段／FPU／記憶體與非算術旗標保持，不呼叫邏輯旗標函式。

## 候選實作與驗收

- 完整取得 ModRM 與 imm8 後才發布目的及旗標。裸 C1 /0、mod=11，八暫存器共用同一 CPU 契約，不特判原版地址或 08h。
- 截短、67／segment／REP／LOCK 前綴與記憶體形式拒絕，不發布資料／旗標；解碼 EIP 可前進。66 C1 word ROL 保持既有合法子集，不錯列為未知前綴。
- 不擴張 D1／D3、記憶體 ROL、word 大計數或 dword ROR 的既有範圍。其他 shift／rotate 由 CPU 全套回歸。
- 全八目的、全部256計數、32位邊界／每 bit 及補集、CF／OF 初態，以逐次單 bit 算術循環的獨立 oracle 驗結果／CF／計數1 OF；未定義 OF 的保留模型另驗。完整外層保持、截短／拒絕與原始完整初態另驗。
- 全部 CPU、固定官方 EXE 全套、兩原版自然排程的完整初態與後態／下一消費點後才限定 CONFORMED。不以工具測試代替正常玩家路徑。

## 停止線

先保存完整唯讀初態與審查為 READY 才實作。只記下一標準平台停點原始定位／bytes／地址空間，不解圖形 helper、runtime、timer driver、ISR 或 busy-wait。255 完整座標／游標、主選單／正常玩家路徑、音效／受控亂數與整款 remake 仍另行驗收；主庫玩法 RE 閘門不變。


## 唯讀初態與 READY 審查

已證實：未改 ROL CPU 的 50M／分離 DOS／固定官方 EXE、417 原檔與 MOX.SET 自然命令沿 287；Go 1.24.13、golang:1.24-bookworm 映像 1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，240 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。第 20,651,583 步高位 LE 0x254510，完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：2000000 60906C 272610 3FF09 2BDA90 74723 6BCE4C 609070；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=286h。有限 dword_rol_state 探針只讀，不注入資料／旗標；moo2-probe-288-input.txt.gz SHA-256 c2934fd9fdeed05324d546e449171232d86ee88e0d7ebcaaac029d39c3852659。

原始 bytes／完整初態、公開計數／定義旗標與未定義 OF 近似審查足夠；288 轉 READY 後才補裸暫存器形式。預期下一步 0x254513 的完整 EAX=2h、CF=0，其他 R／段保持；flags=286h 僅含工具保留 OF 的模型。下一條 66 39 的 word 比較位於 0x254513，後續地址 0x25451A 的 flags 由比較決定，不混作 ROL 後態。CPU／固定 EXE／兩自然排程仍待驗。


## 受限實作與自然消費

限定 CONFORMED：裸 C1 /0 的八個 dword 暫存器、全部 imm8 遮罩計數，完整取得 bytes 才發布結果／CF／計數1 OF；零計數全部保持。多位 OF 保留只屬工具近似；SF／ZF／AF／PF、其他 R／段／FPU／記憶體保持。截短、未知前綴、記憶體及 D1／D3 /0 拒絕，既有 word ROL 與 dword ROR 保持。

GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過。八目的×256計數×32位邊界／每 bit 與補集×四 CF／OF 組合×全清／全設保持旗標，以逐步算術循環獨立驗資料與定義旗標；未定義 OF 模型另驗。完整狀態／原始完整 R、截短／10前綴／記憶體與其他 opcode 拒絕及既有 word／ROR 回歸通過。初次全套通過後審查補強保持旗標的全清初態，再以同容器／命令乾淨重跑通過。moo2-288-cpu-tests.txt SHA-256 1abfe7dcb15a6419bb53484c49815b8432aa3ba1e482d513c76d82202e198c22。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-288.txt SHA-256 de3e2f0f9e1a7af3ca5379ab9518b4dbecf3ac2533d81a22be1db185c4aab8db。

沿既有自然命令 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game；另一條件加 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1，並各以 DOSGOLEM_MOO2_VBE_PNG 保存截圖。600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none，固定官方 EXE／417 原檔／MOX.SET 未修改。兩自然排程自行重生三組 ROL：

| 步數，初態→後態 | EAX，初態→後態 | flags，初態→後態 |
| --- | --- | --- |
| 20,651,583→20,651,584 | 02000000h→2h | 286h→286h |
| 20,651,773→20,651,774 | 11000003h→311h | 286h→287h |
| 20,651,966→20,651,967 | D4000000h→D4h | 286h→286h |

每組高位 LE 0x254510→0x254513，完整目的外 R／段保持，CF 分別0／1／0。第一組完整 EAX=2h，其他 R／段同唯讀初態。三組下一 word CMP 自行執行並到 0x25451A，flags=293h；這是比較後態，不當作 ROL 旗標。沒有注入遊戲狀態／計時／亂數，不追所在 helper 的控制流。

兩條第 26,396,706 步轉停 dosgolem 高位 LE 線性 0x2545EF，bytes F6 D9 8A D1 89 54 AF FC 4D 75 A3 5D C3 87 DB 87；F6 /3、mod=11 的 byte CL NEG 缺件，ECX=FFFFFFF6h、EAX=230001h、EBX=3FF09h、EDX=458F000h、DS／ES／SS=188h、flags=297h。下一步只按公開 NEG 的 byte／六旗標與唯讀完整初態建窄規格，不解圖形 helper／runtime／driver／ISR／busy-wait。

IRQ0 started=completed=2810、active=false／failed=false，等待 DS:00271148=C0 04 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false，Micros=27167604／Deliveries=2830／Pending=false／InService=false。事件條件已注入；255 完整座標／游標仍 READY，主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。

自然 gzip SHA-256 c08ea3956f0eb327e05d87a4900ced02ba29f1b1f3fab853748ba090aaff9992／8d72e98ab94cd187ba69133e23d70f088e2fa5d76741a4973d18803086875c93。兩 PNG 同已檢視黑圖 SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，indexed SHA-256 7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf。原版素材與完整終端／記憶體／gzip／PNG 留本機，不進公開版控。

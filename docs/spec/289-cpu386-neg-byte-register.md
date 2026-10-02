# 289：byte 暫存器取負

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 F6 /3、mod=11，AL／CL／DL／BL／AH／CH／DH／BH。

## 原始定位與 CPU 契約

工具基線 c605a7f13e00c061d09f14bbf7583c201335f980，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417 原檔／MOX.SET、Go 映像與自然命令沿 [288-cpu386-rol-dword-register-imm8.md](288-cpu386-rol-dword-register-imm8.md)。兩自然排程第 26,396,706 步停 dosgolem 高位 LE 線性 0x2545EF，bytes F6 D9 8A D1 89 54 AF FC 4D 75 A3 5D C3 87 DB 87。D9 是 /3、mod=11、CL，ECX=FFFFFFF6h、flags=297h；完整 R／段另以唯讀探針保存。

[原廠 Intel 80386 NEG](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/NEG.htm) 定義 F6 /3 r/m8，結果為 byte 的 0-value。[Intel SDM Volume 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 253667-060US，NEG 印刷頁 4-161 的 Flags Affected 明列 CF／OF／SF／ZF／AF／PF。非零來源 CF=1，零 CF=0；80h 取負有號溢位 OF=1，其他 OF=0。AF 依 0 減低 nibble 的借位；ZF／SF 依結果，PF 依結果 byte 的偶同位。此形式六旗標均定義，不沿 ROL 的 OF 近似或邏輯操作的 AF 近似。

## 候選實作與驗收

- 裸 F6 /3、mod=11，完整取得 ModRM 後讀 byte、發布結果與六旗標。低／高 byte 以既有 reg8／setReg8 存取，不改同一完整暫存器的其他24位。
- 八目的／全部256來源／六算術旗標全部64初態，以較寬 256-value、低 nibble 借位、有號界限及逐次除2算同位的獨立 oracle 驗結果／旗標；完整目的外 R／段／FPU／記憶體與非算術旗標保持。
- 全部截短、11種前綴及記憶體／未知 group 拒絕，不發布資料／旗標，解碼 EIP 可前進。既有 F6 TEST／MUL／DIV、F7 dword／堆疊 NEG 保持，word NEG 與硬體 exception 不擴張。
- 垂直鏈為原版 bytes／完整 CL → byte 取負／六旗標 → 下一 MOV DL,CL 消費。全部 CPU、固定官方 EXE 全套、兩自然排程完整初態／後態／下一消費點後才限定 CONFORMED。

## 停止線

先保存完整唯讀初態，公開契約與拒絕邊界審查 READY 後才實作。只記下一平台停點原始定位／bytes／地址空間，不追 helper／runtime／driver／ISR／busy-wait 或猜欄位用途。255 完整座標／游標、主選單／正常玩家路徑、音效／受控亂數與整款 remake 仍另行驗收；主庫玩法 RE 閘門不變。


## 唯讀初態與 READY 審查

已證實：未改 NEG CPU 的自然命令沿 288，50M／分離 DOS／固定 EXE／417 原檔／MOX.SET。第 26,396,706 步高位 LE 0x2545EF，完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：230001 FFFFFFF6 458F000 3FF09 2BDAC0 800 6CD700 601BF0；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=297h。有限 byte_neg_state 探針只讀，不注入資料／旗標；moo2-probe-289-input.txt.gz SHA-256 80eaa0cf867706bd147a7a759fcb8372e9c4881d5641af16f0e128bf6072604e。

原始 bytes／完整初態、公開 byte NEG／六旗標與既有 F6 拒絕邊界審查足夠，289 轉 READY 後才實作。預期 0x2545F1 的完整 ECX=FFFFFF0Ah，CL=F6h→0Ah，flags=217h，其他 R／段保持；下一 MOV DL,CL 消費後 0x2545F3 的完整 EDX=458F00Ah、flags 仍217h。不推定 CL 的遊戲語意；CPU／固定 EXE／兩自然排程後態仍待驗。


## 受限實作與自然消費

限定 CONFORMED：裸 F6 /3 的八個 byte 暫存器，完整讀取來源後依 0-value 更新目的與六旗標；其餘24位、目的外 R／段／FPU／記憶體與非算術旗標保持。全部截短／11前綴／記憶體／未知 group 拒絕，F6 TEST／MUL／DIV 與 F7 dword／堆疊 NEG 保持。

GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過；八目的×256來源×64算術旗標初態、完整外層保持與獨立算術／同位 oracle、全部拒絕邊界、既有 TEST／DIV／dword／堆疊 NEG 及原始完整狀態／MOV 消費通過。首次全套遇舊 TestByteTESTRegister 的 F6 DA /3 DL 被當作未知 group，與新 READY 支援範圍衝突；改用仍未支援的 F6 D2 /2 DL byte NOT 保留拒絕護欄，再以同映像／命令乾淨重跑通過。首次失敗收據 moo2-289-cpu-tests-first-failure.txt SHA-256 a33fc805c9e93df095966f032ef1ccea935c29ddbef8bbc1e4aec527d73d7325；通過收據 moo2-289-cpu-tests.txt SHA-256 96e676993d762f55ba4f5f4524730510bd3470643dce79388243d28cec0848a9。

固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-289.txt SHA-256 e404aab29a5b4a0e1e0e9fdd2b7e32559942c55e839ed292e8de777f2c30882b。兩自然命令沿 288 的50M／分離 DOS／固定 EXE／417 原檔／MOX.SET，有無 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1；600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。

| 步數，初態→後態 | 完整 ECX，初態→後態 | flags，初態→後態 |
| --- | --- | --- |
| 26,396,706→26,396,707 | FFFFFFF6h→FFFFFF0Ah | 297h→217h |
| 26,396,849→26,396,850 | FFFFFFF5h→FFFFFF0Bh | 297h→213h |
| 26,396,922→26,396,923 | FFFFFFFBh→FFFFFF05h | 293h→217h |

每組高位 LE 0x2545EF→0x2545F1，完整 ECX=FFFFFF0Ah 為第一組；CL 外24位、完整目的外 R／段保持，六旗標符合公開契約。下一 MOV DL,CL 自行執行到 0x2545F3，三組完整 EDX=458F00Ah／329840Bh／3D5C405h，除了 DL 其餘完整 R／段與 NEG 旗標保持。未注入資料／遊戲時計／亂數，也不推定欄位用途或追所在 helper。

兩條第 38,427,368 步轉停 dosgolem 高位 LE 線性 0x254A04，bytes D2 E5 08 2C 17 83 C6 04 48 75 DA C3 81 C6 68 74；D2 /4、mod=11 的 SHL CH,CL 缺件，ECX=102h、EAX=F91Ch、EBX=7CC1EF00h、EDX=0、DS／ES／SS=188h、flags=202h。下一步只按公開 byte shift 的計數／定義旗標與未定義邊界建立窄 CPU 規格，先保存唯讀完整初態，不追圖形 helper／runtime／driver／ISR／busy-wait。

IRQ0 started=completed=5346、active=false／failed=false，等待 DS:00271148=A8 0E 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false，Micros=39852240／Deliveries=5366／Pending=false／InService=false。事件條件已注入；255 完整座標／游標仍 READY，主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。

自然 gzip SHA-256 495b237c2a7af2f16deeb12bf859a0a07ea36a3b3c0248c4c0d56bb9ac49406d／a444e4bbb0e97291d82b6376d26d4d9f2484b59eee3d52481c18398136e2a9dd。兩 PNG 同已檢視黑圖 SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。原版素材與完整終端／記憶體／gzip／PNG 留本機，不進公開版控。

# 292：REPE 掃描 dword 與 EAX 比較

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 的32位 F3 AF；裸 AF、F2 AF、66／67／段覆寫及其他前綴不擴張。

## 原始定位與公開 CPU 契約

工具基線 0c6d53b869cb52fd716d95c868326244614abbf1。固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；輸入與兩自然排程命令見 [291-cpu386-or-byte-memory-register.md](291-cpu386-or-byte-memory-register.md)。兩條第 39,983,174 步停 dosgolem 高位 LE 線性 0x25488F，bytes F3 AF 83 EF 04 8B 07 2B 3D 00 2C 27 00 83 F0 FF；EAX=FFFFFFFFh、ECX=800h、ES=188h、flags=246h。完整 R／段及 ES dword 尚待唯讀觀測，不推定掃描用途。

[原廠 Intel 80386 SCAS](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SCAS.htm) 定義 EAX 減 ES:[EDI]，只發布六算術旗標，依 DF 將 EDI 加減4。[原廠 REP 說明](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/REP.htm) 的文字說明於比較後檢查 ZF，其偽碼退出條件卻相反；以 [Intel SDM 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 253667-060US、2016年9月印刷頁4-549至4-551與4-592至4-594核對：REPE 在 ECX 耗盡或比較後 ZF=0 停止，初始 ZF 不限制第一次比較。

## 候選實作與驗收

- 僅 F3 AF：ECX=0 不讀資料、不改 R／旗標；非零讀 ES:[EDI] 的完整 little-endian dword，沿既有 sub32 設 CF／PF／AF／ZF／SF／OF，EAX與記憶體保持，EDI 依 DF 加減4並32位繞回，ECX 減1；不相等或計數歸零退出。
- 六算術旗標全部定義，無未定義旗標模型。其餘 R／段／FPU／記憶體／非算術旗標保持。獨立有號寬值減法／低 nibble 借位／同位 oracle 抽測邊界與不同初始旗標。
- 初始 ZF=0／1、零計數／一筆／先等後不等／全部相等與 DF 正反／ES與DS分離／非對齊／段末完整 dword／32位地址繞回、未知selector／段越界／bus讀失敗及前綴／截短拒絕均驗。
- 原版資料 → 完整 R／段／ES 掃描 → 比較結果／計數／EDI → 0x254891 的 SUB EDI,4 → 0x254894 的 MOV EAX,[EDI] → 0x254896 下一入口。CPU 全套、固定 EXE 全套與兩自然排程驗完才限定 CONFORMED。

## 模型界限與停止線

沿既有字串操作的單次 Step 模型，不插入每次比較的硬體 IRQ 或逐週期時計。讀取拒絕回傳工具 Error；成功完成的 EDI／ECX 保持，旗標回復指令前狀態，解碼 EIP 可前進，沒有硬體 exception／restart 交付聲明。先審查 READY 才接 CPU；不解 runtime／圖形 helper／driver／ISR／busy-wait 或猜用途。主庫玩法 RE 閘門不變，255／主選單／正常玩家路徑、音效／受控亂數與整款 remake 另驗。

## 唯讀初態與 READY 審查

已證實：CPU.go SHA-256 29722c18be61bf7d12ba1df846f1b2ffe9cd19fe772a8439a16bf718b9528ff7 未修改；有限 probe SHA-256 8d572940f2a90c3d25c7b816b638907aca468d0e34c456471bace7f3b464dbdd，最多三次入口／2048 dword，只讀不注入。第39,983,174步完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：FFFFFFFF 800 0 7CC1EF00 2BDACC 0 6BBC50 6BBC60；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=246h／DF=0。ES:006BBC60 起8192 bytes全可讀，前58個dword=FFFFFFFFh，第59個 ES:006BBD48=FFFFFBFFh。掃描資料 SHA-256 537f81277c17ca5cf4690e5283c951f23916016cfbc7f366b12613877dc5abe0；初態 gzip SHA-256 771d9f4279c812b2220c45a4856b5b137af9e7ed9dd4cac8fec985bd9796529b。

初態命令沿291的新鮮417原檔／官方EXE，在240秒／2GiB／2CPU／UID:GID=1000:1000／network none 的 golang:1.24-bookworm 執行 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game；只加診斷，保存 workplace/moo2-probe-292-input.txt.gz。Go1.24.13，映像 sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac。

公開契約、完整初態與掃描資料審查足夠，292轉 READY 才實作。規格推導的預期是比較59次，完整 ECX=7C5h、EDI=6BBD4Ch、flags=206h，其餘R／段與資料保持；下一 SUB EDI,4 回6BBD48h，MOV EAX,[EDI] 應讀 FFFFFBFFh。這些後態尚待自然重跑，不把推導當已驗結果。零計數、比較後退出、六旗標、32位繞回與拒絕契約照前述測試。失敗恢復指令前 EFLAGS、保留已成功元素的 ECX／EDI，符合 SDM 的掃描暫存器進度與旗標恢復；Error／解碼 EIP、單次 Step／沒有內部IRQ仍只屬工具模型，不宣稱硬體 exception 重啟。

## 受限實作、回歸與原版自然消費

限定 CONFORMED：32位 F3 AF，ECX=0 不讀資料／不改 R 或旗標，非零比較完整 ES dword 與 EAX，六算術旗標全部定義；成功比較才更新 EDI／ECX，按比較後 ZF 或計數終止。裸 AF／F2 AF／operand16／段覆寫、其他前綴仍拒絕。沒有遊戲位址特例。CPU.go SHA-256 a137e3c3878b8589328a9fa8d78c9a7a95c7f4d787eb4304b1d418aa84c64c61。

GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過。19×19個32位邊界配對／64算術旗標／兩方向、零計數／1／2／17／2048／65536個元素的不同退出位置與初始 ZF、ES與DS分離、唯讀／非對齊／段末完整 dword／EDI32位繞回、逐 byte 拒絕及失敗旗標恢復／進度保持、前綴／截短與既有 SCASB 已驗。完整 R／段／FPU／記憶體保持及原版初態的自製 SCASD→SUB→MOV 規則測試通過。CPU收據 workplace/moo2-292-cpu-tests.txt SHA-256 9b8c4a93e7acb119fb45469cfe71faf486f6b87e68653b2837b73aa69412f1da；固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-292.txt SHA-256 c5e4313060d6a556a5aaadf314221247c57d5f30a6b84e42f0fc3de891fa026f。兩者首次通過。

兩自然命令沿291的50M／分離 DOS／新鮮固定EXE／417檔／MOX.SET，有無 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1；600秒／2GiB／2CPU／UID:GID=1000:1000／network none。第39,983,174步從完整唯讀初態比較59次，後續原版自行執行：

| 步數／高位 LE 線性位址 | 完整 ECX／EDI | EAX／旗標／ES 讀取 |
| --- | --- | --- |
| 39,983,175／0x254891 | 7C5h／6BBD4Ch | FFFFFFFFh／206h，下一 dword=FFFFFFFFh |
| 39,983,176／0x254894 | 7C5h／6BBD48h | FFFFFFFFh／206h，SUB EDI,4 後讀 ES:006BBD48=FFFFFBFFh |
| 39,983,177／0x254896 | 7C5h／6BBD48h | FFFFFBFFh／206h，MOV EAX,[EDI] 真實消費 |

所有其他 R／段保持初態，逐連續步數核對完整狀態。從未改 CPU 的原始8192 bytes獨立找第59個不相等元素，再算六旗標／ECX／EDI，比對兩條收據。未注入資料／時計／亂數或推定掃描用途。第39,983,178步轉停 0x25489C，bytes 83 F0 FF C1 E7 03 0F BC D0 03 FA 66 89 3D D0 26；83 /6 暫存器目的的 XOR imm8 缺件，EAX=FFFFFBFFh、ECX=7C5h、flags=206h。下一步建立公開 XOR／符號延伸與未定義 AF 邊界的窄 CPU 規格，保存完整唯讀初態，READY 後才實作；不解 helper／runtime／driver／ISR／busy-wait。

兩條 IRQ0 started=completed=5675、active=false／failed=false，等待 DS:00271148=F1 0F 00 00；PIT Mode=2／Reload=5966／Generation=5，Micros=41492925／Deliveries=5695／Pending=false／InService=false。受控事件條件已注入；255 完整座標／游標仍 READY，主選單／正常玩家路徑、音效／受控亂數及整款 remake 未完成。

自然 gzip SHA-256 8985ec08936482ab06857d488af15171bd12743b5ca16504b6984b917d76b9c6／3f0d8a42a931b65f6c4175b1d4eba59cf2e56b87137562d91f2ec9f7dc440251。兩 PNG 同已檢視黑圖 SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。單次 Step／內部IRQ、硬體時計與 Error／restart 模型界限仍照前節，未以掃描通過宣稱硬體逐週期或玩法 parity。原版素材與完整終端／記憶體／gzip／PNG 留本機，不進公開版控。

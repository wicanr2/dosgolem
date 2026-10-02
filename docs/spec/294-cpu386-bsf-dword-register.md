# 294：dword 暫存器的向前位元掃描

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386裸0F BC /r、八來源／八目的32位暫存器，含來源目的別名。

## 原始定位與 CPU 契約

工具基線e914184166c2397dba5041617793a58e42f2f927加READY293。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，輸入／兩自然命令與收據沿 [293-cpu386-xor-dword-register-imm8.md](293-cpu386-xor-dword-register-imm8.md)。第39,983,180步停 dosgolem 高位LE線性0x2548A2，bytes 0F BC D0 03 FA 66 89 3D D0 26 27 00 8B 35 04 2C；ModRM=D0，來源EAX=400h、目的EDX=0，flags=202h。完整R／段待收據審查，不猜位元用途。

[80386 BSF 網頁轉錄](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/BSF.htm)的ZF文字與偽碼矛盾，使用 [正式Intel SDM2A](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 253666-060US、2016年9月印刷頁3-108核對。來源非零寫最低置1位元的0至31索引並清ZF；來源零設ZF，目的未定義。CF／OF／SF／AF／PF全部未定義，不可當定義旗標對拍。

## 候選實作與驗收

- 只接裸32位暫存器形式，先保存完整來源再寫目的，非零從bit0向上找第一個1、清ZF；其他R／段／FPU／記憶體與非算術旗標保持。
- 來源零的目的保留、五個未定義旗標保留，都是明示工具模型，分開驗ZF定義與模型保持；不宣稱原版硬體的未定義結果一致。不擴張word／記憶體／前綴／其他0F指令或hardware exception。
- 全64來源目的配對／別名、全部32個置1位置／高位混合、全部低16位輸入及64算術旗標初態／零來源與不同目的值抽樣；資料索引用獨立「可被2的次方整除」oracle。截短／11前綴／全部非暫存器ModRM拒絕，不讀資料且不發布結果。
- 原版XOR資料 → 完整EAX → BSF目的EDX → 0x2548A5的ADD EDI,EDX → 0x2548A7的word MOV儲存。全部CPU／固定EXE及兩自然完整後態／真實consumer後才限定CONFORMED，並完成293的消費驗收。

## 停止線

完整原始資料與公開契約審查READY後才改CPU。單次Step與硬體時序／Error模型沿既有邊界；不解helper／runtime／ISR／driver／busy-wait或猜遊戲欄位。255／主選單／正常玩家路徑、音效／受控亂數與整款remake另驗，主庫玩法RE閘門不變。

## 唯讀初態與 READY 審查

已證實：兩份293收據尚未修改BSF，CPU.go SHA-256 606fd2bd8482e012a840f5b4cf3ccccb96fa2fefba27f64643b4c31341448aac，有限probe SHA-256 8ee80535e8a15e1be151d588894ee324888af502d053ceef1f31926097e30a64。第39,983,180步完整R順序EAX ECX EDX EBX ESP EBP ESI EDI：400 7C5 0 7CC1EF00 2BDACC 0 6BBC50 740；段CS DS ES FS GS SS：8 188 188 0 20 188，flags=202h。ModRM D0的來源EAX只在bit10置1，目的EDX=0；公開契約預期目的EDX=Ah／ZF=0。兩收據雜湊與完整命令沿293，不另補測試資料或注入遊戲狀態。

完整來源／目的與公開非零資料／ZF、零來源未定義目的、五未定義旗標保持模型及拒絕邊界審查足夠，294轉READY才實作。下一ADD EDI,EDX預期EDI=74Ah，word MOV應將DI存往DS:002726D0；尚待自然重跑，不猜儲存欄位用途。非零來源保留未定義旗標的完整flags=202h是工具模型，只有ZF可當定義旗標驗收。零來源目的保留同樣只驗工具模型，不宣稱硬體exact。

## 受限實作、回歸與原版真實 consumer

限定CONFORMED：裸32位0F BC暫存器，先保存來源，非零寫最低置1索引／清ZF，零設ZF；五個未定義旗標保留與零來源目的保留只驗工具模型。其他R／段／FPU／記憶體／非算術旗標保持，全部前綴／非暫存器ModRM／截短拒絕，不讀資料或發布結果。沒有遊戲位址特例；CPU.go SHA-256 8d1d338408883a85abd8226c94624cd5045b126cf251034a8ab9155c123643ce，有限probe SHA-256 f6888838c1cf3877fc4b07b53583c762ee17e4e4bad2c20cecbb154622376d1a。

GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v全部通過；全64來源目的配對／別名、全部32置1位置與高位混合／64初始旗標、全部低16位值／兩初始旗標、零來源與目的保持模型、全部拒絕及自製XOR→SHL→BSF→ADD→word MOV資料鏈已驗。獨立整除oracle核對資料，ZF與五未定義旗標模型分開驗。CPU收據moo2-294-cpu-tests.txt SHA-256 9f9f260317a0228cbb6b012de9546c128ae770ecb6dcf743213dfc2c016b6c90；固定官方EXE全套DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1通過，full-test-294.txt SHA-256 e865e3ac8f9ca3794654a9503e91b01dc413b06398b59ca6d8672c9f578f646a。294首次通過。

兩自然命令／唯讀輸入沿293，600秒／2GiB／2CPU／UID:GID=1000:1000／network none／Go1.24.13。有無受控滑鼠事件均由原版自行完成完整資料鏈：

| 步數／高位LE線性位址 | 完整EDX／EDI | 定義ZF／模型flags／word儲存 |
| --- | --- | --- |
| 39,983,180／0x2548A2 | 0h／740h | 來源完整EAX=400h，flags=202h；DS word=0000h |
| 39,983,181／0x2548A5 | Ah／740h | 完整 EDX=Ah／ZF=0，五未定義旗標保留後flags=202h |
| 39,983,182／0x2548A7 | Ah／74Ah | ADD EDI,EDX真實消費，flags=202h |
| 39,983,183／0x2548AE | Ah／74Ah | DS:002726D0 word=074Ah，MOV儲存真實消費 |

XOR完整EAX=400h由BSF讀取，目的外所有R／段保持；ADD只更新EDI與算術旗標，MOV只改兩byte儲存。從XOR原始FFFFFBFFh／FFh獨立推導400h及最低置1索引Ah，再逐連續步數比對兩條完整收據，不注入遊戲資料／時計／亂數。此時293與294一併CONFORMED。

兩條第41,223,220步停dosgolem高位LE線性0x23C36B，bytes 09 86 84 03 00 00 EB 25 8B 04 24 8B 94 86 04 04；09 /r的記憶體dword OR缺件，ModRM=86、來源EAX=2000h、disp32=384h、目的DS:[ESI+384h]。ECX=Dh／EDX=6A0h／flags=206h，完整目的資料待唯讀觀測；下一步公開OR／定義五旗標與未定義AF、ModRM／段／拒絕契約建窄CPU規格，READY後才實作。不解helper／runtime／driver／ISR／busy-wait或猜欄位用途。

IRQ0 started=completed=5936、active=false／failed=false，等待DS:00271148=F6 10 00 00；PIT Mode=2／Reload=5966／Generation=5、Micros=42800171／Deliveries=5956／Pending=false／InService=false。受控滑鼠條件已注入；255完整座標／游標仍READY，主選單／正常玩家路徑、音效／受控亂數及整款remake未完成。

兩自然gzip SHA-256 76a750012abac608e9a4a481372344d014400f0ad727376ed7bb510413fc0e81／b8c34ef4981903fab0e2f25a3a1f180d65a1b14f530858a2ce3c4c1e0645eb05。兩PNG同已檢視黑圖 SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。原始ZIP／patch／固定EXE雜湊另行核對一致；原版素材與完整終端／記憶體／gzip／PNG只留本機，公開範圍只有自製來源／測試／有限診斷與文字證據。

## 後續 OR 已接通

記憶體 dword OR 停點已由規格 295 接通，見 [295-cpu386-or-dword-memory-register.md](295-cpu386-or-dword-memory-register.md)。兩自然原版自行將DS:00325864完整40h寫成2040h，再由0x23B693的MOV EAX讀取完整dword。295限定CONFORMED；第42,347,254步轉停實模式DSP OUT 022Ch／C6h，主選單仍未驗收。

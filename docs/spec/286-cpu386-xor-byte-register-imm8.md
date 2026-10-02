# 286 — byte 暫存器目的與 imm8 的 XOR

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 80 /6、ModRM=3 的八個 byte 暫存器目的；既有邏輯旗標模型。

## 原始定位與公開契約

工具基線 96aba2440ce181a1808a508ee897985a2fbcdc99，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。417 原檔／MOX.SET、Go 映像與自然命令沿 [285-cpu386-add-byte-memory-imm8.md](285-cpu386-add-byte-memory-imm8.md)。兩個排程第 20,637,105 步停 **dosgolem 高位 LE 線性** 0x254275，bytes 80 F1 FF 83 C6 04 D3 ED 59 89 07 83 C7 04 83 F9；F1 是 /6、mod=11、r/m=001，CL 與 FFh XOR。ECX=6BBCF9h／flags=282h；完整 R／段與後態見下節，不推定所在 helper 用途。

[原廠 Intel 80386 XOR 契約鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XOR.htm)：80 /6 ib 將 r/m8 與 imm8 的逐 bit 不同結果寫目的。CF／OF 清零，ZF／SF／PF 依八位結果；AF 未定義。沿既有 setLogicFlags8 清 AF 的模型，與 [274-cpu386-xor-word-register.md](274-cpu386-xor-word-register.md) 同一近似，不冒稱未定義旗標與原版硬體全值一致。

## 候選轉移與拒絕邊界

- 只接無 66／67／segment／REP／LOCK 的 80 /6 暫存器形式，目的 AL／CL／DL／BL／AH／CH／DH／BH 由 r/m 選，立即值只讀一個 byte。目的八位以外、其他 R／段／記憶體／FPU 與非算術旗標保持。
- ModRM／imm8 完整取得後才提交目的與旗標；截短或前綴拒絕時不發布資料／旗標。解碼 EIP 可前進。
- 既有記憶體 XOR、其他 group、其他寬度與 30 /r byte XOR 保持，不新增其他前綴或權限／精確硬體例外。
- 垂直鏈為原版 bytes／原始 CL → typed byte 解碼 → XOR／定義旗標 → 原版自然繼續；不改主庫玩法。

## 審查、驗收與停止線

唯讀完整初態與公開契約審查 READY 後才實作。八目的×全部 byte 配對×兩種舊旗標，以逐 bit 比較、低 byte 計數與有號範圍獨立驗定義五旗標；AF 清除另驗工具模型。完整目的外狀態／FPU／記憶體保持、所有前綴與截短拒絕、其他 group 與既有 byte／word／dword XOR 回歸。CPU 全套、固定官方 EXE 全套及兩自然排程後態後才限定 CONFORMED。

只保存下一標準平台停點的原始定位／地址空間／bytes，不解圖形 helper／runtime／driver／ISR／busy-wait。255 完整座標／游標、正常玩家路徑／主選單、音效／受控亂數與整款 remake 仍分開驗收。


## 唯讀初態與 READY 審查

已證實：未改 XOR CPU 的固定原版自然命令沿 285，50M／分離 DOS／同固定 EXE／417 原檔／MOX.SET。第 20,637,105 步高位 LE 0x254275，完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：80000080 6BBCF9 10 28F70880 2BDA90 8A3DC220 6BCC64 6BBC8C；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=282h。CL=F9h、imm8=FFh 的原始狀態已具備。有限 byte_xor_state 探針只讀，不注入暫存器或旗標；本機 moo2-probe-286-input.txt.gz SHA-256 66dfddbeec7a39edd9b5a2707f591613ddbae17fb01db9d1ca2930664e1b5cfb。

公開 XOR 定義旗標與既有 AF 清除模型審查一致，預期 ECX=6BBC06h／flags=206h，其他 R／段保持；AF 僅作工具近似，不聲稱硬體未定義值。完整初態與原始 bytes 足夠，286 轉 READY，才接 byte 暫存器目的。全部 CPU／固定 EXE／兩自然排程後態仍待驗證。


## 受限實作與原版自然後態

限定 CONFORMED：裸 80 /6 的八個 byte 暫存器；完整取得立即值後才寫目的與定義五旗標，AF 清除只屬既有工具模型。未放寬前綴或記憶體／其他 group，目的之外的位元／其他 R／段／FPU／記憶體與非算術旗標保持。八目的×256×256×兩初始旗標全部通過，結果由逐 bit 比較獨立產生；AF 模型另驗，所有截短／11 種前綴／未知 group 拒絕、既有記憶體 XOR 與其他 group 回歸通過。

Go 1.24.13／既有映像與 GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全套通過，moo2-286-cpu-tests.txt SHA-256 de88162fa7262e3ae1513d3a13ef2593465bd0647cd0549673931f4c1ddb60ea。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-286.txt SHA-256 f0469982e157e380e3146b72922d63c4c35bc1bbb2251f464bc364d5393c28da。

沿既有 50M／分離 DOS／417 原檔／固定 MOX.SET 兩自然排程，第二條件加 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1；600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。高位 LE 0x254275→0x254278，第 20,637,105→20,637,106 步 ECX=6BBCF9h→6BBC06h／flags=282h→206h；第二次 6BBCFCh 的 CL=FCh→03h、flags=282h→206h；第三次 CL=FDh→02h、flags=206h→202h。目的 CL 以外的 ECX 位元及所有其他 R／段保持。三筆樣本原先 AF=0，不能推定硬體未定義 AF 的其他取值；沒有注入暫存器／旗標／計時／亂數，不作跨原版完整同狀態聲明。

兩條第 20,651,437 步轉停高位 LE 0x254499，bytes F7 C1 00 00 00 80 75 2A FE 0D C0 26 27 00 75 0C；32 位元暫存器 ECX 與 imm32 的 TEST 缺件。ECX=400h／EAX=80000929h／EBX=3FF09h／EDX=6BBC60h／flags=213h，DS／ES／SS=188h。只按公開 TEST 定義／未定義旗標與下一分支消費建窄規格，不追 helper／runtime／driver／ISR／busy-wait。

IRQ0 started=completed=1598、active=false／failed=false，等待來源 DS:00271148=04 00 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false、Micros=21109785、Deliveries=1618、Pending=false／InService=false。受控滑鼠條件已注入，但 255 完整座標／游標仍 READY。自然 gzip SHA-256 ee93f0ae4120843bf105d14ca5c7f60971fac7f29d3129e743318c1ac6c5bafc／01118b0985ecbc60bed4cf5be32bf718a637f7353241f18d257486e8d5e76489。兩 PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，同已檢視黑圖。原版素材與完整終端／記憶體／gzip／PNG 留本機，主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。


## 後續停點回填

dword 暫存器 TEST 停點已由規格 287 接通，見 [287-cpu386-test-dword-register-imm32.md](287-cpu386-test-dword-register-imm32.md)。兩個自然排程維持完整 R／段並由第一 JNZ 不跳轉續行，下一停點為 0x254510 的 dword ROL；不推定主選單或玩家路徑已完成。

# 285 — byte 記憶體目的與 imm8 的 ADD

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 80 /0 記憶體 byte 與既有 32 位有效地址／段／可寫 bus。

## 原始停點與公開契約

工具基線 c1c8e731c4bcb0a5fda529f43e066508db74ba49。固定官方 EXE／417 原檔／MOX.SET／Go 映像及自然命令沿 [284-cpu386-sub-byte-memory-imm8.md](284-cpu386-sub-byte-memory-imm8.md)。兩個排程第 20,637,097 步停 **dosgolem 高位 LE 線性** 0x25425F，bytes 80 05 C0 26 27 00 18 8A C3 8B 1E FE C9 8B EB D3；ModRM 05 是 /0、mod=00、r/m=101，DS:002726C0 byte 加 18h。原始目的 byte 與自然後態見下節，不猜欄位用途。

[原廠 Intel 80386 ADD 契約鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/ADD.htm)：80 /0 ib 的 r/m8 加 imm8，八位結果寫目的並更新 CF／PF／AF／ZF／SF／OF，其他旗標保持。沿既有段讀寫邊界與 byte bus，不新增完整權限／頁表或精確硬體 exception。

## 候選轉移與拒絕邊界

- 只接無 66／67／segment／REP／LOCK 的 80 /0 記憶體，使用既有 decodeAddress32 的 ModRM／SIB／disp8／disp32／32 位地址繞回及 DS／SS 預設段，不按固定遊戲位址特判。
- 地址與 imm8 取得後讀原始 byte，計算八位和並寫回。只有成功才以既有 add8 發布六算術旗標；完整 R／段／FPU、非算術旗標及其他記憶體保持。SegmentRead8／descriptor／bus 的既有來源優先序保持。
- 截短、未知／越界／不可寫段、bus 讀寫拒絕或前綴失敗即關閉，不部分發布旗標。解碼 EIP 可前進，客製 bus 外部副作用不倒帶。
- 既有暫存器 ADD、284 記憶體 SUB 及其他 group 保持。垂直鏈為原版 bytes／原始 byte → typed CPU 解碼 → 資料／旗標提交 → 原版自然繼續，不改主庫玩法。

## 審查、驗收與停止線

原始目的 byte／完整 CPU 狀態與公開契約審查後 READY 才實作。以較寬和／低 nibble 進位／有號範圍建立獨立旗標預期，驗全部 byte 配對與兩種初始旗標；全部 ModRM／SIB／DS／SS、地址繞回、鄰 byte／FPU 保持、截短／來源／目的／前綴拒絕及既有 register ADD。全部 CPU、固定 EXE 全套與兩條自然排程收據後才限定 CONFORMED。

只保存下一標準平台停點定位／地址空間／bytes，不解所在 helper、runtime、driver／ISR／busy-wait 或硬體逐週期。正常玩家路徑／主選單、255 完整座標／游標、音效／受控亂數與整款 remake 另行驗收。


## 只讀初態與 READY 審查

已證實：未改 ADD CPU 的原版自然命令沿 284，50M／分離 DOS／同固定官方 EXE／417 原檔／MOX.SET。第 20,637,097 步 **高位 LE 線性** 0x25425F 的 DS:002726C0 byte=03h、imm8=18h、flags=297h；R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：8000008A 6BBC03 10 0 2BDA90 0 6BCC64 6BBC8C，段 CS DS ES FS GS SS：8 188 188 0 20 188。有限 byte_add_state 只讀探針不注入資料。本機 moo2-probe-285-input.txt.gz SHA-256 1e3b173ebad3774e900d189dec3c167827a4889cd0e9e514cb6b09c324917d73。

公開 ADD 與既有 byte 可寫提交契約審查一致，預期八位和為 1Bh、flags=206h，尚不作已實測後態；其他 R／段／FPU 保持。285 轉 READY，才補 80 /0 記憶體目的，不改既有 SUB、其他 group 或主庫玩法。完整 CPU／固定 EXE／兩自然排程後態仍待實作驗證。


## 受限實作、回歸與自然前沿

限定 CONFORMED：裸 80 /0 記憶體 byte，沿既有 DS／SS 的 32 位 ModRM／SIB 解碼與可寫 byte 提交；寫入成功後才更新六算術旗標。不放寬其他 group／前綴／未知段，不改主庫玩法。全部 byte 配對／兩種初始旗標、完整 R／段／FPU／非算術旗標／鄰 byte 保持、所有 ModRM／SIB 消費形狀／地址繞回及截短／來源／目的／前綴拒絕通過；既有 register ADD 與 284 SUB 保持。

Go 1.24.13／既有映像，GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過，moo2-285-cpu-tests.txt SHA-256 047482ae709f6d2d8d949b61a3c4e54ceedc44c77ae1b0b661d257b0fb7c5ec0。首次繞回測試誤留 SUB 的 ModRM AF，實作正確執行 SUB，因此預期 ADD 失敗；只將測試改為 ADD 編碼 87，同命令乾淨重跑通過。首次失敗 SHA-256 880b2ef157822ba424aef9c1c7f748d66f32db1d59ba250c7f8d5784f357f97e 保留本機，不歸為產品失敗。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-285.txt SHA-256 8ef615163deaa99eb1a79206023e87d14294c722ba1e0a274fa685de911fd65e。固定 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。

沿既有 50M／分離 DOS／417 原檔／固定 MOX.SET 的兩個自然命令，另一條件加 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1；600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。兩條第 20,637,097 步高位 LE 0x25425F、DS:002726C0 byte=03h／flags=297h，下一步 0x254266 byte=1Bh／flags=206h；完整 R／段保持，符合公開 CPU 契約。未注入目的 byte 或遊戲計時／亂數，不作跨執行器完整同狀態聲明。

兩條第 20,637,105 步轉停高位 LE 0x254275，bytes 80 F1 FF 83 C6 04 D3 ED 59 89 07 83 C7 04 83 F9；ModRM F1 是 /6、mod=11、r/m=001，byte 暫存器 CL 與 imm8 FFh 的 XOR 缺件。EAX=80000080h、EBX=28F70880h、ECX=6BBCF9h、EDX=10h、DS／ES／SS=188h、flags=282h。下一最小行動只按公開 XOR 定義／未定義旗標契約建窄 CPU 規格，不追圖形 helper／runtime／driver／ISR／busy-wait。

IRQ0 started=completed=1595、active=false／failed=false，等待來源 DS:00271148=01 00 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false、Micros=21094693、Deliveries=1615、Pending=false／InService=false。事件條件第 1,612,067 步已注入 x=657／y=189；255 完整座標／游標消費仍 READY。

自然 gzip SHA-256 6c22f3705380a934f0ff26565b3b21f88926c149147b2f6a1d143348eebeff3f／7c8a522b61673f4d145b18bc3e4d5b367f3cfba1b5e54aa28f9dcd39428f4800；兩 PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，同已檢視黑圖。原版素材、完整終端／記憶體／gzip／PNG 留本機；主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。

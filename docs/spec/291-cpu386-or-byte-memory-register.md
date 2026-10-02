# 291：記憶體 byte 目的與暫存器的 OR

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 08 /r、32位 memory ModRM／SIB，八個來源 byte 暫存器。

## 原始定位與 CPU 契約

工具基線 78738dc77eeaee6a1578f7ee98027d679408226f 加 READY 290 的 D2 接線；固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，417 原檔／MOX.SET、工具映像／命令與兩排程初態收據見 [290-cpu386-shl-byte-register-cl.md](290-cpu386-shl-byte-register-cl.md)。兩條第 38,427,369 步停 dosgolem 高位 LE 線性 0x254A06，bytes 08 2C 17 83 C6 04 48 75 DA C3 81 C6 68 74 00 00。ModRM=2C，來源 CH，SIB=17、scale=1、index=EDX、base=EDI；DS:006BBC60 byte=00h，來源 CH=4h／flags=202h。

[原廠 Intel 80386 OR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/OR.htm) 定義 08 /r 的 byte inclusive OR，寫回目的、清 CF／OF，ZF／SF／PF 依結果；AF 未定義，沿既有 setLogicFlags8 清 AF 工具近似。

## 候選實作與驗收

- 保持既有 08 暫存器形式；記憶體形式用既有 decodeAddress32，依 ModRM／SIB 的 DS／SS 預設與32位地址繞回取得目的，讀 byte 與來源 reg8 後寫回 byte，成功才發布定義五旗標。來源 R／段／FPU／非目的記憶體及非算術旗標保持。
- 八來源×全部 byte 配對／初始旗標，逐 bit inclusive OR 與 byte 同位的獨立 oracle 驗資料／定義五旗標；AF 清除模型另驗。全部 ModRM／SIB／scale／index／base、DS／SS 分離、負位移／地址繞回／段末一 byte 與來源別名抽樣。
- 全部截短／11前綴、未知 selector／越界／唯讀／bus 讀寫失敗拒絕，不發布結果或旗標；解碼 EIP 可前進。byte 寫入不得傷鄰接資料。既有 08 暫存器與 09／0A／0C OR 保持，不擴張 word memory、prefix、頁表或硬體 exception。
- 垂直鏈為原版完整 CH／CL → 290 移位 → DS 目的 byte／五旗標 → 下一 ADD ESI,4 消費。全部 CPU、固定 EXE 全套與兩自然排程完整初態／後態及下一入口後才限定 CONFORMED，同次完成 290 的真實 consumer 驗收。

## 停止線

已捕捉完整初態後仍先完成 DRAFT 與公開契約審查，READY 才實作。只保存原始定位／bytes／地址空間，不解圖形 helper／runtime／driver／ISR／busy-wait 或猜欄位用途。255／主選單／正常玩家路徑、音效／受控亂數及整款 remake 另行驗收；主庫玩法 RE 閘門不變。


## 唯讀初態與 READY 審查

已證實：兩份 290 自然初態收據未改 08 CPU，命令／輸入沿 290，第 38,427,369 步高位 LE 0x254A06。完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：F91C 402 0 7CC1EF00 2BDAC8 0 609070 6BBC60；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=202h，DS:006BBC60 byte=00h、可讀，來源 CH=04h。gzip SHA-256 08c1c6979a6dc18bce0378c7cf103e0b61b23f4658ebec0b6f3a05321ba1cd1e／7c7ca00b8d131bb493d68779bd3c5c4c0ed40689c0f2840a4dfae4fdb49d9c76；該次 CPU.go SHA-256 358023dbcf4cf703c522b5984b9ea464678dd7182a19cf38731c5298320464c3。probe 有限診斷只讀，未注入 byte／旗標。

完整 byte 目的／來源、ModRM／SIB、公開 OR 定義旗標與 AF 未定義近似審查足夠；291 轉 READY 後才新增記憶體形式。預期下一步 0x254A09 的 DS:006BBC60 byte=04h／flags=202h、完整 R／段保持；下一 ADD ESI,4 尚待自然消費。不推定圖形用途，CPU／固定 EXE／兩自然後態待驗。


## 受限實作、回歸與自然資料鏈

限定 CONFORMED：裸 08 的32位記憶體目的／八來源 byte，沿既有 ModRM／SIB／DS／SS 解碼；byte 寫回成功才發布五定義旗標，AF 清除只屬工具近似。全部 R／段／FPU／非目的記憶體及非算術旗標保持；截短／11前綴、未知 selector／越界／唯讀／bus 讀寫失敗不發布結果與旗標。既有暫存器 OR 不變。

GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v 全部通過，八來源／全部 byte 配對／兩初始旗標、逐 bit OR／同位獨立 oracle、全 ModRM／SIB／scale／index／base／DS／SS 分離、負位移／地址繞回／段末單 byte、拒絕／保持及64既有暫存器配對已驗；原始完整 R 的 SHL→OR→ADD 抽樣通過。其他 OR 入口由 CPU 全套回歸。moo2-291-cpu-tests.txt SHA-256 5bd1e18e86ed13e733fb07a34419b14bf39c95e9c9a7966a7e860760f31b773e。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-291.txt SHA-256 1bdcc7759989a46b372c57b698c214bd874acce584ac651d0bc2be1767eea084。

兩自然命令沿 290／289 的50M／分離 DOS／固定 EXE／417 原檔／MOX.SET，有無 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1；600 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。原版自行重生三組連續資料鏈：

| 起始步數／OR 前後步數 | DS offset／OR byte 寫回 | 下一 ADD 的 ESI／flags |
| --- | --- | --- |
| 38,427,368／369→370 | 006BBC60／00h→04h | 609070h→609074h／206h |
| 38,427,390／391→392 | 006BBCC2／00h→02h | 60907Ch→609080h／212h |
| 38,427,412／413→414 | 006BBC7A／00h→10h | 609088h→60908Ch／202h |

每組高位 LE 0x254A04→0x254A06→0x254A09→0x254A0C。第一 OR 後 DS:006BBC60 byte=04h，全部 R／段與 flags=202h 保持；第二／三 OR 寫回時同樣保持全部 R／段、flags=202h，來源 CH=02h／10h。下一 ADD ESI,4 分別在起始+3步自然完成，只有 ESI 與算術旗標依 ADD 改變，DS byte 保持。各組完整前後樣本依連續步數比對；未注入資料／遊戲時計／亂數或推定圖形用途。

兩條第 39,983,174 步轉停 dosgolem 高位 LE 線性 0x25488F，bytes F3 AF 83 EF 04 8B 07 2B 3D 00 2C 27 00 83 F0 FF；REPE SCASD 缺件，EAX=FFFFFFFFh、EBX=7CC1EF00h、ECX=800h、EDX=0、ES／DS／SS=188h、flags=246h。下一步按公開 SCASD／REPE 的比較／計數／方向與拒絕邊界建窄 CPU 規格，先保存唯讀完整初態；不解圖形 helper／runtime／driver／ISR／busy-wait。

IRQ0 started=completed=5675、active=false／failed=false，等待 DS:00271148=F1 0F 00 00；PIT Mode=2／Reload=5966／Generation=5、readPending=false，Micros=41492921／Deliveries=5695／Pending=false／InService=false。事件條件已注入；255 完整座標／游標仍 READY，主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。

自然 gzip SHA-256 f3668c23c7c88587088a792fde27e0d982f93240e74883713500d91c148c6dc5／834d501e25dc3159120740655d123f9687bc87961fbd1ccaac7993246a8f413e。兩 PNG 同已檢視黑圖 SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。原版素材與完整終端／記憶體／gzip／PNG 留本機，不進公開版控。

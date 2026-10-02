# 290：以 CL 計數左移 byte 暫存器

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸 D2 /4、mod=11，八個 byte 暫存器與 CL 低五位計數。

## 原始定位與 CPU 契約

工具基線 78738dc77eeaee6a1578f7ee98027d679408226f，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417 原檔／MOX.SET、Go 映像與自然命令沿 [289-cpu386-neg-byte-register.md](289-cpu386-neg-byte-register.md)。兩自然排程第 38,427,368 步停 dosgolem 高位 LE 線性 0x254A04，bytes D2 E5 08 2C 17 83 C6 04 48 75 DA C3 81 C6 68 74。E5 是 /4、mod=11、CH，ECX=102h、flags=202h；完整 R／段與下一 OR 的記憶體初態另以唯讀探針保存。

[原廠 Intel 80386 Shift](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SAL.htm) 定義 D2 /4 SHL r/m8,CL，CL 只取低五位，低 bit 補零。[Intel SDM Volume 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 253667-060US，印刷頁 4-582 編碼、4-584 相容性與 4-585 Flags Affected：計數0不改資料／全部旗標；非零 SF／ZF／PF 依結果，CF 為最後移出 bit，但 byte 計數>=8 時 CF 未定義；計數1 OF=結果最高 bit XOR CF，多位 OF 未定義；非零 AF 未定義。

## 候選實作、近似與驗收

- 重用既有 D0／C0 byte shift 路徑，只加入裸 D2 /4 的暫存器形式。ModRM 與來源 CL 完整取得後才發布結果；CL 目的須用舊 CL 作計數，CH 目的須保留 CL。
- 所有256 CL 遮罩計數都接受。零計數全部保持；定義旗標獨立驗證。沿既有 byte shift 模型：非零清 AF，多位保留 OF；計數8的 CF 取原始 bit0，計數9–31清 CF。這些未定義值只屬工具近似，不宣稱硬體逐旗標相等。
- AL／CL／DL／BL／AH／CH／DH／BH 只寫目的 byte，其餘24位、目的外 R／段／FPU／記憶體及非算術旗標保持。全部截短／11前綴／記憶體及 D2 非 /4 群組拒絕，不發布結果／旗標，解碼 EIP 可前進。既有 D0／C0 SHL／SHR 與 D3 dword shifts 範圍保持。
- 八目的×256來源×256 CL×CF／OF／AF 與保持旗標初態，以較寬乘法／模256、有號 bit／零／逐次除2同位獨立 oracle 驗資料與定義旗標，未定義模型另驗；CL 目的只採來源與計數相同的可表示初態，不注入不可能組合。
- 垂直鏈為原版 bytes／完整 CH／CL → byte 移位／定義旗標 → 下一 OR 消費。全部 CPU、固定官方 EXE 全套與兩自然排程完整前後態後才限定 CONFORMED；下一 consumer 若有缺件，明列未驗，不猜 consumer 或所在 helper 的語意。

## 停止線

完整唯讀初態與公開契約審查 READY 後才實作。不擴張 memory、D2 SHR／SAR／rotate 或其他 opcode；只記下一平台停點原始定位／bytes／地址空間，不解圖形 helper／runtime／driver／ISR／busy-wait。255 完整座標／游標、主選單／正常玩家路徑、音效／受控亂數與整款 remake 仍另行驗收；主庫玩法 RE 閘門不變。


## 唯讀初態與 READY 審查

已證實：未改 D2 CPU 的自然命令沿 289，50M／分離 DOS／固定官方 EXE／417 原檔／MOX.SET；Go 1.24.13、映像 1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，240 秒／2 GiB／2 CPU／UID/GID 1000:1000／network none。第 38,427,368 步高位 LE 0x254A04，完整 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：F91C 102 0 7CC1EF00 2BDAC8 0 609070 6BBC60；段 CS DS ES FS GS SS：8 188 188 0 20 188，flags=202h。下一 OR 的 DS:006BBC60 byte=00h，可讀。有限 byte_shl_cl_state 探針只讀，不注入資料／旗標；moo2-probe-290-input.txt.gz SHA-256 42d61a1478e4daec200bab774810fe1c0c0ba16be81d184f801fc6bec63837f5。

原始 bytes／完整初態、公開低五位計數／定義旗標與既有 byte shift 未定義模型審查足夠，290 轉 READY 後才接 CL 計數。預期下一步 0x254A06 完整 ECX=402h，CH=1h→4h、CL=2h 保持；CF=0／ZF=0／SF=0／PF=0，模型 flags=202h。計數2的 OF 與 AF 未定義，保留 OF／清 AF 只屬工具近似。下一 OR 若可自然執行，應到 0x254A09 並將 DS:006BBC60 byte=00h→04h；consumer 尚待實測，不把預期當通過。


## 移位後態與 consumer 缺件

全部 CPU 及固定官方 EXE 全套通過，CPU SHA-256 7b5cad772c7136a0458c1d4d64004b10596b490230332cf94e81381d4d56b77f，full-test-290.txt SHA-256 b30ffafbfdba0a709b7b76459d7e875fb8b9422efd19b543d92485558dc3e16f。兩自然排程第 38,427,368 步 0x254A04 的完整初態同唯讀樣本；下一步 0x254A06 完整 ECX=402h，目的外 R／段、CL=2h 與 flags=202h 保持。定義 CF／ZF／SF／PF 已核對，計數2的 OF／AF 未定義工具模型不冒稱硬體相等。

下一原版 bytes 08 2C 17 83 C6 04 48 75 DA C3 81 C6 68 74 00 00，dosgolem 高位 LE 0x254A06 的 OR byte [EDI+EDX],CH 缺件，DS:006BBC60 仍00h，尚未寫回；不宣稱 OR 消費已驗，當時 290 保持 READY，後續 291 接通資料鏈的驗收見下節。自然 gzip SHA-256 08c1c6979a6dc18bce0378c7cf103e0b61b23f4658ebec0b6f3a05321ba1cd1e／7c7ca00b8d131bb493d68779bd3c5c4c0ed40689c0f2840a4dfae4fdb49d9c76。本輪原版素材與完整終端／記憶體／gzip／PNG 留本機。


## OR 消費與受限驗收

限定 CONFORMED：裸 D2 /4 的八個 byte 暫存器與全部 CL 遮罩計數，沿既有 byte shift 模型。所有來源／CL／別名、定義旗標／未定義近似、完整目的外狀態、截短／11前綴／其他 group／記憶體拒絕及既有 D0／C0／D3 回歸通過。291 接通後的全部 CPU／固定官方 EXE 全套與兩自然排程資料鏈均通過，精確收據集中於 [291-cpu386-or-byte-memory-register.md](291-cpu386-or-byte-memory-register.md)。

三組高位 LE 0x254A04→0x254A06 完整 ECX=102h→402h、101h→201h、104h→1004h，CL 分別2／1／4保持、目的外 R／段保持，flags=202h。byte 計數2／4的 OF 與 AF 未定義，保留／清除只是工具近似，不宣稱全旗標硬體相等；計數1的 OF=0 已按定義核對。三筆下一 OR 自行寫回 DS 目的 byte=04h／02h／10h，下一 ADD ESI,4 也自行執行。

byte 記憶體 OR 停點已由規格 291 接通。新的 F3 AF 原始停點與未知邊界集中於 291；正常玩家路徑、主選單／音效／受控亂數與整款 remake 未完成。

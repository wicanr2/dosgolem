# 293：dword 暫存器與符號延伸 imm8 的 XOR

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 的裸83 /6、八個32位暫存器目的。word／記憶體／前綴不擴張。

## 原始定位與 CPU 契約

工具基線e914184166c2397dba5041617793a58e42f2f927。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，417原檔／MOX.SET與命令沿 [292-cpu386-repe-scasd.md](292-cpu386-repe-scasd.md)。兩自然排程第39,983,178步停 dosgolem 高位LE線性0x25489C，bytes 83 F0 FF C1 E7 03 0F BC D0 03 FA 66 89 3D D0 26；EAX=FFFFFBFFh／ECX=7C5h／flags=206h。完整目的外R／段待唯讀觀測。

[原廠Intel80386 XOR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XOR.htm)定義83 /6先將imm8符號延伸到目的寬度，逐bit不同才置1，寫回目的；CF／OF清除，SF／ZF／PF依結果，AF未定義。[旗標附錄](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm)以結果低byte的偶數同位設PF。沿既有setLogicFlags清AF的工具近似，不宣稱未定義旗標的硬體一致。

## 候選實作與驗收

- 僅裸83 /6暫存器，保存目的與完整imm8後才發布結果；符號延伸及逐bit XOR，用既有setLogicFlags。五定義旗標與AF清除模型分開驗，其他R／段／FPU／記憶體及非算術旗標保持。
- 八目的／全部256個imm8／各bit及補集、0／FFFFFFFFh／符號邊界／混合值、初始旗標清設；獨立算術式符號延伸與逐bit差異／同位oracle核對。特別驗imm8=80h／FFh高24位延伸及ESP／EBP目的。
- 截短、11種前綴與全部非暫存器ModRM拒絕且不發布R／旗標或讀資料；保持既有83其他group、byte／word／dword XOR入口，不擴張word／記憶體、硬體exception或其他未知group。
- 原版完整R／段 → EAX與imm8 → 0x25489F的XOR後態 → 既有SHL EDI,3 → 0x2548A2的BSF EDX,EAX → 後續ADD消費。全部CPU／固定EXE與兩自然完整後態及真實consumer後才限定CONFORMED。consumer缺件時保留READY。

## 停止線

先保存未改CPU的有限唯讀完整初態，公開規格審查READY才實作。不追helper／runtime／ISR／driver／busy-wait或猜位元用途。原版素材與完整終端／記憶體／gzip／PNG留本機。主庫玩法RE閘門不變；255／主選單／正常玩家路徑、音效／受控亂數及整款remake另驗。

## 唯讀初態與 READY 審查

已證實：CPU.go SHA-256 a137e3c3878b8589328a9fa8d78c9a7a95c7f4d787eb4304b1d418aa84c64c61未修改；有限probe SHA-256 8ee80535e8a15e1be151d588894ee324888af502d053ceef1f31926097e30a64，只讀且每入口最多三次。第39,983,178步完整R順序EAX ECX EDX EBX ESP EBP ESI EDI：FFFFFBFF 7C5 0 7CC1EF00 2BDACC 0 6BBC50 E8；段CS DS ES FS GS SS：8 188 188 0 20 188，flags=206h。原版先由SUB EDI,[00272C00]產生此EDI，不把前一入口6BBD48h混作XOR初態。初態gzip SHA-256 28d2d180e2a51429b93c3341b8a55a8aed2509f632e13a85c9b3c49e69608359。

命令沿292的240秒／2GiB／2CPU／UID:GID=1000:1000／network none與相同映像，重新展開417原檔及固定EXE；DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game，保存workplace/moo2-probe-293-input.txt.gz。公開契約、完整初態及定義五旗標／AF清除近似審查足夠，293轉READY才實作。預期EAX=400h／flags=206h及目的外完整狀態保持，待原版自然重跑；下一C1已接，BSF尚缺件，未完成真實消費前保持READY。

## 已接 XOR，真實 BSF 消費待驗

CPU.go SHA-256 606fd2bd8482e012a840f5b4cf3ccccb96fa2fefba27f64643b4c31341448aac。八目的／76個32位來源／全部256個imm8／四種旗標初態、獨立符號延伸／逐bit XOR／定義五旗標與AF清除模型、完整R／段／FPU／記憶體保持、截短／11前綴／全部非暫存器ModRM與未知group拒絕，以及既有83其他group／XOR寬度回歸已驗。

首次全部CPU失敗是新附帶ADD回歸預期把AF算錯：100h加符號延伸FFh後低四位0+F無進位，正確flags=207h；CPU未改，修測試後同映像／命令乾淨重跑全部通過。失敗收據SHA-256 f9a550438847e8b9e4ad2993628a3152bfdc61530f517d47861b8b6732ca9650，成功CPU收據444963b618d077058288dcd823c5524bed25bbcf2d29e6d0e49c9fec7c325d9a。固定官方EXE全套通過，full-test-293.txt SHA-256 94d4cfb58f36ccea5f2561c87bf266a0a1b55f16e630e727c17292957bd0540b。

兩自然命令沿292的600秒／2GiB／2CPU／50M／分離DOS，有無受控滑鼠事件。第39,983,179步高位LE0x25489F，完整EAX=400h／flags=206h，所有其他R／段保持；下一既有SHL EDI,3在39,983,180步到0x2548A2，完整EDI=740h／flags=202h，其餘R／段與XOR資料保持。BSF仍缺件，293維持READY，沒有把尚未消費的EAX資料鏈稱為完成。兩自然gzip SHA-256 09340c18d5c119c2e9f673e76947b15558278c26532a6b00560f2a72fcb893eb／9707be50def57f1a7387fe609434407623588564895ef5a4cf31dd30abcebee7；下一規格294先沿兩份未改BSF的完整初態審查。

## 後續 consumer 已閉合

BSF 停點已由規格 294 接通，見 [294-cpu386-bsf-dword-register.md](294-cpu386-bsf-dword-register.md)。全部CPU／固定EXE與兩自然重跑完成真實資料消費後，293才限定CONFORMED。第39,983,179步完整 EAX=400h／flags=206h；39,983,181步BSF自行讀取EAX，產生EDX=Ah／定義ZF=0，下一ADD與word MOV存回DS:002726D0=074Ah，其他完整R／段保持規定。AF清除與BSF五未定義旗標保持只屬工具模型，既有C1的多位OF／AF同樣沒有硬體exact聲明。

最終CPU／全套／兩自然收據與下一停點見294，第41,223,220步轉停高位LE0x23C36B的09記憶體dword OR；主選單／正常玩家路徑、音效／受控亂數與整款remake仍未驗收。

此後記憶體 dword OR 停點已由規格 295 接通，見 [295-cpu386-or-dword-memory-register.md](295-cpu386-or-dword-memory-register.md)。40h→2040h的完整寫回／dword MOV消費通過，新停點為實模式DSP OUT 022Ch／C6h；XOR／BSF的原有證據範圍不擴張。

## 後續 C6h 停點回填

SB16 C6h 停點已由規格 296 接通，見 [296-sb16-c6-auto-init-dma.md](296-sb16-c6-auto-init-dma.md)。原版完整C6 20 FF 07接受、實模式333步返回、原版MOV／CMP／JZ成功分支已驗；保護模式連續PCM／IRQ7仍未閉合。第42,347,639步轉停高位LE0x257662的dword ROR立即數10h；本檔原有CPU證據範圍不擴張。

## 後續 ROR 停點回填

dword ROR停點已由規格 297 接通，見 [297-cpu386-ror-dword-register-imm8.md](297-cpu386-ror-dword-register-imm8.md)。兩自然完整ROR／CF與MOV AX,DX消費已驗，新停點為原版IRQ0呼叫內的高位LE0x2520B7、D1 E0的SHL EAX,1。原有CPU／DSP證據範圍不擴張，主選單／正常玩家路徑仍未完成。

dword SHL單位移停點已由規格 298 接通，見 [298-cpu386-shl-dword-register-one.md](298-cpu386-shl-dword-register-one.md)。兩自然IRQ0內完整EAX0／EBX2與兩個dword真實寫回已驗，後續ADC停高位LE0x25179F。原有CPU／平台證據範圍不擴張，完整IRQ0返回與正常玩家路徑仍未完成。

dword ADC停點已由規格 300 接通，見 [300-cpu386-adc-dword-register.md](300-cpu386-adc-dword-register.md)。兩自然三組完整ADC／六旗標與索引ADD真實dword消費已驗，後續word XOR停高位LE0x24678C。原有CPU／平台證據範圍不擴張，完整IRQ0返回與正常玩家路徑仍未完成。

word XOR立即數停點已由規格 301 接通，見 [301-cpu386-xor-word-register-imm8.md](301-cpu386-xor-word-register-imm8.md)。兩自然完整word XOR／五旗標與PUSH EDI、PUSH EAX的兩個stack dword真正突變已驗；此IRQ0已返回，後續第42603292步停高位LE0x256171的66 93、XCHG AX,BX。原有CPU／平台與299原版自然OF=1限制保持，主選單／正常玩家路徑仍未完成。

word XCHG停點已由規格 302 接通，見 [302-cpu386-xchg-ax-word-register.md](302-cpu386-xchg-ax-word-register.md)。兩自然三組完整word交換／兩高16位與全部旗標保持、下一ROR EBX,8完整消費已驗，持續到50M上限且IRQ0 started7789／completed7789／failed=false。畫面已見星空片段，仍未驗主選單／正常玩家路徑；原有其他CPU／平台與299原版自然OF=1限制保持。


## 共用裝置時間後續

保護模式裝置時計缺口由規格 304 接線，見[304-le-shared-device-clock.md](304-le-shared-device-clock.md)。2026-10-03兩自然首block真正PCM2048個80h／兩時計44032078已驗；第42356668步以absolute IVT1201:0682停在未建模的保護模式IRQ7，pending保留。只解時間到首block，IRQ7轉送／連續PCM、人耳及正常玩家路徑仍未知，其他原有證據與限制保持。

## 保護模式IRQ7轉送由規格 305 接線

[305-moo2-irq7-real-mode-passdown.md](305-moo2-irq7-real-mode-passdown.md)以固定1.31 EXE、實際IVT1201:0682／實模式線性0x12692及公開DOS/4GW契約閉合14次原版73步、EOI／22E／IRET；兩自然PCM29175及兩時計44647204已驗。較早IRQ7未派送／返回的記錄屬該舊基線，現行限定轉送依305。平台寄存器映射與1µs時鐘是近似，人耳、完整音訊、鍵盤IRQ1、主選單／正常玩家路徑與整款remake仍未知；其他CPU／255／299邊界保持。

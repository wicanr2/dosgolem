# 212 — 保護模式 83 /1 記憶體 OR 立即數

狀態：**CONFORMED**（僅通用 CPU 指令形狀）
日期：2026-10-01

## 證據與邊界

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，已綁定 DPMI 的 dosgolem 合成環境診斷於第 803 步、**重定位 LE 線性位址** `0x151648`，對原始 bytes `83 0E 01` 的 ModRM `0E` 失敗即關閉。DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在其 **CS:EIP** `0180:00375648 → 0180:0037564B` 以 `LOG 2` 重生同一次指令序列。原版 `DS=0188h`、`ESI=003EC034h`；`MEMDUMPBIN 0188:003EC034 4` 在執行前後得到 `90 00 00 00 → 91 00 00 00`。原版 `LOG` 解讀為 `or dword [esi],0001`，輸出後 CF／ZF／SF／OF／PF 都為 0，其他擷取的通用暫存器未改。版控 `apps/moo2/tools/startup_probe_131.py --or-memory` 重生的私有 `or-memory-registers.json` SHA-256 `333b36bbc93717e6b74a0f0abde417658dcd40b3db47cd750d6fe7b6bc9901d6`；同次 `or-memory-logcpu.txt` SHA-256 `33bff7dea3164d1af83c2c588354fd9c47f078737ea276b2653b8d3e6581781f`；前後四位元組檔 SHA-256 分別為 `0e6c738e4fe755a64a276418309bb5dc7e6bf36772bc0238010718e091a780da` 與 `f00061f6703ccf02a5d5d1ad9d83f2d4db90c9481268364eccdada3e2214d0fe`。原版檔、完整終端及記憶體資料留在私有工作區。

[Intel® 64 and IA-32 架構軟體開發手冊第 2B 卷](https://cdrdv2-public.intel.com/868141/253667-089-sdm-vol-2b.pdf) 定義 `83 /1 ib` 的 32 位形式為 `OR r/m32,imm8`，立即數先符號擴展，再寫回目的；清除 CF／OF，依結果設定 SF／ZF／PF，AF 未定義。`0E` 的 ModRM 是 `mod=00, reg=/1, r/m=ESI`；原版樣本證實 DS:[ESI] 原值 `90h` 相或 `01h` 寫回 `91h`。無前綴的一般 32 位 ModRM 記憶體形式及 sign extension 依 Intel 契約與已有 `decodeAddress32` 語意實作，原版只實測上述 `[ESI]`／立即數 `01h`。玩家路徑、PSP／環境可比性與完整資料消費仍未知。

## 擬議實作與驗收

在既有 `83` 群組中，對無前綴、`/1`、`mod!=11` 的 32 位記憶體形狀，用 `decodeAddress32` 取得 segment／offset，讀 dword，對 `imm8` 作 32 位符號擴展並相或，成功寫回後才呼叫既有 `setLogicFlags`。段權限／界限與讀取失敗須保持記憶體、旗標不變；總線寫入失敗時旗標不變，但既有 `Bus` 介面不提供交易式回滾，不能保證任意底層寫入錯誤沒有部分位元組副作用。不擴充 16 位／repeat／段覆寫或暫存器 `/1` 形狀。`setLogicFlags` 對 Intel 未定義的 AF 採既有清除慣例，不宣稱原版逐情形相同。

合成測試須核對原版 `90h → 91h`、負立即數的符號擴展、另一個解碼位址形狀與 DS／SS 選擇、非可寫或越界記憶體拒絕且旗標與記憶體不變；固定原檔合成環境診斷越過此處並記錄下一停點；`go test ./internal/cpu386 ./internal/machine -count=1` 與 `go test ./... -count=1` 通過。此切片不能代替正常玩家路徑或玩法同狀態對拍。

## 證據審查

固定輸入、原始 bytes、DOSBox-X 同次 `LOG 2` 位址與 DS:ESI、前後四位元組，以及 Intel 的 `83 /1 ib` 編碼相互吻合。原版只實測 `[ESI]` 與 `01h`，其他 32 位 ModRM 記憶體位址及立即數取公開 CPU 契約與既有解碼器為依據，驗收不冒稱原版逐形狀對拍。寫入總線的非交易限制已列明。審查核准**無前綴 32 位記憶體 OR 指令切片**為 READY；完整 DOS 啟動與玩家玩法仍未知。

## 驗收結果

`go test ./internal/cpu386 ./internal/machine -count=1` 與固定原檔輸入的 `go test ./... -count=1` 通過。合成 CPU 測試核對原版形狀的 `90h → 91h`、SS:[EBP-4] 的負立即數符號擴展、DS 與 SS 選擇、非可寫與越界段的記憶體／旗標不變。已綁定 DPMI 的固定原檔**合成環境診斷**越過 `83 0E 01`，從第 803 步推進到第 819 步，在 dosgolem 重定位 LE 線性位址 `0x13CC50` 的 `0F A9` 失敗即關閉。後者只確認 dosgolem 停點，尚未核對 DOSBox-X 原版前後狀態；沒有正常玩家畫面或玩法同狀態對拍。

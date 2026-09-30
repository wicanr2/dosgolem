# 213 — 32 位堆疊的 POP GS

狀態：**CONFORMED**（僅通用 CPU 指令與 MOO2 限定的 selector 載入許可）
日期：2026-10-01

## 證據與邊界

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；已綁定 DPMI 的 dosgolem 合成環境診斷在第 819 步、**重定位 LE 線性位址** `0x13CC50` 的原始 bytes `0F A9` 失敗即關閉。DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在其 **CS:EIP** `0180:00360C50 → 0180:00360C52` 以 `LOG 2` 記錄同一次指令。原版堆疊 `SS=0188h`、`ESP=003EBC50h` 的四位元組前後均為 `20 00 00 00`；執行後 `ESP=003EBC54h`、`GS=0020h`、旗標不變。私有 `pop-gs-registers.json` SHA-256 `d2b6f5a108d1bf8eb035f20eb024871e5b680efc4a6f95777f09f5ad5a35ce1b`；同次 `pop-gs-logcpu.txt` SHA-256 `4636abf3ae7a64765c94b013718b146dfd56f47a13467987299753f3099cd39e`；前後堆疊四位元組檔 SHA-256 同為 `8d71b3faab8201459ad37ef499beb336ba88bdcfa0f51ee6f0a46ec3192d750a`，均由版控 `apps/moo2/tools/startup_probe_131.py --pop-gs` 重生。原版檔及完整終端資料留在私有工作區。

[Intel® 64 and IA-32 架構軟體開發手冊第 2B 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 定義 `0F A9` 為 `POP GS`，在 32 位 operand-size 時讀堆疊 dword、載入 GS 選擇子並讓堆疊指標加 4，不改旗標。原版樣本只證實此寬度及前後狀態；由於彈出的 `0020h` 與原 GS 相同，**不能單憑此樣本證明不同值的選擇子載入**。不同值與失敗情形依 Intel 契約、既有 `POP FS` 的描述子檢查及合成測試驗證。

## 擬議實作與驗收

在現有 `0F A8` `PUSH GS` 與 `0F A1` `POP FS` 的 32 位保護模式範圍，新增無 operand-size／segment／repeat 前綴的 `0F A9`。讀 `SS:[ESP]` 的 dword，以低 16 位作 GS selector；確認堆疊位址及 `ESP+4` 不溢位、selector 可載入後，才更新 GS 與 ESP。讀取／選擇子驗證失敗時 GS、ESP、旗標與堆疊記憶體不變。16 位與其他前綴維持失敗即關閉；不擴大到一般 DOS 或玩家玩法。

合成測試核對不同值 selector、dword 寬度、ESP+4、旗標與堆疊不變；無效 selector、SS 越界及 ESP 溢位拒絕且狀態不變。固定原檔合成環境診斷應越過第 819 步並記錄下一停點；`go test ./internal/cpu386 ./internal/machine -count=1` 與 `go test ./... -count=1` 通過。此切片不是正常玩家路徑或玩法同狀態對拍收據。

## 證據審查

固定原檔 bytes、DOSBox-X 同次前後 CS:EIP、堆疊四位元組、ESP 增量與 Intel `0F A9` 定義相符。原版 GS 值相同的限制已明列，選擇子變更由公開 CPU 契約及不同值測試驗證；沿用現有 `POP FS` 的失敗原子性。核准此 32 位無前綴通用 CPU 指令切片為 READY，未核准 MOO2 完整啟動或玩法對拍。

## 實作時發現的啟動環境缺口

通用 `POP GS` 已以不同 selector、無效 selector、SS 越界和 ESP 溢位通過合成 CPU 測試；固定 1.31 原檔合成環境診斷仍於第 819 步拒絕 `GS selector 0020 未登錄`。這不是 `POP GS` 取值或堆疊寬度錯誤：`MOO2StartupDOS` 的兩次已核對啟動返回都把 GS 設為 `0020h`，原版同次 `LOG` 亦顯示 `SS:[ESP]=00000020h` 且 `POP GS` 成功、ESP 加 4。工具服務層的 `SegmentLoadOK` 閉包只接受 `0028h`／`0030h`，漏列 MOO2 此次已證實可重新載入 GS 的 `0020h`。原先 READY 審查未涵蓋這個啟動環境依賴，因此回到 DRAFT。

擬補的最小服務契約：只在 `moo2Profile` 中允許 `(selector=0020h, destination=SegGS)` 透過既有 `SegmentLoadOK`；其他目的與 FD2 profile 維持現有拒絕。這只建立**載入許可**，不推測 `0020h` 的描述子 base／limit／權限；之後若程式實際用 GS 讀寫記憶體，必須重新查該描述子。需新增服務層測試核對 MOO2 許可、其他組合拒絕，以及固定原檔診斷越過此停點。

## 補充證據審查

原版同次 `LOG` 的 `SS:[ESP]=00000020h`、POP GS 成功與 ESP 加 4，配合目前程式中的 GS=`0020h` 啟動返回和 `SegmentLoadOK` 拒絕條件，直接定位工具環境缺口。只開放 MOO2 的 GS=`0020h` 載入，且不宣稱描述子內容或其他 selector 形狀，已是足以讓這次指令通過的最小契約。核准此補充範圍為 READY；原版描述子詳細屬性仍未知，若後續 GS 記憶體消費出現則另開證據切片。

## 驗收結果與範圍

`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 及固定 1.31 原檔作輸入的 `go test -buildvcs=false ./... -count=1` 全部通過；測試包含 MOO2 的 `0020h → GS` 許可、其他目的拒絕、FD2 profile 拒絕，以及不同 GS selector 的通用 CPU 行為。已綁定 DPMI 的合成 PSP／環境診斷由第 819 步跨過 dosgolem **重定位 LE 線性位址** `0x13CC50`，下一個實際停點是第 2460 步 `0x153E84`，錯誤為帶前綴的 `SBB` 尚未支援。前後步數是工具診斷，不是原版正常玩家路徑；原版同次 `CS:EIP` 輔助觀測與 Intel 契約只對本指令切片提供依據。此有限範圍符合本規格，標為 CONFORMED；玩法、畫面、GS 描述子屬性與同狀態對拍仍未驗收。

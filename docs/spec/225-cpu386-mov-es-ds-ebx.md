# 225 — DS:[EBX] 的 word 載入 ES

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔啟動期的 `8E 03` 停點。

## 原版證據

輸入 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582` 的連續 `LOG 80h`，於 **DOSBox-X CS:EIP** `0180:003801E7 → 003801E9` 記為 `mov es,[ebx]`；輸入 EBX=`003EB9A0h`、DS=`0188h`、ES=`0188h`，來源 DS:[EBX] 顯示 DWORD=`01800188h`，因此低 word=`0188h`，離開 ES=`0188h`，其他通用暫存器與可見旗標不變。版控 `apps/moo2/tools/startup_probe_131.py --startup-value` 可重生私有 `startup-value-logcpu.txt` SHA-256 `f209eb9b55ec3c9184f19f335142880e747bb68848ddf091c6764066f7dc954c` 與 `startup-value-registers.json` SHA-256 `bf96d8bebcb9169c43466c4b3216c0f294ffe783478610c02677e4075c28f25c`。

dosgolem 固定原檔在合成 PSP／環境下至第 5808 步、**重定位 LE 線性位址** `0x15C1E7` 遇 `8E 03` 失敗即關閉；私有 `workplace/moo2-probe-224-after.txt` SHA-256 `23bb76d25b0ce27fb659f558402547cce708676dacda849a9ae5206ad16f12f0`。其 EBX=`0x1CDB48`，不可把原版的絕對堆疊位址直接拿來比較。

## 擬議契約與範圍

僅支援無前綴 `8E 03`，即 32 位位址、DS:[EBX] 的低 16 位 selector 載入 ES。來源必須由既有段讀取路徑檢查 descriptor 基址與界限；候選 selector 必須通過既有 `canLoadSegment`。讀取失敗或 selector 不可載入時保留 ES；成功時只更新 ES 與 EIP，不改通用暫存器、來源記憶體或 EFLAGS。其餘未支援的 `8E` 形狀維持拒絕。本切片為 CPU／DOS 啟動能力，無玩家可見玩法或存檔格式變更。

## 驗收條件

合成測試覆蓋非零 DS base、低 word 寬度、來源越界、無效 selector、無前綴與舊拒絕形狀。固定原檔整合測試自 LE entry 與明示合成 PSP／環境自然至第 5808 步，核對來源 word、單步後 ES/EIP／旗標與下一停點；再跑 `go test -buildvcs=false ./... -count=1`。原版與合成環境的 PSP、堆疊和旗標仍不同；通過只構成受限 CPU 契約，不升格為正常玩家路徑或玩法同狀態對拍。

READY 審查：原版同一次連續記錄保留來源、指令與下一指令，足以確認 `8E 03` 的 ES 目的與 DS:[EBX] 低 word。非零 selector、不同來源 base 與拒絕路徑由現有段描述子契約及合成測試約束；原版樣本僅證實來源與目的同為 `0188h`。`readSegment16` 與 `canLoadSegment` 已是其他 `8E` 記憶體形狀的既有路徑，本規格只新增一個明確 ModRM 分支。無玩家可見規則改動，准予實作。

## 實作與有限驗收

`internal/cpu386/cpu.go` 只新增無前綴、32 位位址 `8E 03` 的 DS:[EBX] word 路徑，透過既有 `readSegment16` 與 `canLoadSegment` 檢查來源及目的；其餘 `8E` 形狀照舊拒絕。`TestMoveESFromDSEBXWord` 核對非零 DS base、低 word、旗標／通用暫存器不變、來源越界、無效 selector 與拒絕形狀。固定原檔 `TestMOO2MoveESFromDSEBXCheckpointWhenProvided` 自 LE entry、合成 PSP／環境自然抵達第 5808 步、**dosgolem 重定位 LE 線性位址** `0x15C1E7`；EBX=`0x1CDB48`、DS:[EBX] 低 word=`0188h`，單步後 EIP=`0x15C1E9`、ES=`0188h`，其餘通用暫存器與 EFLAGS 不變。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-226.txt` SHA-256 `8c5966816d8635d21bf5c0f2c763a9c8412302e37b373ca4ff4e10743d5c578e`。

有界原檔診斷前進至第 5818 步、**dosgolem 重定位 LE 線性位址** `0x15C31B` 的 `CD 33`（`INT 33h`）失敗即關閉；私有 `workplace/moo2-probe-225-after.txt` SHA-256 `9a2ecd6a1d4d9237df587490139ccc4529889753dfed727686e0b929ab1edb17`。原版連續 LOG 在其 **CS:EIP** `0180:0038031B` 也顯示 `int 33`，但 DOSBox-X `LOG` 深入中斷宿主的內部流程並不構成 dosgolem 對拍收據。下個服務的呼叫／返回、輸入裝置狀態與玩家路徑仍待獨立審查；本規格的 CONFORMED 只覆蓋 `8E 03` 指令形狀。

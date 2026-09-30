# 229 — MOO2 保護模式滑鼠軟體重設

狀態：**CONFORMED**
日期：2026-10-01
用途：固定 MOO2 1.31 原檔第二次 `INT 33h` 啟動停點；不是玩家輸入對拍。

## 原版與平台證據

輸入為 1.31 ZIP 內未修改的 `ORION2.EXE`，SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。輔助執行器是 DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 `fd2-dosbox-x:debug-0d7b272b`，ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控探針 `apps/moo2/tools/startup_probe_131.py --mouse-function-21` 在同次啟動序列的 **DOSBox-X CS:EIP** `0180:0038031B` 記錄 `INT 33h`：進入 `EAX=21h`、`EBX=ECX=EDX=0`；於 `0180:0038031D` 的同次返回 LOG，`EAX=FFFFh`、`EBX=3`、`ECX=EDX=0`、旗標 `0006h`。呼叫端接著把 EAX／EBX／ECX／EDX 寫入傳入 record `+0/+4/+8/+0Ch`，再把輸入旗標等寫入其他欄位。私有 `mouse-function-21-call-logcpu.txt` SHA-256 `2acd8b3e8bb5cdbdaf4ddf82ffe31cd38d307e5bf27b80dcbed05115b1aae4ba`；`mouse-function-21-return-logcpu.txt` SHA-256 `587eb82e218255df69c9f09bc49ac9b61058db51a9fe496f6a44888426d5be12`；`mouse-function-21-registers.json` SHA-256 `67f63ba0d3301e080848c8e8e808ec2a0b5eb1868655dd8b8fb1f15f12f75797`。

先前候選 `EV` 返回快照曾顯示首次 `AX=3` 查詢的返回值，與同次連續 LOG 矛盾；**該候選已撤回**，舊快照為何混入本次讀取尚未定位。新版探針從第二次呼叫連續記錄 512 指令，再設返回斷點並取同次返回與 consumer；斷點清理後重跑成功。保留舊私有輸出，只以新版連續 LOG 為本次返回證據。

[DOSBox-X `INT 33h/AX=21h` 實作](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 以 `Mouse_Reset()` 執行軟體重設，回 AX=`FFFFh`、BX=滑鼠按鍵數；其 `Mouse_Reset()` 清空按鍵、按鍵歷史與移動量，位置設為當前範圍中心。這是平台模擬器契約，**不是**原版 MOO2 對所有滑鼠驅動的逐位元實測。固定基準的初次位置 `320,100`，但本筆未直接擷取重設後再次查詢的位置。dosgolem 的合成 PSP／環境診斷於第 6007 步、**重定位 LE 線性位址** `0x15C31B` 停在 `CD 33`，輸入 AX=`21h`；私有 `workplace/moo2-probe-228-after.txt` SHA-256 `dd4ec7d5ac8b04bfc8bb2c8acd977b342f2d6f10654dc6657d379d702f065da7`。

## 擬議受限契約

只在 `NewMOO2StartupDOS` 設定中支援 `INT 33h/AX=0021h`；保留一般 FD2 設定及其他滑鼠功能的拒絕。低 16 位 AX 回 `FFFFh`、BX 回 `0003h`，CX／DX、通用暫存器高 16 位、段與 EFLAGS 保留。三按鍵與高位保留只由固定 DOSBox-X 基準的零高位樣本及受控合成測試界定，不外推至實機。MOO2 專屬受控滑鼠狀態清掉 buttons，並把座標重設為本設定的初始中心 `320,100`；此內部狀態採 DOSBox-X 的軟體重設契約，標為 **platform-spec approximation**，不宣稱原版後續滑鼠查詢已實測。未建模的按鍵歷史與移動量仍未知，若後續 caller 消費則重新開規格。

## 驗收與邊界

合成測試需從非初始座標和非零按鍵進入，核對返回、後續 `AX=3` 查詢的受控重設狀態、高位與旗標保留；一般 FD2 設定與未列功能仍拒絕。固定原檔整合測試須由 LE entry 自然抵達第 6007 步，單步核對返回並由原檔自行寫入 record，然後記錄下一個實際停點。完整 `go test -buildvcs=false ./... -count=1` 通過。這只補執行器啟動服務；尚無 dosgolem 正常玩家畫面、等價 PSP／環境、可重播玩家輸入或與 remake 玩法同狀態收據。

READY 審查：固定版本與同次原版入口／返回／record consumer 足以限定這次 AX／BX 輸出；DOSBox-X 原始碼明示按鍵及座標重設，故受控狀態遷移只採該模擬器契約，並以近似等級及固定起始範圍標示。高 16 位、未建模的歷史及其他滑鼠功能沒有原版非零樣本，明示未知或保守拒絕。此規格不改 remake 玩法，也不把平台模擬器內部寫成原版 MOO2 規則。

CONFORMED 驗收：`internal/machine/le_startup.go` 只在 MOO2 專屬設定接 `AX=21h`，回 AX=`FFFFh`、BX=`3`，清受控按鍵並回到此設定的初始 `320,100`；一般 FD2 及未列滑鼠功能維持拒絕。`TestMOO2ProtectedMouseSoftwareReset` 從非初始座標、非零按鍵與高 16 位合成輸入驗證返回及後續查詢。`TestMOO2MouseQueryCheckpointWhenProvided` 由固定原檔 LE entry 自行重生第 6007 步，單步核對 EIP=`0x15C31D`、AX=`FFFFh`、BX=`3`、CX=DX=`0`，再由原檔把前四欄寫入 record。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過；私有 `workplace/full-test-230.txt` SHA-256 `ff619b50ad8cf1c146c56b3bdc675c7da01d81f5d55cc1d1cdd6c12f8f63a47e`。有界真檔診斷的下一停點是第 6126 步、dosgolem **重定位 LE 線性位址** `0x15C31B`，此次 AX=`1Ah` 尚未支援；私有 `workplace/moo2-probe-229-after.txt` SHA-256 `25ef4143e67248625582e718a6f03e5d677b0233cd68bacd3ecf019b6f270643`。CONFORMED 僅涵蓋上述啟動服務與受控內部狀態，沒有玩家畫面或玩法同狀態對拍。

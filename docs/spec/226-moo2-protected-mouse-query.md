# 226 — MOO2 保護模式滑鼠位置查詢

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔啟動期的 `INT 33h/AX=0003h` 停點。

## 原版輸入與可見回傳

輸入為固定 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，以版控 `apps/moo2/tools/startup_probe_131.py --mouse-query` 在其 **CS:EIP** `0180:00348105` 起連續執行；同次 `LOG 80h` 於 `0180:0038031B` 記錄 `int 33`，輸入 EAX=`3`、EBX=ECX=EDX=`0`、EFLAGS=`0006h`。返回斷點 **CS:EIP** `0180:0038031D` 記錄 EAX=`3`、EBX=`0`、ECX=`0140h`（320）、EDX=`0064h`（100）、EFLAGS=`0006h`。後續連續 `LOG 20h` 顯示 caller 把 EAX／EBX／ECX／EDX 依序存入指標 record 的 `+0/+4/+8/+0Ch`，故座標與按鍵狀態有實際資料消費端。私有 `mouse-query-logcpu.txt` SHA-256 `f209eb9b55ec3c9184f19f335142880e747bb68848ddf091c6764066f7dc954c`、`mouse-query-return-logcpu.txt` SHA-256 `f78b6b183041b58bdebbfdcd7c0a9495126b3a8eb8b756c5582ac80528cbb7bd`、`mouse-query-registers.json` SHA-256 `a3d2148d16441bc034176c6c09e5b52031b754b0d08b48c8e79a1ca10b98abcc`。

dosgolem 固定原檔在明示合成 PSP／環境下第 5818 步、**重定位 LE 線性位址** `0x15C31B` 遇 `CD 33` 失敗即關閉；私有 `workplace/moo2-probe-225-after.txt` SHA-256 `9a2ecd6a1d4d9237df587490139ccc4529889753dfed727686e0b929ab1edb17`。其 EAX=`3`、EBX=ECX=EDX=`0`，ES／DS／SS=`0188h`，EFLAGS=`0016h`；絕對堆疊和旗標與 DOSBox-X 不同。

## 擬議受限契約

只在明示的 MOO2 保護模式啟動設定處理 `INT 33h/AX=0003h`；一般 FD2 啟動設定及其他 `INT 33h` 功能繼續失敗即關閉。滑鼠狀態明示保存為按鍵遮罩、水平／垂直座標；固定 1.31 DOSBox-X 輔助基準的**初始測試設定**為按鍵 `0`、`X=320`、`Y=100`，不冒稱所有 DOSBox、實機或顯示模式的普遍預設。允許上層控制測試狀態，避免正式執行永久固定單一座標。查詢時將低 16 位的 BX／CX／DX 分別設為按鍵遮罩／X／Y，保留高 16 位、EAX、其餘暫存器、段與 EFLAGS；高位保留是 dosgolem 的決定性保守契約，原版樣本只實測高位為零。未列的功能一律拒絕。

此切片只建立查詢與受控輸入，不建立真實 GUI 滑鼠事件、範圍設定、按鍵歷史或畫面功能。remake 玩家可見規則、UI 與存檔不變；正常玩家路徑與玩法同狀態仍待後續驗證。

## 驗收

合成測試核對固定初始狀態、受控非零座標與按鍵、高 16 位保留、未列服務拒絕及一般 FD2 服務不被誤開。固定原檔整合測試從 LE entry 及合成 PSP／環境自然至第 5818 步，核對前態和後態、EIP 與既有來源 record 的後續消費；若下游尚有 CPU 缺口，記錄最早停點而不改測試輸入掩蓋。全套 `go test -buildvcs=false ./... -count=1` 須通過。DOSBox-X 只作輔助；正式遊戲玩法對拍需 dosgolem 自生、同狀態且可重播的玩家路徑收據。

READY 審查：原版同一條執行序列已證明功能號、輸入與中斷返回，也顯示 caller 讀取 BX／CX／DX 並寫入 record；`internal/dos/bios.go` 的既有 16 位 `AX=3` 契約提供低 word 的通用語意，但不自動證明保護模式已接線。固定初始座標只屬這個 DOSBox-X 基準，故放在 MOO2 專用啟動設定並提供可控狀態；不把它寫進通用 FD2 回傳。高 16 位與其他旗標未由原版非零樣本證實，明標為決定性保守契約。證據已足以補受限服務，不足以擴充到其他滑鼠功能或宣稱正常玩家路徑。

## 實作與驗收收據

`internal/machine/le_startup.go` 在 `NewMOO2StartupDOS` 設定此固定輔助基準的初始 `X=320、Y=100、buttons=0`，由 `SetMouseState` 可明示改動測試輸入；受保護模式 `INT 33h` 只允許 `AX=3`，其他功能與一般 FD2 設定仍拒絕。合成測試核對初始與非零受控狀態、高 16 位保留、旗標不變及拒絕路徑。固定原檔 `TestMOO2MouseQueryCheckpointWhenProvided` 由 LE entry 自然到第 5818 步、**dosgolem 重定位 LE 線性位址** `0x15C31B`；輸入 EAX=`3`、EBX=ECX=EDX=`0`，服務後 EIP=`0x15C31D`，EBX=`0`、ECX=`0140h`、EDX=`0064h`、EAX 與 EFLAGS=`0016h` 不變。其後原檔自行把 EAX／EBX／ECX／EDX 寫至 `EDI+0/+4/+8/+0Ch`，整合測試在 `0x15C1C6` 前讀回 `3、0、320、100`；對應原版 caller 寫入見前述 DOSBox-X 連續 LOG。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-227.txt` SHA-256 `127a6b85e658be9521863d188a6e6cad67dd9a25a4af30f624bd5e6d65ee4b4f`；新增消費端斷言另以 `-v` 跑過。

合成環境原檔有界診斷接著在第 5830 步、**dosgolem 重定位 LE 線性位址** `0x15C1C6` 的 `8F 47 14` 失敗即關閉；私有 `workplace/moo2-probe-226-after.txt` SHA-256 `7190527fa2c8dfc170172864e4dd46ef31edeca94d62aac78a53299aa611b1b6`。原版 **CS:EIP** `0180:003801C6` 的同次 return LOG 有 `pop dword [edi+0014]`；下一指令仍需獨立受限 CPU 規格。此 CONFORMED 僅驗證固定啟動滑鼠查詢及 record 寫入；沒有 GUI 輸入事件、畫面或玩法同狀態收據。

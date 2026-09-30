# 230 — MOO2 保護模式滑鼠零敏感度設定

狀態：**CONFORMED**
日期：2026-10-01
用途：固定 MOO2 1.31 啟動序列第三次 `INT 33h` 停點；不是玩家輸入或移動對拍。

## 原版與平台證據

輸入為 1.31 ZIP 內未修改的 `ORION2.EXE`，SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。輔助執行器是 DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 `fd2-dosbox-x:debug-0d7b272b`，ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --mouse-function-1a` 從已核對的 `AX=3` 查詢及 `AX=21h` 重設延續。於 **DOSBox-X CS:EIP** `0180:0038031B` 的 `INT 33h`，進入 EAX=`1Ah`、EBX=ECX=EDX=`0`、EFLAGS=`0012h`；同一次於 `0180:0038031D` 返回，EAX／EBX／ECX／EDX 與 EFLAGS 不變。呼叫端把四個值寫到 record `+0/+4/+8/+0Ch`。首次有界容器在完整輸出後命中外層 170 秒逾時；改為 240 秒並以相同映像／命令乾淨重跑成功，輸出雜湊完全相同：私有 `mouse-function-1a-registers.json` SHA-256 `5d877c4ccaaeaf841c28ef7f66ead671e3a81fe1d9691d091b5d7a62ea6723e0`、`mouse-function-1a-call-logcpu.txt` SHA-256 `9405aafaff6e2bdb1ac39c626f618d9b9906eb2293cd8549e63602fba0fe1e62`、`mouse-function-1a-return-logcpu.txt` SHA-256 `02127b37ba6ce8e1347a92bdb2bd789a86a84fff6a1281b35e588a9c49453f82`。

[DOSBox-X `INT 33h/AX=1Ah` 原始碼](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 將 BX／CX／DX 作為水平敏感度、垂直敏感度及倍速值，交給 `Mouse_SetSensitivity`；設定值先上限裁切至 100，儲存原始三值，只有 X 與 Y 都非零才更新內部速度係數。這是**平台模擬器契約**，不是原版 MOO2 對所有滑鼠驅動的實測；固定原版只觀測三值皆零的呼叫，未觀測後續滑鼠移動／讀回的效果。

dosgolem 在合成 PSP／環境的固定原檔診斷於第 6126 步、**重定位 LE 線性位址** `0x15C31B` 停在 `CD 33`，EAX=`1Ah`、EBX=ECX=EDX=`0`，私有 `workplace/moo2-probe-229-after.txt` SHA-256 `25ef4143e67248625582e718a6f03e5d677b0233cd68bacd3ecf019b6f270643`。該停點不是正常玩家畫面或玩法同狀態收據。

## 擬議受限契約

只在 `NewMOO2StartupDOS` 啟動設定中接受 `INT 33h/AX=001Ah` 且 BX／CX／DX 的低 16 位皆零；非零值及一般 FD2 設定仍拒絕，以免未建模的移動係數造成靜默差異。成功時保留所有通用暫存器、段與 EFLAGS，記錄三個已設為零的敏感度原始值；不改已受控的座標與按鍵。這是固定輸入的工具能力，無法證明完整滑鼠移動、敏感度讀回或正常 GUI 輸入。

## 驗收

合成測試覆蓋零輸入的寄存器／段／旗標不變、受控位置與按鍵不變、非零值與非 MOO2 設定拒絕。固定原檔從 LE entry 自然走到第 6126 步，單步確認返回，再由原檔寫入 record 的前四欄，記錄下一個實際停點。含原檔的全套 `go test -buildvcs=false ./... -count=1` 通過。正式玩法同狀態對拍仍須 dosgolem 由等價起始狀態、正常玩家輸入自行重生。

READY 審查：固定輸入、同次入口／返回／caller record 足以限定零輸入的玩家路徑可見返回；DOSBox-X 原始碼明示三值儲存及兩個軸非零才更新移動係數，故零值只更新 raw 設定、保留尚未建模的移動速度。非零設定會改移動，因此在尚無上層移動模型及測試前維持拒絕。初始 `50,50,50` 只依 DOSBox-X 初始化原始碼列為平台近似；固定原版沒有直接讀回。此規格只開執行器啟動服務，沒有改 MOO2 remake 玩法。

CONFORMED 驗收：`internal/machine/le_startup.go` 只在 MOO2 啟動設定處理 BX／CX／DX 低 16 位全零的 `AX=1Ah`，保留暫存器、段、旗標、受控座標與按鍵，將三個 raw 敏感度值設為零；一般 FD2 與非零設定仍拒絕。`TestMOO2ProtectedMouseZeroSensitivity` 核對此前後的受控狀態及高 16 位。固定 1.31 原檔整合測試由 LE entry 自行走到第 6126 步、**dosgolem 重定位 LE 線性位址** `0x15C31B`，單步至 `0x15C31D`，原檔將 `1Ah、0、0、0` 寫入 record。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-231.txt` SHA-256 `a983b838b0b84c14b442a82b127d937f4101c88c5fe6dfc0ce6db6742f536e6a`。有界原檔診斷下一停點第 6256 步、同一 **重定位 LE 線性位址** `0x15C2B2` 的 `CD 10`；私有 `workplace/moo2-probe-230-after.txt` SHA-256 `aab5a75b6ec1b381e7a7008968ae53844b0e7ce1deed221ef8d8b6872ca7073e`。此狀態僅涵蓋上述固定啟動服務及受控 raw 值，不表示視訊服務或正常玩家畫面已可用。

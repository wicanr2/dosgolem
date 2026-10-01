# 257 — MOO2 保護模式滑鼠座標設定

狀態：**CONFORMED（受控位置平台模型）**
日期：2026-10-01
範圍：`INT 33h/AX=0004h` 的受控位置狀態；不畫系統游標、不改 remake 玩法。

## 證據

- **已證實，自生停點**：隔離 dosgolem `438d6cc5971c3e212e0ce949e1ddd61de307f794` 加已測規格 255／256，Go 1.24.13，官方 1.31 EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版根層 417 檔及固定 `MOX.SET`。第 1,546,487 步 **dosgolem 高位 LE 線性** `0x24C31B` 的 `CD 33`：AX=`4`、BX=`0`、ECX=`0280h`、EDX=`002100F0h`、ES／DS／SS=`0188h`、EFLAGS=`0012h`。私有 `workplace/moo2-probe-256-full-game.txt.gz` SHA-256 `6d5c5026bd1668c5d7e438147665c7919931e34980bd6c7b7ed3372a2a561df1`。有事件分支在一個回呼完成後抵達相同服務，不等於兩側全部狀態一致。
- **已證實，原版同次返回**：DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b`；版控 `apps/moo2/tools/startup_probe_131.py --mouse-set-position`。**DOSBox-X CS:EIP** `0180:0038031B` 原始 `CD 33 C3`：AX=`4`、BX=`0`、ECX=`0280h`、EDX=`003400F0h`、ESI／EDI=`0`、DS／ES／SS=`0188h`、ESP=`003EBB18h`、EFLAGS=`0016h`。下一指令 `0180:0038031D` 所擷取欄位全部保持，caller 寫入服務 record 後繼續初始化。JSON SHA-256 `166d48402e3b63d8025a6f4faa6873d6756f22e6ca4382b4a1be5553c6ad4bfd`、caller LOG SHA-256 `fa8bc678aa5cf75bd2ca7f9d40ebb2cd46b300a5825556527abbceecee6e6187`、終端 SHA-256 `0727711577ffa2bb3fbd3a64247f1bd29f96677b8491de8da9116961dc17132e`。工具映像 ID、ZIP／MOX.SET 雜湊沿用 [255-moo2-protected-mouse-callback.md](255-moo2-protected-mouse-callback.md)。
- **公開平台契約**：[DOSBox-X 滑鼠來源](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html) 的 `INT33_Handler case 04h` 按 CX／DX 有號低 word 設定位置、限制於現行兩軸範圍；不改輸入暫存器，不排入移動事件。其游標繪製不屬於本規格。

## 擬議契約與驗收

READY 審查：實際同次暫存器保持與低 word 輸入、caller 繼續初始化及公開位置裁切契約已核對；重用既有範圍與查詢狀態，不引入新的座標模型。據此 DRAFT 轉 READY，設定後原版讀回及游標畫面仍未知。

只在明示 MOO2 設定接受 `0004h`；低 CX／DX 經既有範圍模型後保存為位置。完整一般暫存器、段、旗標、按鍵、敏感度、範圍與回呼註冊／佇列保持，不由程式設定座標憑空製造一般輸入事件。未知範圍／未設定時沿用既有受控位置模型，不自創資產或畫面比例。

測試實際 `640/240` 且 EDX 有高位雜訊、負值／上下界／端點、query 讀回、高位與架構狀態保持、回呼不觸發、一般 FD2 拒絕；固定 EXE 全套回歸、無事件／受控事件兩條原檔自然路徑重跑。沒有原版設定後 query 收據，不把自製讀回稱為原版值對拍。

## CONFORMED 收據

實際 `640/240` 與 EDX 高位雜訊、負值、端點、範圍裁切、受控讀回、架構欄位保持、按鍵／敏感度／回呼不改與一般 FD2 拒絕均通過。舊查詢測試用 `AX=4` 代表未知功能，依本規格改為仍未支援的 `AX=5`；保留未知服務拒絕判準。

固定原檔全套 `go test -buildvcs=false ./... -count=1` 通過，最終 `workplace/full-test-257.txt` SHA-256 `33ca5589160c47f992c5cdeec08b6b65ded02dfc6a03fb705aa698c21a05f26e`。第一次命令把 `sha256sum` 誤隨檔名替換成 `sha257sum`，測試與兩份診斷實際成功，錯在收尾 shell；保留 `*-257-first*` 私有收據，修正命令後以同一映像、同一資料及命令乾淨重跑。

無事件原檔路徑於第 6,216,999 步、設定後受控事件路徑於第 6,217,034 步，都在 **dosgolem 高位 LE 線性** `0x217888` 的 `CD 2F`、AX=`160Ah` 拒絕；受控路徑回呼 started=1／completed=1。私有無事件診斷 SHA-256 `3b7e90bfcb64cfdbbbf9f834e7f7bd185ddc6f94fcee56cd8312c5fc812653c2`、受控事件診斷 SHA-256 `5cc4ec566b82857814f12e271e0dedd88f8f283835e9505cac0f7c6dc88324ba`。只驗位置平台狀態及自然前進，沒有正常玩家畫面或設定後原版 query 值比對。

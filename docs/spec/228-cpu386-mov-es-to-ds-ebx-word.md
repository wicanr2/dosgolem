# 228 — 將 ES 的 word 寫入 DS:[EBX]

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔的 `66 8C 03` 啟動指令停點。

## 原版證據與位址

輸入固定 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，以版控 `apps/moo2/tools/startup_probe_131.py --es-store` 從同一啟動序列取得前後記憶體及 `LOG 2`。原版 **CS:EIP** `0180:003801D6 → 003801D9` 顯示 `mov [ebx],es → pop es`；當次 EBX=`003EB9A0h`、DS=ES=`0188h`、EFLAGS=`0046h`，DS:[EBX] 兩位元組前後均為 `88 01`。私有 `es-store-before.bin`／`es-store-after.bin` SHA-256 都是 `cc808bee2be109604fc5c47d2ad89282d6ced79ee8fd598a5f7ada73ddab4a81`；`es-store-logcpu.txt` SHA-256 `7a0e2d9e565f3c677bbee16d20ea6bb5d84f43edb7b1fe3d3a49c21537e66340`；`es-store-registers.json` SHA-256 `b122d06aa4017c04594f1489b70b07c5c664e65c91d3b22ee4aa045014a4b07f`。來源與原目的相等，因此這筆原版記錄**不能單獨證明非相等值的寫入**。

dosgolem 的固定原檔、合成 PSP／環境診斷在第 5838 步、**重定位 LE 線性位址** `0x15C1D6` 遇 `66 8C 03` 拒絕；私有 `workplace/moo2-probe-227-after.txt` SHA-256 `3becc0caf1cd574f0818da4e265a4358c163caa08f86c1f4b6b91a2ae468db50`。該診斷仍未形成正常玩家畫面或玩法同狀態對拍。

## 擬議受限 CPU 契約

只新增單一 `66 8C 03` 形狀：operand-size 前綴、`8C /0`、`mod=00`、`rm=EBX`，將 ES 的低 16 位依小端序寫入 DS:[EBX]。寫入須經現有 DS descriptor／界限檢查；成功時 EIP 加 3，其餘通用暫存器、段與旗標不變。來源 ES 不因讀取而改變；目的越界須拒絕且保留目的。其他尚未支援的 `8C` 形狀與段／repeat 前綴仍拒絕。非相等值的搬移依 Intel `MOV r/m16,Sreg` 契約及合成測試證明，不能冒稱原版非零差異實測。

這是工具 CPU 能力，原版 caller 屬啟動／平台包裝鏈；不把此指令當玩法規則或 remake 玩法完成度。

## 驗收

合成測試覆蓋非零 ES、非零 DS base、小端序、界限失敗、旗標／暫存器不變與未列形狀拒絕。固定原檔整合測試由 LE entry 自然抵達第 5838 步，核對單步前後 DS:[EBX]、EIP、ES、旗標及下一停點；全套 `go test -buildvcs=false ./... -count=1` 通過。正式原版玩家對拍仍須由 dosgolem 從可重播的等價狀態自行重生。

READY 審查：原版同次 LOG、固定輸入雜湊、位址基準、來源 selector 與記憶體前後 bytes 足以定位這一指令的已觀測結果；前後相等不能證明一般搬移，所以該部分只依 [Intel `MOV r/m16,Sreg` 指令契約](https://cdrdv2-public.intel.com/835752/253667-sdm-vol-2b.pdf)及合成非相等值測試。既有 `8C` 記憶體形狀已使用 `writeSegment16`，只開放這個明確 ModRM，不推廣成所有 8C。證據足夠支援受限工具能力，未知邊界保留。

CONFORMED 驗收：`TestMoveESWordToDSEBX` 用 ES=`1234h`、DS base=`20h`、目的原值 `BBAAh`，通過小端序 `34 12` 與暫存器／旗標不變檢查；拒絕測試覆蓋跨界 word 及未列編碼。固定 1.31 原檔整合測試於第 5838 步、dosgolem **重定位 LE 線性位址** `0x15C1D6` 讀到目的 `0188h`，單步至 `0x15C1D9`，目的仍為 `0188h`，通用暫存器、段與旗標不變。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-229.txt` SHA-256 `78c15a85c6a0902893dfde545d0db82ea8ebc4dfe275812eefed49cee5d1e0d1`。有界原檔診斷下一停點為第 6007 步、`0x15C31B` 的 `INT 33h`，本次 AX=`21h` 尚未支援；私有 `workplace/moo2-probe-228-after.txt` SHA-256 `dd4ec7d5ac8b04bfc8bb2c8acd977b342f2d6f10654dc6657d379d702f065da7`。此狀態僅涵蓋上述 CPU 指令與合成環境，並非 GUI 或玩法對拍。

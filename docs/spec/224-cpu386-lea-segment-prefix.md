# 224 — LEA 的段前綴不參與有效位址

狀態：**CONFORMED**  
日期：2026-10-01  
用途：固定 MOO2 1.31 原檔的啟動指令缺口。

## 原版證據與狀態差異

輸入固定 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，於 **DOSBox-X CS:EIP** `0180:003801DF → 003801E6` 的同次 `LOG 2` 顯示 `lea eax,cs:[esi+00380282] → push eax`。進入 ESI=`00000093h`、EAX=`00000031h`、EFLAGS=`0212h`、CS=`0180h`、DS=`0188h`；離開 EAX=`00380315h`（`93h+00380282h`），其餘可見暫存器與旗標不變。版控 `apps/moo2/tools/startup_probe_131.py --lea-cs` 重生私有 `lea-cs-logcpu.txt` SHA-256 `44697865de21f50b0417f3924b52878aec2988a62dbc045ab4113befdacc7f2c`、`lea-cs-registers.json` SHA-256 `5e6bf786def83d5bcc73bb49f015bf42c901c34e4355f244ca05e01a71362adb`。候選 EV 前態只供定位，前後值以同次 LOG 為準。

dosgolem 在合成 PSP／環境下的固定原檔第 5806 步停於**重定位 LE 線性位址** `0x15C1DF`，bytes `2E 8D 86 82 C2 15 00`。進入 ESI=`00000099h`、EAX=`00000033h`、EFLAGS=`0016h`，與原版本次狀態不同；不能稱同狀態對拍。私有 `workplace/moo2-probe-224-before.txt` SHA-256 `029288f59181b5ab9299cfde239b176d6923fad1b90a805d9f37b75c0c31606a`。兩工具位址基準不同，`0x224000` 的數值差也不取代原版語意證據。

## 擬議通用 CPU 契約

依 [Intel LEA 指令表](https://cdrdv2-public.intel.com/789581/325383-sdm-vol-2abcd.pdf)，`LEA` 寫入的是來源運算式的有效位址（offset），不讀取該記憶體，也不修改旗標。對現有 32 位元 `8D /r` 記憶體來源路徑，允許單一 CS／DS／ES／SS 段覆寫前綴；該前綴只佔指令 bytes，不加入段基址、不檢查 selector／descriptor，也不改有效位址計算。原版只實測 CS 這個形狀；其他段前綴屬手冊加合成測試的通用擴充。16 位元 operand-size、repeat、重複段前綴與 `mod=11` 仍拒絕；其他 opcode 的段前綴限制不變。

## READY 審查與驗收

審查原版連續 LOG、固定雜湊、位址基準與 dosgolem 狀態差異。合成測試核對四種單段前綴、不同或無效 selector、無目標記憶體、負位移／溢位、旗標與非目的暫存器不變，以及拒絕形狀。固定原檔從 LE entry 自然越過第 5806 步並記錄下一停點；全套 `go test -buildvcs=false ./... -count=1` 須通過。既有 [021-cpu386-lea-disp8](021-cpu386-lea-disp8.md) 與 [186-fd2-platform-gap-continuation](186-fd2-platform-gap-continuation.md) 的「segment prefix 拒絕」只是舊支援範圍，完成後應附註本規格擴充，避免被誤讀成永久 CPU 定義。

READY 審查結論：原版 `2E 8D 86` 的有效位址和 `ESI+disp32` 相符，CS／DS selector 不同而結果仍是 offset；旗標未變。其餘段前綴依 Intel 對 LEA 的有效位址契約擴充，不冒稱 MOO2 原版實測。dosgolem 的 ESI、EAX 與 EFLAGS 已知不同，因此驗收只要求它在**自己的**合成狀態算出正確 offset；不設兩側暫存器逐值相等的假閘門。證據已足以實作這個有限 CPU 改動。

本切片只補 dosgolem 的 CPU 能力，不修改 MOO2 remake 玩法；合成環境與原版差異仍需另查，不能由此聲明正常玩家路徑。

## 實作與驗收收據

`internal/cpu386/cpu.go` 的前綴路由只為 `8D /r` 開放單一 CS／DS／ES／SS 段覆寫；LEA 本身繼續使用既有 32 位元有效位址計算流程，不讀目標記憶體或描述子。合成測試以超出 testBus 的位址、無效 ES／SS selector、四種段前綴、負位移／32 位元環回驗證結果和旗標，並拒絕重複段前綴、operand-size／repeat、截短指令及 `mod=11`。舊 [021-cpu386-lea-disp8](021-cpu386-lea-disp8.md) 與 [186-fd2-platform-gap-continuation](186-fd2-platform-gap-continuation.md) 已附註本次擴充；舊拒絕清單是當時支援範圍，並非原版 CPU 永久規則。

固定原檔 `TestMOO2CSLEACheckpointWhenProvided` 從 LE entry 在合成 PSP／環境下自然至第 5806 步、**dosgolem 重定位 LE 線性位址** `0x15C1DF`；以 ESI=`99h`、位移 `15C282h` 得 EAX=`15C31Bh`，EIP 進至 `0x15C1E6`，其他暫存器、段與 EFLAGS=`0016h` 不變。這是 dosgolem 自行重生的**本身狀態** CPU 收據；DOSBox-X 原版同次 LOG 的 ESI=`93h`、EAX=`380315h`、EFLAGS=`0212h` 不同，不得稱完整同狀態。進入指令時 ESI 差 6 的原因仍未知，合成 PSP／環境長度是待查候選。

含固定原檔的 `go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-225.txt` SHA-256 `0a64b5aaa486fe09688bfa7453952c1f75866b6932c45d3d606af965d7f00469`；新 CPU 與原檔整合測試另以 `-v` 確認執行。固定原檔有界診斷在第 5808 步停於**同位址基準** `0x15C1E7` 的 `8E 03`（報錯 EIP=`0x15C1E9`）；此新停點尚未由原版獨立核對。私有 `workplace/moo2-probe-224-after.txt` SHA-256 `23bb76d25b0ce27fb659f558402547cce708676dacda849a9ae5206ad16f12f0`。原版 EXE 與完整收據不入 Git；仍無 dosgolem 正常玩家畫面或 MOO2 玩法同狀態收據。

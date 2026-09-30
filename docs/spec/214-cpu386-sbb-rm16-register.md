# 214 — 16 位暫存器目的的 SBB r/m16,r16

狀態：**CONFORMED**（限 `66 19 /r` 的 16 位暫存器形狀）
日期：2026-10-01

## 原版問題與證據

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，dosgolem 以內嵌 MZ `0x26654` 載入後，在第 2460 步的**重定位 LE 線性位址** `0x153E84` 遇到原始 bytes `66 19 C0 5F 5E 5A 59 5B C3`，因 operand-size 前綴失敗即關閉。這是已綁定 DPMI、仍使用合成 PSP／環境的啟動診斷，不能稱為正常玩家路徑。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在其**CS:EIP** `0180:00377E84 → 0180:00377E87` 的同次 `LOG 2` 顯示 `sbb ax,ax → pop edi`。原版進入 `EAX=00000600h`、CF=0、ZF=0、AF=1、PF=1；離開 `EAX=00000000h`、CF=0、ZF=1、AF=0、PF=1，其他受觀測暫存器不變。版控 `apps/moo2/tools/startup_probe_131.py --sbb-word` 可重生私有 `sbb-word-registers.json` SHA-256 `9c894626819a99de98bde84a95a6365a4544eb0f4779f56ea88e0e4f0b858eb5`；同次 `sbb-word-logcpu.txt` SHA-256 `19ba3515c9b318bef01351e27c6d335405e6695c3d6649311ec9f9dc78f5883a`。原始終端與 EXE 留在未版控工作區。

[Intel® 64／IA-32 架構軟體開發手冊第 2B 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 的 `SBB` 條目定義 `19 /r` 的 16 位目的／來源形狀：`dest = dest - src - CF`，更新 CF、PF、AF、ZF、SF、OF。`66` 在此 32 位預設模式選 16 位運算元，`C0` 的 `mod=11`、reg/rm 均為 AX。原版樣本只證實 CF=0、AX 自減的結果；CF=1、不同暫存器及高 16 位保留由 Intel 契約與合成測試驗證，不能寫成原版實測。

## 擬議行為與驗收

只擴充 `66 19 /r`、`mod=11` 的暫存器形狀；目的由 `r/m` 指定，來源由 `reg` 指定，採低 16 位參與運算並只替換目的暫存器低 16 位。先讀進入時 CF，計算 16 位結果，更新六個算術旗標，保留 IF 等其他旗標；不改堆疊或記憶體。沿用已驗收 `19 /r` 的目的／來源方向與 `sub16` 旗標邏輯；`1B` 的 16 位形狀、其他前綴及記憶體形狀仍失敗即關閉。這是通用 CPU 工具補強，不直接修改 remake 的資料、規則、畫面或存檔。

合成測試覆蓋原版 CF=0 的 AX 自減、CF=1 的借位與旗標、不同暫存器的方向、高 16 位不變、未支援形狀拒絕；固定真檔的有界診斷須越過第 2460 步並記錄下一停點。`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與有固定原檔輸入的 `go test -buildvcs=false ./... -count=1` 均須通過。驗收只可聲明此通用指令形狀；不得外推 MOO2 正常玩家路徑或同狀態玩法對拍。

## 證據審查

原版連續兩行 `LOG` 證實指令位置、助憶碼、AX 結果與旗標；dosgolem 固定原檔診斷的 bytes 與 3 位元組長度一致。Intel 手冊明定 `19 /r` 的 16 位方向、CF 輸入及受影響旗標；現有 `sub16` 可處理一般減法旗標，CF／AF／OF 必須依未加進位的來源與完整借位另行核對。原版 CF=0 樣本不足以驗證 CF=1，因此驗收測試必須包含 CF=1 與不同暫存器。範圍足以核准上述通用 CPU 切片為 READY，不核准服務、玩家玩法或其他 `SBB` 形式。

## 驗收結果與界線

`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與固定 1.31 原檔作輸入的 `go test -buildvcs=false ./... -count=1` 全通過。合成測試核對 CF=0 原版樣本、CF=1 借位、16 位溢位、不同暫存器方向、高 16 位保留及未支援形狀拒絕。已綁定 DPMI 的合成 PSP／環境診斷越過第 2460 步與 `0x153E84`，至第 2475 步、dosgolem **重定位 LE 線性位址** `0x1005B` 的 `C8 AC 00 00` 停於未支援 opcode。跨越 `0x13EF5C → 0x10018 → 0x10057` 後進入此位址；下一停點是否與原版同一路徑仍待獨立核對。此規格僅在已列原版指令樣本及合成測試範圍內標為 CONFORMED；沒有正常玩家路徑或 remake 玩法對拍。

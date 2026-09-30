# 208 — 保護模式 SBB r/m32,r32 的暫存器形狀

狀態：**CONFORMED**（僅限所列通用 CPU 指令形狀）
日期：2026-09-30

## 證據與適用範圍

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；以內嵌 MZ `0x26654` 載入後，dosgolem 重定位 LE 線性位址 `0x1519EF` 的原始 bytes 是 `19 C0 40 74`。正確綁定 `NewMOO2StartupDOS` 與 `LEMachine` 的合成環境診斷至 `19 C0` 失敗即關閉；先前未綁定 DPMI 時第 554 步的 `AH=4Ah` 是錯誤分支，不能作此規格來源。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在其 **CS:EIP** `0180:003759EF` 的固定原檔進入值為 `EAX=00000501`、`EFLAGS=0246`（CF=0）；`0180:003759F1` 為 `EAX=00000000`、`EFLAGS=0246`。其餘寄存器不變。由版控 [`startup_probe_131.py --sbb`](../../apps/moo2/tools/startup_probe_131.py) 重生的私有 `workplace/dosbox-moo2-dpmi/sbb-registers.json` SHA-256 是 `93082156064ceb921571b445e19b7646b27e809133bd3186c35e2808b27b297b`；先前暫態探針的相同前後數值留在 SHA-256 `15f26e97d95998a85b788349e37126d31e339489b8a43d6bb0b756d0f771aee3` 的歷史輸出。指令位置亦見有界 4,096 指令 `LOGC`，但這都不是正式 dosgolem 玩家對拍。

[Intel® 64 and IA-32 架構軟體開發手冊第 2 卷](https://cdrdv2-public.intel.com/774492/325383-sdm-vol-2abcd.pdf) 的 `SBB` 條目定義 `19 /r` 為 `SBB r/m32,r32`：目的運算元減去來源及進入時 CF，更新 CF、PF、AF、ZF、SF、OF。`19 C0` 的 ModRM `C0` 是 `mod=11, reg=EAX, r/m=EAX`，故此原版樣本是 `SBB EAX,EAX`；CF=0 時結果為 0。

## 擬議實作

只支援無前綴的 `19 /r`、`mod=11`、32 位暫存器目的與來源；目的由 ModRM `r/m` 指定，來源由 `reg` 指定。進入時先讀 CF，依 `dest - src - CF` 寫回目的及六個算術旗標，保留 IF 與未受影響的旗標。沿用同檔已有 `1B /r` 的 32 位 SBB 旗標方法，但不可改變它的方向：`1B` 的目的在 `reg`，`19` 的目的在 `r/m`。16 位前綴、段覆寫、repeat 與記憶體形狀仍失敗即關閉，不能默許成功。

## 驗收

- 合成 CPU 測試核對 `19 C0`、CF=0 的零結果及旗標、CF=1 的 `FFFFFFFFh` 與借位旗標、不同暫存器的方向、未受影響旗標與 EIP 增量。
- 未支援的前綴與記憶體形狀拒絕且不改目的暫存器或旗標；原 `1B /r` 測試保持通過。
- 固定真檔在已綁定 DPMI 的診斷中越過 dosgolem `0x1519EF`，記錄下一個停點；前述合成環境與 PSP 限制保持明示。
- `go test ./internal/cpu386 ./internal/machine` 與 `go test ./...` 通過。

## 證據審查

固定雜湊原檔在 DOSBox-X 兩側斷點的指令位置與暫存器已核對，且與 Intel 指令編碼的目的／來源方向一致；相同原檔在修正 DPMI 綁定後也確實以 `19 C0` 失敗即關閉。原版只提供 CF=0 的一個樣本，因此 CF=1 與其他暫存器組合依 Intel 規格和獨立合成測試驗證，不冒稱原版逐組合實測。未綁定 DPMI 時的 `AH=4Ah` 不再列為此路徑的服務需求。上述範圍足以核准這個通用暫存器指令切片；不核准 MOO2 玩家玩法或完整 DOS 服務。

## 驗收結果

`DOSGOLEM_MOO2_EXE=/input/ORION2-1.31.EXE go test ./...` 全套通過，包含固定 1.31 原檔的 DPMI `AX=0006h` 回歸測試。已綁定 DPMI 的隔離診斷越過 `19 C0`，在 dosgolem LE 線性位址 `0x15171D`、原始 bytes `87 FA` 停於未支援的暫存器交換形狀；DOSBox-X 4,096 指令輔助位址記錄也到達對應 `0180:0037571D`。這僅驗證通用 CPU 切片與下一個工具缺口，原版環境／PSP 仍是合成近似，沒有正常玩家路徑或正式對拍。

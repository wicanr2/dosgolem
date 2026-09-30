# 209 — 保護模式 XCHG 暫存器與暫存器

狀態：**CONFORMED**（僅通用 CPU 指令形狀）
日期：2026-10-01

## 證據與邊界

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，在 dosgolem LE 線性位址 `0x15171D` 的原始 bytes 為 `87 FA`，現有 `decodeAddress32` 以「memory operand不可為register」失敗即關閉。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 `fd2-dosbox-x:debug-0d7b272b`（ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`）在其 CS:EIP `0180:0037571D` 與 `0180:0037571F` 命中前後斷點。原版前後 `EDX=EDI=003EC028`，`EFLAGS=0216` 未變；此樣本能證明越過指令與旗標不變，**不能單憑原版樣本證明交換方向**。私有 `workplace/dosbox-moo2-dpmi/xchg-registers.json` SHA-256 `69deda4f6e766c9ffba973bea26dcade18e6f6b0b5b6ff5a780036bf6c6fb18d`，由版控 `apps/moo2/tools/startup_probe_131.py --xchg` 重生。

[Intel® 64 and IA-32 架構軟體開發手冊第 2D 卷](https://cdrdv2-public.intel.com/789589/334569-sdm-vol-2d.pdf) 的 `XCHG` 條目定義 `87 /r` 可交換 `r/m32` 與 `r32`，不影響旗標。`FA` 的 ModRM 為 `mod=11, reg=EDI, r/m=EDX`，故此處是 `XCHG EDX,EDI`。證據等級：指令語意已由 Intel 規格證實；原版兩側寄存器值與位址已證實；MOO2 完整啟動及玩家路徑仍未知。

## 擬議實作與驗收

在既有 `87 /r` 分支的位址解碼前，對 `mod=11` 直接交換兩個 32 位暫存器，保持所有旗標與其他暫存器；相同暫存器視為原值。保留既有記憶體形式。16 位前綴及其他尚未支援的前綴仍失敗即關閉，不擴大範圍。

以不同暫存器值的合成測試核對交換、旗標、EIP、記憶體不變；以固定原檔綁定 DPMI 的診斷越過 `87 FA` 並記錄下個停點；執行 `go test ./...`。原版執行器此處只有輔助斷點收據，正式對拍仍需 dosgolem 重生。

## 證據審查

原始 bytes、兩個 DOSBox-X CS:EIP 斷點、手冊的 `87 /r` 編碼及 ModRM 欄位相互吻合。原版兩側 EDX 與 EDI 相等的侷限已明列，交換方向由公開 CPU 契約與不同值的獨立測試驗證。既有記憶體形式不受此切片影響，未支援前綴維持拒絕。審查核准上述通用 CPU 切片為 **READY**，不核准 MOO2 玩法或完整原版對拍。

## 驗收結果

`go test ./internal/cpu386 ./internal/machine -count=1` 與固定原檔輸入的 `go test ./... -count=1` 通過。不同值測試驗證 `87 FA` 交換 EDX／EDI，EIP 加 2、`EFLAGS=0216h` 與指令記憶體不變；`66 87 FA` 拒絕且不改寄存器／旗標。已綁定 DPMI 的固定 1.31 原檔合成診斷越過原停點第 673 步，現於第 760 步、dosgolem 重定位 LE 線性位址 `0x1515CA` 的 opcode `F5` 失敗即關閉。此仍為合成環境中的 CPU 診斷，**不是**原版正常玩家路徑或玩法對拍。

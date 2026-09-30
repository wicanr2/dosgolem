# 207 — 16 位暫存器與符號延伸立即數 AND

狀態：**CONFORMED**（限通用 CPU 指令形狀；不代表 MOO2 玩家路徑對拍）
日期：2026-09-30

固定 MOO2 1.31 原版 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，經 [201](201-moo2-dos4g-startup-returns.md) 的兩次啟動服務回傳與合成 PSP／環境診斷，在第 419 步、dosgolem 重定位 LE 線性位址 `0x13CBAA` 停於 `66 83 E7 FC`。`66` 選 16 位運算元，`83 E7 FC` 是 `AND DI, sign_extend8(0xFC)`，即以 `0xFFFC` 清除 DI 低兩位。此停點只定位通用 CPU 缺口，不證明正常玩家路徑。

實作只擴充 `83 /4` 且 `mod=11`、operand-size 為 16 位、無段覆寫及 repeat 前綴的形狀。來源立即數由 signed byte 延伸到 word；結果只覆寫目的暫存器低 16 位，高 16 位保留。沿用既有 `setLogicFlags16`：依結果設定 SF／ZF／PF，清 CF／OF；AF 依現有邏輯指令慣例清除，IF 等無關旗標保留。EIP 應跨過四 bytes。未知 `83` 形狀維持失敗即關閉。

驗收用合成 CPU 測試核對 `0xFC` 的符號延伸、目的高 word 保留、零值與非零旗標、無關暫存器不變，以及非法前綴仍拒絕；再跑 `go test ./internal/cpu386 ./internal/machine` 與固定 1.31 真檔有界診斷。後續步數僅作工具缺口定位。

驗收結果：`go test ./internal/cpu386 ./internal/machine` 通過。固定 1.31 真檔在合成 PSP／環境診斷中越過第 419 步，至第 554 步、dosgolem LE 線性位址 `0x15E07C` 停於 `CD 21`，進入時 `EAX=4A88h`、`EBX=1CF00h`、`ES=DS=SS=0188h`。這是尚未支援的 DOS `AH=4Ah` 服務，不屬本 CPU 規格的完成範圍。該服務的原版返回值未經獨立核對，不能以合成猜值繼續推進。

### 同日勘誤：第 554 步不是現行原版服務閘門

上述第 554 步只描述當時**未呼叫 `services.AttachMachine(m)`** 的私有診斷探針。固定 1.31 DOSBox-X 輔助快照在 `0180:00334072` 證實 `INT 31h/AX=0006h` 返回 `CX:DX=0`、CF 清除；未綁定的 dosgolem DPMI 主機在對應 `0x110070` 返回錯誤，使 `0x110079` 走另一條分支。補上綁定後，固定原檔在 `0x110079` 的 ZF 分支與原版吻合，`AH=4Ah` 停點消失；原版環境字串較長使迴圈次數不同，但掃描後的已觀測指令順序重新對齊。現行診斷停點由 [208](208-cpu386-sbb-rm32-register.md) 更新；第 554 步保留作錯誤形成史，不得再列成待取得原版返回的工作閘門。

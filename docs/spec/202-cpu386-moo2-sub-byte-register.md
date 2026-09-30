# 202 — MOO2 啟動鏈的 8 位暫存器 SUB

狀態：**CONFORMED**（只限 `28 /r`、`mod=11`）
日期：2026-09-30

固定 MOO2 1.31 原版 SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，以 [197](197-bound-mz-le-file-base.md) 載入，並以 [201](201-moo2-dos4g-startup-returns.md) 核對的兩次啟動服務返回作**有合成環境限制的診斷**。第 65 步、dosgolem LE 線性位址 `0x1100FA` 的 `28 C0` 是 `SUB AL,AL`；目的 byte 歸零，CF／OF／SF 清除，ZF／PF 設定，高位 EAX 不變。這是通用 386 指令，不是 MOO2 特有規則。

只新增無前綴的 `28 /r`、`mod=11` 形狀，目的為 r/m8、來源為 r8；使用既有 `sub8` 旗標實作，memory 形狀仍失敗即關閉。合成測試驗證 `SUB AL,AL` 的零值／高位保存、`SUB AL,BL` 的方向與借位／輔助進位／同位性／符號旗標、IF 保存及未支援 memory 形狀拒絕。`go test ./internal/cpu386 ./internal/machine` 通過。

固定 1.31 真檔隔離診斷越過 `28 C0`，在第 72 步、dosgolem LE 線性位址 `0x110102` 停於 `26 66 8E 1D 29 CA 17 00` 的未支援 16 位 ES 覆寫段載入。此步數只能定位下個通用 CPU 缺口；PSP／環境仍是合成輸入，沒有玩家畫面或玩法同狀態對拍。

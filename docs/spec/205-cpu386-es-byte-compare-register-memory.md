# 205 — ES 覆寫的 byte 暫存器／記憶體比較

狀態：**CONFORMED**（限 `26 3A /r` 的記憶體來源）
日期：2026-09-30

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。依 [201](201-moo2-dos4g-startup-returns.md) 的兩次啟動返回，以合成 PSP／環境診斷至第 393 步、dosgolem LE 線性位址 `0x163A90`，停於 `26 3A 10`。`3A /r` 是 `CMP r8,r/m8`；此處目的 reg 欄位 2 為 DL，記憶體來源為 `ES:[EAX]`。只更新減法旗標，DL、記憶體、EIP 之外暫存器均不改。

既有 `3A` 有通用 byte 比較與 `sub8` 旗標語意。新增只接受 ES 覆寫的記憶體來源，依 `decodeAddress32` 算 offset，但用 ES descriptor 讀取；來源越界應拒絕，且未完成比較不修改旗標。其他 segment 覆寫及未知前綴仍失敗即關閉。合成測試需讓 DS／ES 指向不同 bytes，驗證方向、旗標、來源界限與不變的操作數；再執行 `go test ./internal/cpu386 ./internal/machine`、固定真檔有界診斷。下一停點只作工具缺口定位，不是玩家對拍。

驗收結果：合成測試、`go test ./internal/cpu386 ./internal/machine` 通過；固定 1.31 真檔越過第 393 步，於第 406 步、dosgolem LE 線性位址 `0x13CB80` 停於 `0F A8`。此處仍無玩家可見對拍收據。

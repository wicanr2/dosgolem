# 204 — 暫存器立即數 MOV 前的 DS 前綴

狀態：**CONFORMED**（限 `3E B8..BF` 的暫存器立即數形狀）
日期：2026-09-30

固定 SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f` 的 MOO2 1.31 原版，在 [203](203-cpu386-es-word-segment-load.md) 之後，於第 125 步、dosgolem LE 線性位址 `0x11014C` 停於 `3E B9 50 21 1C 00`。`3E` 是 DS 段覆寫；`B9` 是 `MOV ECX,0x001C2150`，沒有記憶體運算元，所以段覆寫不改立即數、ECX 以外的暫存器或旗標。此停點來自 [201](201-moo2-dos4g-startup-returns.md) 的固定兩次服務返回與仍具合成環境限制的診斷。

既有 `3E BA` 已按相同理由支援。將此規則限定為 `3E B8..BF` 的暫存器立即數 `MOV`，保留 `66` 的 operand-size 語意；其他尚未支援的 DS 覆寫 opcode 仍失敗即關閉。驗收至少測 `3E B9` 的目的暫存器、立即數、EIP、EFLAGS 與非目的暫存器不變，並重跑 `go test ./internal/cpu386 ./internal/machine` 及固定真檔有界診斷。後續步數僅作工具缺口定位。

驗收結果：`3E B9` 的 32／16 位立即數與旗標測試、`go test ./internal/cpu386 ./internal/machine` 均通過；固定 1.31 原檔在合成環境診斷中前進至第 393 步、dosgolem LE 線性位址 `0x163A90`，停在 `26 3A 10` 的 ES 覆寫 byte 比較。未取得玩家路徑收據。

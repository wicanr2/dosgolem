# 203 — MOO2 啟動鏈的 ES 覆寫段載入

狀態：**CONFORMED**（限 `26 66 8E 1D` 的絕對位址形狀）
日期：2026-09-30

固定 MOO2 1.31 `ORION2.EXE` 的 SHA-256 為 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。依 [197](197-bound-mz-le-file-base.md) 載入、以 [201](201-moo2-dos4g-startup-returns.md) 固定兩次啟動服務回傳後，隔離診斷在第 72 步、dosgolem LE 線性位址 `0x110102` 停於 `26 66 8E 1D 29 CA 17 00`。這是 `MOV DS,word ptr ES:[0x0017CA29]`；位址為 **dosgolem LE 線性空間**，不能當 DOSBox-X 的 `CS:EIP`。此診斷仍使用合成 PSP／環境，僅定位通用 CPU 缺口。

通用 CPU 契約：`26` 選 ES 而非預設 DS 取記憶體；`66` 不改變 segment selector 的 16 位讀取寬度；`8E /r` 的 `mod=00,rm=101` 取後續 32 位絕對位移。只在來源兩 bytes 可讀且 selector 可載入 DS 時，更新 DS；失敗時不改 DS。EFLAGS 不變。既有 `8E` 絕對記憶體與 selector 驗證可沿用；不因本切片放行其他未知 ModRM 或前綴組合。

驗收以合成 descriptor 測試來源段覆寫、成功載入、越界與無效 selector 拒絕；再跑 `go test ./internal/cpu386 ./internal/machine` 及固定真檔有界診斷。真檔後續步數僅能作下一缺口定位，不能宣稱玩家路徑對拍。

驗收結果：合成測試與 `go test ./internal/cpu386 ./internal/machine` 通過；固定 1.31 原檔在合成環境診斷中越過第 72 步，於第 125 步、dosgolem LE 線性位址 `0x11014C` 停在 `3E B9 50 21 1C 00`。這仍是工具診斷，無玩家畫面或玩法收據。

# 200 — MOO2 暫定保護模式 DOS 服務入口

狀態：**CONFORMED**（僅限明示的合成環境與暫定服務入口；原版服務回傳未知）  
日期：2026-09-30

## 證據與邊界

固定 SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f` 的 MOO2 1.31 DOS 原版，和 SHA-256 `7ae2ac2e5904ca330009af2827279d889906b0b9b7a8854c38eb707a56e955b5` 的 1996 版，經 [197](197-bound-mz-le-file-base.md) 的內嵌 MZ 載入後，在 dosgolem LE 線性位址 `0x10FFB5` 均自然要求 `INT 21h/AH=30h`，`EBX=0x50484152`。先前借 FD2 服務的**診斷**另觀察到第二次服務為 `AX=FF00h`、`DX=0078h`。要求的指令 bytes 和順序已證實；服務回傳值、selector 編號、PSP 與環境內容在 MOO2 上尚未獨立實測。

本切片提供獨立的 `NewMOO2StartupDOS` 入口，使用明示的合成 `ORION2.EXE` 最小環境；DOS/4G 兩次握手及其 selector／回傳值暫沿用 FD2 已測過的受限實作。這屬**平台近似**，不宣稱原版 MOO2 真實 DOS/4GW 配置，也不能作畫面、規則或正常玩家路徑對拍收據。FD2 零值及原建構器行為不得變更；未知 DOS 呼叫必須失敗即關閉。正式對拍必須有可核對的 MOO2 原版服務回傳證據或可比較的玩家狀態，並在收據明列所有近似。

## 驗收

- 合成測試核對 MOO2 最小環境 bytes 與範圍、前兩次握手順序，並保持 FD2 舊測試通過。
- `go test ./internal/machine ./internal/cpu386`、`go test ./...` 通過。
- 使用本入口重跑兩版原檔的**隔離診斷**，記錄精確停點；不得把診斷步數登錄為正式 parity。

驗收收據：`go test ./internal/machine ./internal/cpu386` 通過；兩版固定雜湊原檔都經 `NewMOO2StartupDOS(nil)` 的診斷入口走過兩次呼叫，於第 65 步、dosgolem LE 線性位址 `0x1100FA` 停在 `28 C0`。本收據只驗證合成環境切換與已列近似服務的可執行性，不驗證原版回傳值或玩家路徑。

# 242 — 第二組 DMA 控制器單通道遮罩

狀態：**CONFORMED（平台硬體規格近似；僅限單通道遮罩）**
日期：2026-10-01
範圍：MOO2 原檔初始化時的 `OUT 00D4h,05h`，只保存第二組 DMA 控制器的單通道遮罩狀態。不執行 16 位元 DMA 傳輸、不模擬音訊時鐘或逐波形輸出。

## 證據與分級

- **已證實，原檔在 dosgolem 的停點**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`，內含 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。明示高位 LE 載入、合成 PSP／環境的 dosgolem 自生執行至第 1,166,995 步，`DPMI 0300h → INT 66h` 的實模式第 299 步在 `1201:0220` 發出 `OUT 00D4h,05h`，因未知埠拒絕。此 `1201:0220` 是 **dosgolem 實模式 segment:offset**，不是 IDA 或 DOSBox-X 線性位址。私有收據 `workplace/moo2-probe-241-full-game.txt` SHA-256 `7338276b9565381915d397c6bdebdfc3d26aafa11e56b22e4396a63744f0faf9`；工具版本是隔離工作樹 `fc12492add999c5eb9713014b16151e28735b2b1` 加當輪未提交的 240／241 規格實作。
- **已證實，平台埠映射與遮罩語意**：[IBM PC AT 技術參考手冊](https://www.minuszerodegrees.net/manuals/IBM_5170_Technical_Reference_1502243_MAR84.pdf)把 `C0h..DFh` 分配給第二組 DMA 控制器；[Intel 8237A 資料表](https://www.pcjs.org/documents/datasheets/intel/INTEL_8237A_DMA.pdf)定義單通道遮罩命令的低兩位選通道、bit 2 表示設置或清除遮罩。[DOSBox-X `dma.cpp`](https://github.com/joncampbell123/dosbox-x/blob/master/src/hardware/dma.cpp)把 `C0h..DFh` 的偶數埠轉為 `(port-C0h)>>1`，並在索引 `0Ah` 使用上述 bit。故 `D4h` 是第二控制器的索引 `0Ah`，`05h` 選相對通道 1 並設遮罩，對應 PC 全域 DMA 通道 5。此推導是 **hardware-spec approximation**，不是原版同狀態播放收據。
- **未知**：MOO2 在這段初始化後是否真的使用通道 5 傳輸，以及原版音效完成時間。兩者不由這筆 `OUT` 推定；不深入 DAC／PIT／DMA 的實機時序。

## 契約與 READY 審查

在既有 `LEOPLPorts` 中另存第二控制器四個相對通道的遮罩位元，初值四通道皆遮罩。`Out8(D4h,v)` 以 `v&3` 選相對通道，以 `v&4` 設置／清除遮罩；高位依 8237A 命令形狀忽略。輸出事件照既有埠日誌記錄；第一控制器的 `0Ah` 遮罩、DSP、PIC 狀態不得改動。`D4h` 目前只接受寫入，讀取或其他第二控制器埠仍失敗即關閉。狀態快照需可檢查第二控制器遮罩。此契約只影響 dosgolem 平台，無 MOO2 remake 玩法資料、UI 或存檔變更。

## 驗收

1. 合成測試覆蓋初值、`05h` 遮罩相對通道 1、`01h` 清除、其他通道獨立、第一控制器不變、未知讀取／其他埠拒絕，並驗證保護模式與實模式共用實例。
2. 含官方 1.31 EXE 的全套 Go 測試通過，正版資料自 LE 入口自行越過 `D4h`，記錄下一個自然停點。
3. 驗收範圍只限埠狀態與自生執行前進；不宣稱音訊、原版正常玩家畫面或玩法同狀態。

## 實作與驗收

- `internal/machine/le_opl_ports.go` 增加獨立第二控制器遮罩及唯值快照；只接受 `D4h` 寫入。`internal/machine/le_opl_ports_test.go` 覆蓋初值、設置／清除、高位、通道隔離、第一控制器不變及未知埠拒絕；`internal/machine/le_startup_test.go` 覆蓋 MOO2 的雙模式共用接線。
- 固定官方 EXE 的 `go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-242.txt` SHA-256 `63ce81a460661c68c24979ee50e1e3d3720c8c7f9fd309c627f0fd1558e3daf7`。
- 同一正版 ZIP 根層 417 檔、官方 EXE 及明示高位 LE 載入，自原檔入口重生第 1,166,995 步的 `DPMI 0300h → INT 66h`；實模式越過第 299 步 `OUT D4h,05h`，於第 305 步、**dosgolem 實模式位址** `1201:0244` 的 `OUT 00D8h,00h` 拒絕。私有 `workplace/moo2-probe-242-full-game.txt` SHA-256 `142f0d05f0feea086353bd9cfbd5679f0dda095b9644da82cd8007376c187c06`。這只證明平台服務路徑前進；第二控制器其他暫存器及玩家畫面仍未驗收。

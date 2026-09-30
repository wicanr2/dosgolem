# 206 — 32 位堆疊的 PUSH GS

狀態：**CONFORMED**（通用 CPU 指令與固定原版診斷驗收；不代表 MOO2 玩家路徑對拍）
日期：2026-09-30

固定 MOO2 1.31 原版 SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，依 [201](201-moo2-dos4g-startup-returns.md) 的兩次啟動回傳與仍有合成環境限制的診斷，在第 406 步、dosgolem LE 線性位址 `0x13CB80` 停於 `0F A8`（`PUSH GS`）。它是通用 386 指令，使用目前 32 位堆疊：先確認 ESP 可減 4，再將零延伸的 GS selector 以 32 位寫入 `SS:[ESP-4]`，成功後更新 ESP；失敗時堆疊與 ESP 不變，EFLAGS 不變。

沿用既有 `0F A0`（`PUSH FS`）的堆疊寫入與描述子界限處理，只新增無 operand-size／segment／repeat 前綴的 `0F A8`。合成測試核對寫入寬度、selector、高位清零、ESP、旗標、下溢與描述子越界，再跑 `go test ./internal/cpu386 ./internal/machine` 及固定真檔有界診斷。後續步數只用於定位工具缺口，不能取代正常玩家路徑對拍。

驗收：`go test ./internal/cpu386 ./internal/machine` 通過。固定 1.31 真檔診斷跨過第 406 步，至第 419 步、dosgolem LE 線性位址 `0x13CBAA` 停於 `66 83 E7 FC` 的未支援 16 位 `83` 形狀。原版啟動返回與 PSP／環境仍包含合成條件，因此此結果僅驗收所列 CPU 能力。

# 152 — CPU386 short JLE

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 104`](../re/104-fd2-bounded-line-reader-length-gate.md)

- 支援 opcode `7Eh` 的 short `JLE rel8`。
- 將 rel8 視為有號位移；當 `ZF=1` 或 `SF != OF` 時，以指令結尾 EIP 為基準
  加上位移，否則順序前進；不修改任何旗標。
- operand-size、segment 與 repeat prefix 維持失敗即關閉。

驗收：CPU 單元測試分別覆蓋 ZF、SF≠OF、未跳轉及負位移；固定原版自然執行
`0x46C6E`，目前正長度路徑不得跳轉並抵達 `0x46C70`。

本規格不宣告 `fgetc` 或 DOS read 已完成。

驗收收據（2026-09-06）：`TestShortJumpLessOrEqual` 覆蓋 ZF、SF≠OF、
未跳轉、負位移與旗標不變；`TestFD2EntersBoundedLineReadLoop` 由固定原版
LE entry 自然執行 `0x46C6E`，目前正長度路徑抵達 `0x46C70`。後續有界
探針進入 `fgetc`，下一阻塞移至 `0x3D9E4` 的 `FF 4B ...`。

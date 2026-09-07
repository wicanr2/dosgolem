# 154 — CPU386 short JGE

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 105`](../re/105-fd2-fgetc-buffer-count.md)

- 支援 opcode `7Dh` 的 short `JGE rel8`。
- rel8 有號延伸；當 `SF == OF` 時由指令結尾 EIP 加上位移，否則順序前進；
  不修改旗標。
- operand-size、segment、repeat prefix 維持失敗即關閉。

驗收：CPU 單元測試覆蓋 SF／OF 四種組合與負位移；固定原版自然執行
`0x3D9EB`，目前負緩衝計數不得跳轉，並抵達 `0x3D9ED` 的 `push ebx`。

本規格不宣告 `__filbuf` 或 DOS read 已完成。

驗收收據（2026-09-06）：`TestShortJumpGreaterOrEqual` 覆蓋 SF／OF 四種
組合、負位移與旗標不變；`TestFD2EntersFilbufForEmptyBuffer` 由固定原版
LE entry 自然執行 `0x3D9EB`，負緩衝計數不跳轉並抵達 `0x3D9ED`。
後續有界探針進入 `__filbuf`，下一阻塞移至 `0x3D97D` 的 `80 4B ...`。

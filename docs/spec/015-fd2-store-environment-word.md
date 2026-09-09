# 015 — FD2 保存環境 word

狀態：**CONFORMED**
日期：2026-09-06
前置：[`014`](014-dos4gw-flat-descriptors.md)、[`RE 007`](../re/007-fd2-store-environment-word.md)

cpu386 的 opcode `89 /r` 新增 `operand-size=16`、`mod=0,r/m=5` 的窄形式：讀取
`disp32`，把 ModR/M `reg` 指定通用暫存器的低 16-bit 寫至平坦記憶體。EIP 前進
7 bytes，EFLAGS 不變；其他 16-bit addressing form 與 segment override 仍拒絕。

驗收須涵蓋合成記憶體、旗標不變及固定雜湊 FD2 的
`0x3CAC7 -> 0x3CACE`，並核對 `[0x52838]=0x0030`。

2026-09-06 上述單元測試與固定雜湊整合測試均通過；本切片已符合規格。

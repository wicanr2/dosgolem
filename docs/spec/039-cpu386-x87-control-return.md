# 039 — x87 FLDCW／FLDZ／WAIT 與 32-bit RET

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`038`](038-cpu386-x87-init-control.md)、[`RE 031`](../re/031-fd2-x87-callback-return.md)

- `80 /7` 新增 register-direct byte CMP，包含 AH 等高 byte register，不寫回。
- `9B` WAIT 在同步軟體模型中完成為無狀態 barrier；不掩蓋其他 x87 opcode。
- `D9 2D disp32` 從 DS descriptor 載入 16-bit control word；`D9 EE` 將 0 push
  至八層 x87 stack，overflow 失敗即關閉。
- `C3` 從 SS:ESP 讀 32-bit EIP，成功後 ESP+4；`66 C3` 尚未支援。
- 固定雜湊 FD2 從 `0x45E40` 返回 `0x45DD3`，FPUControl=`0x127F`、FPUDepth=4、
  ESP=`0x5569C`，並恢復 callback 保存的 EDX／EBX／ECX／ES。

驗收包含 AH compare、control load、兩次以上 FLDZ、WAIT、RET 與固定 callback 返回。

2026-09-06：上述單元測試與固定雜湊 x87 callback 完整返回測試通過，抵達 `0x45DD3`。

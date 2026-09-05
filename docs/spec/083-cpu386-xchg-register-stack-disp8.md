# 083 — 386 暫存器與 stack disp8 交換

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 058`](../re/058-fd2-watcom-stack-probe.md)

- 擴充無前綴 opcode `87`，只接受 `mod=1`、SIB `scale=0/index=none/base=ESP`
  的 `XCHG r32,[SS:ESP+disp8]`。
- 地址使用有符號 disp8；讀寫均必須通過 SS descriptor 邊界與寫入權限。
- 只有 stack dword 成功寫入後才更新暫存器；其他 32 位 ModRM/SIB 形狀與
  所有未列前綴失敗即關閉。
- 現有 `66 87 04 24` 的 16 位 x87 啟動路徑契約保持不變。
- 固定 FD2 在 `0x36CD7` 以 `87 44 24 04` 執行 `xchg eax,[esp+4]`。

驗收：單元測試覆蓋正 disp8 交換與越界不改暫存器；固定雜湊 FD2
必須由 LE entry 自然經過 `sub_36CD7` 的正常路徑。

驗收收據（2026-09-06）：`TestXchgRegisterStackDisp8` 與固定雜湊 stack probe
實驗通過。

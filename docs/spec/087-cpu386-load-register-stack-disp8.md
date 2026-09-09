# 087 — 386 由 stack disp8 載入暫存器

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 058`](../re/058-fd2-watcom-stack-probe.md)

- 擴充無前綴 opcode `8B`，只接受 `mod=1`、SIB
  `scale=0/index=none/base=ESP` 的 `MOV r32,[SS:ESP+disp8]`。
- 使用有符號 disp8 與 SS descriptor；讀取失敗不得修改目的暫存器。
- 其他 SIB 與前綴形狀失敗即關閉，現有 `8B` 形狀保持不變。
- 固定 FD2 在 `0x36CE0` 以 `8B 44 24 04` 將 caller 的 EAX 還原。

驗收：單元測試覆蓋載入與越界不改目的；固定雜湊 FD2 必須由 LE entry
自然經過 `0x36CE0`。

驗收收據（2026-09-06）：`TestLoadRegisterFromStackDisp8` 通過；固定雜湊
stack probe 實驗確認 EAX 復原為 `0x556A4`。

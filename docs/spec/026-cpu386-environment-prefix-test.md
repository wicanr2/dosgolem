# 026 — 386 environment prefix immediate ALU

狀態：**CONFORMED**
日期：2026-09-06
前置：[`025`](025-dos4gw-environment-block.md)、[`RE 018`](../re/018-fd2-environment-prefix-test.md)

- `0D id`：OR EAX,imm32，清除 CF／OF／AF，更新 PF／ZF／SF。
- `3D id`：CMP EAX,imm32，更新 subtraction flags，不修改 EAX；保留既有
  operand-size 16-bit 形式。
- `75 cb`：ZF=0 時採 signed rel8，否則落下；operand-size override 拒絕。
- 固定雜湊 FD2 從 `0x3CB14` 執行至 `0x3CB27`，核對 EAX=`0x20212020`、ZF=0。

2026-09-06 immediate ALU／branch 單元測試與固定雜湊 FD2 整合測試通過；
執行抵達一般 environment NUL 掃描入口 `0x3CB27`。

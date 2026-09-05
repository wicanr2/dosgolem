# 018 — 386 command-tail 前置算術

狀態：**CONFORMED**
日期：2026-09-06
前置：[`017`](017-dos4gw-selector-load-validation.md)、[`RE 010`](../re/010-fd2-command-tail-alignment.md)

cpu386 新增兩個 register-direct 窄形式：

- `83 /0 ib`：`ADD r32,sign_extend(imm8)`，更新 CF／PF／AF／ZF／SF／OF；
- `80 /4 ib`：`AND r8,imm8`，清除 CF／OF／AF並更新 PF／ZF／SF。

其他 ModR/M、operand-size override 或 group extension 維持失敗即關閉。固定雜湊 FD2
須從 `0x3CADA` 執行至 `0x3CAE2`，核對 `EDX=0x546B0`、`ECX=0`，並停在尚未
實作的 ES-relative byte read。

2026-09-06 全套 dosgolem 測試與固定雜湊 FD2 整合測試通過；目前停止點為
`0x3CAE2`，PSP byte read 尚未接入。

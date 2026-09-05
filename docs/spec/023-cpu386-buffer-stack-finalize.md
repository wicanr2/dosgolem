# 023 — 386 buffer／stack 收尾指令群

狀態：**CONFORMED**
日期：2026-09-06
前置：[`022`](022-dos4gw-selector-swap-jz.md)、[`RE 015`](../re/015-fd2-command-tail-buffer-finalize.md)

本切片新增：register-direct `2A /r` byte subtraction、`STOSB`、32-bit register
`POP`、32-bit register `DEC`。STOSB 必須透過 ES descriptor；POP 必須透過 SS
descriptor，失敗時不得提交 register／ESP 狀態。DEC 更新 arithmetic flags 但保留 CF。
operand-size override 與未列 ModR/M 形狀皆拒絕。

固定雜湊 FD2 須執行至 `0x3CB09`，核對 `ESI=0`、`EDI=0x546B1`、
`ESP=0x556A8`、兩個 buffer zero bytes，以及 stack top 依序為 `0x160`、`0x546B1`。

2026-09-06 cpu386 與固定雜湊 FD2 整合測試通過；執行抵達 environment selector
載入點 `0x3CB09`。

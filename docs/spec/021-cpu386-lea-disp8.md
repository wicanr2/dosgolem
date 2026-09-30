# 021 — 386 LEA register 加 signed disp8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`020`](020-cpu386-repe-scasb.md)、[`RE 013`](../re/013-fd2-command-tail-pointers.md)

cpu386 的 `8D /r` 新增 `mod=1`、無 SIB 的 register base 加 signed disp8 形式，
把 32-bit effective address 寫入目的 register。不得讀 Bus，EFLAGS 不變；
operand-size override、segment override、SIB 與其他 ModR/M 形狀維持失敗即關閉。

固定雜湊 FD2 從 `0x3CAEB` 執行至 `0x3CAF2`，同時重用既有 `8B`／`8C`
register-direct 支援，核對 `ESI=0x80`、`EDI=0x546B0`、`EBX=0x28`。

2026-09-06 cpu386 單元測試與固定雜湊 FD2 整合測試通過；執行抵達
`0x3CAF2`。

後續範圍擴充（2026-10-01）：本規格當時的「segment override 維持失敗即關閉」只描述原有支援形狀；[224-cpu386-lea-segment-prefix](224-cpu386-lea-segment-prefix.md) 依固定 MOO2 原檔的 CS 前綴樣本與 Intel 契約，另行開放 `LEA` 單一段前綴並忽略其段基址。本規格的無前綴 disp8 收據與原始證據仍成立。

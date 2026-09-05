# 029 — 386 byte register move 與 REP STOSD

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`014`](014-dos4gw-flat-descriptors.md)、[`RE 021`](../re/021-fd2-startup-buffer-clear.md)

- `8A /r` 新增 register-direct `MOV r8,r8`；不改旗標。既有 ES memory form保留。
- `AB` 在 32-bit operand size 將 EAX 寫入 ES:`[EDI]`，依 DF 將 EDI 加／減 4。
- `F3 AB` 重複上述動作 ECX 次，每次遞減 ECX；ECX 初值為 0 時不得讀寫。
- 每次寫入都必須通過 ES descriptor range／writable 驗證；越界立即失敗即關閉。
- `66 AB` 與 segment override 維持不支援。
- 固定雜湊 FD2 從 `0x3CB3C` 抵達 `0x3CB6C`，核對 EAX=0、ECX=0、
  EDX=`0x1C4`、EBX=`0x556B0`、ESI=`0x546B1`、EDI=`0x546B0`、ESP=`0x556B0`。
- `cmd/leprobe -execute-entry-prefix` 的公開 checkpoint 同步推進到 `0x3CB6C`，並明確
  回報 environment path 已搬移、startup buffer 已清除；總體執行能力標為 partial，
  不得保留舊 `0x3CAB3` 或 `execution_supported=false` 宣稱。

驗收包含 byte 子暫存器搬移、兩次非零 dword 寫入與索引／count 更新、固定雜湊
FD2 啟動整合。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x3CB6C`。

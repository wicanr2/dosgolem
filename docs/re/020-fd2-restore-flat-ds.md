# 020 — FD2 恢復 flat DS selector

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw byte、dosgolem 堆疊狀態）

沿用 `019` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x3CB3B  1F   pop ds
```

進入此指令前，SS:ESP=`0x160:0x556A8` 的 32-bit stack cell 為 `0x00000160`；DS
仍是 environment selector `0x30`。執行後 DS 恢復為已登錄的 flat selector
`0x160`，ESP 前進至 `0x556AC`，EIP 抵達 `0x3CB3C`。

這是 DOS/4GW 啟動路徑的 selector／堆疊行為，不建立 FD2 遊戲層語意。

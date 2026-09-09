# 029 — FD2 第一個 startup callback thunk

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、線性位址計算、dosgolem EIP receipt）

沿用 `028` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x3CBCC  E9 65920000   jmp rel32 +0x9265
0x3CBD1                next EIP basis
0x45E36                calculated target
```

選中 record 的 pointer `0x3CBCC` 是 near-jump thunk。執行後 EIP=`0x45E36`，ESP
保持 `0x55698`，因此 callback 的真正入口應由 `0x45E36` 繼續分析；不可把 thunk
後相鄰 bytes 當成順序執行路徑。

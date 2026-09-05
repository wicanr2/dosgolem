# 028 — FD2 callback 前的 segment transfer 與間接 CALL

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、selector／stack／EIP receipt）

沿用 `027` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x45DCF  1E      push ds
0x45DD0  07      pop es
0x45DD1  FF D0   call eax
0x45DD3          return address
0x3CBCC          callback target from selected record
```

進入時 DS=ES=`0x160`、EAX=`0x3CBCC`、ESP=`0x5569C`。segment stack transfer 後
ES 保持已驗證的 flat selector；間接 CALL 將 `0x45DD3` 寫至 SS:`0x55698`，EIP
進入 `0x3CBCC`。這證實 record pointer 的呼叫消費端，未判讀 callback 內部語意。

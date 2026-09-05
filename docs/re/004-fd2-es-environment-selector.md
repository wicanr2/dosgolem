# FD2 `ES:[2Ch]` 啟動讀取

日期：2026-09-05  
證據等級：**已證實**（固定雜湊、IDA 9.4、DOSBox-X normal core 同狀態執行）

## 直接控制流

沿用 [`003`](003-fd2-segment-bootstrap.md) 的輸入與位址空間。固定檔指令：

| 線性位址 | bytes | 原始指令 |
|---|---|---|
| `0x3CA92` | `66 26 8B 0D 2C 00 00 00` | `mov cx,es:[2Ch]` |
| `0x3CA9A` | `EB 17` | `jmp 0x3CAB3` |

## 同狀態執行結果

為避免 dynamic core 已知的單步問題，本次以同一 heavy-debug image 加
`-set "cpu core=normal"`，仍從 `AH=30h, EBX=50484152h` 的 FD2 caller 定位
`CS=0158`，再依 IDA 位址差設 `BP 0158:001F2AB3`。斷點命中：

```text
EAX=00000001 ESI=00000000 DS=0160 ES=0028 FS=0000 GS=0020 SS=0160
EBX=00000160 EDI=00000081 CS=0158 EIP=001F2AB3
ECX=00000030 EBP=00000000 EDX=00000078 ESP=0019E690
next: A2 32 28 05 00  mov [0x52832],al
```

所以固定環境的 `ES=0028h, offset=002Ch` word 值是 `0030h`。本證據只支援這個
啟動 oracle cell，不足以推導 selector `0028h` 的 descriptor base、limit 或其他
offset；未列 segment 讀取必須失敗即關閉。

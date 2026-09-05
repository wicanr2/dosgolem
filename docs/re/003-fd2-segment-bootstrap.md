# FD2 DOS/4G selector 啟動片段

日期：2026-09-05  
證據等級：**已證實**（固定檔 bytes、IDA 9.4 指令、輸入 selector 實機快照）／
**硬體規格推導**（`MOV r/m,Sreg` 的標準 x86 效果）

## 輸入與位址

沿用 [`002`](002-fd2-dos4g-install-check.md) 的固定 `FD2.EXE` 雜湊、IDA 9.4
線性位址空間與 DOSBox-X 同狀態證據。`INT 21h` 返回時已證實：

```text
DS=0160 ES=0028 GS=0020 SS=0160 EAX=4734FFFF EIP=0x3CA76（IDA 對應）
```

IDA 直接輸出與固定檔 bytes：

| 線性位址 | bytes | 原始指令 |
|---|---|---|
| `0x3CA76` | `3C 00` | `cmp al,0` |
| `0x3CA78` | `74 22` | `jz 0x3CA9C` |
| `0x3CA7A` | `8C E8` | `mov eax,gs` |
| `0x3CA7C` | `66 3D 00 00` | `cmp ax,0` |
| `0x3CA80` | `74 06` | `jz 0x3CA88` |
| `0x3CA82` | `66 A3 F0 27 05 00` | `mov [0x527F0],ax` |
| `0x3CA88` | `B0 01` | `mov al,1` |
| `0x3CA8A` | `8C DB` | `mov ebx,ds` |
| `0x3CA8C` | `8C 05 10 28 05 00` | `mov word [0x52810],es` |

標準 32-bit x86 語意使 register 目的地取得零延伸的 16-bit selector，memory
目的地只寫 16-bit。因此固定輸入推導至 `0x3CA92` 時：

- `EAX=00000001h`；
- `EBX=00000160h`；
- `[0x527F0]=0020h`；
- `[0x52810]=0028h`。

## 執行證據限制

DOSBox-X dynamic core 對 `CS:0x1F2A92` 的後續執行斷點未命中，工具本身同時輸出
「Single-stepping may not work correctly with Dynamic core」警告。因此本段不宣稱
新增 `0x3CA92` DOSBox 執行收據；不再為標準指令語意重刷 heavy debugger。
`0x3CA92` 的 `ES:[2Ch]` segment-memory 讀取尚未涵蓋，維持失敗即關閉。

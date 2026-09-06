# RE 125 — FD2 以 DOS AH=42h 定位 MDI.INI

日期：2026-09-06
證據等級：**已證實**（原始指令、函式、呼叫者與回傳消費端）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；一次性資料庫與 JSON
  只寫入 `/tmp/fd2-ida-3cc1e`。
- 位址均為 IDA 線性位址。原始函式是 `sub_3CC00`，邊界
  `0x3CC00..0x3CC4B`。

## 原始指令與資料流

```text
0x3CC08  8B 7D 10        mov  edi,[ebp+0x10]
0x3CC0B  8B 55 14        mov  edx,[ebp+0x14]
0x3CC0E  8B 45 18        mov  eax,[ebp+0x18]
0x3CC11  66 89 FB        mov  bx,di
0x3CC17  B4 42           mov  ah,42h
0x3CC19  89 D1           mov  ecx,edx
0x3CC1B  C1 E9 10        shr  ecx,10h
0x3CC1E  CD 21           int  21h
0x3CC20  66 36 89 07     mov  ss:[edi],ax
0x3CC24  66 36 89 57 02  mov  ss:[edi+2],dx
```

BX 由第一參數低 16 位取得控制代碼；32 位位移由第二參數拆成 `CX:DX`；
第三參數先進 EAX，`mov ah,42h` 後 AL 保留 DOS 定位起點模式。呼叫後 AX、DX
分別寫入同一個 stack local 的低／高 word，直接消費 DOS 回傳的新位置。

固定自然路徑在讀完 `MDI.INI` 後由 `fclose`／`__shutdown_stream` 鏈抵達本
helper。DOS `AH=42h` 的起點模式、32 位有號位移、`DX:AX` 回傳與 CF／錯誤碼
採公開 DOS 介面契約；本文只證明 FD2 的暫存器組裝與消費方式，不重做 DOS
平台規格。

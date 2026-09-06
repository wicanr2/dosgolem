# 103 — FD2 `__SetIOMode` 寫回 FILE record

日期：2026-09-06  
證據等級：函式、參數、位元設定、stride 與寫入端為**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式名 `__SetIOMode`，範圍 `0x463A4..0x463BC`；直接 caller 位於
`0x3C373`、`0x3C39D`、`0x3C4B6`。完整函式核心如下：

```text
0x463A8  8B 5D 0C       mov ebx,[ebp+arg_0]
0x463AB  8B 55 10       mov edx,[ebp+arg_4]
0x463AE  A1 E4 37 05 00 mov eax,[0x537E4]
0x463B3  80 CE 40       or dh,40h
0x463B6  89 14 98       mov [eax+ebx*4],edx
```

第一個參數是 handle index，第二個參數是要保存的 dword record；函式強制
設定該 record byte 1 的 bit `0x40`，再以四 bytes stride 寫回 `0x537E4`
所指表格。handle、stride、來源值與寫入端均已由指令資料流閉合；把 bit
`0x40` 稱為「I/O mode 已初始化」仍依 `__IOMode`／`isatty` consumer 列為
強推論，不把原始 runtime 名稱當成 ABI 證據。

勘誤：[`spec 150`](../spec/150-cpu386-load-dword-scaled-index.md) 曾將此處
誤抄為 `89 04 98`。探針錯誤訊息只列 opcode `89` 與 SIB `98`；本次 IDA
匯出的 raw bytes `89 14 98` 證實 ModRM 是 `14`，來源 register 為 EDX。

本證據授權 fixed shape `MOV [EAX+EBX*4],EDX`；不授權推廣其他 SIB store。

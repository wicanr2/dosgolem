# 106 — FD2 `__ioalloc` 緩衝配置成功旗標

日期：2026-09-06  
證據等級：函式邊界、配置結果、欄位寫入與後續 buffer consumer 為**已證實**；
bit `0x08` 的「擁有配置緩衝」名稱為**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

固定原版由 `fgetc` 進入 `__filbuf` 後，又呼叫原始 `__ioalloc`
（`0x3D919..0x3D990`）。配置成功路徑：

```text
0x3D954  FF 73 14       push dword ptr [ebx+14h]
0x3D957  ...            call malloc
0x3D95F  89 43 08       mov [ebx+8],eax
0x3D962  85 C0          test eax,eax
0x3D964  75 17          jnz 0x3D97D
0x3D97D  80 4B 0C 08    or byte ptr [ebx+0Ch],8
0x3D981  8B 43 08       mov eax,[ebx+8]
0x3D984  89 03          mov [ebx],eax
0x3D986  C7 43 04 ...   mov dword ptr [ebx+4],0
```

bit `0x08` 只在配置成功分支設定，隨後配置指標成為目前 buffer pointer 並將
剩餘 count 清零。故位元寫入及 buffer consumer 已證實；「擁有配置緩衝」是
依生命週期形成的強推論，不冒稱原始結構欄位名。

本證據授權 `80 /1 base+disp8` byte OR。真正 DOS read 仍在後續 `__filbuf`
路徑，尚未由本切片宣告完成。

# FD2 LE entry 第一個執行閘門

日期：2026-09-05  
證據等級：**已證實**（固定雜湊 bytes、IDA 指令與直接執行驗收範圍）

## 輸入與工具

- 檔案：`FD2.EXE`，357,074 bytes
- MD5：`b97caf2239a27a896069d03549d96e1e`
- SHA-256：`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
- 第一工具：授權 Docker 內 IDA Pro 9.4，LE loader，線性位址空間
- 原始 IDA 名稱：`start`，`0x3C964–0x3CBCC`
- 格式依據：Open Watcom Programmer's Guide 的 DOS/4GW Linear Executables
  說明；LE 程式在 32 位 386 平坦位址空間執行。

## 第一個可驗收指令片段

IDA 直接輸出與固定檔 bytes 一致：

| 線性位址 | bytes | 原始指令 |
|---|---|---|
| `0x3C964` | `EB 78` | `jmp short 0x3C9DE` |
| `0x3C9DE` | `FB` | `sti` |
| `0x3C9DF` | `83 E4 FC` | `and esp, 0FFFFFFFCh` |
| `0x3C9E2` | `8B DC` | `mov ebx, esp` |
| `0x3C9E4` | `89 1D 18 28 05 00` | `mov [0x52818], ebx` |
| `0x3C9EA` | `89 1D 04 28 05 00` | `mov [0x52804], ebx` |
| `0x3C9F0` | `66 B8 24 00` | `mov ax, 24h` |
| `0x3C9F4` | `66 A3 10 28 05 00` | `mov [0x52810], ax` |
| `0x3C9FA` | `BB 52 41 48 50` | `mov ebx, 50484152h` |
| `0x3C9FF` | `2B C0` | `sub eax, eax` |
| `0x3CA01` | `B4 30` | `mov ah, 30h` |
| `0x3CA03` | `CD 21` | `int 21h`（DOS version） |

第一次中斷若沒有回傳 Phar Lap 的 EAX 高字 `DX` 或 Intel Code Builder 的 `BC`，
直接控制流會到：

| 線性位址 | bytes | 原始指令 |
|---|---|---|
| `0x3CA05` | `A2 3A 28 05 00` | `mov [0x5283A], al` |
| `0x3CA0A` | `88 25 3B 28 05 00` | `mov [0x5283B], ah` |
| `0x3CA10` | `8B C8` | `mov ecx, eax` |
| `0x3CA12` | `2B F6` | `sub esi, esi` |
| `0x3CA14` | `BF 81 00 00 00` | `mov edi, 81h` |
| `0x3CA19` | `C1 E8 10` | `shr eax, 10h` |
| `0x3CA1C` | `66 3D 58 44` | `cmp ax, 4458h` |
| `0x3CA20` | `0F 85 0D 00 00 00` | `jnz 0x3CA33` |
| `0x3CA33` | `66 3D 43 42` | `cmp ax, 4243h` |
| `0x3CA37` | `0F 85 2F 00 00 00` | `jnz 0x3CA6C` |
| `0x3CA6C` | `66 BA 78 00` | `mov dx, 78h` |
| `0x3CA70` | `66 B8 00 FF` | `mov ax, 0FF00h` |
| `0x3CA74` | `CD 21` | `int 21h`（DOS/4GW installation check） |

RBIL 記錄第一次呼叫搭配 `EBX=50484152h` 是 Phar Lap／Intel Code Builder
安裝檢查；第二次 `AX=FF00h, DX=0078h` 是 Rational Systems DOS/4G 安裝檢查。
FD2 隨附並由固定檔證實的是 DOS/4GW 1.92，所以第二個閘門走「第一次呼叫沒有
Phar Lap／Code Builder signature」的分支。

`0x3C964` 沒有 caller；LE header 指定 entry object 1／offset `0x2C964`，
重定位基底 `0x10000`，所以執行線性位址為 `0x3C964`。stack object 2／offset
`0x56B0` 與基底 `0x50000` 合成初始 `ESP=0x556B0`。

## 範圍限制

此證據只足以建立「已由 DOS/4GW 載入、重定位並進入平坦 32 位程式」後的前兩段
CPU 與 loader 驗收。它不證明 selector 值、初始 `EAX/EDX`、DPMI、paging、例外、
DOS 指標轉換或第一次中斷後的行為；上述項目未有直接證據前維持失敗即關閉。

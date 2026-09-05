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

`0x3C964` 沒有 caller；LE header 指定 entry object 1／offset `0x2C964`，
重定位基底 `0x10000`，所以執行線性位址為 `0x3C964`。stack object 2／offset
`0x56B0` 與基底 `0x50000` 合成初始 `ESP=0x556B0`。

## 範圍限制

此證據只足以建立「已由 DOS/4GW 載入、重定位並進入平坦 32 位程式」後的第一段
CPU 與 loader 驗收。它不證明 selector 值、初始 `EAX/EDX`、DPMI、paging、例外、
DOS 指標轉換或第一次中斷後的行為；上述項目未有直接證據前維持失敗即關閉。


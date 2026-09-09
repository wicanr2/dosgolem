# 075 — FD2 AIL handler 巢狀閘門遞增

日期：2026-09-06
證據等級：函式、指令與欄位讀寫為**已證實**；「停用深度」用途為
**強推論**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E724`（`0x3E724..0x3E731`）保存 EBX／ESI／EDI，在 `0x3E727`
執行 `inc dword_52BEA` 後恢復並返回。直接 caller 為 `0x37953`，另有
`0x37D1A` 尾跳。IDA 對 `dword_52BEA` 的直接資料交叉參照為：

- `0x3E952`：初始化為零；
- `0x3E727`：遞增；
- `0x3E734`：相鄰函式遞減；
- `0x3E7C6`：interrupt handler 比較是否為零。

因此可證實它是 handler 的巢狀閘門計數欄位；結合成對增減與 handler 零值
判斷，將它解釋為「停用深度」屬強推論。remake／dosgolem 不需要追查 PIT
硬體時序。本切片所需 CPU 形狀僅為無前綴 `FF /0`、`mod=0 r/m=5` 的
`INC dword ptr [disp32]`。

一次性 IDA 函式報告 SHA-256：
`7755d7521b15d9cfae480cdad188f1411abd11e82878932bca5b92dc78a91020`；
資料交叉參照報告 SHA-256：
`1069565e439e79f110b009ebeb8afb7ee04dbbe804dd1822ff32700adca5590e`。

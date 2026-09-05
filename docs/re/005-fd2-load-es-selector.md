# 005 — FD2 載入 ES selector

日期：2026-09-06
證據等級：**已證實**（固定雜湊、原始 bytes、dosgolem 逐步執行停止點）

## 輸入與位址空間

- `FD2.EXE`：357,074 bytes
- MD5：`b97caf2239a27a896069d03549d96e1e`
- SHA-256：`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
- 位址：DOS/4GW 載入後 LE 線性位址
- 工具：dosgolem `a411a10` 隔離分支的一次性 Docker 探針

## 原始證據

既有 `004` 切片在 `0x3CAB3` 完成 ES environment cell 讀取。dosgolem 繼續一步後，
在 `0x3CAB8` 遇到第一個尚未支援的 opcode：

```text
steps=37 eip=0x3CAB8
bytes=8E C3 26 8C 1D D8 C9 03 00 89 35 34
EBX=0x160 ES=0x28
error=opcode 8E 尚未支援
```

`8E C3` 是標準 x86 `MOV ES,BX`，來源為 register-direct ModR/M；執行後 ES 應為
`0x0160`，EIP 應為 `0x3CABA`，旗標不變。緊接的 `26 8C ...` 是第一個 consumer，
但它涉及 ES override 下的記憶體寫入與 selector base，尚未在本切片宣稱語意。

## 邊界

本證據只授權 `8E /r` 的 register-direct、有效 segment destination 形式。不得由
`EBX=0x160` 推論一般 selector descriptor、segment base 或下一筆記憶體寫入已完成。

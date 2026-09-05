# 007 — FD2 保存環境 word

日期：2026-09-06
證據等級：**已證實**（固定雜湊、原始 bytes、dosgolem 執行停止點）

沿用 `006` 的固定 FD2.EXE 與 LE 線性位址。descriptor 切片完成後，dosgolem
自行執行至：

```text
EIP=0x3CAC7 ECX=0x00000030
66 89 0D 38 28 05 00    mov word [0x52838],cx
```

本指令把 `CX=0x0030` 以 little-endian word 寫至平坦線性位址 `0x52838`，
不使用 segment override、不改旗標。後續 `0x3CACE: 56` 尚未納入本證據。

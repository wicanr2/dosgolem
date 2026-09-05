# 018 — FD2 environment 首 dword 判別

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、dosgolem 執行狀態）

沿用 `017` 的 FD2.EXE 身分與 LE 線性位址。首 dword 載入後：

```text
0x3CB14  0D 20 20 20 20       or eax,0x20202020
0x3CB19  3D 6E 6F 38 37       cmp eax,0x37386F6E
0x3CB1E  75 07                jnz 0x3CB27
0x3CB27  80 3E 00             ...
```

最小 environment 輸入 `EAX=0x00010000`，OR 後為 `0x20212020`，與立即值不等，
所以跳至 `0x3CB27`。該常數的文字／產品語意尚未證實，不建立名稱。

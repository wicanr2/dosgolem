# 017 — FD2 environment block 第一筆讀取

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes／selector 資料流）＋**DOS 規格近似**

沿用 `016` 的 FD2.EXE 身分與 LE 線性位址。載入 `DS=0x0030` 並清除 EBP 後：

```text
0x3CB12  8B 06    mov eax,[esi]
```

此時 `ESI=0`，因此讀取 environment block 開頭的 dword。DOS environment 是以 NUL
結尾的 `NAME=VALUE` 字串序列，雙 NUL 後接額外內容；具體環境變數取決於啟動主機，
不能從 FD2 bytes 推導。dosgolem 預設提供規格合法的最小 block，並允許 host 日後配置；
這是平台規格近似，不是原版機器逐 byte parity。

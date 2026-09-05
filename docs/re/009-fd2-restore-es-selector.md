# 009 — FD2 從保存欄位恢復 ES selector

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、writer／consumer、dosgolem 停止點）

沿用 `008` 的 FD2.EXE 身分與 LE 線性位址。啟動流程先在 `0x3CA8C` 以
`8C 05 10 28 05 00` 把原始 `ES=0x0028` 寫至 `[0x52810]`；目前 consumer 是：

```text
0x3CACF  8E 05 10 28 05 00        mov es,word [0x52810]
```

因此載入值 `0x0028` 的來源、writer 與 consumer 已閉合。先前同狀態收據也證實
`0x0028:0x2C` 是可讀取的 host environment cell，所以 selector `0x0028` 可載入；
但其一般 base／limit 仍未知，只能由既有窄 hook 服務已證實的 cell。

下一筆指令與恢復後的其他 ES consumer 必須另行取證，不得因 selector 可載入就把
它升格為完整 flat descriptor。

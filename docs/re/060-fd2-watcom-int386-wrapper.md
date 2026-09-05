# 060 — FD2 的 Watcom `int386` 包裝器

日期：2026-09-06
證據等級：原始符號、函式邊界、cdecl 參數順序與 caller 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA Watcom 簽章保留 `int386` 原始名稱於 `0x36D98..0x36DC1`。其 cdecl
參數依序是中斷號、輸入 `REGS*`、輸出 `REGS*`；包裝器先透過
`segread`（`0x3D6F9..0x3D726`）取得 segment register，再把四個參數送入
`int386x`。`sub_36284` 在 `0x362D8` 以中斷號 `0x31` 呼叫此入口。

dosgolem 的 hook 位於這個已證實的 runtime ABI 邊界：它必須解讀 cdecl
堆疊與 `REGS` 內容，不可僅依 EIP 跳過包裝器。未列中斷號或功能維持
失敗即關閉。

IDA 報告 SHA-256：`int386` 函式
`1182ce650e62e959efddbf239bc2446192ea3a8fee2fe04cc58911de01309e30`；
名稱位址查詢 `67c9e4cb113bd5098da848697e44b222b56e235c7617fe5b112b1d9a8c2b161a`。

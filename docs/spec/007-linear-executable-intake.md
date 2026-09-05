# 007 — LE 執行檔接入與 FD2 能力探針

狀態：**READY**（§1–§4 的唯讀解析與失敗即關閉）／**DRAFT**（§5 的實際執行）
日期：2026-09-05
前置：[`001`](001-scope-and-mvp.md)、[`003`](003-machine-and-loader.md)

## 1. 問題與範圍

第二個接入案例《炎龍騎士團 2》的 `FD2.EXE` 不是目前 `LoadEXE` 所支援的
16 位元實模式 MZ 映像。固定研究版本為：

- 大小：357,074 bytes
- MD5：`b97caf2239a27a896069d03549d96e1e`
- SHA-256：`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`

其 MZ `e_lfanew`（檔案偏移 `0x3C`）為 `0x28B8`，該位置的簽章是 `LE`。
這證實檔案含 32 位元 Linear Executable 映像；隨附 `DOS4GW.EXE` 的存在與
專案既有 Watcom／DOS4GW 證據一致，但本規格不以檔名推導 loader 行為。

格式欄位依 IBM《32-bit Linear eXecutable Module Format》公開規格解讀；FD2
數值仍以固定雜湊實檔原始 bytes 為準。LE 與 LX 不可混用，解析器只接受 `LE`。

## 2. READY：唯讀標頭解析

新增通用、無遊戲名稱的 LE inspector：

- 先驗證 MZ 簽章、`e_lfanew` 邊界及 `LE` 簽章；
- 解析 byte／word order、CPU／OS、module flags、pages、EIP／ESP object 與 offset、
  page size、object table offset／count、page map、fixup、import、data pages 等欄位；
- object table offset 以 LE header 起點為基準，每筆固定 24 bytes；
- 所有加法與乘法先做邊界檢查，截斷、超界、`LX` 或其他簽章一律回錯誤；
- 不執行 MZ stub、不載入 object、不套 fixup，也不假裝已支援保護模式。

FD2 驗收錨點：header `0x28B8`、3 objects、page size `0x1000`、EIP object 1／
offset `0x2C964`、ESP object 2／offset `0x56B0`；三筆 object 的
`(virtual size, relocation base, flags, page index, page count)` 分別是：

1. `(0x3EBD9, 0x10000, 0x2045, 1, 0x3F)`
2. `(0x56B0, 0x50000, 0x2043, 0x40, 4)`
3. `(0x34D2, 0x60000, 0x2043, 0x44, 4)`

## 3. READY：能力探針

`cmd/leprobe` 只輸出格式與物件摘要。原版檔案由使用者以 `-exe` 提供；儲存庫
不含、也不產生原版 bytes。探針不接受解析失敗後回落成普通 MZ 執行，避免把
「跑了 DOS stub」誤報成「已跑 FD2」。

## 4. 驗收

- 合成 fixture 覆蓋有效 LE、非 MZ、非 LE、截斷 header、超界 object table；
- 可選實檔測試只在 `DOSGOLEM_FD2_EXE` 存在時執行，先核對大小、MD5、SHA-256，
  再檢查 §2 錨點；缺檔必須 skip；
- `go test ./...` 全綠；
- `leprobe` 對固定雜湊 FD2.EXE 輸出三個 objects，且明示 `execution_supported=false`。

## 5. DRAFT：從解析走到可對拍執行

以下尚未 READY，不可猜接：80386 指令與保護模式、描述子／分頁模型、LE page
載入與 fixup、DPMI／DOS4GW 契約，以及 FD2 實際使用的 DOS／VGA 服務。每一層須以
獨立語料或 DOSBox-X／原版同狀態結果驗證；硬體時序只採規格近似，不追逐週期一致。


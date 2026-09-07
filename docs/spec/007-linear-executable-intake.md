# 007 — LE 執行檔接入與 FD2 能力探針

狀態：**READY**（§1–§4 的唯讀解析、page map、object 映像、fixup 索引、
record 解碼與 FD2 所需的 internal relocation 套用）／**DRAFT**（§5 的實際執行）
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
- LE page map 每筆固定 4 bytes：前三 bytes 是高位在前的 24-bit 實體頁碼，末 byte
  是 page flags；`PAGE_VALID=0` 與 `PAGE_ZEROED=3` 可重建，iterated／invalid／range
  在尚無 decoder 前必須回錯誤；
- data pages offset 是檔案絕對偏移；有效頁 N 從
  `data_pages_offset + (N-1)*page_size` 取值，最後實體頁使用 `last_page_size`；
- object 映像配置成 `virtual_size`，依 object 的 1-based page index 複製並保留尾端
  zero fill；任何來源頁或目的範圍超界一律回錯誤；
- 所有加法與乘法先做邊界檢查，截斷、超界、`LX` 或其他簽章一律回錯誤；
- 不執行 MZ stub、不載入 object、不套 fixup，也不假裝已支援保護模式。

### 2.1 READY：fixup page table 與 record 解碼

依 IBM LE 規格，fixup page table 含 `module_pages + 1` 筆 little-endian
`uint32`，每筆是相對於 fixup record table 起點的位元組偏移；相鄰兩筆界定
一個 logical page 的 record 範圍，最後一筆界定整張 record table 的結尾。

- page offsets 必須單調不減，空頁可由相等 offsets 表示；table、最後 offset、
  record range 或 import table 邊界超出檔案時失敗即關閉；
- source type 僅接受規格定義的 `0, 2, 3, 5, 6, 7, 8`；`1, 4` 與保留值拒絕；
- 單一 source offset 解為 signed 16-bit，以保存跨頁的負偏移；source-list 模式
  則先讀一個 byte count，target 與 additive 後再讀該數量的 signed 16-bit offsets；
- target type 支援 internal、import-by-ordinal、import-by-name 與 internal-entry，
  並依 flags 選擇 8／16／32-bit ordinal、target offset 與 additive 寬度；
- alias flag 只允許 selector、16:16 pointer、16:32 pointer；chaining 只允許
  32-bit offset 且 target 是 internal 或 internal-entry，並禁止與 source-list 並用；
- parser 保留原始 source／target flags 及 record bytes。此層只建立 typed record，
  不解析 import 名稱、不套 relocation，也不宣稱執行支援。

主要格式證據：IBM《[32-bit Linear eXecutable Module Format](https://komh.github.io/os2books/os2tk45/lxref.htm)》
的 Fixup Page Table、Fixup Record Table 與 Fixup Record 區段。

FD2 驗收錨點：header `0x28B8`、3 objects、page size `0x1000`、EIP object 1／
offset `0x2C964`、ESP object 2／offset `0x56B0`；三筆 object 的
`(virtual size, relocation base, flags, page index, page count)` 分別是：

1. `(0x3EBD9, 0x10000, 0x2045, 1, 0x3F)`
2. `(0x56B0, 0x50000, 0x2043, 0x40, 4)`
3. `(0x34D2, 0x60000, 0x2043, 0x44, 4)`

固定實檔的 71 筆 page map 都是依序頁碼 1–71、flags 0。重建後三個 object 的
SHA-256 分別是 `e6e686d4a6081e697d925d8ec3951cb25141a0baa567dda753049803f4bdb504`、
`bf7abfabc1b49ea2ff1078a7d59a068d5cb5c359eac4baff6294fba718dd962f`、
`1fb82889fdaa70b2e3f376ff76638ed565b1ef98070e2704efb27b66b361955b`。

同一實檔的 fixup page table 有 72 筆 offsets；71 個 logical pages 中 68 頁
含 record，共 7,944 筆。這只證實 §2.1 的 record 邊界與格式可完整消費，不能
提升為 relocation 已套用或程式可執行。

### 2.2 READY：FD2 所需的 internal relocation 套用

固定雜湊 FD2.EXE 的 7,944 筆 record 全部是 source type／flags `7`（32-bit
offset）與 target type `0`（internal）；target flags 僅 `0`（7,055 筆）及
`0x10`（889 筆），target object 分布為 object 1：890、object 2：6,788、
object 3：266。沒有 import、additive、chaining、alias 或 source-list。

因此本階段的 relocation 套用器只接受上述形狀：

- 先重建全部 object images，再以 logical page 找到唯一來源 object；
- patch 位置是該 logical page 在來源 object 的 page-relative base 加 signed
  source offset，允許規格定義的跨頁負 offset，但寫入的四個 bytes 必須完整落在
  object image；
- 寫入值為 target object 的 relocation base 加 target offset，採 little-endian
  `uint32`，object ordinal、target offset 或加法溢位一律回錯誤；
- 任一非 FD2 已證實形狀仍回錯誤，不以未測試的通用 relocation 支援掩蓋缺口；
- 回傳新 images，不修改輸入 bytes 或未 relocation 的 `ObjectImage` 結果。

固定實檔套用後三個 object 的 SHA-256 依序為
`3306c118632220426483866025e5e6dd980a7461361036164d4ca5e63fda9b08`、
`1461d8d5dc1aacbadfd0d96f322dda24026dcb3f4ed26d7cf371a78af5c2e14c`、
`89d650594b23d5a797cc10bd6f67cfe608c02304d806dc6fe6fd2b7f65928dae`。
探針稱之為 relocation preview；尚未把 images 掛入 CPU runtime。

## 3. READY：能力探針

`cmd/leprobe` 只輸出格式與物件摘要。原版檔案由使用者以 `-exe` 提供；儲存庫
不含、也不產生原版 bytes。探針不接受解析失敗後回落成普通 MZ 執行，避免把
「跑了 DOS stub」誤報成「已跑 FD2」。

## 4. 驗收

- 合成 fixture 覆蓋有效 LE、非 MZ、非 LE、截斷 header、超界 object／page table、
  valid／zeroed page 與未支援 page flag；
- 合成 fixup fixture 覆蓋單筆／空頁、負 source offset、source list、四種 target、
  寬度 flags、additive，以及截斷、非單調 page offsets、未定義 source type、
  非法 alias／chaining 組合；
- 可選實檔測試只在 `DOSGOLEM_FD2_EXE` 存在時執行，先核對大小、MD5、SHA-256，
  再檢查 §2 錨點；缺檔必須 skip；
- `go test ./...` 全綠；
- `leprobe` 對固定雜湊 FD2.EXE 輸出三個 objects，且明示 `execution_supported=false`。
- internal relocation 合成測試涵蓋 16／32-bit target offset、跨頁負 source offset、
  object／寫入範圍與加法溢位；固定雜湊實檔另鎖定 relocation 後 object hashes。

## 5. DRAFT：從解析走到可對拍執行

以下尚未 READY，不可猜接：80386 指令與保護模式、描述子／分頁模型、FD2 未使用的
其他 LE fixup 形狀、DPMI／DOS4GW 契約，以及 FD2 實際使用的 DOS／VGA 服務。每一層須以
獨立語料或 DOSBox-X／原版同狀態結果驗證；硬體時序只採規格近似，不追逐週期一致。

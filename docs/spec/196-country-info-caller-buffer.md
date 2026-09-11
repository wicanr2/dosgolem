# 196 — `int 21h AH=38h AL=00h`：國別資訊寫進呼叫端的緩衝區

狀態：**READY**（行為照 DOSBox-X 原始碼；觸發案例見 §2）
日期：2026-09-11
前置：[`010-dosv`](010-dosv.md) §2.1（國碼 81 的理由）

---

## 1. 規則

`AH=38h AL=00h` 的輸入是 **DS:DX ＝ 呼叫端的緩衝區**。服務把國別資訊表寫進那裡，然後：

| 暫存器 | 結果 |
|---|---|
| DS、DX | **不變** |
| AL | 國碼 |
| AH | 不變 |
| BX | 國碼 |
| CF | 清除 |

寫入長度 `18h` bytes。國碼維持 `010-dosv` §2.1 的 **81（日本）**與同一張表的內容——
DOS/V 程式靠它判斷日文環境，這個理由不變；變的只有「表放在哪、DS:DX 指到哪」。

依據：DOSBox-X `src/dos/dos.cpp` 的 `case 0x38`（第 1866 行起）：
`MEM_BlockWrite(SegPhys(ds)+reg_dx, dos.tables.country, 0x18)`，
接著只設 `reg_al` 與 `reg_bx`，清 CF，沒有寫 DS 或 DX。

這條取代 `010-dosv` §2.1「表放 StubSeg 的 `+80h`，`DS:DX` 指過去」那一句；
`AH=63h`（DBCS 表，回傳 DS:SI 指向 DOS 自己的表）才是「回傳指標」的形式，兩者不同。

## 2. 觸發案例

Turbo Assembler 2.5（`TASM.EXE`）啟動時呼叫 `AH=38h`，緩衝區在自己的資料段 `1706:4E1A`。
舊實作把 DS 改成 `0080`（StubSeg）、DX 改成 `0080`，TASM 之後所有以 DS 為基底的
存取都落到錯的段，在第 848 條指令以回傳碼 7 結束、沒有任何訊息
（`cmd/run -trace-tail` 看到 DS 在 `10AB:18BC` 的 `int 21h` 之後從 `1706` 變成 `0080`）。

## 3. 驗收

1. 契約測試：呼叫前 DS:DX 指向呼叫端緩衝區；呼叫後 DS、DX 不變，緩衝區第 0 byte
   是日期格式 2，AL＝BX＝81，CF 清除。反面對照：修改前要失敗在 DS 被改。
2. `tools/go.sh test ./internal/dos ./apps/...` 全綠。
3. `TASM /mx STRLEN.ASM` 在 `cmd/run -cpu 8086` 下產出 `.OBJ`。

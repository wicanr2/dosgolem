# 012 — CPU：80386 最小子集（DOSJP 的 0x66 前綴）

狀態：**READY**（指令清單全部來自對原版二進制的掃描，見 §2）
日期：2026-09-06
前置：[`002`](002-cpu-8086.md)（CPU 驗收準則）、[`011`](011-xms.md)

---

## 1. 定位與硬規則

DOSJP.COM 的初始化與常駐 handler 使用 **80386 的 operand-size 前綴
（0x66）** 做 32 位元運算（算 XMS 裡的字型位移）。CPU 目前是
8086／80186（`docs/spec/002`），跑到第一道 0x66 就報
「未實作的 80186 opcode」。

**CPU 的驗收判準不變：語料全部通過。** 這個子集：

- 只在 `Model >= Model80386` 才解 0x66——`cpu.New()` 預設仍是
  Model8086，**語料路徑不受影響**；
- `machine.New()`（跑 DOS 軟體的那台）升成 Model80386——
  8086 上 0x66 不是合法前綴，8086／80186 語料都不含它，
  升 model 對既有程式的唯一影響是「遇到 0x66 不再報錯」。

## 2. 指令清單（全部掃自 DOSJP.COM，**confirmed**）

對 `yuan/org_game/DOSJP.COM` 全文掃 0x66 的結果（檔案位移）：

| 位元組串 | 指令 | 出現處 |
|---|---|---|
| `66 98` | `CWDE`（AX 符號擴展到 EAX） | 0x14B（int 10h handler） |
| `66 C1 E0 ib` | `SHL EAX, imm8` | 0x1C2（handler） |
| `66 A3 moffs16` | `MOV [moffs], EAX` | 0x1C6（handler） |
| `66 2E A1 moffs16` | `MOV EAX, CS:[moffs]` | 0x122（handler） |
| `66 2E A3 moffs16` | `MOV CS:[moffs], EAX` | 0x43A（init） |
| `66 C7 06 moffs16 imm32` | `MOV dword [moffs], imm32` | 0x14D、0x16E（handler）、0x3FE、0x40A（init） |
| `66 69`／`66 6F` | **是字串資料**（"Not enough…" 內），不執行 | 0x9C、0xA5、0xBD |

用途（反組譯閱讀）：handler 把字型的 JIS 碼經 `CWDE`／`SHL EAX,4`
算成 XMS 裡的 byte 位移，再用 `MOV dword` 填 XMS move 描述子——
**EAX 是唯一被用到的 32 位元暫存器**。

## 3. 語意（照 Intel 手冊的 386 定義）

- `CWDE`：EAX ＝ sign-extend(AX)。
- `SHL EAX, imm8`：對 32 位元做，旗標照 386 規則（這裡的程式
  不讀旗標，但仍照規則設）。
- `MOV EAX, m16`／`MOV m16, EAX`：32 位元記憶體搬移，
  段前綴（2E）照常。
- `MOV [moffs16], imm32`（C7 /0，modrm 06h）：寫 4 個位元組。
- **16 位元寫 AX 不清 EAX 的高半**（386 的行為）；EAX 以
  `R[AX]`（低半）＋ `EAXHi`（高半）表示，讀寫經 helper 同步。

範圍宣告：**不做**一般化的 0x66 解碼（其他 opcode 帶 0x66 仍報
「未實作」）、不做 address-size 前綴（0x67）、不做 386 的
其他 32 位元暫存器。遇到再說，遇到之前靜靜放行才是錯的。

## 4. 驗收

1. `tools/go.sh test ./internal/cpu` 全綠（語料路徑沒動）；
2. 新增釘死測試：六道指令各一個 case（含旗標與高半保留語意）；
3. DOSJP 走完 init（`AH=31h` 常駐），收據在
   `yuan/workplace/boot-20260906-02/`。

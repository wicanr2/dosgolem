# 008 — 平坦 386 LE entry 第一個執行閘門

狀態：**READY**（載入與 `0x3C964→0x3CA74` 前兩個中斷閘門的最小指令集）／
**DRAFT**（完整 386、DPMI 與一般化 DOS/4GW 服務）；安裝檢查的固定 FD2 oracle
回傳與成功分支已移至 [`009`](009-fd2-dos4g-install-check.md)
日期：2026-09-05
前置：[`007`](007-linear-executable-intake.md)、
[`FD2 entry 證據`](../re/001-fd2-le-entry.md)

## 1. 隔離架構

新增獨立 `cpu386` 與 LE machine，不改寬既有 `cpu.CPU`。原本的 8086／80186
暫存器、20-bit wrap、real-mode IVT 與 SingleStepTests 契約保持不變。

- 386 暫存器與 `EIP/EFLAGS` 使用 32-bit；記憶體位址不做 20-bit wrap；
- 本階段採 DOS/4GW 已建立完成的 base-0 平坦 code/data 執行環境，不模擬 extender
  自己從 real mode 切換保護模式的過程；
- `LoadLE` 使用 `InspectLE` 與 `RelocatedObjectImages`，把每個 object 寫入其
  `RelocationBase`，再設定 `EIP=entry base+offset`、`ESP=stack base+offset`；
- object 重疊、entry／stack object 超界、記憶體範圍溢位均回錯誤；
- 中斷只透過明確 hook 交給上層；沒有 handler 時失敗即關閉，不走 real-mode IVT。

## 2. 第一批 READY 指令

只實作證據檔第一個閘門所需形狀：`JMP rel8`、`STI`、`AND r/m32,imm8`
（目前只接受 register）、`MOV r32,r32`、`MOV [disp32],r32`、
`MOV r32,imm32`、operand-size `66` 下的 `MOV r16,imm16` 與
`MOV [disp32],AX`、`MOV r8,imm8`、`SUB r32,r32`、`INT imm8`。

第二個閘門再加入 `MOV [disp32],r8`、`SHR r32,imm8`、operand-size `66`
的 `CMP AX,imm16`、`JNZ rel32`。測試中的一般 DOS version handler 只修改 AX，
不偽造 Phar Lap／Code Builder 高字 signature；因此必須到達 `0x3CA74`，並以
`AX=FF00h, DX=0078h` 發出第二次 `int 21h`。

未列 opcode、未列 ModRM addressing、其他 prefix 或中斷未被 hook 時均回含
`EIP/opcode` 的錯誤；不能當作空操作略過。

## 3. 驗收

- 合成測試逐一覆蓋上述指令、32-bit flags、絕對記憶體寫入與未知 opcode；
- 固定雜湊 FD2 optional test 從真實 LE entry 執行到第一次 `int 21h`，hook 必須看到
  `AH=0x30`；此時 `ESP=0x556B0`，`[0x52818]`、`[0x52804]` 等於該值，
  `[0x52810]=0x24`，`EBX=0x50484152`；
- 同一測試回傳可設定的一般 DOS 版本後，必須保存 major／minor 到 `0x5283A/B`，
  排除兩種非 DOS/4GW signature，並在有界 steps 內到達第二次 DOS/4GW 安裝檢查；
- 該閘門只能報「entry prefix executed」，不得報遊戲已啟動、畫面可對拍或完整 386。

`cmd/leprobe -execute-entry-prefix` 提供可重跑入口；預設仍只檢查檔案，明確給旗標
才執行，且最多 20 steps。輸出必須保留 `execution_supported=false`，另行回報
`entry_prefix_executed=true`，避免把窄閘門誤讀為完整遊戲執行。

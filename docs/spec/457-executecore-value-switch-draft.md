# DRAFT 3a：executeCore 由布林 switch 改為值 switch

狀態：DRAFT（實作前筆記）。範圍：`internal/cpu/ops.go` 的 `executeCore`（a3cfff5 第 134–605 行）。只改分派結構，各 case 的本體逐字保留。

## 1. 動機

`executeCore` 寫成 `switch { case <布林式>: … }`。Go 對這種 switch 只能依序求值每個條件，後段 opcode（例如 E8 CALL、EB JMP、
FE/FF 群組）要先比過前面約 80 條條件。改成 `switch op { case 常數…: }` 後，編譯器可以產生跳表或二分搜尋。
profile（`prof/s2base-760p.pprof`）中 `executeCore` 自身 8.6%，預估省 5–10%。

## 2. 原 case 順序（第 134–605 行）與互相重疊的條件

依序：

1. `op < 0x40 && op&7 < 6 && op ∉ {0F,27,2F,37,3F}` → aluGroup。排除的五個值的 `op&7` 都是 7，排除條件多餘；
   命中集合＝ 00–05、08–0D、10–15、18–1D、20–25、28–2D、30–35、38–3D（48 個值）。
2. 06、07、0E、0F、16、17、1E、1F（段暫存器 PUSH／POP、POP CS）。
3. 27、2F、37、3F（BCD）。
4. 40–47、48–4F、50–57、58–5F。
5. **重疊 A**：`60–6F && Model ≥ 80186` → op186_6x；其後 `60–7F` → 條件跳躍。60–6F 依機型分流。
6. 80–83 → aluImm；84–8F 各自一條。
7. 90–97、98–9F 各自一條；A0–A3 各自一條。
8. **重疊 B**：`A4–AF 且 ≠ A8、A9` → stringOp；A8、A9 → TEST。兩者互斥，無順序問題。
9. B0–B7、B8–BF。
10. **重疊 C**：`(C0 || C1) && Model ≥ 80186` → shiftImm；`C8 && ≥186` → enter；`C9 && ≥186` → leave；
    其後 `C0 || C2` → RET imm16；`C1 || C3` → RET；C4、C5 → LES/LDS；C6；C7；`C8 || CA` → RETF imm16；`C9 || CB` → RETF；CC、CD、CE、CF。
    C0、C1、C8、C9 依機型分流。
11. D0–D3 → shiftGroup；D4、D5、D6、D7 各自；D8–DF → ESC。
12. E0–E3 → LOOP 族；E4–EF 各自一條。
13. F4、F5、F6/F7、F8、F9、FA、FB、FC、FD、FE/FF。
14. default → 「未實作的 opcode」。沒有任何 case 命中的值：26、2E、36、3E（段前綴；Step 的前綴迴圈先吃掉，不會到這裡）、
    F0、F1、F2、F3（LOCK／REP；同上）。原式與新式都落到 default。66、67、64、65 在 8086/80186 落在 60–6F，照重疊 A 處理；
    在 80386 由 Step 當前綴吃掉。

除了重疊 A 與 C，各 case 的命中集合兩兩不相交，所以順序無關。

## 3. 改寫後的 case 列表

- 第 1 條改為 48 個常數的 case 列表。
- 重疊 A：`case 0x60 … 0x7F`（32 個常數）：`if op < 0x70 && c.Model >= Model80186 { return c.op186_6x(op) }`，其後條件跳躍本體。
  60–6F 在 80186 以上走 op186_6x、以下走條件跳躍；70–7F 一律條件跳躍。與原式相同。
- 重疊 C：
  - `case 0xC0, 0xC2`：`if op == 0xC0 && c.Model >= Model80186 { return c.shiftImm(op) }`，其後 RET imm16 本體；
  - `case 0xC1, 0xC3`：`if op == 0xC1 && ≥186 { return c.shiftImm(op) }`，其後 RET 本體；
  - `case 0xC8, 0xCA`：`if op == 0xC8 && ≥186 { c.enter(); break }`，其後 RETF imm16 本體；
  - `case 0xC9, 0xCB`：`if op == 0xC9 && ≥186 { c.leave(); break }`，其後 RETF 本體。
  原式中 enter／leave 的 case 沒有 return，落到函式尾的 `return nil`；新式 `break` 跳出 switch 後同樣 `return nil`。
- 其餘範圍條件（40–47、A4–AF 去掉 A8/A9、D8–DF、E0–E3 等）展開成常數列表，本體不動。
- 本體內原有的 `break`（8C、8E、9C、9D、CF、F4、FA、FB）在值 switch 中同樣跳出外層 switch。8E 的 `break` 位在內層
  `switch m.reg` 之後，不受影響。

## 4. 等價論證

對每個 op（0–255）與 Model（8086、80186、80386），原式命中的第一個 case 本體與新式命中的本體相同：

- 不相交的 case：集合相同、本體相同。
- 重疊 A、C：新式的 if 條件正是原式前一條的條件，不成立時執行原式後一條的本體。
- default 集合相同。

另以測試釘住：新增 `TestExecuteCoreDispatchTable`，對 256 個 op × 3 個 Model，以一份保留在測試檔中的「原布林式」
分類函式（逐字搬自 a3cfff5）與新 switch 的分類（從 case 列表展開）比對，兩者應指到同一個本體編號。

## 5. 驗收

- **硬條件**：SingleStepTests 語料（`/home/anr2/cht/dosgolem/testdata/8088` 唯讀掛載）全數通過，改動前後通過數相同。
- 裸 RunUntil：s760、s800 × 觀察器關／開，新舊交錯多輪，記錄最小 CPU 時間。Digest、cpu32、Cycles、full 相同。
- 截短 Q on23：HASH/SHOT 行與 `flight-q-on23/` 211 檔相同。
- 長測試 Q、C11、存讀檔 F–K 對 g-formal 相同。
- `go vet ./...` 與短測試通過。
- 加速未達 5% 時照實回報。

## 6. 第 2 點（前綴表）的評估（另一個改動，這次不併做）

`CPU.Step` 開頭清 6 個前綴欄位，前綴迴圈對每個 opcode 走一次 `switch op`。這已經是值 switch，Go 會編成跳表；
前綴表能省的只有欄位清零與一次分派，profile 中 `CPU.Step` 自身 8.3%，其中大部分是迴圈與呼叫 execute。預估 1–3%，
在目前的量測雜訊之下（交錯 3 輪最小值仍有約 ±10% 波動），量不出來的機率高。建議先不做。

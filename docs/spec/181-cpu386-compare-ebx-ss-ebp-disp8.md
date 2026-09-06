# 181 — CPU386 比較 EBX 與 SS:EBP+disp8 dword

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 123`](../re/123-fd2-line-reader-empty-eof-compare.md)

- 擴充無 prefix 的 `3B 5D disp8`：`cmp ebx,dword ptr [ebp+disp8]`。
- disp8 有號加入 EBP；右運算元固定透過 SS 描述子讀取 32 位小端序值。
- 依 32 位 `EBX-right` 更新算術旗標；記憶體、EBX、EBP 與其他暫存器不變。
- 讀取越界或描述子不存在時失敗，且不得修改 CPU 狀態。
- operand-size／segment／repeat prefix、其他 ModRM、base 或來源暫存器維持
  失敗即關閉（fail-closed）。

## 驗收條件

- CPU 測試覆蓋正／負 disp8、相等、借位、有號溢位、狀態不變與越界拒絕。
- 固定雜湊 FD2 自然執行 `0x46C91`，確認右運算元來自
  `SS:[EBP+0x14]`、比較後抵達 `0x46C94`，且暫存器與記憶體不變。
- 有界自然執行記錄下一個失敗即關閉位址，不順帶放寬後續指令。

## 驗證收據

- 固定雜湊 FD2 自然執行抵達 `0x46C91`；右運算元由
  `SS:[EBP+0x14]` 讀取，執行後抵達 `0x46C94`，暫存器保持不變，ZF 與
  EBX／起始 buffer 位址是否相等一致。
- CPU 聚焦測試覆蓋正／負 disp8、相等、借位、有號溢位、旗標與記憶體不變，
  以及越界拒絕。
- 強制重建後的有界自然探針抵達下一個失敗即關閉位址 `0x3725A`：
  opcode `3B`、ModRM `50`。本規格未順帶解釋或放寬該指令。

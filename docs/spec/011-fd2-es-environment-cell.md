# 011 — FD2 `ES:[2Ch]` 固定啟動 cell

狀態：**CONFORMED**（segment prefix、固定 word 讀取、執行至 `0x3CAB3`）／
**DRAFT**（一般 descriptor 與 segment memory）  
日期：2026-09-05  
前置：[`010`](010-fd2-segment-bootstrap.md)、
[`ES cell 證據`](../re/004-fd2-es-environment-selector.md)

## 1. CPU 契約

- `Step` 接受至多一個 `66` operand-size prefix 與至多一個 `26` ES override，
  兩者可依原版順序連續出現；重複 prefix 或其他 segment override 仍拒絕；
- `66 26 8B /r` 目前只接受 `mod=0, r/m=5` 的 `disp32` word 讀取；
- CPU 透過明確 `SegmentRead16(selector,offset)` hook 取值；hook 缺失或拒絕時回錯誤，
  不把 selector 當平坦位址。

## 2. 固定 oracle

`FD2StartupDOS` 只接受 `selector=0028h, offset=002Ch`，回傳 word `0030h`；
任何其他 selector／offset 拒絕。此值來自原版同狀態執行，不一般化為 PSP 或環境
結構語意。

## 3. 驗收

- 合成測試覆蓋雙 prefix word read、目的 register 高 16 位保留、hook 缺失與錯誤
  selector 拒絕；
- 固定雜湊 FD2 從 entry 執行到 `EIP=0x3CAB3`，核對 `ECX=0x30`；
- 不執行 `0x3CAB3` 後的狀態寫入，不宣稱一般 segment descriptor 已完成。

2026-09-05 固定雜湊 `FD2.EXE` 的容器全套測試通過；實際 `leprobe` 輸出
`steps=36 eip=0x3CAB3 ecx=0x30 es_environment_cell=true`。本節窄切片已符合規格，
其他 segment cell 仍維持 DRAFT 並失敗即關閉。

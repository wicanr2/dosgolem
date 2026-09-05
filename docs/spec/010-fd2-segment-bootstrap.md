# 010 — FD2 selector 啟動寫入

狀態：**CONFORMED**（固定 selector 狀態、`MOV r/m,Sreg`、執行至 `0x3CA92`）／
**DRAFT**（descriptor base 與一般 segment-memory）；固定 `ES:[2Ch]` cell 已移至
[`011`](011-fd2-es-environment-cell.md)
日期：2026-09-05  
前置：[`009`](009-fd2-dos4g-install-check.md)、
[`selector 啟動證據`](../re/003-fd2-segment-bootstrap.md)

## 1. 固定 oracle 狀態

`FD2StartupDOS` 在接受第一個固定 FD2 版本查詢後，設定實機已證實且後續會消費的
`DS=0160h`、`ES=0028h`、`GS=0020h`、`SS=0160h`。第二個 DOS/4G 安裝檢查仍
重新確認 `GS=0020h`。這些是固定 oracle 值，不是一般化 selector 配置。

## 2. 指令契約

新增 opcode `8C /r` 的兩種必要形狀：

- `mod=3`：把 ModRM `reg` 指定的 segment selector 零延伸寫入 32-bit general
  register；
- `mod=0, r/m=5`：讀取 `disp32`，把 selector 以 little-endian 16-bit 寫入平坦記憶體。

ModRM segment 編碼只接受 ES／CS／SS／DS／FS／GS（0–5）。其他 addressing、
operand-size override 或 segment-memory 一律拒絕。

## 3. 驗收

- 合成測試覆蓋 `mov eax,gs`、`mov ebx,ds`、`mov word [disp32],es` 與非法 segment；
- 固定服務測試確認實機 selector 值；
- 固定雜湊 FD2 在有界步數內由 entry 執行至 `EIP=0x3CA92`，並核對
  `EAX=1`、`EBX=0x160`、`[0x527F0]=0x20`、`[0x52810]=0x28`；
- 不執行 `0x3CA92` 的 `ES:[2Ch]`，不宣稱 segment descriptor 已完成。

2026-09-05 以固定雜湊 `FD2.EXE` 在無網路 Go 容器執行全套測試通過；實際
`leprobe` 輸出為 `steps=34 eip=0x3CA92 eax=0x1 ebx=0x160 stored_gs=0x20
stored_es=0x28`。本節窄切片因此符合規格，segment-memory 仍維持 DRAFT。

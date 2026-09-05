# 009 — FD2 DOS/4G 安裝檢查與成功分支

狀態：**CONFORMED**（固定 FD2 oracle 回傳、`CMP AL,imm8`、`JZ rel8`）／
**DRAFT**（segment memory、descriptor base 與 `0x3CA7A` 後續）  
日期：2026-09-05  
前置：[`008`](008-flat-386-entry.md)、
[`DOS/4G 執行證據`](../re/002-fd2-dos4g-install-check.md)

## 1. 服務契約

新增明確的 FD2 啟動服務，不把回傳值藏在命令列匿名 hook：

- `INT 21h, AH=30h, EBX=50484152h` 回傳 DOS 6.22 的 `AX=1606h`；
- `INT 21h, AX=FF00h, DX=0078h` 回傳固定 oracle 的
  `EAX=4734FFFFh`，並設定 `GS=0020h`；
- 其他中斷、相同中斷的其他參數、錯誤順序均失敗即關閉；
- 服務保留其他一般暫存器，不宣稱模擬 DOS/4GW 載入器或 DPMI。

`cpu386.CPU` 增加可觀察的六個 segment selector 槽位；此切片只允許服務設定
`GS`，尚不實作 selector descriptor、segment base 或 segment memory。

## 2. 指令契約

在既有 32-bit default operand 模式新增兩個已證實形狀：

- `3C ib`：`CMP AL,imm8`，依 8-bit 減法更新 `CF/PF/AF/ZF/SF/OF`，不改 AL；
- `74 cb`：`JZ rel8`，只在 `ZF=1` 時套用有號 8-bit 位移。

其他 `Jcc`、segment 指令與未列形狀仍回錯誤。

## 3. 驗收

- 合成測試覆蓋 `CMP AL,0` 的相等／不相等旗標及 `JZ rel8` 取分支／不取分支；
- 服務測試驗證兩次呼叫的輸入、輸出、`GS` 與未列呼叫拒絕；
- 固定雜湊 FD2 從 LE entry 由服務執行，在有界步數內越過第二次 `INT 21h`，
  執行 `cmp al,0` 與 `jz`，並停在成功分支 `EIP=0x3CA7A`；
- 此結果只能稱為「DOS/4G installation branch entered」，不得稱為遊戲畫面已啟動。

2026-09-05 以固定雜湊 `FD2.EXE` 在 `golang:1.25-bookworm`、無網路容器中執行
`go test -count=1 ./...` 全數通過；`cmd/leprobe -execute-entry-prefix` 實際輸出
`steps=27 eip=0x3CA7A eax=0x4734FFFF gs=0x20`。因此本節列出的窄切片已符合規格；
`0x3CA7A` 後仍依 DRAFT 邊界失敗即關閉。

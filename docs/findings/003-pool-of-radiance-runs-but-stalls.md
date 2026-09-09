# 003 — 《Pool of Radiance》停在等 BIOS 時鐘的迴圈：載入器沒開中斷

日期：2026-09-05
狀態：**已解**（`internal/machine/loader.go`）

## 一句話

第二個案例（SSI《Pool of Radiance》DOS 版）的 `START.EXE` 一開始停在一個兩道
指令的自旋迴圈裡。原因不在 CPU，也不在 DOS 服務：**載入器沒有把 IF 打開**，
所以 IRQ0 一次都送不出去，BIOS 的時鐘永遠是 0，而程式在等它跳。

## 症狀

```
CS:IP ＝ 0622:0071  AX=0000
計時器：送出 0 次  IF=false
視訊模式 03h　開過的檔 0
```

`0622:0071` 與 `0622:0074` 兩道指令來回，AX 一直是 0、SP 不動。看起來像
「程式本來就沒事做」。

## 迴圈長什麼樣

`-peek 0622:0060:40` 拿到的位元組解出來是：

```
0069  8E C0        MOV ES, AX      ; ES = 0040（BIOS 資料區）
006B  BF 6C 00     MOV DI, 006C    ; 0040:006C ＝ BIOS tick counter
006E  26 8A 05     MOV AL, ES:[DI]
0071  26 3A 05     CMP AL, ES:[DI] ┐ 等 tick 變
0074  74 FB        JZ  0071        ┘
0076  26 8A 05     MOV AL, ES:[DI]
0079  B9 FF FF     MOV CX, FFFF
007C  E8 3F 02     CALL +023F      ; 校時
```

Turbo Pascal 的老套路：先等一次 tick 對齊，再跑一段固定圈數量速度。

## 為什麼 tick 不動

`machine.tick()` 這一行擋住了全部：

```go
if !m.irq0Pending || !m.CPU.Flag(cpu.IF) { return }
```

而 IF 從頭到尾是 0——`CPU.Reset` 做 `SetFlags(0)`，**而 `LoadEXE` 沒有補**。
真機上 DOS 把控制權交給程式的時候 IF 是 1。

`bumpBDATicks()` 本來就寫好了，只是永遠跑不到，所以 `0040:006C` 一直是 0。

## 修法

`LoadEXE` 設好暫存器之後 `c.SetFlags(c.Flags | cpu.IF)`。

`rich2` 沒踩到是因為它自己會開中斷；**第二個程式才把這個洞照出來**。

## 修完之後跑到哪

同一支 probe，6000 萬道指令：

```
計時器：送出 364 次  IF=true  int08 向量 038F:21BD（程式自己掛的）
開過的檔 103：game.ovr POOL.CFG 8x8d1.dax sqrpaci.dax comspr.dax …
寫過的 I/O 埠：020 040 042 043 061 0C0 3C4 3C5
A0000 非零像素 3430 / 64000
```

會掛自己的 `int 08h`、會載 overlay 與素材、會動 PIT／PIC／VGA 序列器，
而且**開始畫東西了**。

## 還沒解的

1. **視訊模式還報 03h。** `int 10h AH=00` 一次都沒被呼叫，所以
   `SetVideoMode` 沒跑到——**遊戲怎麼進到 mode 0Dh 的仍然沒解**。

   > **更正（2026-09-06）**：這一條原本寫「遊戲是直接寫 `3C4`／`3C5` 設 EGA」。
   > 量過之後不成立：`3C4` 只被設成索引 `02`（Map Mask），`3C5` 收到的只有
   > `01`／`02`／`04`／`08`／`0F`——那是**逐平面選擇**，不是設模式
   > （`docs/spec/007` §2.1）。

   **平面式 VRAM 已經做了**（`docs/spec/007`、`internal/machine/ega.go`）：
   `oracle.IndexedEGA()` 把四個平面組成 320×200 的色號陣列，
   SSI 的開場畫面畫得出來。
2. **`int 10h AH=08`（讀游標處的字元與屬性）沒實作**，被呼叫一次。

## 順手補的兩個工具改動

- `cmd/probe` 的摘要多印計時器狀態（送出幾次、IF、`int 08h`／`int 1Ch` 向量）。
  **「等 tick 的迴圈轉不出來」與「程式本來就沒事做」從 CS:IP 看起來一模一樣**，
  就是這一行把它們分開的。
- `-peek` 多一個通用形式 `<段>:<偏移>:<長度>`。原本只有 `ds:` 與「IDA 線性
  位址」兩種，而那兩種的基底都是 rich2 專屬的——拿別的程式的位址進去會回
  垃圾，而且不會報錯。

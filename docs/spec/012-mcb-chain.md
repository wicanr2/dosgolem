# 012：記憶體控制區塊鏈

狀態：`READY`

## 1. 契約

配置器的狀態要**發布到客體記憶體**，形狀是 DOS 的 MCB 鏈：

| 位移 | 長度 | 內容 |
|---|---|---|
| `+0` | 1 | 簽章：`M`（0x4D，後面還有）／`Z`（0x5A，鏈尾）|
| `+1` | 2 | 擁有者的 PSP 段；`0` 表示自由 |
| `+3` | 2 | 資料段數（不含 MCB 自己那一段）|
| `+5` | 3 | 保留，填 0 |
| `+8` | 8 | 程式名，填空白 |

區塊的資料從 `seg+1` 開始，下一個 MCB 在 `seg+1+size`。

鏈的範圍是 `PSPSeg−1` 到 `MemTop`：

```
PSPSeg−1   M  owner=PSPSeg  size=freeSeg−PSPSeg     ← PSP ＋ 映像
freeSeg    …  arena 的每一塊，自由的 owner=0
最後一塊   Z
```

`[HARD]` 三條，違反任何一條都會讓走鏈的程式得到**自洽但錯**的地圖：

1. 鏈上每一塊的 `size` 與擁有權要與配置器當下的狀態一致。
   `AH=48h`／`49h`／`4Ah` 之後立刻同步。
2. 鏈要連續無空隙（`seg + 1 + size == 下一個 seg`），終點正好是 `MemTop`。
3. MCB 只能寫在配置器保留給它的那一段。寫死位置會落進程式映像裡。

## 2. 為什麼要做

`docs/spec/009` §3 當時的判斷是「目前沒有觀測到走訪 MCB 鏈的程式，
先做會用到的部分」，並預期這種程式會以「讀 `AH=52h` 之後掃記憶體」
的形狀浮現。

實際的形狀不是那樣。智冠《三國演義》的走法是：

```
0583:3068   mov bx,0FFFFh / mov ah,48h / int 21h   ← 用「配置最大值」失敗的回傳當起點
            cmp al,7                               ← AL=7 才是「MCB 鏈壞掉」
            mov bx,ds:[13D0h]                      ← 自己的 PSP 段
            dec bx                                 ← 第一個 MCB
    迴圈：  mov ds,bx / mov ax,ds:[1]              ← 擁有者
            cmp ax,dx / je 繼續                     ← 是我的
            or ax,ax / jne 結束                     ← 別人的 → 停
            mov cx,bx                              ← 自由 → 記下第一塊
            inc bx / add bx,ds:[3]                 ← 下一個 MCB
            mov al,ds:[0] / cmp al,'M' / je 迴圈
            cmp al,'Z' / jne 重來
            sub bx,dx                              ← ★ 總共構得到多少段
```

**它沒有碰 `AH=52h`。** 起點是自己 PSP 裡的段值，鏈是它自己一段一段爬的。
爬完的 `bx − PSP` 直接當成「可用記憶體總量」傳給載入函式。

同一支程式也用同一條鏈找自己的區塊來釋放（`06DD:32C3`、`0583:3215`）。

## 3. 驗收

1. `internal/dos.TestMCBChainMirrorsArena`：走完鏈能對上配置器的每一塊，
   終點是 `MemTop`。
2. `internal/dos.TestMCBChainDoesNotClobberImage`：`PSPSeg+0x2000` 的內容不被動到。
3. 智冠《三國演義》`AA.EXE` 走到開 `DATA2.IDX`／`DATA2.NAM`／`DATA3.*`／`DATA2.GRP`。

## 4. 這一版沒做的

- `AH=58h`（配置策略：first／best／last fit、UMB 連結）。目前固定 first fit。
- 子程序的擁有權：`AH=4Bh AL=00` 會建立新的 PSP，鏈上要記不同的擁有者。
  目前只實作 `AL=03`（overlay），共用同一個 PSP。
- MCB 的 `+8` 程式名一律填空白。有程式拿它顯示「誰佔著記憶體」，但沒觀測到。

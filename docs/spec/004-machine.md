
## 2.2 BIOS 計時器 stub 的還原順序（`int 08h` → `int 1Ch`）

`int 08h` 的預設處理是 StubSeg 裡一段真的程式碼（`machine.biosTimerOff`）。
它的**還原順序**要照 IBM PC BIOS 的 `TIMER_INT`：

```
push ds / push ax / push dx
mov ax,40h / mov ds,ax
inc word [6Ch] / jnz +4 / inc word [6Eh]
int 1Ch
pop dx / pop ax / pop ds
iret
```

**還原在 `int 1Ch` 之後，不是之前。** `int 1Ch` 的處理常式是應用程式掛的，
它弄髒 DS／AX／DX 都由 BIOS 這三個 `pop` 收拾。80186 之後常見的寫法是
`pusha`／`popa`，而**那兩道不含段暫存器**——掛 1Ch 的程式因此可以
理直氣壯地 `mov ds, 自己的段` 而不還原。

反面（在 `int 1Ch` 之前就 pop 完）沒有任何錯誤訊息：DS 漏回被中斷的
程式，它接著用錯的段讀資料，照樣一路執行下去。

> 實例（源平合戰 `MAIN.EXE`，2026-09-06）：它的 1Ch 是
> `cli / pusha / mov ds,2A1E … popa / sti / jmp far 舊向量`。DS 漏回
> 位元碼直譯器之後，直譯器改從 `2A1E:SI` 抓位元碼（原本是 `336E:SI`），
> 執行了四百道別的模組的位元碼，物件型別查表取到第 82 項（表只有 27 項），
> `DS` 變成 0，再過十九萬道指令才走進低位記憶體結束。
> **症狀離成因十九萬道遠，而且畫面停在一個正常的對話框上。**
> 修好之後遊戲跑進主選單並開始輪詢滑鼠。

# 013：檔案 handle 的號碼配置

狀態：`READY`

## 1. 契約

- handle 是 PSP 那張 job file table 的索引。`0`–`4` 是標準 handle
  （stdin／stdout／stderr／stdaux／stdprn），`5` 起才給程式。
- `AH=3Dh` 給出**最小的空號碼**。關檔（`AH=3Eh`）把那格標成空，
  下一次開檔會拿回同一個號碼。
- 上限預設 20（含標準 handle），`AH=67h` 可以調高。
- 沒有空格時 `AH=3Dh` 以錯誤 4（too many open files）失敗，
  **不是**給一個超出上限的號碼。

## 2. 為什麼上限不能省略

上限看起來像「擋人用的」，可以先不做。實際上它是**號碼配置的邊界**。

MSC 的低階 I/O 拿 handle 當自己那張表（`_osfile`／`_osfhnd`）的索引，
表的大小和 JFT 一樣。號碼超出範圍時 `fopen` 的行為是：

1. `_open` → `int 21h AH=3Dh` **成功**，拿到號碼
2. 發現號碼放不進自己的表 → `int 21h AH=3Eh` 把它關掉
3. 回 `NULL`

呼叫端看到的是「開檔失敗」，而且**檔案確實存在、確實開得起來**。
在服務層的追蹤裡它長這樣：

```
open  h=20 DATA2.GRP  → 20
close h=20
```

智冠《三國演義》就是死在這裡：`openGroup(1)` 做的是
`fopen(name,"rb")` → `fclose` → `fopen(name,"r+b")`，第二次回 NULL，
於是印 `ErrMsg: File #1  ErrNo: 9` 並以離開碼 1 結束。
它一路開開關關十幾個檔，號碼只增不重用就爬到了 20。

## 3. 驗收

1. `internal/dos.TestHandleNumbersAreReused`：關掉的號碼會被下一次開檔拿到；
   開到上限時回錯誤 4，不會給越界號碼。
2. 智冠《三國演義》`AA.EXE` 走到主選單（`docs/findings/` 的畫面）。

## 4. 這一版沒做的

- `AH=45h`（複製 handle）／`AH=46h`（強制複製）：JFT 的同一格會被兩個
  號碼指到，關掉一個不該把另一個也關掉。目前沒有觀測到。
- 每個號碼的存取模式（`AL` 的讀／寫／共用位元）一律當唯讀處理。

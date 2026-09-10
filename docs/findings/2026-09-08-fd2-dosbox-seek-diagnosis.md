# FD2 檔案定位後停止：DOSBox-X／dosgolem 交叉驗證

日期：2026-09-08。狀態：**原因已證實；CPU 修正尚未實作**。

## 結論

同一份原版自然執行到讀完 MDI.INI 的檔案定位呼叫，DOSBox-X 與 dosgolem
都以 AH=42h、AL=1、BX 低字組=5、CX:DX=0 呼叫 DOS，並成功回傳
DX:AX=0000:00DA（218 bytes）、CF=0。因此，本次停止不是該次定位服務回傳錯誤。

DOSBox-X 接著完成原始 bytes `66 36 89 07` 和 `66 36 89 57 02`：
先把 AX 寫入 SS:[EDI]，再把 DX 寫入 SS:[EDI+2]。dosgolem 在第一條指令
的 `36h` 解碼處拒絕執行；其目標記憶體仍未被寫入。

[機器收據](../evidence/fd2-dosbox-seek-20260908.json)保存輸入身分、雙方數值、
工具與原始紀錄雜湊。這是 DOSBox-X 的**輔助交叉基準**，不是遊戲畫面對拍
或一般玩家路徑通過的證據。

## 同一操作的量測

| 項目 | DOSBox-X | dosgolem |
|---|---|---|
| 呼叫位置 | CS=0158，EIP=001F2C1E | LE 線性 0003CC1E |
| 呼叫參數 | AX=4201，BX低字組=5，CX:DX=0 | 相同 |
| 回傳 | AX=00DA，DX=0，CF=0 | 相同 |
| 第一條寫入指令 | 0158:001F2C20 | 0003CC20 |
| SS | 0160，descriptor base=0 | 0160，既有平坦段契約 |
| EDI／ESP | 0019E318 | 00055338 |
| 寫入前的4 bytes | 00 00 21 00 | C2 28 05 00 |
| AX 寫入後 | DA 00 21 00 | 前綴解碼失敗，未寫入 |
| DX 寫入後 | DA 00 00 00 | 未抵達 |

**位址空間不可混用：**此次程式碼視窗的 DOSBox-X EIP 比 dosgolem／既有
IDA LE 線性定位多 `0x1B6000`；以連續16個 code bytes 核對。
這只是該 code 視窗的對應，不能拿同一偏移換算堆疊、資料物件或所有 selector。
兩邊的配置、暫存器高字組、初始 stack local 與部分旗標不同；本次只核對
定位呼叫的有效參數、CF 與回傳，以及緊接的 store，不宣稱整段初始化狀態一致。

## 直接原因與完整修正邊界

以主線 `fd746405cf0ef9e535121e215b935b0bf59cd1de` 的
`internal/cpu386/cpu.go` 為準，SHA-256
`ff3986067de7ae0e452cb35b166a2eaaa33718a5aab90d23af61a1cc389695a0`：

1. 第454行 prefix loop 只接受 66／26／F2／F3，沒有 36。
   因此此次直接錯誤是「opcode 尚未支援」，而非 SS 位址無效。
2. 第489行 segment override 白名單沒有 89。
   即使加上 36，下一層仍會拒絕該 MOV。
3. 第2237–2271行的 89 實作只列部分16位元形式；缺少這次
   `SS:[EDI]←AX` 與 `SS:[EDI+disp8]←DX` 的形式。

以上三項由程式碼確認；第一項另由實際 dosgolem 執行驗證。
不能僅略過36、改用32位元寫入、把SS當成一律等於DS，或注入成功值。
後續修正須支援16位元寬度、SS段選擇、無位移／有號disp8、保持暫存器與旗標，
並驗證非法段拒絕及相鄰記憶體不受影響。此輪只定位原因，未修改CPU正式路徑。

原始遊戲語意沿用 [RE 125](../re/125-fd2-dos-seek-mdi-ini.md)
的 IDA Pro 9.4 固定雜湊證據，不重建資料庫、不重命名任何原始定位。

## 可重現過程與限制

- 沿用 `fd2-dosbox-x:debug-0d7b272b`，實際版本
  DOSBox-X 2026.07.02 SDL2；normal CPU、fixed 12000 cycles、32 MB。
- 原版目錄唯讀掛載，Docker 內複製為可寫遊戲沙箱；未改原版 bytes、
  遊戲記憶體、亂數、角色或回合。
- 先以 `DEBUGBOX FD2.EXE` 停在 DOS 入口，再以 `BPINT 31 06 00`
  抵達受保護模式；此後才建立 `BP 0158:001F2C1E`。
  若在尚未建立 descriptor 的實模式直接設該斷點，會錯過目標，
  屬 debugger 設定問題，不是產品缺陷。
- 自然抵達 AH=42h 後設下一指令斷點，回讀 EV／SELINFO／MEMDUMP。
  清除斷點後用兩次 `LOG 2` 單步觀察。LOG 尾行可能包含下一條尚未執行
  指令，真正已寫入範圍以各次記憶體回讀及最後 EIP=001F2C29 判斷。
- 正式成功回放、commands.json、terminal.raw 與局部記憶體文字留於本機
  `workplace/fd2-input-parity-20260907/dosbox-seek-20260908/`；
  完整原始資料、terminal 或記憶體擷取不加入公開庫。
- 本輪 Docker 容器均有界並已結束／移除；未重建映像。無音訊或硬體週期一致性宣稱。

## 同日修正續記

上述診斷後已依 [185](../spec/185-cpu386-ss-word-store.md) 完成實作與驗收。
原先「尚未實作」是診斷當時狀態；目前已越過3CC20，下一阻塞為37381。
未重寫前次收據，也未將遊戲畫面門檻升級。

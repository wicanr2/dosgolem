package cpu

// 週期成本：讓時鐘不再是「一道指令算一格」。
//
// ⚠ **這不是週期精確**，是**按類別計費的近似**。目的很明確：讓「等 N 個
// 計時器 tick」的迴圈在模擬器上花掉的遊戲內時間，與真機差在合理的倍數內。
//
// 為什麼一道算一格不夠：真機上一道 `mov ax,bx` 是 2 個週期，一道
// `out dx,al` 是十幾個，一次 `rep movsb` 是每個 byte 幾個。老遊戲的繪圖
// 迴圈是「切平面（out）→ 讀改寫記憶體」的重複，全部照一道一格算，
// 等於把最貴的那幾種算得跟最便宜的一樣便宜。症狀不是報錯，是
// **畫面上的動畫比原版慢好幾倍，而每一步看起來都對**。
//
// 量過（源平合戰開場，`yuan/docs/re/003`）：一格扇面的繪圖，程式碼裡
// 等 50 個 tick，一道一格的模型下量到 171 個——多出來的 121 全是
// 「繪圖被算得太貴」。
//
// 數字取 80386 手冊的量級（暫存器對暫存器那一欄），不逐 opcode 抄——
// 逐 opcode 抄要有手冊在手邊逐條核對，抄錯了比近似更難發現。
// 這裡的每一個常數都標明它代表什麼，改的時候知道自己在改什麼。
const (
	// cycBase 是最便宜的一道：暫存器對暫存器的搬移與算術。
	cycBase = 2
	// cycPrefix 是每一個前綴（段覆寫、REP、0x66…）。
	cycPrefix = 1
	// cycMem 是「這道指令碰了記憶體運算元」再加的。
	// 位址算完還要走匯流排，386 上大約是這個量級。
	cycMem = 4
	// cycStack 是 push／pop 各一次堆疊存取。
	cycStack = 2
	// cycIO 是 in／out 一個 byte。**這是最貴的一類**，
	// 而 VGA 平面切換的迴圈一直在用它。
	cycIO = 14
	// cycJump 是跳成功再加的（管線要重灌）。跳不成功只算 cycBase。
	cycJump = 5
	// cycCall／cycRet 是近呼叫與返回。
	cycCall = 7
	cycRet  = 8
	// cycFar 是遠轉移與段暫存器載入：real mode 也要重載段快取。
	cycFar = 18
	// cycInt 是一次中斷：推三個字、查向量、遠跳。
	cycInt = 40
	// cycMul／cycDiv 是乘除法。8／16 位元差得不多，取一個量級。
	cycMul = 14
	cycDiv = 25
	// cycString 是字串指令**每一次迭代**（`rep movsb` 每個 byte）。
	cycString = 5
)

// charge 把成本直接加到累計上。
//
// ⚠ **不要改成「先累在本道指令、Step 結束再加總」。** 計時器中斷是由外層
// 在兩道指令之間注入的（`machine.tick` → `CPU.Interrupt`），那一次的成本
// 不屬於任何一道指令；累在「本道」的話會被下一道 Step 的歸零吃掉。
func (c *CPU) charge(n int) { c.Cycles += uint64(n) }

// in8／out8 是帶計費的埠存取。**CPU 內部一律走這兩支**，
// 直接呼叫 `c.Bus.In8` 會少算掉最貴的那一類。
func (c *CPU) in8(port uint16) uint8 {
	c.charge(cycIO)
	return c.Bus.In8(port)
}

func (c *CPU) out8(port uint16, v uint8) {
	c.charge(cycIO)
	c.Bus.Out8(port, v)
}

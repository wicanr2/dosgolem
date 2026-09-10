# 未完成項

<!-- 這一份由 `go run ./cmd/worklist render` 產生，不要手改。 -->
<!-- 權威是 docs/worklist.json；在這裡打勾，下一次 render 會蓋掉。 -->

每一條掛一個 `verify`：**跑起來為真＝這一條仍然未完成**。
核實用 `go run ./cmd/worklist verify`。

## consistency

同一件事有兩份不一致的實作或常數，還沒裁決哪一份對

### StepsPerSecond 有兩份標定，差 3.85 倍

`steps-per-second-two-calibrations`

speaker.go 的 StepsPerSecond() 把 DefaultIRQ0Every 當「分頻 65,536 時的間隔」，與 machine.go 的定義和 recalcIRQ0 依 PITDefaultDivisor 縮放的做法一致，算出來約 3.0 M 指令／秒。pit.go 的 parityStepsPerSecond 把它當「rich2 那個 17,000 分頻下的間隔」，約 11.6 M。

兩份都在用：前者換算喇叭波形的取樣時間（oracle/speaker.go、cmd/probe 的 -dump-wav），後者是 PITStepsPerTick 的基準、被 pit_test 釘住。2026-09-10 合併 perf/rich2-oracle-hotpath 與 san1-draw-speed-and-speech 時撞名，當時只把後者改成未匯出以避開撞名，沒有裁決。

**卡在**：要裁決得先確定 DefaultIRQ0Every 這個常數到底標定在哪個分頻上——那要回去看當初對拍是怎麼量的。

**怎樣算做完**：只剩一份標定，另一份改成從它推導；docs/spec 寫清楚 DefaultIRQ0Every 對應哪個分頻，以及為什麼。

**核實**：`present` ／ `還沒有裁決哪一個對`

### 8253 的輸入頻率有三個常數

`pit-input-hz-three-constants`

machine.go 的 PITHz ＝ 1_193_182、speaker.go 的 pitInputHz ＝ 1193182（同值，四捨五入）、pit.go 的 PITBaseHz ＝ 315e6/264（精確值 1193181.8181…）。三者描述的是同一個硬體常數。

差 3×10⁻⁷ 在單次換算上看不出來，但 PITHz 用在 recalcIRQ0 的週期計算（乘上 CPUHz 再除），而 PITBaseHz 用在 parityStepsPerSecond；拿兩邊的結果對拍時差額會浮出來。這是三條分支各自帶進來的，不是誰刻意要兩種精度。

**怎樣算做完**：只剩一個常數，值取精確的 315e6/264；改動前後跑一次既有測試確認沒有數值回歸。

**核實**：`present` ／ `pitInputHz\s*=`

## correctness

模擬出來的行為與真機不符，程式會據此做錯決定

### 段:偏移 到不了 HMA，A20 開關對定址沒有作用

`hma-unreachable-by-seg-off`

cpu.Addr(seg, off) 回傳前做 `& 0xFFFFF`，把位址遮成 8086 的 20 條位址線。真機上那個環繞是**匯流排**的行為：8086 只有 20 條線所以環繞，286 之後多的那條線由 A20 gate 控制，打開之後 FFFF:0010 就是線性 0x100000（HMA 的第一個位元組）。遮在 CPU 裡等於把閘門焊死在關的位置。

後果不是「偵測不到 A20」而已。internal/dos/xms.go 完整實作了 HMA：AH=00h 回報 DX=1（HMA 存在）、AH=01h 讓程式配置它、AH=03h–06h 開關 A20，而且那裡的註解寫著「A20 沒開的話 1 MB 之上環繞回 0，程式寫進去的東西會蓋掉中斷向量表」。實際情況是那個災難**無條件發生**：程式拿到 HMA、開了 A20，往 FFFF:xxxx 寫的每一個位元組都落在 0000:xxxx，蓋掉的正是中斷向量表。

Machine.Read8／Write8 已經有 hmaAddr 分支，但它只在位址 ≥ MemSize 時才成立，而經過 cpu.Addr 的位址永遠 < MemSize，所以那條分支只有拿線性位址直接呼叫（測試、工具）時才進得去。

**怎樣算做完**：A20 開著時，段:偏移 形式的讀、寫與取指令都到得了 HMA（FFFF:0010 ＝ 線性 0x100000）；A20 關著時維持 1 MB 環繞。XMS 那條路的 Request HMA → 寫入 → 讀回拿得到寫進去的值，而中斷向量表不被蓋。

**核實**：`present` ／ `到不了 1 MB 之上`

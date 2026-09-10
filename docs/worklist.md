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

## 已解決

核實不掃這一段——它記的是歷史，不是斷言。

### 段:偏移 到不了 HMA，A20 開關對定址沒有作用（2026-09-10）

`hma-unreachable-by-seg-off`

位址遮罩從 CPU 移到匯流排：新增 cpu.Linear（不遮），CPU 內部六處記憶體存取原語改用它；cpu.Addr 維持遮 20 位不動，給算線性位址比對的地方用。取指令快路徑在 A20 開著時關掉（Mem[] 蓋不住 HMA），執行護欄放行 1 MB 以上。規格 docs/spec/189-a20-gate-and-hma-addressing.md。

**被什麼抓到**：verify 自己開口：修好之後 machine.go 那句「到不了 1 MB 之上」的自承跟著改掉，present 就找不到 pattern 了。

### 8253 的輸入頻率有三個常數（2026-09-10）

`pit-input-hz-three-constants`

四份收成一份：留 pit.go 的 PITBaseHz（精確值 315e6/264），刪掉 machine.go 的 PITHz 常數與 speaker.go 的 pitInputHz。整數運算的地方改用分數 ×264/315e6，不把 PITBaseHz 轉成整數——uint64(PITBaseHz) 會截成 1,193,181，比四捨五入的 1,193,182 少 1，兩種寫法算出來的週期數差得出來。

**被什麼抓到**：verify 開口（找不到 pitInputHz 了）。動手時發現條目只數到三份：le_bios_clock.go 的定點累加裡還硬寫著第四份 1193182，那條路（LEMachine）不經過任何常數，grep 常數名找不到它。

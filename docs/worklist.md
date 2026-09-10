# 未完成項

<!-- 這一份由 `go run ./cmd/worklist render` 產生，不要手改。 -->
<!-- 權威是 docs/worklist.json；在這裡打勾，下一次 render 會蓋掉。 -->

每一條掛一個 `verify`：**跑起來為真＝這一條仍然未完成**。
核實用 `go run ./cmd/worklist verify`。

## consistency

同一件事有兩份不一致的實作或常數，還沒裁決哪一份對

### 週期時鐘與指令數時鐘的隱含機器速度沒有互相推導過

`two-clocks-implied-speed-mismatch`

指令數時鐘的速度是 StepsPerSecond()（11.58 M 指令／秒，從 DefaultIRQ0Every 與它標定的 17,000 分頻反推，`docs/spec/190`）。週期時鐘是 DefaultCPUHz ＝ 33 MHz，為源平合戰的扇面校準的（`docs/spec/004` §5.2）。**兩者從來沒有對過表。**

2026-09-10 量測：`determinism_test` 的 spinner 是 9.67 週期／指令，換算成 3.41 M 指令／秒——與指令數那邊差 3.4 倍。但那段程式是 `in al,0x40`／`add [mem],al`／`jmp` 的三道迴圈，每三道就有一個 14 週期的 `in`，**不代表真實程式的指令混合**，所以這個數字只能說明「兩者可能差很多」，不能拿來校準。

`docs/spec/004` §5 已經寫明兩個時鐘是獨立模型、切換會讓相位移位，所以不一致本身不是錯的。但只要沒量過，就沒有人知道差多少——而切換時鐘的人會以為「只是相位移位」，實際上是整段時間軸縮放。

**怎樣算做完**：有一份有代表性的指令混合量測（跑真實程式而不是合成迴圈），據此說明兩個時鐘的隱含速度差多少、要不要讓它們對齊。結論寫進 `docs/spec/004` §5 或新規格。

**核實**：`present` ／ `沒有互相推導過`

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

### DefaultIRQ0Every 被當成 65,536 分頻的基準，但它是 17,000 分頻量出來的（2026-09-10）

`steps-per-second-two-calibrations`

裁決：165,000 是 17,000 分頻（《大富翁2》）下的一刻，不是 65,536 的基準。calibrationDivisor ＝ 17000 寫進 pit.go，StepsPerSecond 與 PITStepsPerTick 同源，recalcIRQ0 的指令數時鐘改走 stepsPerTick(base, 分頻)。speaker.go 那一份標定刪掉。各分頻的間隔：17,000 → 165,000（回到 2026-09-04 對拍收據的時序）、65,536 → 636,085、4,096 → 39,755。規格 docs/spec/190-irq0-calibration.md，同時更正 004 §5 的公式基準。

**被什麼抓到**：verify 開口（找不到「還沒有裁決哪一個對」了）。裁決的主要依據是史實：2026-09-04 量那個值的時候 IRQ0Every 是固定的（分頻跟隨是五天後才加的），而 rich2 寫 17,000——那次量的就是 17,000 分頻的一刻。佐證是 DefaultVGAFrameEvery：一次垂直回掃 165,000 道指令，而 mode 13h 的 70 Hz 是顯示硬體的事實、與 PIT 無關。

**推論等級**：強證據，非 confirmed。第一版規格另外列了兩條「支持」，事後查證都站不住，已在 docs/spec/190 §3 記明：PITStepsPerTick 那條是循環論證，CPU 週期那條實際量過方向相反且量測程式不具代表性。真正的獨立確認要等有原版的環境跑一次對拍（素材不在版控裡，[HARD]）。

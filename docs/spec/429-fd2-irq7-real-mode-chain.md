# 429：FD2的IRQ7實模式處理器與返回鏈

狀態：**READY**
日期：2026-10-15

## 問題與證據

合併後main81c85bebdefe2e29ea1d5dc2628b2081d464e927的FD2正常啟動，在第5780804步、
LE線性0x373FE拒收IRQ7。固定FD2.EXE為357074bytes，MD5 b97caf2239a27a896069d03549d96e1e，
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
actual IVT0F為67FF:06F3，實模式線性0x686E3；DOS保護模式0F與DPMI實模式0F皆零。
目前只證明已安裝的入口與待派送狀態，ISR是否返回尚未知。
主收據由fd2_re的docs/data/fd2-remake-toolchain-recovery-20261015.json保存。

使用者選擇先補完整IRQ7鏈再繼續章對拍，不恢復先前輪詢近似。
本規格的完整鏈是實際原版ISR進場、來源確認、EOI、IRET與caller續行；
時鐘、私有host堆疊與寄存器映射沿305的平台近似，不宣稱硬體逐週期或所有IRQ情境。

## 平台前提與範圍

[Open Watcom Programmer Guide的DOS/4GW中斷服務](https://open-watcom.github.io/open-watcom-1.9/pguide.html)
及[DPMI中斷契約](https://www.delorie.com/djgpp/doc/dpmi/ch4.4.html)定義預設實模式轉送與各模式向量。
重用[305-moo2-irq7-real-mode-passdown](305-moo2-irq7-real-mode-passdown.md)已驗的私有CPU、
低位堆疊及真實IVT框架。FD2沒有MOO2的IRQ0／滑鼠上下文，不為取得IRQ7而啟用其他作品能力。
不深挖原版ISR、DMA硬體driver或DOS4GW核心。

## 接線契約

- 增加明示的InstallLERealIRQ7入口，接收同一LEMachine、FD2StartupDOS與LEOPLPorts。
  三者的DPMI／CPU及已安裝BIOSClock須一致，重複安裝與缺件在改狀態前拒絕。
- 此入口只註冊實模式轉送上下文，不增加虛擬時間，不配置client記憶體，不安裝IRQ0或滑鼠。
  私有堆疊仍由305首次派送時配置並重用。
- 沿既有IF、PIC mask、pending、in-service、IRQ0／IRQ1互斥閘門。
  只在DOS保護模式0F及DPMI實模式0F皆零時消費實際absolute IVT0F。
  非零的其他向量仍拒收，不猜優先序，不改寫原版IVT。
- 由原版ISR的IN確認DSP及OUT完成EOI，必須真正IRET並恢復私有SS:SP才返回。
  成功／失敗均保存完整32位caller、六段、旗標、FPU及保護模式堆疊。
- 沿305的20000步上限、未知CPU／port／INT／HLT及錯誤frame拒收；失敗後不可繞過。
- 唯一apps/fd2/cmd/oracle明示安裝此橋接，保留IRQ7開始／完成／最後I/O邊界於診斷報告。
  不另建runner、不在容器動態改寫來源，不修改FD2的遊戲或heap規則。

## 驗收

1. 自製ISR驗證FD2上下文的真實入口、DSP確認、EOI、IRET、caller／FPU／堆疊保持與私有配置重用。
2. IF／mask／pending、其他向量、無效上下文、重複安裝及中途失敗都保留拒收；既有MOO2的305測試保持。
3. 固定EXE從自然LE入口越過原停止，原始ISR的入口／I/O／返回與下一caller有收據。
4. 同固定槽與受版控正常鍵盤計畫重新取得第一個輸入及後續操作。舊／新平台差異完整記錄，
   不用BOOT圖相同宣稱整段同狀態；未知消費端另開窄任務。

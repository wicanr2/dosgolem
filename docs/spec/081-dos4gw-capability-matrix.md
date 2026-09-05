# 081 — DOS/4GW 能力矩陣

狀態：**DRAFT**（能力一般化尚未完成）
日期：2026-09-06

dosgolem 的 DOS/4GW 目標是讓保護模式 DOS 程式可以被程式化觀測與決定性
重播，不是複製 DOS/4GW 載入器的實模式切換過程。每項能力只在有實際
binary 路徑與測試收據後才能標為已驗證。

| 能力 | 目前狀態 | 證據或限制 |
|---|---|---|
| MZ 內嵌 LE 偵測、object 載入與已知 fixup | 部分已驗證 | [`007`](007-linear-executable-intake.md)；未列 fixup 形狀失敗即關閉 |
| 32 位平坦 code/data/stack descriptor | 已驗證 | [`014`](014-dos4gw-flat-descriptors.md)；只允許已註冊 selector |
| 386 指令集 | 增量驗證 | 僅實作已有 READY 規格與執行路徑的 opcode，其餘失敗即關閉 |
| DPMI 與 DOS/4GW API | 增量驗證 | selector／安裝檢查與 `INT 31h/AX=0600h` 線性區域鎖定已通過固定 FD2 路徑；其他功能失敗即關閉 |
| Watcom C runtime 啟動 | 固定 FD2 增量驗證 | environment、near heap、`argv`、決定性 DOS 時間、`__CMain`、通用 stack probe、AIL DPMI 鎖定與 `getenv` 參數路徑已驗證 |
| 一般 DOS 檔案、鍵盤、計時、終止服務 | 待 binary 路徑驅動 | 不從 8086 核心的實作自動推定 386 平坦模式已可用 |
| VGA 索引畫面與輸入觀測 | 待 FD2 main 下一條垂直切片 | 目前尚無 FD2 權威畫面收據 |
| Sound Blaster／AdLib／MIDI | 未實作 | 硬體時序依公開規格近似，不以逐週期相同為目標 |
| 分頁、例外、虛擬記憶體與 extender 自身 UI | 未實作 | 只有實際目標程式消費時才開啟規格 |

## 當前收據

固定雜湊 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）已可由
LE entry `0x3C964` 執行 1094 步，經過 5 次已列 DOS 服務，自然停在
FD2 `main` 入口 `0x25BF4`。其後的增量收據已自然完成 AIL 程式／資料區
DPMI 鎖定，並進入 Watcom `getenv`，完成第一個 `strlen`，再經 AIL
preference 表格的索引讀寫、共用包裝器尾端與預設表格迴圈比較至
`0x3FA50`；此收據只證明這些已列啟動路徑，不證明一般
DOS/4GW 程式或 FD2 遊戲畫面已可執行。

## 下一個門檻

從 `main` 第一條玩家可見路徑向下執行，用 IDA 先確認初始化 caller／
consumer，再補必要的 386 指令、DOS 服務與 VGA 觀測層。當能產生固定
雜湊、固定輸入的第一張索引畫面收據時，才將畫面能力升級。

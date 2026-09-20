# Buck Rogers 診斷用 BIOS 多鍵排程

狀態：**CONFORMED**

本規格只擴充 `cmd/buckrogers-text-receipt` 的可重播診斷輸入，使同一固定狀態可依絕對
instruction step 排入多筆 BIOS 鍵。它不屬於 `MenuRequestWatcher`，不得成為正式遊戲輸入、
自動作答或狀態修改 API。

## 命令列契約

可重複使用：

```text
-bios-key-at STEP:SCAN_HEX:ASCII_HEX
```

- `STEP` 必須是非零 ASCII 十進位 `uint64`。
- `SCAN_HEX`、`ASCII_HEX` 必須各為恰好兩個小寫十六進位字元。
- 所有 generic schedule 與既有 `-bios-enter-at` 合併後按 step 排序；step 必須唯一。
- `until` 必須嚴格晚於最後一筆 step；否則在啟動 machine 前拒絕。
- 每筆只在 `machine.Steps >= STEP` 時呼叫一次 `PushBIOSKey(scan, ascii)`；鍵盤緩衝區滿時
  立即失敗，不重試、不丟棄也不移動下一筆。
- 執行結束時若仍有未排入鍵，收據失敗。

既有只使用 `-bios-enter-at` 的命令與 `bios_input` 字串必須保持相容。只要有 generic
schedule，另輸出按 step 排序的 `bios_keys` 陣列，每筆只含：

```text
queued_at, scan, ascii
```

不得在 metadata 猜測按鍵語意；Down／Up 等名稱只能由呼叫端依 IBM PC BIOS scan code
另行標註。排程資訊不是「原版已消費按鍵」證據；消費時機必須由原版狀態或後續畫面事件
另行量測。

可選 `-screen-out PATH` 在成功通過全部收據閘門後，使用既有 `machine.Indexed()` 將終態
320×200、64,000-byte palette-index framebuffer 寫到明示路徑。寫檔失敗必須使命令失敗；
不得以 PNG 轉換、縮放或 RGB 色彩取代原始 indexed 收據。

## 驗收

- parser 覆蓋有效值、零 step、十進位／十六進位格式、欄數、超界與大小寫。
- 合併器覆蓋排序、重複 step、既有 Enter 衝突與 `until` 邊界。
- 同一固定狀態的 Enter → Down → Up 排程重跑兩次，事件與 VRAM 收據一致。
- 全部正式 packages test／vet 與 receipt command race detector 通過後可標為 CONFORMED。

## Conformance 收據

2026-09-20 由同一 #99,999,999 固定狀態排入 Enter、Down、Up，兩次重播的 13 筆完成事件
逐欄相同；終態 64,000-byte indexed framebuffer SHA-256 同為
`d0f70a73b80b1998c0744ae2bb2903dba4783104fbfccc3ded41d70e8eacc1cd`。Down-only 同終點
framebuffer 與 steady 的差異恰為 `(24,24)–(79,39)`、832 pixels；Up 後逐位元回到 steady。
全部正式 packages test／vet 與 receipt command race detector 通過。

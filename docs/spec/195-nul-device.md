# 195 — NUL 字元裝置

狀態：**READY**（行為照 DOSBox-X 原始碼；觸發案例見 §2）
日期：2026-09-11
前置：[`013-file-handles`](013-file-handles.md)、[`007-dos-exec-overlay`](007-dos-exec-overlay.md)（EMMXXXX0 字元裝置）

---

## 1. 規則

檔名的 basename 去掉副檔名之後是 `NUL`（不分大小寫）時，`AH=3Dh`（開檔）與
`AH=3Ch`（建檔）不找目錄，直接配一個指向 NUL 裝置的 handle：

| 操作 | 結果 |
|---|---|
| `AH=3Fh` 讀 | 成功，讀到 0 bytes |
| `AH=40h` 寫 | 成功，回報寫了 CX bytes，內容丟棄 |
| `AH=42h` 移動檔案指標 | 成功，DX:AX ＝ 0 |
| `AH=44h AL=00h` 取裝置資訊 | DX ＝ `0084h`（bit 7 是字元裝置、bit 2 是 NUL） |
| `AH=3Eh` 關閉 | 成功 |

副檔名不影響判定（`NUL.LST` 也是 NUL 裝置），這是 DOS 比對裝置名的方式。
`EMMXXXX0` 維持 `007-dos-exec-overlay` 的既有行為，不在本規格範圍。

依據：DOSBox-X `src/dos/dos_devices.cpp` 的 `class device_NUL`（第 308 行起）：
`Read` 回 `*size = 0` 且成功、`Write` 成功、`Seek` 成功、
`GetInformation` 回 `DeviceInfoFlags::Device | DeviceInfoFlags::Nul`；
兩個旗標在 `include/dos_inc.h` 定義為 `1<<7` 與 `1<<2`。

## 2. 觸發案例

Turbo Assembler 2.5（`TASM.EXE`，Borland C++ 2.0 附帶）啟動時開 `NUL`。
dosgolem 回「找不到檔案」之後，TASM 在第 623 條指令以回傳碼 7 結束，沒有任何訊息
（`cmd/run -prog TASM.EXE -args " /mx STRLEN.ASM"`）。BCC 用 `-B` 編 `.CAS` 時會
EXEC TASM，所以同一個缺口也讓 BCC 的內嵌組語路徑失敗。

## 3. 驗收

1. 契約測試逐列驗 §1 的表；反面對照：同一支測試在修改前要失敗在開檔那一步。
2. `tools/go.sh test ./internal/dos` 全綠。
3. `TASM /mx STRLEN.ASM` 在 `cmd/run -cpu 8086` 下產出 `.OBJ`，回傳碼 0。

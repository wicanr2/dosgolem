# 443 — EMMXXXX0 的裝置資訊與輸出狀態

狀態：**READY**（行為照 DOSBox-X 原始碼；觸發案例見 §2）
日期：2026-10-22
前置：[`007-exec-ems`](007-exec-ems.md) §5（EMMXXXX0 字元裝置）、[`195-nul-device`](195-nul-device.md)

---

## 1. 規則

| 操作 | 結果 |
|---|---|
| 開 `EMMXXXX0` 後 `AH=44h AL=00h` 取裝置資訊 | DX ＝ `4080h`（bit 7 字元裝置、bit 14 支援 IOCTL），CF=0 |
| 任何字元裝置 handle 的 `AH=44h AL=07h` 取輸出狀態 | AL ＝ `FFh`（就緒），CF=0 |

開關、讀寫維持 `007-exec-ems` §5：讀回 EOF、寫丟棄。

依據：DOSBox-X（`e0b4287`）
`src/ints/ems.cpp` 第 249–251 行，`device_EMM::GetInformation` 回
`DeviceInfoFlags::Device | DeviceInfoFlags::IoctlSupport`；兩個旗標在
`include/dos_inc.h` 定義為 `1<<7` 與 `1<<14`。
`src/dos/dos_ioctl.cpp` 第 723–730 行，`AL=07h` 對非外部裝置回 `AL=0xFF`。

`007-exec-ems` §5 把 EMMXXXX0 的裝置資訊留 0，理由是當時的 launcher 只開關、
不做 IOCTL，屬於「沒證據不做」，不是為了既有收據而存在的機制。
回 0 等於宣告「這是檔案」，做標準偵測的程式會判定沒有 EMS 驅動。

## 2. 觸發案例

智冠中文版《魔法門 V》`XEEN.EXE`（SHA-256 前綴 `7908828d896c`）的初始化
`1113:1EC6`（載入後段位址）依 Ralf Brown 的標準 EMS 偵測：

1. `1113:2015` 開 `EMMXXXX0`；
2. `1113:2021` `AX=4400h`，`1113:2028` `and dl,80h`，bit 7 為 0 就跳失敗；
3. `1113:202D` `AX=4407h`，`1113:2034` AL 為 0 就跳失敗；
4. 之後才呼叫 `int 67h` 40h、41h、42h、43h。

修改前第 2 步失敗，函式回 1，程式顯示
`Darkside of Xeen Memory Requirements`（主記憶體與 EMS 缺額皆 0）後以
離開碼 1 結束，總共 136519 道指令。

## 3. 驗收

1. 契約測試逐列驗 §1 的表；反面對照：同一支測試在修改前要失敗。
2. `go test ./internal/dos` 全綠。
3. 整合：觸發案例越過 `1113:202B`、`1113:2036`，到達 `1113:205E` 的 `AH=43h` 配置。

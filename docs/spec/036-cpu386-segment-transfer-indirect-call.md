# 036 — 386 PUSH DS／POP ES 與 register indirect CALL

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`017`](017-dos4gw-selector-load-validation.md)、[`031`](031-cpu386-near-call.md)、[`RE 028`](../re/028-fd2-callback-segment-call.md)

- `1E` 以 32-bit stack slot 零擴展寫入 DS selector；`66 1E` 尚未支援。
- `07` 從 32-bit stack cell 取低 16 位，經 selector gate 驗證後載入 ES 並使 ESP+4；
  失敗不得修改 ES／ESP。`66 07` 尚未支援。
- `FF /2` 本切片只支援 register-direct 32-bit near indirect CALL；將下一個 EIP 經
  SS descriptor 寫入 ESP-4 後，才提交 ESP 與 register target EIP。
- 固定雜湊 FD2 從 `0x45DCF` 抵達 `0x3CBCC`，ES=`0x160`、ESP=`0x55698`、
  SS:`0x55698`=`0x45DD3`。

驗收包含 segment transfer、register indirect CALL return cell 與固定 callback 入口。

2026-09-06：上述單元測試與固定雜湊 FD2 callback 入口測試通過，抵達 `0x3CBCC`。

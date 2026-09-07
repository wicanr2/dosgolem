# dosgolem 知識庫

這裡放**跨程式通用**的介面知識：DOS／BIOS／保護模式擴充的契約、
dosgolem 目前做到哪裡、缺的那些各自值多少。與 `docs/spec/` 的分工是：

- `docs/spec/`：**這一支程式要什麼**，逐項標 DRAFT／READY，帶原版位址與 bytes。
- `docs/knowledge-base/`：**這個介面本來長什麼樣**，與程式無關；換一支程式照樣成立。

## 來源分級

寫進這裡的每一條都要標來源，等級由高到低：

| 等級 | 來源 | 用途 |
|---|---|---|
| 契約 | 公開規格（RBIL、LIM EMS 4.0、XMS 3.0、DPMI 1.0、IBM VGA／EGA 技術參考） | 決定「介面應該回什麼」 |
| 實作參考 | DOSBox-X 原始碼（本機樹 `~/cht/DOSBox-X-MCP-Debugger/dosbox-src`，commit `5fcf624`，2026-08-13） | 決定「真的被程式用到的是哪些子功能、邊界怎麼處理」 |
| 收據 | 原版執行檔的軌跡（`docs/spec/` 裡的位址與 bytes） | 決定「這一項現在要不要做」 |

⚠ **DOSBox-X 是實作參考，不是契約。** 它為了跑得動有不少寬鬆處置；
拿它當唯一依據會把它的取捨一起抄進來。兩者衝突時以公開規格為準，
並在條目裡寫清楚差在哪。

⚠ **推論等級要標**：confirmed（有收據）／強證據（規格 ＋ 實作參考一致）／
假說（只有一邊）／未知。

## 目錄

| 檔 | 內容 |
|---|---|
| [`010-dos-int21-coverage.md`](010-dos-int21-coverage.md) | `int 21h` 逐 AH 的涵蓋表與缺口優先序 |
| [`020-bios-services-coverage.md`](020-bios-services-coverage.md) | `int 10h`／`13h`／`16h`／`1Ah`／`33h` 的涵蓋表 |
| [`030-protected-mode-and-dos4gw.md`](030-protected-mode-and-dos4gw.md) | DOS/4GW 的兩條路線、成本與建議 |
| [`040-memory-ems-xms-vcpi.md`](040-memory-ems-xms-vcpi.md) | 傳統記憶體、EMS、XMS、VCPI 的分工與缺口 |

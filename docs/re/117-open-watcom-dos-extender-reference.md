# RE 117 — Open Watcom v2 DOS extender 參考基線

日期：2026-09-06  
用途：公開實作交叉驗證，不是 FD2 行為證據，也不直接複製程式碼

## 固定來源

- 上游：<https://github.com/open-watcom/open-watcom-v2>
- 檢查 commit：`b36230f83e0ad7c40fc682f2d2c10bc034efcf23`
- 專案根授權：Sybase Open Watcom Public License 1.0；個別再散布元件仍須讀取
  其自身授權。dosgolem 不直接納入第三方原始碼，避免形成未審查的授權混合。

## 可採用的證據層

| 上游路徑 | 可交叉驗證內容 | 限制 |
|---|---|---|
| `bld/causeway/asm/loadle.asm` | LE 載入、object／fixup 流程 | CauseWay，不等於 DOS/4GW 的逐位元實作 |
| `bld/causeway/asm/api.asm`、`int21h.asm` | extender API、DOS 服務轉接 | 只用於公開契約與控制流比較 |
| `bld/causeway/asm/ldt.asm`、`memory.asm` | descriptor 與記憶體管理 | 不自動證明 FD2 消費了全部功能 |
| `bld/wstub/` | Watcom 32 位 DOS 啟動 stub | 需與 FD2 bytes／IDA caller 對照 |
| `bld/clib/startup/c/dpmihost.c`、`bld/watcom/h/dpmi.h` | DPMI host 偵測與 API 常數 | 可取代無謂猜測，不取代 FD2 執行收據 |
| `bld/redist/dos4gw/` | DOS/4GW 文件與可再散布 binary | 不是完整 DOS/4GW 原始碼 |
| `bld/redist/dos32a/` | DOS/32A binary、文件與授權 | 不是本輪採用的原始碼基線 |

## 使用規則

1. DOS/4GW／DPMI／LE loader 的公開契約先查上述來源，再以 FD2 的固定雜湊、
   IDA 位址與實際 consumer 決定是否需要實作。
2. x86 CPU 指令語意依處理器規格；Open Watcom 只協助辨識編譯器與 runtime
   模式，不把特定程式碼排列誤當 ISA 規格。
3. 外部名稱、註解與原始碼不能把 FD2 的未知語意直接升級為已證實；仍須保留
   已證實／強推論／假說／未知分級。
4. 若日後需要移植任何程式碼，先另做授權與來源審查；目前只作潔淨室的行為及
   介面參考。

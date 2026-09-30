# 201 — MOO2 1.31 的 DOS/4G 啟動回傳

狀態：**CONFORMED**（僅限固定 1.31 原版、固定 DOSBox-X 輔助基準與兩次服務）
日期：2026-09-30

## 證據

原版輸入 `ORION2-1.31.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。輔助執行器為 `fd2-dosbox-x:debug-0d7b272b`，映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，內含 DOSBox-X 2026.07.02 SDL2 heavy debugger；設定 `core=normal`、`cycles=fixed 12000`、32 MB。原檔唯讀掛載並複製到一次性容器的 `/tmp/game/ORION2.EXE`，無其他遊戲資料；透過 Xvfb、PTY 與 `DEBUGBOX` 重播。可重播腳本及設定在 [`apps/moo2/tools/startup_probe_131.py`](../../apps/moo2/tools/startup_probe_131.py) 與 [`dosbox.conf`](../../apps/moo2/tools/dosbox.conf)；容器內將 dosgolem 工作樹唯讀掛到 `/repo`、原版輸入唯讀掛到 `/input`、私有輸出目錄可寫掛到 `/shots`，由目前使用者 UID/GID 執行，Xvfb 設 `trap`，外層 `timeout 150s`。重生的 `startup-registers.json` SHA-256 為 `341ce45f695a350958a74c7a219b2bcf621ff1d86ac3cfb390d0726356da0a44`；完整命令及終端原始紀錄保留在 MOO2 專案未版控的 `workplace/dosbox-moo2/`。這是 **DOSBox-X 輔助基準**，正式 dosgolem 玩家路徑仍待重生。

DOSBox-X 使用 `CS:EIP`（**不是** dosgolem 的 LE 線性位址）。先以 `BPINT 21 30` 略過載入器呼叫，直到 `CS:EIP=0180:00333FB5`、`EAX=00003000`、`EBX=50484152` 的 MOO2 `CD 21`；再設同 selector 返回斷點 `0180:00333FB7`。其後用 `BPINT 21 FF 00` 命中 MOO2 `0180:00334054`，返回斷點 `0180:00334056`。只把同時符合 caller 指令 bytes、暫存器與返回 EIP 的快照列為證據；載入器的其他 `AH=30h` 命中已排除。

| 時點，DOSBox-X `CS:EIP` | EAX | EBX | ECX | EDX | DS／ES／FS／GS／SS | EFLAGS |
|---|---:|---:|---:|---:|---|---:|
| `0180:00333FB5`，`AH=30h` 前 | `00003000` | `50484152` | `0` | `0` | `0188／0028／0000／0020／0188` | `0246` |
| `0180:00333FB7`，返回 | `00000005` | `5048FF00` | `0` | `0` | `0188／0028／0000／0020／0188` | `0246` |
| `0180:00334054`，`AX=FF00h` 前 | `0000FF00` | `5048FF00` | `5` | `0078` | `0188／0028／0000／0020／0188` | `0297` |
| `0180:00334056`，返回 | `4734FFFF` | `5048FF00` | `5` | `0078` | `0188／0028／0000／0020／0188` | `0296` |

**已證實，固定輔助環境**：MOO2 1.31 第一個返回的 `EAX=5`、`EBX=5048FF00`、資料與堆疊 selector `0188`；第二個返回的 `EAX=4734FFFF`、`GS=0020`，並清 CF。先前 [200](200-moo2-provisional-protected-dos.md) 沿用 FD2 的 `AX=1606h`、`DS=SS=0160h` 是診斷近似，已被上述 MOO2 caller 返回反例取代。`ES:[2Ch]` 的環境 selector、PSP 內容、合成 `ORION2.EXE` 環境 bytes 仍**未知或近似**，不能隨本切片升格。1996 版尚無獨立 DOSBox-X 回傳證據；以 1.31 設定跑 1996 版只能標強推論／診斷。

## 實作與驗收

`NewMOO2StartupDOS` 只在前兩次明列呼叫套用固定 1.31 回傳；FD2 零值及 `NewFD2StartupDOS` 保持原有測試值。合成測試釘住兩次服務的暫存器、selector、CF 與環境越界；`go test ./internal/machine ./internal/cpu386` 通過。固定 1.31 原檔透過 dosgolem 此入口仍於第 65 步、dosgolem LE 線性位址 `0x1100FA` 停在未支援 `28 C0`，`EDX` 低 word 隨 `DS=0188` 修正；此步數只供工具診斷，**不是**正式 MOO2 玩家路徑對拍。未實作的 DOS 呼叫仍失敗即關閉。

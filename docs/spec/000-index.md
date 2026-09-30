# 規格索引

`docs/spec/` 是 dosgolem 的規格目錄。SDD 的規則沒變：**只有標 `READY`
的規格可以動手**，反組譯／量測 → 規格 → 才寫程式。

## 引用規格時要連號碼帶檔名

目前有 288 份規格，而編號**不是唯一鍵**：十條分支各自從 007 開始編，
合併之後同一個號碼底下有好幾份不同主題的文件。

| 號碼 | 底下有幾份 | 檔名 |
|---:|---:|---|
| `004` | 2 | `004-dos-bios-services.md`、`004-machine.md` |
| `007` | 6 | `007-com-loader-and-keyboard.md`、`007-dos-exec-overlay.md`、`007-ega-mode-0dh-planar-vram.md`、`007-exec-ems.md`、`007-linear-executable-intake.md`、`007-vga-planar.md` |
| `008` | 7 | `008-bios-keyboard-injection.md`、`008-blocking-console-input.md`、`008-ems-paging.md`、`008-eob1-adapter.md`、`008-flat-386-entry.md`、`008-tsr-resident.md`、`008-wolong-services.md` |
| `009` | 8 | `009-bios-time-of-day.md`、`009-exec.md`、`009-fd2-dos4g-install-check.md`、`009-keyboard-irq1.md`、`009-memory-allocator.md`、`009-mouse-event-handler.md`、`009-planar-vga.md`、`009-scratch-writes.md` |
| `010` | 6 | `010-dosv.md`、`010-eob1-title-menu.md`、`010-exec-memory-reclaim-and-planar-write-modes.md`、`010-fd2-segment-bootstrap.md`、`010-input-verbs.md`、`010-overlay-loading.md` |
| `011` | 5 | `011-bios-palette-and-vector-stubs.md`、`011-ega-mode10h.md`、`011-eob1-new-party-entry.md`、`011-fd2-es-environment-cell.md`、`011-xms.md` |
| `012` | 5 | `012-cpu-386-subset.md`、`012-eob1-first-character-race.md`、`012-fd2-parity-capture.md`、`012-input-keyboard-irq-and-mouse-scale.md`、`012-mcb-chain.md` |
| `013` | 5 | `013-eob1-first-character-class.md`、`013-fd2-load-es-selector.md`、`013-file-handles.md`、`013-mouse-event-callback.md`、`013-vga-planar.md` |
| `014` | 5 | `014-dos4gw-flat-descriptors.md`、`014-ems.md`、`014-eob1-first-character-alignment.md`、`014-hardware-keyboard.md`、`014-poke-state-not-luck.md` |
| `015` | 4 | `015-bytecode-observation.md`、`015-eob1-first-character-stats.md`、`015-fd2-store-environment-word.md`、`015-outermost-cmdline.md` |
| `016` | 3 | `016-dos4gw-protected-push.md`、`016-eob1-first-character-review.md`、`016-right-mouse-button.md` |
| `017` | 2 | `017-dos4gw-selector-load-validation.md`、`017-mouse-callback.md` |
| `018` | 2 | `018-cpu386-command-tail-prelude.md`、`018-eob1-first-character-name.md` |
| `019` | 2 | `019-dos4gw-psp-command-tail.md`、`019-eob1-first-character-alfa.md` |
| `020` | 2 | `020-cpu386-repe-scasb.md`、`020-eob1-second-character-race.md` |
| `021` | 2 | `021-cpu386-lea-disp8.md`、`021-eob1-second-character-review.md` |
| `022` | 2 | `022-dos4gw-selector-swap-jz.md`、`022-eob1-second-character-beta.md` |
| `023` | 2 | `023-cpu386-buffer-stack-finalize.md`、`023-eob1-third-character-review.md` |
| `024` | 2 | `024-dos4gw-environment-selector-load.md`、`024-eob1-third-character-gamma.md` |
| `025` | 2 | `025-dos4gw-environment-block.md`、`025-eob1-fourth-character-review.md` |
| `026` | 2 | `026-cpu386-environment-prefix-test.md`、`026-eob1-fourth-character-delta.md` |
| `027` | 2 | `027-cpu386-environment-byte-scan.md`、`027-dos-dta-find-first.md` |
| `028` | 2 | `028-cpu386-pop-ds.md`、`028-eob1-play-level1-entry.md` |
| `029` | 2 | `029-cpu386-startup-buffer-clear.md`、`029-eob1-level1-first-move.md` |
| `030` | 2 | `030-cpu386-startup-buffer-tail.md`、`030-eob1-level1-first-pickup.md` |
| `031` | 2 | `031-cpu386-near-call.md`、`031-eob1-level1-first-drop.md` |
| `032` | 2 | `032-cpu386-push-es.md`、`032-eob1-level1-camp-lifecycle.md` |
| `033` | 2 | `033-cpu386-register-cmp-jae.md`、`033-eob1-level1-memorize-spell.md` |

所以：**引用寫 `docs/spec/009-memory-allocator`，不要只寫 `docs/spec/009`**。
程式碼註解裡既有的「`docs/spec/NNN`」是各分支寫的，指的是那條分支自己的
那一份——照上表對回去（用寫那一行的子系統判斷），不要照號碼硬套。

> 沒有重編號是刻意的取捨：註解裡幾百處「`docs/spec/NNN`」在合併之後每一處
> 都有 2–7 個候選，重編號要逐處判斷它原本指哪一份。判錯的結果是一個
> **看起來對、其實指到別份規格**的指標，比號碼重複更難發現。

## 依案例分組

### 共同基礎（master）（14 份）

- `001-scope-and-mvp.md`
- `002-cpu-8086.md`
- `003-machine-and-loader.md`
- `004-dos-bios-services.md`
- `005-oracle-api.md`
- `006-layering.md`
- `187-frame-clock.md`
- `188-pit-divisor-decoding.md`
- `189-a20-gate-and-hma-addressing.md`
- `190-irq0-calibration.md`
- `191-two-clocks-relationship.md`
- `192-crtc-effects.md`
- `193-crtc-timing.md`
- `194-stubseg-font-collision-and-program-path.md`

### 銀河英雄傳說 3（logh3）（10 份）

- `007-exec-ems.md`
- `008-ems-paging.md`
- `009-planar-vga.md`
- `010-exec-memory-reclaim-and-planar-write-modes.md`
- `011-bios-palette-and-vector-stubs.md`
- `012-input-keyboard-irq-and-mouse-scale.md`
- `013-mouse-event-callback.md`
- `014-poke-state-not-luck.md`
- `015-outermost-cmdline.md`
- `016-right-mouse-button.md`

### 源平合戰（yuan-genpei）（9 份）

- `004-machine.md`
- `008-tsr-resident.md`
- `009-exec.md`
- `010-dosv.md`
- `011-xms.md`
- `012-cpu-386-subset.md`
- `013-vga-planar.md`
- `014-ems.md`
- `015-bytecode-observation.md`

### 三國演義（san1）（7 份）

- `008-blocking-console-input.md`
- `009-memory-allocator.md`
- `010-overlay-loading.md`
- `011-ega-mode10h.md`
- `012-mcb-chain.md`
- `013-file-handles.md`
- `014-hardware-keyboard.md`

### 臥龍傳（wolong）（4 份）

- `007-vga-planar.md`
- `008-wolong-services.md`
- `009-mouse-event-handler.md`
- `010-input-verbs.md`

### Eye of the Beholder（eob1）（28 份）

- `007-dos-exec-overlay.md`
- `008-eob1-adapter.md`
- `009-bios-time-of-day.md`
- `009-keyboard-irq1.md`
- `010-eob1-title-menu.md`
- `011-eob1-new-party-entry.md`
- `012-eob1-first-character-race.md`
- `013-eob1-first-character-class.md`
- `014-eob1-first-character-alignment.md`
- `015-eob1-first-character-stats.md`
- `016-eob1-first-character-review.md`
- `017-mouse-callback.md`
- `018-eob1-first-character-name.md`
- `019-eob1-first-character-alfa.md`
- `020-eob1-second-character-race.md`
- `021-eob1-second-character-review.md`
- `022-eob1-second-character-beta.md`
- `023-eob1-third-character-review.md`
- `024-eob1-third-character-gamma.md`
- `025-eob1-fourth-character-review.md`
- `026-eob1-fourth-character-delta.md`
- `027-dos-dta-find-first.md`
- `028-eob1-play-level1-entry.md`
- `029-eob1-level1-first-move.md`
- `030-eob1-level1-first-pickup.md`
- `031-eob1-level1-first-drop.md`
- `032-eob1-level1-camp-lifecycle.md`
- `033-eob1-level1-memorize-spell.md`

### Pool of Radiance（por）（3 份）

- `007-ega-mode-0dh-planar-vram.md`
- `008-bios-keyboard-injection.md`
- `009-scratch-writes.md`

### UCSD p-System（psys）（1 份）

- `007-com-loader-and-keyboard.md`

### 風之谷 2／DOS4GW（fd2）（177 份）

- `007-linear-executable-intake.md`
- `008-flat-386-entry.md`
- `009-fd2-dos4g-install-check.md`
- `010-fd2-segment-bootstrap.md`
- `011-fd2-es-environment-cell.md`
- `012-fd2-parity-capture.md`
- `013-fd2-load-es-selector.md`
- `014-dos4gw-flat-descriptors.md`
- `015-fd2-store-environment-word.md`
- `016-dos4gw-protected-push.md`
- `017-dos4gw-selector-load-validation.md`
- `018-cpu386-command-tail-prelude.md`
- `019-dos4gw-psp-command-tail.md`
- `020-cpu386-repe-scasb.md`
- `021-cpu386-lea-disp8.md`
- `022-dos4gw-selector-swap-jz.md`
- `023-cpu386-buffer-stack-finalize.md`
- `024-dos4gw-environment-selector-load.md`
- `025-dos4gw-environment-block.md`
- `026-cpu386-environment-prefix-test.md`
- `027-cpu386-environment-byte-scan.md`
- `028-cpu386-pop-ds.md`
- `029-cpu386-startup-buffer-clear.md`
- `030-cpu386-startup-buffer-tail.md`
- `031-cpu386-near-call.md`
- `032-cpu386-push-es.md`
- `033-cpu386-register-cmp-jae.md`
- `034-cpu386-byte-record-scan.md`
- `035-cpu386-callback-pointer-gate.md`
- `036-cpu386-segment-transfer-indirect-call.md`
- `037-cpu386-near-jump.md`
- `038-cpu386-x87-init-control.md`
- `039-cpu386-x87-control-return.md`
- `040-cpu386-record-status-write.md`
- `041-cpu386-absolute-byte-cmp.md`
- `042-cpu386-register-byte-xor.md`
- `043-fd2-second-callback-x87-store.md`
- `044-fd2-second-callback-x87-class-gate.md`
- `045-cpu386-movzx-absolute-word.md`
- `046-fd2-control-baseline-dispatch.md`
- `047-cpu386-x87-self-test.md`
- `048-fd2-second-callback-class-result.md`
- `049-fd2-second-callback-record-state.md`
- `050-fd2-third-callback-selection.md`
- `051-cpu386-push-pop-fs.md`
- `052-cpu386-group83-sub-cmp.md`
- `053-cpu386-lfs-absolute.md`
- `054-cpu386-stack-mov-xor.md`
- `055-cpu386-environment-es-byte-gate.md`
- `056-cpu386-sub-stack-memory.md`
- `057-watcom-near-heap-runtime.md`
- `058-cpu386-register-test.md`
- `059-cpu386-register-shl.md`
- `060-cpu386-register-add.md`
- `061-fd2-environment-copy.md`
- `062-cpu386-mov-eax-moffs.md`
- `063-cpu386-sib-immediate-store.md`
- `064-cpu386-push-sign-extended-byte.md`
- `065-watcom-memset-runtime.md`
- `066-cpu386-leave.md`
- `067-cpu386-null-data-selector.md`
- `068-cpu386-absolute-byte-and-or.md`
- `069-cpu386-cmp-base-disp8.md`
- `070-cpu386-store-base-disp8.md`
- `071-cpu386-load-register-absolute.md`
- `072-cpu386-store-register-indirect.md`
- `073-cpu386-store-immediate-absolute.md`
- `074-watcom-init-argv-runtime.md`
- `075-dos-current-time-deterministic.md`
- `076-cpu386-register-cmp.md`
- `077-cpu386-register-byte-cmp.md`
- `078-cpu386-sub-register-absolute.md`
- `079-cpu386-sub-register.md`
- `080-cpu386-push-absolute-dword.md`
- `081-dos4gw-capability-matrix.md`
- `082-cpu386-push-immediate-dword.md`
- `083-cpu386-xchg-register-stack-disp8.md`
- `084-cpu386-neg-register.md`
- `085-cpu386-cmp-register-absolute.md`
- `086-cpu386-jbe-short.md`
- `087-cpu386-load-register-stack-disp8.md`
- `088-cpu386-ret-immediate.md`
- `089-cpu386-store-immediate-stack-sib.md`
- `090-cpu386-store-register-stack-disp8.md`
- `091-cpu386-and-eax-immediate.md`
- `092-cpu386-and-register-immediate.md`
- `093-cpu386-lea-stack-disp8.md`
- `094-watcom-int386-dpmi-lock.md`
- `095-cpu386-cmp-stack-disp8-immediate.md`
- `096-cpu386-setz-register-byte.md`
- `097-cpu386-add-register-stack-disp8.md`
- `098-cpu386-push-base-disp8-dword.md`
- `099-cpu386-repne-scasb.md`
- `100-cpu386-not-register.md`
- `101-cpu386-mov-absolute-indexed-sib.md`
- `102-cpu386-mov-to-absolute-indexed-sib.md`
- `103-cpu386-dec-absolute-dword.md`
- `104-cpu386-cmp-register-imm8.md`
- `105-cpu386-jl-short.md`
- `106-cpu386-pushfd.md`
- `107-cpu386-cli.md`
- `108-cpu386-store-segment-absolute-word.md`
- `109-cpu386-load-segment-absolute-word.md`
- `110-dpmi-get-real-mode-interrupt-vector.md`
- `111-cpu386-register-mov16.md`
- `112-dos386-interrupt-vectors.md`
- `113-cpu386-store-segment-register16.md`
- `114-cpu386-load-segment-register16.md`
- `115-cpu386-inc-absolute-dword.md`
- `116-cpu386-store-register-base-disp32.md`
- `117-cpu386-store-immediate-base-disp32.md`
- `118-cpu386-cmp-base-disp32-immediate.md`
- `119-cpu386-short-jb.md`
- `120-cpu386-load-register-base-disp32.md`
- `121-cpu386-cmp-base-disp8-immediate32.md`
- `122-cpu386-port-output.md`
- `123-cpu386-test-byte-base-disp8.md`
- `124-cpu386-popfd.md`
- `125-cpu386-sub-register-immediate32.md`
- `126-cpu386-lea-stack-disp32.md`
- `127-cpu386-mov-stack-disp32-read.md`
- `128-cpu386-cmp-register-immediate32.md`
- `129-cpu386-and-byte-base-disp8.md`
- `130-cpu386-movzx-byte-esi.md`
- `131-cpu386-jg-short.md`
- `132-cpu386-or-al-immediate8.md`
- `133-cpu386-or-register8-immediate8.md`
- `134-cpu386-or-base-disp8-register.md`
- `135-cpu386-movzx-byte-eax.md`
- `136-cpu386-mov-byte-base-disp8-register.md`
- `137-cpu386-or-register8-base-disp8.md`
- `138-cpu386-mov-immediate-base-disp8.md`
- `139-read-only-dos-file-provider.md`
- `140-fd2-le-dos-open-readonly.md`
- `141-cpu386-rcl-ror-one.md`
- `142-cpu386-movzx-register-word.md`
- `143-cpu386-test-byte-sib-disp8.md`
- `144-cpu386-or-byte-sib-disp8.md`
- `145-cpu386-mov-register-word-store.md`
- `146-fd2-le-dos-ioctl-device-info.md`
- `147-cpu386-test-register-byte.md`
- `148-cpu386-setnz-register-byte.md`
- `149-cpu386-movzx-register-byte.md`
- `150-cpu386-load-dword-scaled-index.md`
- `151-cpu386-store-dword-scaled-index.md`
- `152-cpu386-short-jle.md`
- `153-cpu386-dec-dword-base-disp8.md`
- `154-cpu386-short-jge.md`
- `155-cpu386-or-byte-base-disp8.md`
- `156-cpu386-push-base-dword.md`
- `157-fd2-le-dos-read.md`
- `158-cpu386-inc-dword-base.md`
- `159-cpu386-load-register-base.md`
- `160-cpu386-store-register-byte-base.md`
- `161-cpu386-load-register-byte-base.md`
- `162-cpu386-load-register-byte-sib-disp32.md`
- `163-cpu386-inc-register-byte.md`
- `164-cpu386-test-byte-base-disp32.md`
- `165-cpu386-store-register-byte-sib-disp32.md`
- `166-cpu386-store-register-stack-disp32.md`
- `167-cpu386-store-immediate-byte-base.md`
- `168-cpu386-cmp-byte-base.md`
- `169-cpu386-add-byte-register.md`
- `170-cpu386-test-byte-register.md`
- `171-cpu386-store-dword-stack-base.md`
- `172-cpu386-store-immediate-stack-disp8.md`
- `173-cpu386-lea-edi-ebp.md`
- `174-cpu386-cmp-register-stack-disp8.md`
- `175-cpu386-movzx-byte-ebx-disp32.md`
- `176-cpu386-load-al-edi-ebp.md`
- `177-cpu386-load-eax-stack-base.md`
- `178-cpu386-imul-eax-stack-disp8.md`
- `179-cpu386-store-ax-stack-disp32.md`
- `180-cpu386-neg-stack-disp8.md`
- `181-cpu386-compare-ebx-ss-ebp-disp8.md`
- `182-cpu386-compare-edx-ds-eax-disp8.md`
- `183-fd2-le-dos-seek.md`
- `184-mvp-scope-review.md`
- `185-keyboard-trace-and-keypad-names.md`


- [185 — SS 段覆寫的16位元 MOV 記憶體寫入](185-cpu386-ss-word-store.md)：CONFORMED。

- [186 — FD2平台缺口持續驗證](186-fd2-platform-gap-continuation.md)：原版標題與BIOS單次方向鍵已驗證，正常START進王宮；目前CPU缺口、音訊與輸入限制統一見186檔首。

- [189 — A20 閘門與 HMA 的定址](189-a20-gate-and-hma-addressing.md)：READY。位址遮罩從 CPU 移到匯流排——`段:偏移` 到得了 1 MB 之上，A20 關著時才環繞。

- [190 — 計時器間隔的標定](190-irq0-calibration.md)：READY。165,000 是 17,000 分頻的一刻，不是 65,536 的基準；更正 `004` §5 的公式基準。

- [191 — 兩個時鐘的關係](191-two-clocks-relationship.md)：READY。比值隨指令混合走（3.25～7.40），不是常數；不強制對齊，改成把關係做成看得到的觀測。

- [192 — CRTC 的效果](192-crtc-effects.md)：READY。列距（offset）與分割畫面（line compare）；少了它們畫面會斜成平行四邊形或狀態列跟著捲走。

- [193 — 時序暫存器](193-crtc-timing.md)：READY。從 CRTC 算掃描位置，`3DA` 回報真的回掃狀態；**預設不啟用**，既有對拍收據建立在行為模型上。
- [194 — StubSeg 字型 stub 撞號與程式路徑](194-stubseg-font-collision-and-program-path.md)：READY。字型 stub 從 0x20／0x24 移到 0x410／0x414，不再蓋掉向量 8、9 的預設 stub；`oracle.Options.ProgramPath` 與 `probe -program-path` 可指定 argv[0]。
- [195 — 明示偏移的 LE 載入探針](195-moo2-explicit-le-offset.md)：CONFORMED，僅限明示標頭解析及零基址載入；MOO2 資料頁勘誤見 197。
- [196 — MOO2 入口的 16 位元絕對位址比較](196-cpu386-moo2-word-cmp-absolute.md)：CONFORMED，僅限通用 `66 3B 15` 指令形狀；它不是 MOO2 真入口，見 197。
- [197 — 內嵌 MZ 的 LE 資料頁基址](197-bound-mz-le-file-base.md)：CONFORMED。MOO2 第二層 MZ `0x26654` 的 LE 資料頁偏移加上該層基址，真入口恢復 `EB 76 WATCOM`。
- [198 — MOO2 啟動鏈的 16 位暫存器 OR](198-cpu386-or-word-register.md)：CONFORMED。只補 `66 09 /r`、`mod=11` 的低 16 位 OR 與旗標；FD2 服務層測跑只作診斷。
- [199 — MOO2 啟動鏈的段覆寫指令形狀](199-cpu386-moo2-segment-prefixes.md)：CONFORMED。補 ES 覆寫的絕對位址 segment word store，以及無記憶體運算元的 `3E BA`；FD2 服務層測跑只作診斷。
- [200 — MOO2 暫定保護模式 DOS 服務入口](200-moo2-provisional-protected-dos.md)：CONFORMED，僅限合成環境與獨立入口；原先借用的 FD2 回傳已由 201 訂正。
- [201 — MOO2 1.31 的 DOS/4G 啟動回傳](201-moo2-dos4g-startup-returns.md)：CONFORMED，固定 DOSBox-X 輔助基準另涵蓋 DPMI `AX=0006h` 的零基底分支；PSP／環境與正式玩家路徑仍未知。
- [202 — MOO2 啟動鏈的 8 位暫存器 SUB](202-cpu386-moo2-sub-byte-register.md)：CONFORMED，僅限 `28 /r` 的暫存器形狀；合成環境診斷至下個通用段載入缺口。
- [203 — MOO2 啟動鏈的 ES 覆寫段載入](203-cpu386-es-word-segment-load.md)：CONFORMED。`26 66 8E 1D` 的 16 位 selector 絕對位址讀取，保留載入與段界限檢查。
- [204 — 暫存器立即數 MOV 前的 DS 前綴](204-cpu386-ds-prefix-register-immediate.md)：CONFORMED。`3E B8..BF` 不含記憶體運算元，前綴不改立即數或旗標。
- [205 — ES 覆寫的 byte 暫存器／記憶體比較](205-cpu386-es-byte-compare-register-memory.md)：CONFORMED。`26 3A /r` 從 ES 讀取來源 byte，依既有 `sub8` 更新旗標。
- [206 — 32 位堆疊的 PUSH GS](206-cpu386-push-gs.md)：CONFORMED，僅限所列 32 位堆疊形狀；合成環境診斷抵達下一個通用 CPU 缺口。
- [207 — 16 位暫存器與符號延伸立即數 AND](207-cpu386-and-word-register-immediate.md)：CONFORMED。`66 83 /4` 的暫存器形狀；舊 `AH=4Ah` 停點已勘誤為未綁定 DPMI 的診斷錯誤。
- [208 — 保護模式 SBB r/m32,r32 的暫存器形狀](208-cpu386-sbb-rm32-register.md)：CONFORMED。固定原版命中 `19 C0`；通用暫存器形狀越過此處，下一停點為 `87 FA`。
- [209 — 保護模式 XCHG 暫存器與暫存器](209-cpu386-xchg-register-register.md)：CONFORMED。固定原版命中 `87 FA`；通用指令越過此處，下一停點為 `F5`。
- [210 — 保護模式 CMC 進位旗標反轉](210-cpu386-cmc.md)：CONFORMED。固定原版 `F5` 命中，`EFLAGS=0202h → 0203h`；下一停點為 `21 C8`。
- [211 — 保護模式 AND r/m32,r32 暫存器形狀](211-cpu386-and-rm32-register.md)：CONFORMED。同次原版指令 LOG 核對 `21 C8`；舊 EV 暫存器快照已撤回，下一停點為 `83 0E 01`。
- [212 — 保護模式 83 /1 記憶體 OR 立即數](212-cpu386-or-rm32-imm8-memory.md)：CONFORMED。原版同次 LOG 加記憶體前後擷取核對 `90h → 91h`；下一停點為 `0F A9`。
- [213 — 32 位堆疊的 POP GS](213-cpu386-pop-gs.md)：CONFORMED，限通用指令及 MOO2 `0020h → GS` 許可。原版同次 LOG 核對 ESP 加 4；合成診斷至下一停點 `0x153E84`。
- [214 — 16 位暫存器目的的 SBB r/m16,r16](214-cpu386-sbb-rm16-register.md)：CONFORMED，限 `66 19 /r mod=11`。固定原版同次 LOG 核對 CF=0、AX 清零；合成診斷至 `0x1005B` 的 `C8` 停點。
- [215 — 32 位堆疊的 ENTER 巢狀層級 0](215-cpu386-enter-level-zero.md)：CONFORMED。原版核對 EBP／ESP 與堆疊寫入；無額外讀取，合成診斷至 `0x109FF` 的 `66 3B 4D CE` 停點。
- [216 — 16 位暫存器與 32 位位址記憶體 CMP](216-cpu386-cmp-word-register-memory.md)：CONFORMED。原版同次 LOG 核對 SS 記憶體與旗標；合成診斷至 `0x126570` 的 `66 A9 89 CF` 停點。
- [217 — 16 位累加器的 TEST 立即數](217-cpu386-test-ax-imm16.md)：CONFORMED。原版同次 LOG 核對 `66 A9 89 CF` 的 EAX 與旗標；合成診斷至 `0x139A53` 的 DOS 服務停點。
- [218 — 受保護模式 DOS 設定 DTA 指標](218-protected-dos-set-dta.md)：CONFORMED。原版 `AH=1Ah` 返回與下一次 `AH=4Eh` 已擷取；合成診斷至 `0x139A59`。
- [219 — 受保護模式 DOS 的精確檔名首次搜尋](219-protected-dos-findfirst-exact.md)：CONFORMED。缺檔與固定空檔兩種原版輔助收據；合成診斷至 `0x148224`。
- [220 — 無前綴 byte 記憶體目的 CMP](220-cpu386-cmp-rm8-register.md)：CONFORMED。`38 10` 原版同次 LOG 已擷取；合成診斷至 `0x146903`。
- [221 — ES 覆寫的 byte 記憶體來源 MOV](221-cpu386-mov-r8-es-memory.md)：CONFORMED。`26 8A 1E` 原版同次 LOG 已擷取；合成診斷至 `0x14822D`。
- [222 — 32 位暫存器 ROR 立即數 8](222-cpu386-ror-r32-imm8.md)：CONFORMED。原版零輸入 LOG 與 dosgolem 原檔整合測試通過；合成診斷至 `0x14701B`。
- [223 — 8 位元暫存器目的的 OR r/m8,r8](223-cpu386-or-rm8-register.md)：CONFORMED。原版 `08 E0` 的 AH 零輸入 LOG 與 dosgolem 原檔整合測試通過；合成診斷至 `0x15C1DF`。
- [224 — LEA 的段前綴不參與有效位址](224-cpu386-lea-segment-prefix.md)：CONFORMED。原版 `2E 8D 86` 與 dosgolem 各自狀態的 CPU 收據通過；後續連續記錄已訂正單點斷點的呼叫次數，對應來源 EAX／ESI 一致；合成診斷至 `0x15C1E7`。
- [225 — DS:[EBX] 的 word 載入 ES](225-cpu386-mov-es-ds-ebx.md)：CONFORMED。固定原檔第 5808 步越過 `8E 03`，合成診斷至 `INT 33h`；不宣稱滑鼠服務或玩家路徑完成。
- [226 — MOO2 保護模式滑鼠位置查詢](226-moo2-protected-mouse-query.md)：CONFORMED。固定原檔 `AX=3` 回傳與 record 消費端通過；合成診斷至 `8F 47 14`，尚無 GUI 玩家對拍。
- [227 — POP dword 至 DS:[EDI+disp8]](227-cpu386-pop-edi-disp8.md)：CONFORMED。固定原檔越過 `8F 47 14`，合成診斷至 `66 8C 03`；原版非零來源未獨立擷取。

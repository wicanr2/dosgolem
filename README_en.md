# dosgolem

*[中文版](README.md)*

**A DOS runner that only has to run one binary**: headless, deterministic,
importable as a Go package. It exists for [`rich2`](https://github.com/wicanr2/rich2),
a remake of *Richman 2* (Softstar, 1993, DOS).

## Why not DOSBox-X

DOSBox-X is built for a person sitting in front of it. The screen goes to X,
input arrives as keyboard events, and observing means taking a screenshot.
For playing a game, that is exactly right.

An AI agent working on a remake does something else: it compares the remake
against the original, field by field. That needs answers you can **query,
measure, and replay** — not pictures you can look at. "What rent did this tile
charge?" can only be inferred from pixels through a screenshot. "Run until the
rent routine, then stop" can only be approximated by sleeping two seconds and
hoping.

dosgolem is a DOS runner for that job: it lets "what does the original do here"
be measured inside `go test` instead of watched through X.

## The name

dosgolem = DOS golem. A golem is built to do work on your behalf, which is what
this tool does in an AI workflow — and "go" is already in the word.

## What the current approach costs

`rich2` drives the original through docker + Xvfb + DOSBox + xdotool +
screenshots. That has grown to 54 scripts and 6,116 lines, each one
re-implementing "send a key → sleep → screenshot → judge pixels".

| Measurement | Value |
|---|---|
| One `docker exec` round trip | 0.058 s |
| Sending one key | 1 exec + 0.35 s sleep |
| Taking one screenshot | 2 execs + 0.7 s sleep |
| One step of a self-driving script | 2.2 s sleep |

The bottleneck is not IPC. A 0.058 s round trip sits next to 0.35–2.2 s of
waiting, a ratio of 6 to 38. Those sleeps exist for one reason: **the host has
no way to ask the emulator "are you done with this frame yet?", so it guesses
using the wall clock.** That is both why it is slow and why it is flaky.

Four costs follow from it:

- Judgments can only read pixels. "Cyan text inside a menu frame" also matches
  the cyan-green buildings on the board.
- Reaching a rare screen is expensive. 150 turns of dice rolls never landed on
  the courthouse; the workaround was editing a save file.
- Phase does not line up. Palette entries 240–249 and 250–254 cycle, so
  comparing a single screenshot yields false conclusions.
- Random sequences do not correspond. The original's BASIC `RANDOMIZE` and the
  remake's RNG have no replayable mapping.

Rewriting DOSBox-X would not fix this. It is 943,452 lines of C++ across 4,501
source files and 479 MB, and the problem is "you can only look from outside",
not the emulation itself. Its debugger is an ncurses UI rather than a
programmable API, so modifying it still means wrapping it in another layer.

Full assessment (in Chinese):
[`rich2/docs/spec/082`](https://github.com/wicanr2/rich2/blob/master/docs/spec/082-parity-oracle-emulator.md).

### DOSBox-X stays

It is the timing reference and the cross-check oracle. Every frame dosgolem
produces has to be verified against a DOSBox-X indexed screenshot before it
counts — otherwise it is checking itself against itself.

## Why this is feasible

Three measurements:

1. The 52,892-byte main program area of `RUN_full.EXE` uses **62 mnemonics**:
   8086 plus two 80186 instructions — `PUSH imm16` and `PUSH imm8`, 3,345
   occurrences in the main program area and 5,280 across the whole file.
   No protected mode, no paging, no 32-bit operands.
2. **No x87 needed.** All 876 `INT 34h`–`3Dh` calls go to the Microsoft
   floating-point emulator, which is linked into the binary; at runtime only
   integer instructions execute.
3. The system-service surface is narrow, and `rich2` already walked it once
   with unicorn, reaching the copy-protection screen.

## What it looks like

```go
o, _ := oracle.Load(exe, root)        // originals supplied by the player
o.RunUntil(oracle.PasswordScreen)     // returns an error if it never gets there
o.Click(102, 125)                     // aligned to instruction count, not sleep

snap := o.Save()                      // a 1 ms snapshot
o.Restore(snap)                       // branch the next variant from it

v := o.Word(o.DS(0x1BE))              // read the original's variable directly
shot := o.Indexed()                   // 320×200 colour indices, no X involved
```

`o.DS(0x1BE)` and `o.IDA(0x25BF6)` take the addresses exactly as they appear in
`rich2`'s reverse-engineering notes, with no further conversion. Judgments
therefore move from pixels to **the original's own call arguments** — `OnCall`
intercepts a drawing routine, which makes the original state "I printed string
229 at (154,54) in colour 60".

Parity becomes a `go test` that CI can run: 1.8 seconds in-process to reach the
copy-protection screen, and 5.3 seconds from a cold start — docker included —
through all three protection questions. The DOSBox route spends 25 seconds on
container start plus boot, then 2.2 seconds per step.

Interface is settled in [`docs/spec/005`](docs/spec/005-oracle-api.md) (READY).

## Where this is going

The end state is **an AI agent that can verify the remake on its own**, with
the original as the reference rather than a human reviewing screenshots
afterwards.

Three parts:

1. **Queryable.** Inside `go test`, the agent asks "when the original reaches
   this point, what is this variable, what colour index is this pixel, what
   arguments did it pass to the drawing routine" and gets an answer. Reading
   variables, reading the screen, and intercepting calls work today.
2. **Replayable.** The same input always produces the same frame. The clock is
   instruction count rather than wall time, and the random seed is aligned with
   the original's fixed-seed build — that is what makes MVP-B's pixel-exact
   match possible.
3. **Reachable.** Verifying a rare screen (courthouse, bankruptcy, a specific
   card) means the agent has to get there. Snapshots make "branch several
   variants from one state" a 1 ms operation; that is how the three
   copy-protection questions are answered automatically now.

With those three, `rich2`'s 54 DOSBox scripts and 6,116 lines collapse into one
declarative parity table that runs in CI.

The scope is still this one binary. Whether other DOS programs run is not a
goal — if they do, that is a side effect.

## Status

| | Milestone | State |
|---|---|---|
| MVP-A | 8086 integer core, SingleStepTests/8088 v2 all green | **323/323 files green**, one known gap below |
| MVP-B | Reach the copy-protection screen, pixel-identical to a DOSBox-X indexed screenshot | **64,000/64,000 = 100%** |
| M2 | Input and timing (keyboard / mouse / PIT) | Mouse, keyboard, and PIT all wired; **fourteen turns played automatically from a cold boot** — rolling, buying, paying rent, backing out of screens like the bank. Cycle-accurate timing not done |
| M3 | Instrumentation: breakpoints / watchpoints / call trace / RND log / savestate | `OnCall`, `Caller`, snapshots, and **RND tracing** work; breakpoints and watchpoints not done |
| M4 | Go API (`oracle` package) | **Usable**, [`docs/spec/005`](docs/spec/005-oracle-api.md) READY |
| M5 | Regression: re-run `rich2`'s existing parity receipts | Not started |

Specs live in [`docs/spec/`](docs/spec/), marked `DRAFT` or `READY`; only READY
ones may be implemented against.

### Wiring it into a remake project

`rich2` uses a Go workspace pointing at a local checkout rather than a `replace`
directive in `go.mod`:

```sh
# rich2/go.work (gitignored, only meaningful inside the container)
go 1.24.0
use .
use /dosgolem
```

Its `tools/go.sh` mounts dosgolem read-only when it finds it alongside. The
parity tests sit behind `-tags oracle`, so anyone without that mount runs
`go test ./...` and skips them rather than failing.

⚠ **Do not add a `require` for a version that does not exist.** Every package
then tries to reach the module proxy, and the build container runs with
`--network none` — the whole project stops compiling, with errors pointing at
files that have nothing to do with it.

The first parity test wired up is the palette: the original's runtime VGA DAC
against the remake's decoding of `256.PAT`. 30 of 256 entries differ, and where
they differ is meaningful — `192`–`206` are the fifteen counties of the Taiwan
map (a blue gradient in `256.PAT`, which the copy-protection screen repaints as
one flat green, leaving only the county being asked about in white), and
`240`–`254` are a cycling animation. **The check is therefore where the
differences land, not how many there are.**

The second is the random number generator. The original's BASIC
`RANDOMIZE TIMER` and the remake's seed had no correspondence, which is what
blocked `rich2`'s same-path parity. Hooking `RND` here answers it directly:
when `int 21h AH=2Ch` returns zero, **the initial state is `000000`**, so the
remake's `seed = 0` is the original fixed-seed build's starting point — across
the 216 draws from cold boot to the board, both sides produce identical states,
float values, and `INT(RND×6)` results, draw for draw.

`Caller()` also answers who is drawing: 150 for new-game setup (a 50-iteration
loop drawing three times each), 62 for the title animation, 4 for copy
protection. **The sequence itself is deterministic; what makes two sides
diverge is always who drew how many times, and when.**

The third is **data tables**. `Array` reads BASIC array descriptors (the first
two words are `(offset, segment)`, and indexing is **column-major**), so every
array in rich2's DIM table becomes readable. That gives the strongest possible
check on whether a decoder is right — one dataset, two paths:

| | remake path | original path | Result |
|---|---|---|---|
| Character stats | `SAVE_7.DSK` → container → decompress segment 0 | runtime `11A2h` | 360 cells, **0 differ** |
| Land value tables | same → `ParseLandTables` | runtime `1174h` | 144 cells, **0 differ** |
| Board array | same → `ParseBoard` | runtime `122Ch` | 5,660 cells, **0 differ** |
| Coord → square | `Board.SquareAt` scans board data | runtime-computed `11FEh` | 108 squares in use, **0 mismatch** |
| Land purchase | price table 0 | take a turn, buy, read what was actually charged | paid 2200, **found in the table** |
| **Movement (destination)** | reachable set of `Board.Exits` after N steps | take a turn, read start, dice roll, destination | 5 turns, **all hit** |
| **Movement (directions)** | `Board.Walk` fed the original's direction picks | intercept `11A32`／`11A87` for the pick sequence | 5 turns, **same destination**, and the sequence is **exhausted exactly** each turn — even the re-roll count matches |
| **Movement (square by square)** | the board's adjacency table | `MoveTrace.Trail` — every square the original stepped on | 5 turns, **every hop is an edge** |

| Rent | `RentBase(street, levels)` | land on someone else's property, read what was charged | 3 samples, **all identical** |

The behavioural checks sidestep the gap between "the original draws 28–74
random numbers per turn" and "the remake draws once" by reading the roll
itself (`ds:1B0h`).

The full list of what has and has **not** been checked lives in rich2's
[`docs/playtest/054`](https://github.com/wicanr2/rich2/blob/master/docs/playtest/054-dosgolem-parity-matrix.md).
Unchecked is not the same as wrong — but it is also not the same as checked.

That last row goes further than the two above it: matching tables only proves
the **decoder** is right; it proves **that number is the one the game actually
charges**. The whole run takes 15 seconds — cold boot to the board, click
"Move", answer the purchase dialog, all automated.

Getting decompression wrong by one byte does not raise an error; it just leaves
most values coincidentally correct. Previously the only check available was
"starting cash 25000 matches the screen", which covers two cells.

The MVP-B check is reproducible (originals supplied by the player):

```sh
# In rich2, produce the oracle: a DOSBox Ctrl+F5 indexed screenshot
tools/pyx.sh tools/dosbox_pw_indexed.py 2

# Here, compare
DOSGOLEM_ORIG=~/cht/rich2/workplace tools/parity.sh <oracle.png>
```

`RUN_full.EXE` is not pure 8086 — the main program area contains 3,345 80186
`PUSH imm` instructions. `machine.New()` therefore selects `Model80186`;
`cpu.New()` stays 8086, which is what the test corpus runs against. The trap
here is silent: 8086 treats `60`–`6F` as aliases of the conditional jumps, so
`68 FF 1F` decodes as `JS` instead of `PUSH imm16`, the instruction length is
off by one, and everything after it is misaligned with no error at all
(see [`docs/spec/002`](docs/spec/002-cpu-8086.md) §1.1).

The CPU is verified against
[SingleStepTests/8088](https://github.com/SingleStepTests/8088) v2:
**323 opcode files × 10,000 cases = 3.23 million cases**, each with register
and memory state before and after every bus cycle. The one unsolved case is
**the flags `IDIV` pushes on overflow**
([`docs/spec/002`](docs/spec/002-cpu-8086.md) §3.4). It is tracked as a
ceiling count: every run counts the mismatches and fails if the count rises, so
it still blocks regressions.

### Four behaviours that contradict the Intel manual

SingleStepTests is hardware-generated, so it wins where the manual disagrees.
Four came out of this:

1. `AAA`/`AAS`: the carry out of `AL + 6` **does not propagate into `AH`**. The
   manual's `AX += 106h` increments `AH` an extra time when `AL ≥ FAh`.
2. `DAA`/`DAS`: the second adjustment is **not** `old_AL > 99h OR old_CF`. Real
   hardware skips it for the six values where `old_AL` is 9Ah–9Fh and `AF` is
   already set on entry.
3. `D0`–`D3` with `/6` is **not** an alias of `SHL` on the 8086. It is the
   undocumented `SETMO`, which sets the destination to all ones; the alias only
   appears on the 186 and later.
4. Shift and rotate recompute `OF` **on every iteration**, not only for
   single-bit shifts. The `flags-mask` for `D2.3` (`RCR` by `CL`) is `FFFF` —
   not one bit is masked off.

The first two were checked against the full corpus (10,000 cases each), zero
mismatches. Following the manual on these points produces a program that runs
and is quietly wrong — which is the reason for using a hardware corpus as the
judge instead of transcribing the manual.

## Running it

Builds and tests go through docker; nothing is installed on the host:

```sh
tools/go.sh build ./...
tools/go.sh test ./...

tools/fetch_cputests.sh                  # fetch SingleStepTests/8088 v2 (761 MB)
tools/go.sh test ./internal/cpu -run TestSingleStep
```

The test corpus is **not** in version control and is not redistributed here; it
has its own licence. Without it the CPU tests skip rather than pretending to
pass.

## No original assets

This repository contains **no** `RUN.EXE`, `.PIX`, `.PAK`, or any other
*Richman 2* file. Running against the original requires your own legal copy.
Tests that need missing files skip, and there are **no stand-in assets** — a
silent substitute makes "not done yet" look like done.

## Licence

**RRSAL-1.0** (Retro Remake Source-Available Licence 1.0), full text in
[`LICENSE`](LICENSE). Non-commercial use, modification, and redistribution are
free and need no prior permission; commercial use requires a separate
arrangement via `wicanr2@gmail.com`. This is source-available, not OSI
open source, and the licence does not cover any original game assets.

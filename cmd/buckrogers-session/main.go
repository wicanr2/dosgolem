// Command buckrogers-session boots one sealed DOS session headless and
// runs bounded no-input turns: bootroot.Prepare, session.New,
// session.BootOriginal, then Deliver(empty) + Advance per turn.
//
// It is the composition root for the future Linux player loop, not a
// diagnostic probe: the original tree is only read, the save tree is a
// fresh copy, and the owner API is the only path to the machine.
// Game-agnostic: the executable name and hash come from flags.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/bootroot"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/session"
)

type turnReceipt struct {
	Turn              int    `json:"turn"`
	Epoch             uint64 `json:"epoch"`
	Steps             uint64 `json:"steps"`
	Reason            uint8  `json:"reason"`
	Phase             uint8  `json:"phase"`
	DOSCallsCommitted uint64 `json:"dos_calls_committed"`
	KeysPendingBefore int    `json:"keys_pending_before"`
	KeysPendingAfter  int    `json:"keys_pending_after"`
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "buckrogers-session:", err)
	os.Exit(1)
}

func main() {
	original := flag.String("original", "", "唯讀原版樹目錄")
	save := flag.String("save", "", "可寫 save 目錄（須不存在或為空）")
	exe := flag.String("exe", "", "開機執行檔名（必填）")
	exeSHA := flag.String("exe-sha256", "", "開機執行檔 SHA-256（hex，必填）")
	scale := flag.Uint("scale", 2, "起始倍率（2 或 3）")
	turns := flag.Uint("turns", 3, "無輸入回合數")
	steps := flag.Uint64("steps", 1000000, "每回合 instruction budget")
	keys := flag.String("keys", "", "每回合投遞的 BIOS 鍵（單字元，如空白鍵填 \" \"；空字串為無輸入）")
	flag.Parse()
	if *original == "" || *save == "" || *exe == "" || *exeSHA == "" {
		flag.Usage()
		os.Exit(2)
	}
	sum, err := hex.DecodeString(*exeSHA)
	if err != nil || len(sum) != 32 {
		fmt.Fprintln(os.Stderr, "buckrogers-session: exe-sha256 須為 64 hex 字元")
		os.Exit(2)
	}
	var want [32]byte
	copy(want[:], sum)
	var outputScale host.OutputScale
	switch *scale {
	case 2:
		outputScale = host.OutputScale2
	case 3:
		outputScale = host.OutputScale3
	default:
		fmt.Fprintln(os.Stderr, "buckrogers-session: scale 須為 2 或 3")
		os.Exit(2)
	}
	out, err := bootroot.Prepare(bootroot.BootRootInput{
		OriginalRoot: *original,
		SaveRoot:     *save,
		Required:     []bootroot.RequiredFile{{Name: *exe, SHA256: want}},
	})
	if err != nil {
		die(err)
	}
	exeBytes, err := os.ReadFile(filepath.Join(out.SaveRoot, *exe))
	if err != nil {
		die(err)
	}
	s := int(outputScale)
	chrome := 18 * s
	layout := host.MouseLayout{Epoch: 1, Scale: outputScale, ChromeHeight: chrome,
		Canvas:      host.Canvas{Width: 320, Height: 200},
		FrameWidth:  320 * s,
		FrameHeight: chrome + 200*s}
	owner, err := session.New(session.Config{InitialScale: outputScale, InitialLayout: layout})
	if err != nil {
		die(err)
	}
	receipt, err := owner.BootOriginal(session.BootInput{EXE: exeBytes, ExpectedEXESHA256: want, SaveRoot: out.SaveRoot})
	if err != nil {
		die(err)
	}
	if receipt.Phase != session.PhaseRunning {
		die(fmt.Errorf("開機未進入 Running: %+v", receipt))
	}
	encoder := json.NewEncoder(os.Stdout)
	var biosKeys []dos.Key
	if *keys != "" {
		for _, r := range *keys {
			key, ok := dos.KeyForRune(r)
			if !ok {
				fmt.Fprintf(os.Stderr, "buckrogers-session: 字元 %q 無 BIOS 鍵映射\n", r)
				os.Exit(2)
			}
			biosKeys = append(biosKeys, key)
		}
	}
	for turn := 1; turn <= int(*turns); turn++ {
		view, err := owner.View()
		if err != nil {
			die(err)
		}
		delivered, err := owner.Deliver(session.CapturedUpdate{StartedPanel: view.Panel, Layout: view.Layout, SourceGeneration: view.SourceGeneration, BIOSKeys: biosKeys})
		if err != nil {
			die(err)
		}
		tick, err := owner.Advance(session.InstructionBudget(*steps))
		if err != nil {
			die(err)
		}
		if err := encoder.Encode(turnReceipt{
			Turn: turn, Epoch: tick.Epoch, Steps: tick.Steps,
			Reason: uint8(tick.Reason), Phase: uint8(tick.Phase),
			DOSCallsCommitted: delivered.DOSCallsCommitted,
			KeysPendingBefore: tick.KeysPendingBefore, KeysPendingAfter: tick.KeysPendingAfter,
		}); err != nil {
			die(err)
		}
		if tick.Phase != session.PhaseRunning {
			break
		}
	}
}

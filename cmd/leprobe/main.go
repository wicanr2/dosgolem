package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu386"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	exe := flag.String("exe", "", "要檢查的 MZ／LE 執行檔（必填）")
	executeEntryPrefix := flag.Bool("execute-entry-prefix", false, "執行 docs/spec/008 定義的 386 entry 第一個中斷閘門")
	flag.Parse()
	if *exe == "" {
		flag.Usage()
		os.Exit(2)
	}
	b, err := os.ReadFile(*exe)
	if err != nil {
		die(err)
	}
	h, err := machine.InspectLE(b)
	if err != nil {
		die(err)
	}
	fmt.Printf("format=LE header_offset=0x%X cpu=%d os=%d pages=%d page_size=0x%X objects=%d\n", h.Offset, h.CPUType, h.OSType, h.ModulePages, h.PageSize, h.ObjectCount)
	fmt.Printf("entry=object:%d+0x%X stack=object:%d+0x%X execution_supported=false\n", h.EIPObject, h.EIP, h.ESPObject, h.ESP)
	totalFixups := 0
	pagesWithFixups := 0
	sourceTypes := map[uint8]int{}
	targetTypes := map[uint8]int{}
	sourceFlags := map[uint8]int{}
	targetFlags := map[uint8]int{}
	targetOrdinals := map[uint8]int{}
	for _, page := range h.Fixups {
		totalFixups += len(page)
		if len(page) != 0 {
			pagesWithFixups++
		}
		for _, fixup := range page {
			sourceTypes[fixup.SourceType]++
			targetTypes[fixup.TargetType]++
			sourceFlags[fixup.SourceFlags]++
			targetFlags[fixup.TargetFlags]++
			if fixup.Ordinal <= 0xff {
				targetOrdinals[uint8(fixup.Ordinal)]++
			}
		}
	}
	fmt.Printf("fixup_pages=%d pages_with_fixups=%d records=%d runtime_applied=false\n", len(h.Fixups), pagesWithFixups, totalFixups)
	printCounts("fixup_source_types", sourceTypes)
	printCounts("fixup_target_types", targetTypes)
	printCounts("fixup_source_flags", sourceFlags)
	printCounts("fixup_target_flags", targetFlags)
	printCounts("fixup_target_ordinals", targetOrdinals)
	relocated, err := h.RelocatedObjectImages(b)
	if err != nil {
		die(err)
	}
	for i, o := range h.Objects {
		image, err := h.ObjectImage(b, uint32(i+1))
		if err != nil {
			die(err)
		}
		fmt.Printf("object[%d] virtual_size=0x%X relocation_base=0x%X flags=0x%X page_index=%d page_count=%d reserved=0x%X image_bytes=%d relocation_preview_sha256=%x\n", i+1, o.VirtualSize, o.RelocationBase, o.Flags, o.PageTableIndex, o.PageCount, o.Reserved, len(image), sha256.Sum256(relocated[i]))
	}
	if *executeEntryPrefix {
		executePrefix(b)
	}
}

func executePrefix(data []byte) {
	m, err := machine.LoadLE(data)
	if err != nil {
		die(err)
	}
	interrupts := 0
	m.CPU.IntHook = func(c *cpu386.CPU, number uint8) bool {
		if number != 0x21 {
			return false
		}
		interrupts++
		if interrupts == 1 && uint8(c.R[cpu386.EAX]>>8) == 0x30 && c.R[cpu386.EBX] == 0x50484152 {
			c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | 0x1606
			return true
		}
		return interrupts == 2 && uint16(c.R[cpu386.EAX]) == 0xff00 && uint16(c.R[cpu386.EDX]) == 0x78
	}
	steps := 0
	for interrupts < 2 && steps < 40 {
		if err := m.CPU.Step(); err != nil {
			die(err)
		}
		steps++
	}
	if interrupts != 2 {
		die(fmt.Errorf("entry prefix 未在 40 steps 內到達 DOS/4GW installation check"))
	}
	stackA, err := m.Read32(0x52818)
	if err != nil {
		die(err)
	}
	stackB, err := m.Read32(0x52804)
	if err != nil {
		die(err)
	}
	startupWord, err := m.Read16(0x52810)
	if err != nil {
		die(err)
	}
	fmt.Printf("entry_prefix_executed=true steps=%d eip=0x%X esp=0x%X interrupts=2 last_ax=0x%X last_dx=0x%X stack_globals=0x%X,0x%X startup_word=0x%X\n",
		steps, m.CPU.EIP, m.CPU.R[cpu386.ESP], uint16(m.CPU.R[cpu386.EAX]), uint16(m.CPU.R[cpu386.EDX]), stackA, stackB, startupWord)
}

func printCounts(label string, counts map[uint8]int) {
	keys := make([]int, 0, len(counts))
	for key := range counts {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)
	fmt.Printf("%s=", label)
	for i, key := range keys {
		if i != 0 {
			fmt.Print(",")
		}
		fmt.Printf("%d:%d", key, counts[uint8(key)])
	}
	fmt.Println()
}

func die(err error) { fmt.Fprintln(os.Stderr, "leprobe:", err); os.Exit(1) }

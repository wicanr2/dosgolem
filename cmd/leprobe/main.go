package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func main() {
	exe := flag.String("exe", "", "要檢查的 MZ／LE 執行檔（必填）")
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

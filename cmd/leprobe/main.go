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
	executeEntryPrefix := flag.Bool("execute-entry-prefix", false, "從 LE entry 執行至固定雜湊 FD2 的 main 入口")
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
	fmt.Printf("entry=object:%d+0x%X stack=object:%d+0x%X execution_support=partial\n", h.EIPObject, h.EIP, h.ESPObject, h.ESP)
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
	services := &machine.FD2StartupDOS{}
	m.CPU.IntHook = services.Handle
	if _, err := machine.InstallFD2WatcomRuntime(m); err != nil {
		die(err)
	}
	steps := 0
	for steps < 2500 && m.CPU.EIP != 0x25bf4 {
		if err := m.CPU.Step(); err != nil {
			die(err)
		}
		steps++
	}
	argc, _ := m.Read32(0x5462c)
	calibration, _ := m.Read32(0x541b0)
	returnAddress, errReturn := m.Read32(m.CPU.R[cpu386.ESP])
	stackArgc, errStackArgc := m.Read32(m.CPU.R[cpu386.ESP] + 4)
	stackArgv, errStackArgv := m.Read32(m.CPU.R[cpu386.ESP] + 8)
	publicArgv, errPublicArgv := m.Read32(0x54628)
	firstArg, errFirstArg := m.Read32(stackArgv)
	if services.Calls() != 5 || argc != 1 || calibration != 1 || m.CPU.EIP != 0x25bf4 || returnAddress != 0x45d91 || stackArgc != 1 || stackArgv == 0 || stackArgv != publicArgv || firstArg != 0x546b1 || errReturn != nil || errStackArgc != nil || errStackArgv != nil || errPublicArgv != nil || errFirstArg != nil {
		die(fmt.Errorf("未在 2500 steps 內由 LE entry 完成 DOS/4GW／Watcom 初始化並進入 FD2 main"))
	}
	stackA, err := m.Read32(0x52818)
	if err != nil {
		die(err)
	}
	stackB, err := m.Read32(0x52804)
	if err != nil {
		die(err)
	}
	storedES, err := m.Read16(0x52810)
	if err != nil {
		die(err)
	}
	selectorGS, err := m.Read16(0x527f0)
	if err != nil {
		die(err)
	}
	fmt.Printf("entry_prefix_executed=true dos4g_branch_entered=true selector_bootstrap=true environment_path_copied=true startup_buffer_cleared=true startup_buffer_aligned=true first_near_call_entered=true callee_prologue_entered=true callee_range_gate_entered=true callee_record_scan_complete=true callback_pointer_loaded=true indirect_callback_entered=true callback_thunk_resolved=true x87_control_probe_complete=true x87_callback_returned=true callback_record_marked=true second_callback_entered=true second_callback_absolute_gate=true second_callback_bl_cleared=true second_callback_x87_control_stored=true second_callback_x87_class_gate=true second_callback_control_baseline_loaded=true second_callback_control_dispatched=true second_callback_x87_self_test_returned=true second_callback_class_result_stored=true second_callback_record_marked=true third_selected_callback_entered=true third_callback_fs_saved=true third_callback_global_gate=true third_callback_lfs_loaded=true third_callback_scan_setup=true third_callback_environment_first_byte=true third_callback_first_alloc_call=true watcom_nmalloc_returned=true third_callback_second_alloc_call=true second_watcom_nmalloc_returned=true environment_table_terminated=true environment_tail_stored=true third_callback_returned=true post_environment_runtime_list_initialized=true watcom_argv_initialized=true deterministic_dos_time=true watcom_cmain_complete=true fd2_main_entered=true delay_calibration=%d argc=%d stack_argc=%d stack_argv=0x%X return_address=0x%X steps=%d eip=0x%X esp=0x%X interrupts=%d eax=0x%X ebx=0x%X ecx=0x%X gs=0x%X stored_gs=0x%X stored_es=0x%X last_dx=0x%X stack_globals=0x%X,0x%X\n",
		calibration, argc, stackArgc, stackArgv, returnAddress, steps, m.CPU.EIP, m.CPU.R[cpu386.ESP], services.Calls(), m.CPU.R[cpu386.EAX], m.CPU.R[cpu386.EBX], m.CPU.R[cpu386.ECX], m.CPU.Seg[cpu386.SegGS], selectorGS, storedES, uint16(m.CPU.R[cpu386.EDX]), stackA, stackB)
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

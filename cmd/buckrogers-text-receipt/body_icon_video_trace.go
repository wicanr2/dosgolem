package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/machine"
)

const bodyIconA000DigestSchema = "v1:ordinal-u64le,step-u64le,cs-u16le,ip-u16le,offset-u32le,value-u8,write-mode-u8;rect digest appends key-count-u16le and sorted UTF-8 key length-u16le plus bytes"

type bodyIconVideoWriteMetaJSON struct {
	Ordinal      uint64             `json:"ordinal"`
	Step         uint64             `json:"step"`
	Instruction  buckrogers.Address `json:"instruction"`
	VideoSegment uint16             `json:"video_segment"`
	VideoOffset  uint16             `json:"video_offset"`
	WriteMode    uint8              `json:"write_mode"`
}

type bodyIconKeyEarliestJSON struct {
	EventKey string                     `json:"event_key"`
	Writes   uint64                     `json:"writes"`
	First    bodyIconVideoWriteMetaJSON `json:"first"`
}

type bodyIconA000GroupJSON struct {
	Instruction    buckrogers.Address         `json:"instruction"`
	WriteMode      uint8                      `json:"write_mode"`
	EventKeys      []string                   `json:"event_keys"`
	Writes         uint64                     `json:"writes"`
	First          bodyIconVideoWriteMetaJSON `json:"first"`
	Last           bodyIconVideoWriteMetaJSON `json:"last"`
	MinOffset      uint16                     `json:"min_offset"`
	MaxOffset      uint16                     `json:"max_offset"`
	SequenceSHA256 string                     `json:"sequence_sha256"`
}

type bodyIconA000SpanJSON struct {
	A000OrdinalStart uint64             `json:"a000_ordinal_start"`
	A000OrdinalEnd   uint64             `json:"a000_ordinal_end"`
	RectOrdinalStart uint64             `json:"rect_ordinal_start"`
	RectOrdinalEnd   uint64             `json:"rect_ordinal_end"`
	StepStart        uint64             `json:"step_start"`
	StepEnd          uint64             `json:"step_end"`
	Instruction      buckrogers.Address `json:"instruction"`
	WriteMode        uint8              `json:"write_mode"`
	EventKeys        []string           `json:"event_keys"`
	VideoOffsetStart uint16             `json:"video_offset_start"`
	VideoOffsetEnd   uint16             `json:"video_offset_end"`
	Writes           uint64             `json:"writes"`
	SequenceSHA256   string             `json:"sequence_sha256"`
}

type bodyIconA000WindowJSON struct {
	AfterStep  uint64                    `json:"after_step"`
	FirstByKey []bodyIconKeyEarliestJSON `json:"first_by_key"`
}

type bodyIconA000TraceJSON struct {
	DigestSchema                   string                      `json:"digest_schema"`
	TraceFromStep                  uint64                      `json:"trace_from_step"`
	TraceUntilStep                 uint64                      `json:"trace_until_step"`
	A000WriteCount                 uint64                      `json:"a000_write_count"`
	A000SequenceSHA256             string                      `json:"a000_sequence_sha256"`
	RectIntersectingWriteCount     uint64                      `json:"rect_intersecting_write_count"`
	RectIntersectingSequenceSHA256 string                      `json:"rect_intersecting_sequence_sha256"`
	GlobalEarliest                 *bodyIconVideoWriteMetaJSON `json:"global_earliest_intersection,omitempty"`
	PerKeyEarliest                 []bodyIconKeyEarliestJSON   `json:"per_key_earliest_intersection,omitempty"`
	FirstAfterSteps                []bodyIconA000WindowJSON    `json:"first_after_steps,omitempty"`
	WriterGroups                   []bodyIconA000GroupJSON     `json:"writer_groups,omitempty"`
	ContiguousSpans                []bodyIconA000SpanJSON      `json:"contiguous_spans,omitempty"`
}

type bodyIconA000GroupAccumulator struct {
	json   bodyIconA000GroupJSON
	digest hash.Hash
}

type bodyIconA000SpanAccumulator struct {
	json            bodyIconA000SpanJSON
	lastA000Ordinal uint64
	lastOffset      uint32
	digest          hash.Hash
}

type bodyIconA000Observer struct {
	rects       []bodyIconRect
	from        uint64
	until       uint64
	allCount    uint64
	rectCount   uint64
	allDigest   hash.Hash
	rectDigest  hash.Hash
	globalFirst *bodyIconVideoWriteMetaJSON
	perKey      map[string]*bodyIconKeyEarliestJSON
	groups      map[string]*bodyIconA000GroupAccumulator
	spans       []bodyIconA000SpanAccumulator
	currentSpan int
	afterSteps  []uint64
	firstAfter  map[uint64]map[string]*bodyIconKeyEarliestJSON
}

type bodyIconTraceAfterSteps []uint64

func (s *bodyIconTraceAfterSteps) String() string {
	parts := make([]string, len(*s))
	for i, step := range *s {
		parts[i] = strconv.FormatUint(step, 10)
	}
	return strings.Join(parts, ",")
}

func (s *bodyIconTraceAfterSteps) Set(value string) error {
	step, err := strconv.ParseUint(value, 10, 64)
	if err != nil || step == 0 {
		return fmt.Errorf("body-icon-prewrite-after-step must be a positive decimal step")
	}
	*s = append(*s, step)
	return nil
}

func validateBodyIconAfterSteps(enabled bool, steps []uint64, from, until uint64) error {
	if len(steps) > 0 && !enabled {
		return fmt.Errorf("body-icon-prewrite-after-step requires body-icon-framebuffer-trace")
	}
	seen := make(map[uint64]bool, len(steps))
	for _, step := range steps {
		if step < from || step >= until || seen[step] {
			return fmt.Errorf("body-icon-prewrite-after-step must be unique and inside trace interval")
		}
		seen[step] = true
	}
	return nil
}

func newBodyIconA000Observer(rects []bodyIconRect, from, until uint64, afterSteps []uint64) *bodyIconA000Observer {
	steps := append([]uint64(nil), afterSteps...)
	sort.Slice(steps, func(i, j int) bool { return steps[i] < steps[j] })
	return &bodyIconA000Observer{
		rects: rects, from: from, until: until,
		allDigest: sha256.New(), rectDigest: sha256.New(),
		perKey:      make(map[string]*bodyIconKeyEarliestJSON),
		groups:      make(map[string]*bodyIconA000GroupAccumulator),
		currentSpan: -1, afterSteps: steps,
		firstAfter: make(map[uint64]map[string]*bodyIconKeyEarliestJSON),
	}
}

func (o *bodyIconA000Observer) Observe(w machine.VideoWrite) {
	if w.Step < o.from || w.Step >= o.until {
		return
	}
	if w.Offset > 0xffff {
		panic(fmt.Sprintf("A000 observer offset out of range: %#x", w.Offset))
	}
	o.allCount++
	ordinal := o.allCount
	meta := bodyIconVideoWriteMetaJSON{
		Ordinal: ordinal, Step: w.Step,
		Instruction:  buckrogers.Address{Segment: w.CS, Offset: w.IP},
		VideoSegment: 0xA000, VideoOffset: uint16(w.Offset), WriteMode: w.WriteMode,
	}
	writeBodyIconA000Record(o.allDigest, ordinal, w)
	keys := bodyIconSpanIntersections(o.rects, uint16(w.Offset), 1)
	if len(keys) == 0 {
		o.currentSpan = -1
		return
	}
	o.rectCount++
	writeBodyIconA000RectRecord(o.rectDigest, ordinal, w, keys)
	if o.globalFirst == nil {
		copy := meta
		o.globalFirst = &copy
	}
	for _, key := range keys {
		if found := o.perKey[key]; found == nil {
			copy := meta
			o.perKey[key] = &bodyIconKeyEarliestJSON{EventKey: key, Writes: 1, First: copy}
		} else {
			found.Writes++
		}
	}
	for _, after := range o.afterSteps {
		if w.Step <= after {
			continue
		}
		firstByKey := o.firstAfter[after]
		if firstByKey == nil {
			firstByKey = make(map[string]*bodyIconKeyEarliestJSON)
			o.firstAfter[after] = firstByKey
		}
		for _, key := range keys {
			if firstByKey[key] == nil {
				copy := meta
				firstByKey[key] = &bodyIconKeyEarliestJSON{EventKey: key, Writes: 1, First: copy}
			} else {
				firstByKey[key].Writes++
			}
		}
	}
	o.addGroup(meta, w, keys)
	o.addSpan(meta, w, keys, ordinal)
}

func (o *bodyIconA000Observer) addGroup(meta bodyIconVideoWriteMetaJSON, w machine.VideoWrite, keys []string) {
	groupKey := fmt.Sprintf("%04x:%04x:%02x:%s", w.CS, w.IP, w.WriteMode, strings.Join(keys, "\x00"))
	g := o.groups[groupKey]
	if g == nil {
		g = &bodyIconA000GroupAccumulator{
			json: bodyIconA000GroupJSON{
				Instruction: meta.Instruction, WriteMode: w.WriteMode,
				EventKeys: append([]string(nil), keys...), First: meta,
				MinOffset: meta.VideoOffset, MaxOffset: meta.VideoOffset,
			},
			digest: sha256.New(),
		}
		o.groups[groupKey] = g
	}
	g.json.Writes++
	g.json.Last = meta
	if meta.VideoOffset < g.json.MinOffset {
		g.json.MinOffset = meta.VideoOffset
	}
	if meta.VideoOffset > g.json.MaxOffset {
		g.json.MaxOffset = meta.VideoOffset
	}
	writeBodyIconA000RectRecord(g.digest, meta.Ordinal, w, keys)
}

func (o *bodyIconA000Observer) addSpan(meta bodyIconVideoWriteMetaJSON, w machine.VideoWrite, keys []string, ordinal uint64) {
	if o.currentSpan >= 0 {
		span := &o.spans[o.currentSpan]
		if span.lastA000Ordinal+1 == ordinal && sameBodyIconSpan(span.json, meta, keys) && w.Offset == span.lastOffset+1 {
			span.json.A000OrdinalEnd = ordinal
			span.json.RectOrdinalEnd = o.rectCount
			span.json.StepEnd = w.Step
			span.json.VideoOffsetEnd = uint16(w.Offset)
			span.json.Writes++
			span.lastA000Ordinal = ordinal
			span.lastOffset = w.Offset
			writeBodyIconA000RectRecord(span.digest, meta.Ordinal, w, keys)
			return
		}
	}
	span := bodyIconA000SpanAccumulator{
		json: bodyIconA000SpanJSON{
			A000OrdinalStart: ordinal, A000OrdinalEnd: ordinal,
			RectOrdinalStart: o.rectCount, RectOrdinalEnd: o.rectCount,
			StepStart: w.Step, StepEnd: w.Step, Instruction: meta.Instruction,
			WriteMode: w.WriteMode, EventKeys: append([]string(nil), keys...),
			VideoOffsetStart: meta.VideoOffset, VideoOffsetEnd: meta.VideoOffset, Writes: 1,
		},
		lastA000Ordinal: ordinal, lastOffset: w.Offset, digest: sha256.New(),
	}
	writeBodyIconA000RectRecord(span.digest, meta.Ordinal, w, keys)
	o.spans = append(o.spans, span)
	o.currentSpan = len(o.spans) - 1
}

func sameBodyIconSpan(span bodyIconA000SpanJSON, meta bodyIconVideoWriteMetaJSON, keys []string) bool {
	if span.Instruction != meta.Instruction || span.WriteMode != meta.WriteMode || len(span.EventKeys) != len(keys) {
		return false
	}
	for i := range keys {
		if span.EventKeys[i] != keys[i] {
			return false
		}
	}
	return true
}

func (o *bodyIconA000Observer) Report() bodyIconA000TraceJSON {
	perKey := make([]bodyIconKeyEarliestJSON, 0, len(o.perKey))
	for _, key := range sortedBodyIconMapKeys(o.perKey) {
		perKey = append(perKey, *o.perKey[key])
	}
	groups := make([]bodyIconA000GroupJSON, 0, len(o.groups))
	for _, key := range sortedBodyIconMapKeys(o.groups) {
		group := o.groups[key].json
		group.SequenceSHA256 = hex.EncodeToString(o.groups[key].digest.Sum(nil))
		groups = append(groups, group)
	}
	spans := make([]bodyIconA000SpanJSON, 0, len(o.spans))
	for _, span := range o.spans {
		span.json.SequenceSHA256 = hex.EncodeToString(span.digest.Sum(nil))
		spans = append(spans, span.json)
	}
	after := make([]bodyIconA000WindowJSON, 0, len(o.afterSteps))
	for _, step := range o.afterSteps {
		byKey := o.firstAfter[step]
		rows := make([]bodyIconKeyEarliestJSON, 0, len(byKey))
		for _, key := range sortedBodyIconMapKeys(byKey) {
			rows = append(rows, *byKey[key])
		}
		after = append(after, bodyIconA000WindowJSON{AfterStep: step, FirstByKey: rows})
	}
	var first *bodyIconVideoWriteMetaJSON
	if o.globalFirst != nil {
		copy := *o.globalFirst
		first = &copy
	}
	return bodyIconA000TraceJSON{
		DigestSchema:  bodyIconA000DigestSchema,
		TraceFromStep: o.from, TraceUntilStep: o.until,
		A000WriteCount: o.allCount, A000SequenceSHA256: hex.EncodeToString(o.allDigest.Sum(nil)),
		RectIntersectingWriteCount: o.rectCount, RectIntersectingSequenceSHA256: hex.EncodeToString(o.rectDigest.Sum(nil)),
		GlobalEarliest: first, PerKeyEarliest: perKey, FirstAfterSteps: after,
		WriterGroups: groups, ContiguousSpans: spans,
	}
}

func sortedBodyIconMapKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeBodyIconA000Record(dst hash.Hash, ordinal uint64, w machine.VideoWrite) {
	var buf [18]byte
	binary.LittleEndian.PutUint64(buf[0:8], ordinal)
	binary.LittleEndian.PutUint64(buf[8:16], w.Step)
	binary.LittleEndian.PutUint16(buf[16:18], w.CS)
	_, _ = dst.Write(buf[:])
	var rest [8]byte
	binary.LittleEndian.PutUint16(rest[0:2], w.IP)
	binary.LittleEndian.PutUint32(rest[2:6], w.Offset)
	rest[6], rest[7] = w.Value, w.WriteMode
	_, _ = dst.Write(rest[:])
}

func writeBodyIconA000RectRecord(dst hash.Hash, ordinal uint64, w machine.VideoWrite, keys []string) {
	writeBodyIconA000Record(dst, ordinal, w)
	var count [2]byte
	binary.LittleEndian.PutUint16(count[:], uint16(len(keys)))
	_, _ = dst.Write(count[:])
	for _, key := range keys {
		var size [2]byte
		binary.LittleEndian.PutUint16(size[:], uint16(len(key)))
		_, _ = dst.Write(size[:])
		_, _ = dst.Write([]byte(key))
	}
}

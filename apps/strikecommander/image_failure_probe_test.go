package strikecommander

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A READY-contract probe, not a patched implementation or report-only mock.
// It reuses the frozen receiver fixture, then drives actual Prepare/Compose.
type imageFailureObservation struct {
	Case                                                    string
	First, Second                                           FrontendCompositeInfo
	FirstKey, SecondKey                                     hdPrepareKey
	SecondError                                             string
	SameKeyBeforeRetry, SameCompleteKey, SameReasonAndBytes bool
	SceneFailedBefore, SceneFailedAfter                     bool
	SceneReasonBefore, SceneReasonAfter                     string
}

func imageMemoRound(r *FrontendRuntime, req FrontendPrepareRequest) (info FrontendCompositeInfo, rgba []byte, err error) {
	p, e := r.Prepare(req)
	if e != nil {
		return info, nil, e
	}
	defer p.Release()
	info, e = p.Info()
	if e != nil {
		return info, nil, e
	}
	c, ok := r.Compose(req)
	if !ok {
		return info, nil, fmt.Errorf("Compose missing")
	}
	defer c.Release()
	ci, e := c.Info()
	if e != nil || !reflect.DeepEqual(info, ci) {
		return info, nil, fmt.Errorf("Compose metadata mismatch: %v", e)
	}
	p.Release()
	rgba = make([]byte, 320*200*req.Scale*req.Scale*4)
	if e = c.CopyRGBA(rgba); e != nil {
		return info, nil, e
	}
	c.Release()
	return info, rgba, nil
}

func TestImageFailureMemoSameKeyAfterAllReleases(t *testing.T) {
	for _, name := range []string{"palette_RGB_H2_then_closed_renderer", "temporary_compose_quota_then_release"} {
		t.Run(name, func(t *testing.T) {
			f := newRGBMemoRig(t)
			slot := f.install(filepath.Join(os.Getenv("SC_RGB_MEMO_CASES"), "runtime-corpus-003/run-s800f2-n3-001/C-group-001"), 0)
			req := FrontendPrepareRequest{Scale: 2, Language: "en", HD: true, SettingsVersion: 1}
			warm, _, e := imageMemoRound(f.rt, req)
			if e != nil || warm.Kind != FrontendReady || f.rt.HDDiagnostics().RGB.GlassSamples == 0 {
				t.Fatal("fixture must reach real visible-glass Render/RGB", warm, e)
			}
			// Read the exact pre-produce key at the existing identity seam.
			// This wraps no VM callback and preserves the real identity computation.
			var candidate hdPrepareKey
			identity := f.rt.output.prepareHDIdentity
			f.rt.output.prepareHDIdentity = func(k *hdPrepareKey) { identity(k); candidate = *k }
			var releaseQuota func()
			if name == "palette_RGB_H2_then_closed_renderer" {
				pal := f.rt.present.cur.palette
				pal[20] = [3]byte{135, 135, 135}
				f.rt.present.derive(PresPalette, pal, nil, false)
			} else {
				// Different target request from warmup, then exactly the same key on retry.
				req.SettingsVersion = 2
				owner := f.rt.hd.owner
				free := owner.TotalBytes() - owner.OwnedBytes()
				if free <= 4<<20 {
					t.Fatal("fixture has no room for controlled transient hold")
				}
				hold, err := owner.ReserveOwned(free - (1 << 20))
				if err != nil {
					t.Fatal(err)
				}
				releaseQuota = hold.Release
				defer hold.Release()
			}
			first, before, e := imageMemoRound(f.rt, req)
			if e != nil || first.Kind != FrontendFixedFallback {
				t.Fatal("first target request must really publish FixedFallback", first, e)
			}
			if candidate != first.key {
				t.Fatal("setup changed key during first production")
			}
			if slot.memo[0].failed {
				t.Fatal("fixture hit scene/N memo rather than image/request failure", first.Reason)
			}
			if name == "palette_RGB_H2_then_closed_renderer" {
				if !strings.Contains(first.Reason, "glass frozen palette profile differs") {
					t.Fatal("wrong true RGB failure", first.Reason)
				}
				if e = f.rt.hd.renderer.Close(); e != nil {
					t.Fatal(e)
				}
			} else {
				if !strings.Contains(first.Reason, "owner total limit") {
					t.Fatal("wrong transient allocation failure", first.Reason)
				}
				releaseQuota()
			}
			o := imageFailureObservation{Case: name, First: first, FirstKey: first.key, SceneFailedBefore: slot.memo[0].failed, SceneReasonBefore: slot.memo[0].reason}
			// Runtime.IsCurrent checks the immutable source/request key after all leases ended.
			o.SameKeyBeforeRetry = f.rt.IsCurrent(req, first)
			second, after, retryErr := imageMemoRound(f.rt, req)
			o.Second, o.SecondKey, o.SceneFailedAfter, o.SceneReasonAfter = second, candidate, slot.memo[0].failed, slot.memo[0].reason
			o.SameCompleteKey = candidate == first.key
			if retryErr != nil {
				o.SecondError = retryErr.Error()
			}
			o.SameReasonAndBytes = retryErr == nil && reflect.DeepEqual(first, second) && bytes.Equal(before, after)
			b, _ := json.MarshalIndent(o, "", "  ")
			if dir := os.Getenv("SC_IMAGE_MEMO_OUT"); dir != "" {
				if e = os.WriteFile(filepath.Join(dir, name+".json"), append(b, '\n'), 0644); e != nil {
					t.Fatal(e)
				}
			}
			t.Logf("OBSERVATION %s", b)
			if !o.SameKeyBeforeRetry {
				t.Fatal("setup changed immutable request/source qualification")
			}
			if !o.SameCompleteKey {
				t.Fatal("setup changed complete key on retry")
			}
			if !o.SameReasonAndBytes {
				t.Errorf("READY gap: same-key FixedFallback was retried or lost; original=%q retry=%q error=%v", first.Reason, second.Reason, retryErr)
			}
		})
	}
}

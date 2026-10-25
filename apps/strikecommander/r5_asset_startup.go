package strikecommander

import (
	"github.com/wicanr2/dosgolem/apps/strikecommander/r5asset"
	"sync"
)

// This private construction path is the only supported app entry. It first
// builds the existing HD common worker/renderer AND protected producer on the
// same Owner; the loader never attempts to infer them from an owned-byte sum.
// No session is constructed and no guest Deliver/Advance is performed here.
type r5Startup struct {
	runtime *FrontendRuntime
	assets  *r5asset.Loader
	once    sync.Once
}

func newR5Startup(cfg FrontendRuntimeConfig, root string, limits r5asset.Limits) (*r5Startup, error) {
	if cfg.HD == nil {
		return nil, r5asset.Error{Code: r5asset.Ownership, Detail: 501}
	}
	r, e := NewFrontendRuntime(cfg)
	if e != nil {
		return nil, e
	}
	good := false
	defer func() {
		if !good {
			r.Close()
		}
	}()
	h := r.hd
	if h == nil || h.owner == nil || h.renderer == nil || h.prepared == nil || h.pool == nil || r.so == nil || r.so.Models == nil || r.so.steps != 0 || r.present.counts.Frames != 0 || r.so.Models.bounded == nil || r.so.Models.bounded.pool != h.pool || r.so.Models.ShadeSourceOwner() != h.owner {
		return nil, r5asset.Error{Code: r5asset.Ownership, Detail: 502}
	}
	assets, e := r5asset.NewForConfiguredHD(h.owner, root, limits)
	if e != nil {
		return nil, e
	}
	defer func() {
		if !good {
			assets.Close()
			assets.WaitDrained()
			_ = assets.Dispose()
		}
	}()
	if e = assets.Load(); e != nil {
		return nil, e
	}
	reader, e := assets.Acquire()
	if e != nil {
		return nil, e
	}
	identity, e := assets.Identity(reader)
	assets.Release(reader)
	if e != nil {
		return nil, e
	}
	if e = assets.Adopt(h.owner, identity.SHA); e != nil {
		return nil, e
	}
	// This fixed wrapper fits the loader control reservation's 8192-byte slack.
	startup := &r5Startup{runtime: r, assets: assets}
	good = true
	return startup, nil
}
func (s *r5Startup) Close() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		s.assets.Close()
		s.assets.WaitDrained()
		_ = s.assets.Dispose()
		s.runtime.Close()
	})
}

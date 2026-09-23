package buckrogers

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// StoryPage9Owner keeps the watcher and RGBA stamp in one lifecycle.
type StoryPage9Owner struct {
	Watcher   *StoryPage9Watcher
	Presenter *RuntimeStoryPage9Overlay
}

func NewStoryPage9Owner(w *StoryPage9Watcher, p *RuntimeStoryPage9Overlay) (*StoryPage9Owner, error) {
	if w == nil || p == nil {
		return nil, fmt.Errorf("buckrogers: 第 9 頁 owner 缺 watcher 或 presenter")
	}
	return &StoryPage9Owner{Watcher: w, Presenter: p}, nil
}

func (o *StoryPage9Owner) Prewrite(v machine.VideoWrite) bool {
	if o == nil {
		return false
	}
	changed := o.Watcher.ObserveVideoWrite(v)
	if changed {
		o.Presenter.Clear()
	}
	return changed
}

func (o *StoryPage9Owner) Stop() {
	if o == nil {
		return
	}
	o.Watcher.Stop()
	o.Presenter.Clear()
}

func (o *StoryPage9Owner) Restore() {
	if o == nil {
		return
	}
	o.Watcher.Restore()
	o.Presenter.Clear()
}

func (o *StoryPage9Owner) ObserveExecutionDiscontinuity() {
	if o == nil {
		return
	}
	o.Watcher.ObserveExecutionDiscontinuity()
	o.Presenter.Clear()
}

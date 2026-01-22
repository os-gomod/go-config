package watch

import (
	"os"
	"time"
)

// Watcher polls files for changes.
type Watcher struct {
	interval time.Duration
	paths    []string
	modTimes map[string]time.Time
}

func New(interval time.Duration, paths []string) *Watcher {
	return &Watcher{
		interval: interval,
		paths:    paths,
		modTimes: make(map[string]time.Time),
	}
}

func (w *Watcher) Start(onChange func()) {
	go func() {
		w.initializeModTimes()
		w.watchLoop(onChange)
	}()
}

func (w *Watcher) initializeModTimes() {
	for _, p := range w.paths {
		if fi, err := os.Stat(p); err == nil {
			w.modTimes[p] = fi.ModTime()
		}
	}
}

func (w *Watcher) watchLoop(onChange func()) {
	for {
		time.Sleep(w.interval)
		if w.checkForChanges() {
			onChange()
		}
	}
}

func (w *Watcher) checkForChanges() bool {
	changed := false
	for p, oldTime := range w.modTimes {
		if fi, err := os.Stat(p); err == nil {
			newTime := fi.ModTime()
			if newTime.After(oldTime) {
				w.modTimes[p] = newTime
				changed = true
			}
		}
	}
	return changed
}

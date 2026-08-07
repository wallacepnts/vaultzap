package ingest

import (
	"sync"
	"sync/atomic"
	"time"
)

// Phase is the step of an import that is currently running.
type Phase string

const (
	PhaseExtracting Phase = "extracting"
	PhaseImporting  Phase = "importing"
)

// Snapshot is what the UI needs to draw the progress bar. A scan can be running with no
// import in flight — walking the inbox, or waiting out the gap between the two passes that
// the stable-file check needs (§5.9) — and the bar has to keep showing something.
type Snapshot struct {
	File      string
	Phase     Phase
	Done      int64
	Total     int64
	Elapsed   time.Duration
	Importing bool
}

// Percent is 0..100, capped. Zero total (unknown) reads as 0.
func (s Snapshot) Percent() int {
	if s.Total <= 0 {
		return 0
	}
	if s.Done >= s.Total {
		return 100
	}
	return int(s.Done * 100 / s.Total)
}

// progress tracks the import in flight. A single package-level value is enough because
// ingestion is one serial goroutine (§5.9): the scanner's own mutex guarantees there is
// never a second import to track.
var progress = &tracker{}

type tracker struct {
	mu       sync.Mutex
	scanning int
	running  bool
	file     string
	phase    Phase
	done     int64
	total    int64
	started  time.Time
}

// imported counts finished imports that brought messages in. The sidebar's badge poll
// carries it, so any screen notices a new conversation — the progress bar's own reload only
// happens while the imports page is open.
var imported atomic.Int64

// Imported is a generation number, not a total: only changes matter.
func Imported() int64 { return imported.Load() }

func markImported() { imported.Add(1) }

// CurrentImport returns what is in flight, or ok=false when nothing is.
func CurrentImport() (Snapshot, bool) {
	return progress.snapshot()
}

// ScanStarted/ScanFinished bracket a whole scan, so the UI can show it before the first
// file is even opened. Counted, not a boolean: startup and the "scan now" button can queue.
func ScanStarted()  { progress.scanStarted() }
func ScanFinished() { progress.scanFinished() }

func (t *tracker) scanStarted() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.scanning++
	if t.started.IsZero() {
		t.started = time.Now()
	}
}

func (t *tracker) scanFinished() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.scanning > 0 {
		t.scanning--
	}
	if t.scanning == 0 && !t.running {
		t.started = time.Time{}
	}
}

func (t *tracker) start(file string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running, t.file, t.phase = true, file, PhaseExtracting
	t.done, t.total, t.started = 0, 0, time.Now()
}

func (t *tracker) stopImport() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	if t.scanning > 0 {
		t.started = time.Now()
	}
}

// setPhase resets the counter: each phase counts its own unit (bytes, then messages).
func (t *tracker) setPhase(phase Phase, total int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.phase, t.done, t.total = phase, 0, total
}

func (t *tracker) add(n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.done += n
}

func (t *tracker) snapshot() (Snapshot, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running && t.scanning == 0 {
		return Snapshot{}, false
	}
	if !t.running {
		return Snapshot{Elapsed: time.Since(t.started)}, true
	}
	return Snapshot{
		File:      t.file,
		Phase:     t.phase,
		Done:      t.done,
		Total:     t.total,
		Elapsed:   time.Since(t.started),
		Importing: true,
	}, true
}

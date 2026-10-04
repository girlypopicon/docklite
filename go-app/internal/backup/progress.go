package backup

import (
	"context"
	"sync"
	"time"
)

// Backups run in the background; the web UI polls a Progress snapshot to
// draw a progress bar and, at the end, a confirmation or the failure reason.
// Progress lives in memory only: if the agent restarts mid-backup the run
// is gone, and the stale database record is failed at startup.

const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"

	// Phase kinds decide how the percentage is computed.
	kindCounting  = "counting"  // sizing up the work
	kindWorking   = "working"   // producing the archive/dump
	kindVerifying = "verifying" // reading the finished backup back

	maxWarnings  = 20
	keepFinished = time.Hour
)

type Progress struct {
	BackupID     int64    `json:"backup_id"`
	TargetType   string   `json:"target_type"`
	TargetID     int64    `json:"target_id"`
	Status       string   `json:"status"`
	Phase        string   `json:"phase"`
	Percent      float64  `json:"percent"` // 0–100, or -1 when the total isn't known
	BytesDone    int64    `json:"bytes_done"`
	BytesTotal   int64    `json:"bytes_total"`
	Files        int      `json:"files"`
	Warnings     []string `json:"warnings"`
	WarningCount int      `json:"warning_count"`
	Error        string   `json:"error,omitempty"`
	FileName     string   `json:"file_name,omitempty"`
	Location     string   `json:"location,omitempty"`
	Size         int64    `json:"size,omitempty"`
	Sha256       string   `json:"sha256,omitempty"`
	Verified     bool     `json:"verified"`
	DownloadURL  string   `json:"download_url,omitempty"`
	StartedAt    string   `json:"started_at"`
	ElapsedSecs  int64    `json:"elapsed_seconds"`

	kind     string
	started  time.Time
	finished time.Time
}

type Tracker struct {
	mu   sync.Mutex
	runs map[int64]*Progress
}

// DefaultTracker is shared by the HTTP handlers and the backup code.
var DefaultTracker = &Tracker{runs: map[int64]*Progress{}}

// Start registers a running backup. It returns nil if one is already
// running for the same target, so a double click can't start two.
func (t *Tracker) Start(backupID int64, targetType string, targetID int64) *Reporter {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked()
	for _, p := range t.runs {
		if p.Status == StatusRunning && p.TargetType == targetType && p.TargetID == targetID {
			return nil
		}
	}
	now := time.Now()
	t.runs[backupID] = &Progress{
		BackupID:   backupID,
		TargetType: targetType,
		TargetID:   targetID,
		Status:     StatusRunning,
		Phase:      "Starting",
		Percent:    -1,
		Warnings:   []string{},
		StartedAt:  now.UTC().Format(time.RFC3339),
		started:    now,
		kind:       kindCounting,
	}
	return &Reporter{t: t, id: backupID}
}

// RunningFor returns the backup id already running for a target, if any.
func (t *Tracker) RunningFor(targetType string, targetID int64) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, p := range t.runs {
		if p.Status == StatusRunning && p.TargetType == targetType && p.TargetID == targetID {
			return p.BackupID, true
		}
	}
	return 0, false
}

// Snapshot returns a copy of a backup's progress, or false if unknown.
func (t *Tracker) Snapshot(backupID int64) (Progress, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked()
	p, ok := t.runs[backupID]
	if !ok {
		return Progress{}, false
	}
	out := *p
	out.Warnings = append([]string{}, p.Warnings...)
	end := time.Now()
	if !p.finished.IsZero() {
		end = p.finished
	}
	out.ElapsedSecs = int64(end.Sub(p.started).Seconds())
	return out, true
}

func (t *Tracker) pruneLocked() {
	for id, p := range t.runs {
		if !p.finished.IsZero() && time.Since(p.finished) > keepFinished {
			delete(t.runs, id)
		}
	}
}

// Reporter is handed to the backup code. All methods are safe on nil, so
// backups started without a progress bar (schedules) need no special case.
type Reporter struct {
	t  *Tracker
	id int64
}

type reporterKey struct{}

func WithReporter(ctx context.Context, r *Reporter) context.Context {
	return context.WithValue(ctx, reporterKey{}, r)
}

func reporterFrom(ctx context.Context) *Reporter {
	r, _ := ctx.Value(reporterKey{}).(*Reporter)
	return r
}

func (r *Reporter) update(fn func(p *Progress)) {
	if r == nil {
		return
	}
	r.t.mu.Lock()
	defer r.t.mu.Unlock()
	if p, ok := r.t.runs[r.id]; ok {
		fn(p)
		recomputePercent(p)
	}
}

// Phase starts a new step. total is the number of bytes the step will
// process, or 0 when that isn't known in advance.
func (r *Reporter) Phase(kind, label string, total int64) {
	r.update(func(p *Progress) {
		p.kind, p.Phase, p.BytesDone, p.BytesTotal = kind, label, 0, total
	})
}

func (r *Reporter) Add(n int64) {
	r.update(func(p *Progress) { p.BytesDone += n })
}

func (r *Reporter) SetFiles(n int) {
	r.update(func(p *Progress) { p.Files = n })
}

// Warn records something that didn't stop the backup but the user should
// know about, e.g. a file that couldn't be read.
func (r *Reporter) Warn(message string) {
	r.update(func(p *Progress) {
		p.WarningCount++
		if len(p.Warnings) < maxWarnings {
			p.Warnings = append(p.Warnings, message)
		}
	})
}

func (r *Reporter) Succeed(res *ArtifactResult, verified bool) {
	r.update(func(p *Progress) {
		p.Status, p.Phase, p.Percent = StatusSuccess, "Done", 100
		p.finished = time.Now()
		p.Verified = verified
		if res != nil {
			p.Size, p.Sha256, p.Location = res.Size, res.Sha256, res.RelativePath
			p.FileName = baseName(res.Path)
		}
	})
}

func (r *Reporter) Fail(message string) {
	r.update(func(p *Progress) {
		p.Status, p.Error = StatusFailed, message
		p.finished = time.Now()
	})
}

func (r *Reporter) SetDownloadURL(url string) {
	r.update(func(p *Progress) { p.DownloadURL = url })
}

// recomputePercent maps each step onto a slice of one overall bar:
// counting 0–5%, working 5–90%, verifying 90–100%. A working step with an
// unknown total (database dumps) reports -1 so the UI shows a moving bar.
func recomputePercent(p *Progress) {
	if p.Status != StatusRunning {
		return
	}
	frac := func() float64 {
		if p.BytesTotal <= 0 {
			return 0
		}
		f := float64(p.BytesDone) / float64(p.BytesTotal)
		if f > 1 {
			f = 1
		}
		return f
	}
	switch p.kind {
	case kindCounting:
		p.Percent = 2
	case kindWorking:
		if p.BytesTotal <= 0 {
			p.Percent = -1
		} else {
			p.Percent = 5 + 85*frac()
		}
	case kindVerifying:
		p.Percent = 90 + 10*frac()
	}
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

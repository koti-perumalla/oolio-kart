package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"coupon-platform/internal/util"
)

type Status struct {
	IsRunning               bool   `json:"isRunning"`
	TotalProcessed          uint64 `json:"totalProcessed"`
	CurrentRunProcessed     uint64 `json:"currentRunProcessed"`
	CurrentRunPersisted     uint64 `json:"currentRunPersisted"`
	CurrentRunTotalLines    uint64 `json:"currentRunTotalLines"`
	CurrentRunValid         uint64 `json:"currentRunValid"`
	CurrentRunInvalidFormat uint64 `json:"currentRunInvalidFormat"`
	LastStartedAt           string `json:"lastStartedAt,omitempty"`
	LastCompletedAt         string `json:"lastCompletedAt,omitempty"`
}

type Processor struct {
	workers           []*Worker
	mu                sync.Mutex
	total             uint64
	currentRunTotal   uint64
	currentPersisted  uint64
	currentRunMetrics FileMetrics
	isRunning         bool
	lastStartedAt     time.Time
	lastCompletedAt   time.Time
	validCouponsCh    chan util.CouponHash
}

func NewProcessor(workerCount int) *Processor {
	baseDir := os.Getenv("ROCKSDB_DIR")
	if baseDir == "" {
		baseDir = "/tmp/coupon-workers"
	}

	w := make([]*Worker, workerCount)
	for i := range w {
		w[i] = NewWorker(filepath.Join(baseDir, fmt.Sprintf("worker-%d", i)))
	}

	return &Processor{workers: w}
}

// dispatch routes the hash to the correct worker shard via merge (write-only).
func (p *Processor) dispatch(hash util.CouponHash, fileIndex byte) {
	index := hash.ShardKey() % uint64(len(p.workers))
	p.workers[index].RecordFile(hash, fileIndex)
	atomic.AddUint64(&p.total, 1)
	atomic.AddUint64(&p.currentRunTotal, 1)
}

// scanWorkers iterates all worker shards and sends valid coupons to ch.
func (p *Processor) scanWorkers(ch chan<- util.CouponHash) {
	var wg sync.WaitGroup
	for _, w := range p.workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			w.Scan(ch)
		}(w)
	}
	wg.Wait()
}

func (p *Processor) Stats() uint64 {

	return atomic.LoadUint64(&p.total)
}

func (p *Processor) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.isRunning
}

func (p *Processor) resetWorkers() {
	for _, worker := range p.workers {
		worker.Reset()
	}
}

func (p *Processor) TryStartRun() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return false
	}

	p.resetWorkers()
	p.isRunning = true
	p.currentRunTotal = 0
	p.currentPersisted = 0
	p.currentRunMetrics = FileMetrics{}
	p.lastStartedAt = time.Now().UTC()
	return true
}

func (p *Processor) finishRun() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isRunning = false
	p.lastCompletedAt = time.Now().UTC()
}

func (p *Processor) CurrentRunProcessed() uint64 {
	return atomic.LoadUint64(&p.currentRunTotal)
}

func (p *Processor) setCurrentRunPersisted(count uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentPersisted = count
}

func (p *Processor) setCurrentRunMetrics(metrics FileMetrics) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentRunMetrics = metrics
}

func (p *Processor) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()

	status := Status{
		IsRunning:               p.isRunning,
		TotalProcessed:          p.total,
		CurrentRunProcessed:     p.currentRunTotal,
		CurrentRunPersisted:     p.currentPersisted,
		CurrentRunTotalLines:    p.currentRunMetrics.TotalLines,
		CurrentRunValid:         p.currentRunMetrics.Valid,
		CurrentRunInvalidFormat: p.currentRunMetrics.InvalidFormat,
	}

	if !p.lastStartedAt.IsZero() {
		status.LastStartedAt = p.lastStartedAt.Format(time.RFC3339)
	}

	if !p.lastCompletedAt.IsZero() {
		status.LastCompletedAt = p.lastCompletedAt.Format(time.RFC3339)
	}

	return status
}

// ValidateRunFiles checks whether the provided file list can be processed.
// Returns an error if the request exceeds supported limits (e.g. >64 files).
func (p *Processor) ValidateRunFiles(files []string) error {
	if len(files) > 64 {
		return fmt.Errorf("too many files: %d (max 64)", len(files))
	}
	return nil
}

// Close shuts down all worker Rocks DBs cleanly. Call on server shutdown.
func (p *Processor) Close() {
	for _, w := range p.workers {
		w.Close()
	}
}

package processor

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"coupon-platform/internal/util"
)

type persistResult struct {
	inserted uint64
	errs     []string
}

type fileProcessResult struct {
	path    string
	metrics FileMetrics
	err     error
}

func persistValidCoupons(ch <-chan util.CouponHash) persistResult {
	const batchSize = 5000
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	batch := make([]util.CouponHash, 0, batchSize)
	result := persistResult{}

	flush := func() {
		if len(batch) == 0 {
			return
		}

		const maxRetries = 3
		var err error
		for attempt := 0; attempt < maxRetries; attempt++ {
			var inserted uint64
			inserted, err = saveValidCouponsBatch(batch)
			if err == nil {
				result.inserted += inserted
				break
			}
			log.Printf("batch insert failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
		if err != nil {
			result.errs = append(result.errs, err.Error())
		}

		batch = batch[:0]
	}

	for {
		select {
		case hash, ok := <-ch:
			if !ok {
				flush()
				return result
			}

			batch = append(batch, hash)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (p *Processor) Run(files []string) {
	startedAt := time.Now()
	if len(files) > 64 {
		log.Printf("refusing to run: too many files=%d (max 64)", len(files))
		return
	}

	log.Printf("coupon processing started: files=%d", len(files))

	// Phase 1: Process all files (write-only merges to RocksDB)
	var wg sync.WaitGroup
	errCh := make(chan error, len(files))
	fileResultCh := make(chan fileProcessResult, len(files))

	for i, f := range files {
		wg.Add(1)
		go func(index int, path string) {
			defer wg.Done()
			metrics, err := p.ProcessFile(path, byte(index))
			fileResultCh <- fileProcessResult{path: path, metrics: metrics, err: err}
			if err != nil {
				errCh <- fmt.Errorf("file %s: %w", path, err)
			}
		}(i, f)
	}

	wg.Wait()
	close(fileResultCh)
	close(errCh)

	// Collect file metrics and errors from phase 1
	runMetrics := FileMetrics{}
	for result := range fileResultCh {
		runMetrics.add(result.metrics)
		log.Printf(
			"file processing stats: file=%s totalLines=%d valid=%d invalidFormat=%d",
			result.path,
			result.metrics.TotalLines,
			result.metrics.Valid,
			result.metrics.InvalidFormat,
		)
	}
	p.setCurrentRunMetrics(runMetrics)

	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}

	// Flush any remaining batched merges before scanning
	for _, w := range p.workers {
		w.FlushBatch()
	}

	// Phase 2: Scan RocksDB workers for valid coupons and save to PostgreSQL
	log.Printf("scanning workers for valid coupons")
	p.validCouponsCh = make(chan util.CouponHash, 50000)

	const persistWorkers = 4
	persistDone := make(chan persistResult, persistWorkers)
	for i := 0; i < persistWorkers; i++ {
		go func() {
			persistDone <- persistValidCoupons(p.validCouponsCh)
		}()
	}

	p.scanWorkers(p.validCouponsCh)
	close(p.validCouponsCh)
	p.validCouponsCh = nil

	var persist persistResult
	for i := 0; i < persistWorkers; i++ {
		r := <-persistDone
		persist.inserted += r.inserted
		persist.errs = append(persist.errs, r.errs...)
	}
	p.setCurrentRunPersisted(persist.inserted)
	errs = append(errs, persist.errs...)

	p.resetWorkers()
	p.finishRun()

	duration := time.Since(startedAt)
	processed := p.CurrentRunProcessed()
	persisted := persist.inserted

	if len(errs) > 0 {
		log.Printf(
			"coupon processing completed with errors: duration=%s processed=%d persisted=%d totalLines=%d valid=%d invalidFormat=%d errors=%d details=%s",
			duration,
			processed,
			persisted,
			runMetrics.TotalLines,
			runMetrics.Valid,
			runMetrics.InvalidFormat,
			len(errs),
			strings.Join(errs, " | "),
		)
		return
	}

	log.Printf(
		"coupon processing completed: duration=%s processed=%d persisted=%d totalLines=%d valid=%d invalidFormat=%d",
		duration,
		processed,
		persisted,
		runMetrics.TotalLines,
		runMetrics.Valid,
		runMetrics.InvalidFormat,
	)
}

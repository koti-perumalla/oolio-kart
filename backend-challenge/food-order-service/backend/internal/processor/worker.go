package processor

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"sync"

	"github.com/lib/pq"
	"github.com/linxGnu/grocksdb"

	"coupon-platform/internal/db"
	"coupon-platform/internal/util"
)

// Worker deduplicates coupon hashes using RocksDB as a disk-backed KV store.

const mergeBatchSize = 1000

type Worker struct {
	mu     sync.Mutex
	db     *grocksdb.DB
	ro     *grocksdb.ReadOptions
	wo     *grocksdb.WriteOptions
	dbPath string
	batch  *grocksdb.WriteBatch
	batchN int
}

// bitwiseORMergeOperator implements grocksdb.MergeOperator.
// Used to accumulate file-presence bits without read-modify-write.
type bitwiseORMergeOperator struct{}

func (m *bitwiseORMergeOperator) Name() string { return "bitwiseOR" }

func (m *bitwiseORMergeOperator) FullMerge(key, existingValue []byte, operands [][]byte) ([]byte, bool) {
	var mask uint64
	if len(existingValue) == 8 {
		mask = binary.BigEndian.Uint64(existingValue)
	}
	for _, op := range operands {
		if len(op) == 8 {
			mask |= binary.BigEndian.Uint64(op)
		}
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], mask)
	return buf[:], true
}

func (m *bitwiseORMergeOperator) PartialMerge(key, leftOperand, rightOperand []byte) ([]byte, bool) {
	var left, right uint64
	if len(leftOperand) == 8 {
		left = binary.BigEndian.Uint64(leftOperand)
	}
	if len(rightOperand) == 8 {
		right = binary.BigEndian.Uint64(rightOperand)
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], left|right)
	return buf[:], true
}

func newRocksDBOptions() *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)

	// Universal compaction — optimal for write-heavy workloads with no range scans.
	// Reduces write amplification ~4× vs leveled compaction at billions scale.
	// Use optimizer API for compatibility with analyzer/tooling that mis-types enum constants.
	opts.OptimizeUniversalStyleCompaction(64 << 20)

	// Direct I/O — bypasses OS page cache entirely.
	// Critical at 100 GB+ datasets where page cache hit rate would be <4%.
	opts.SetUseDirectReads(true)
	opts.SetUseDirectIOForFlushAndCompaction(true)

	// 64 MB write buffer per worker.
	opts.SetWriteBufferSize(64 << 20)
	opts.SetMaxWriteBufferNumber(2)

	// Block cache: 64 MB per worker — the only caching layer with Direct I/O.
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(grocksdb.NewLRUCache(64 << 20))
	opts.SetBlockBasedTableFactory(bbto)
	opts.SetMergeOperator(&bitwiseORMergeOperator{})

	return opts
}

func NewWorker(dbPath string) *Worker {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		panic(fmt.Sprintf("mkdir parent for %s: %v", dbPath, err))
	}

	opts := newRocksDBOptions()

	rdb, err := grocksdb.OpenDb(opts, dbPath)
	if err != nil {
		panic(fmt.Sprintf("rocksdb open %s: %v", dbPath, err))
	}

	wo := grocksdb.NewDefaultWriteOptions()
	wo.DisableWAL(true)

	return &Worker{
		db:     rdb,
		ro:     grocksdb.NewDefaultReadOptions(),
		wo:     wo,
		dbPath: dbPath,
		batch:  grocksdb.NewWriteBatch(),
	}
}

// Reset destroys the RocksDB directory and opens a fresh one.
// Called after all data is persisted — frees disk immediately.
func (w *Worker) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.batch.Destroy()
	w.db.Close()
	os.RemoveAll(w.dbPath)
	if err := os.MkdirAll(filepath.Dir(w.dbPath), 0o755); err != nil {
		panic(fmt.Sprintf("mkdir parent for %s: %v", w.dbPath, err))
	}

	rdb, err := grocksdb.OpenDb(newRocksDBOptions(), w.dbPath)
	if err != nil {
		panic(fmt.Sprintf("rocksdb reopen %s: %v", w.dbPath, err))
	}
	w.db = rdb
	w.batch = grocksdb.NewWriteBatch()
	w.batchN = 0
}

// Close shuts down RocksDB cleanly. Called on server shutdown.
func (w *Worker) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.batch.Destroy()
	w.ro.Destroy()
	w.wo.Destroy()
	w.db.Close()
}

// RecordFile records that the hash was seen in the given fileIndex.
// Accumulates merges in a WriteBatch and flushes when batch full

func (w *Worker) RecordFile(hash util.CouponHash, fileIndex byte) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(1)<<uint(fileIndex))

	w.mu.Lock()
	w.batch.Merge(hash.Bytes(), buf[:])
	w.batchN++
	if w.batchN >= mergeBatchSize {
		if err := w.db.Write(w.wo, w.batch); err != nil {
			w.mu.Unlock()
			panic(fmt.Sprintf("rocksdb write batch: %v", err))
		}
		w.batch.Clear()
		w.batchN = 0
	}
	w.mu.Unlock()
}

// FlushBatch writes any remaining buffered merges to RocksDB.
// Must be called after all RecordFile calls complete (before Scan).
func (w *Worker) FlushBatch() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.batchN == 0 {
		return
	}
	if err := w.db.Write(w.wo, w.batch); err != nil {
		panic(fmt.Sprintf("rocksdb flush batch: %v", err))
	}
	w.batch.Clear()
	w.batchN = 0
}

// Scan iterates all keys and sends hashes seen in 2+ files to ch.
// Called after all files are processed (no concurrent writes).
func (w *Worker) Scan(ch chan<- util.CouponHash) {
	it := w.db.NewIterator(w.ro)
	defer it.Close()
	for it.SeekToFirst(); it.Valid(); it.Next() {
		key := it.Key()
		val := it.Value()
		mask := binary.BigEndian.Uint64(val.Data())
		if bits.OnesCount64(mask) >= 2 {
			ch <- util.CouponHash{
				Hash1: binary.BigEndian.Uint64(key.Data()[:8]),
				Hash2: binary.BigEndian.Uint64(key.Data()[8:]),
			}
		}
		key.Free()
		val.Free()
	}
}

// saveValidCouponsBatch writes a batch of valid hashes to PostgreSQL using COPY
// via a temp table, which is faster than multi-value INSERT.
//
// Flow: COPY rows → temp table → INSERT INTO coupons ON CONFLICT DO NOTHING.
// Both steps run inside one transaction
func saveValidCouponsBatch(batch []util.CouponHash) (uint64, error) {
	if len(batch) == 0 {
		return 0, nil
	}

	ctx := context.Background()

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Temp table uses NUMERIC(20,0) to match the coupons schema.
	// uint64 max exceeds int64 max, so BIGINT cannot
	// safely hold all hash values — NUMERIC(20,0) is used.
	_, err = tx.ExecContext(ctx, `
		CREATE TEMP TABLE tmp_coupons (hash1 NUMERIC(20,0) NOT NULL, hash2 NUMERIC(20,0) NOT NULL)
		ON COMMIT DROP
	`)
	if err != nil {
		return 0, fmt.Errorf("create temp table: %w", err)
	}

	// COPY stream —  faster than batched INSERT for bulk loads.
	stmt, err := tx.Prepare(pq.CopyIn("tmp_coupons", "hash1", "hash2"))
	if err != nil {
		return 0, fmt.Errorf("prepare copy: %w", err)
	}

	for _, h := range batch {
		// Pass as decimal strings — NUMERIC(20,0) accepts text input via COPY.
		if _, err := stmt.ExecContext(ctx, h.Hash1String(), h.Hash2String()); err != nil {
			stmt.Close()
			return 0, fmt.Errorf("copy row: %w", err)
		}
	}

	// Flush the COPY buffer.
	if _, err := stmt.ExecContext(ctx); err != nil {
		stmt.Close()
		return 0, fmt.Errorf("copy flush: %w", err)
	}
	if err := stmt.Close(); err != nil {
		return 0, fmt.Errorf("copy close: %w", err)
	}

	// Move from temp table to coupons, skipping duplicates.
	result, err := tx.ExecContext(ctx, `
		INSERT INTO coupons(hash1, hash2)
		SELECT hash1, hash2 FROM tmp_coupons
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return 0, fmt.Errorf("insert from temp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return uint64(inserted), nil
}

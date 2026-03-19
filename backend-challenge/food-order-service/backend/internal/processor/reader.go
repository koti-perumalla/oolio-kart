package processor

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"coupon-platform/internal/util"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileMetrics struct {
	TotalLines    uint64
	Valid         uint64
	InvalidFormat uint64
}

func (m *FileMetrics) add(other FileMetrics) {
	m.TotalLines += other.TotalLines
	m.Valid += other.Valid
	m.InvalidFormat += other.InvalidFormat
}

func (p *Processor) ProcessFile(path string, fileIndex byte) (FileMetrics, error) {
	if fileIndex >= 64 {
		return FileMetrics{}, fmt.Errorf("fileIndex %d out of range: max 63", fileIndex)
	}

	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".zip" {
		return p.processZipFile(path, fileIndex)
	}

	file, err := os.Open(path)
	if err != nil {
		return FileMetrics{}, err
	}
	defer file.Close()

	// support gzip-compressed files
	if ext == ".gz" {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return FileMetrics{}, err
		}
		defer gz.Close()
		return p.scanCoupons(gz, fileIndex)
	}

	return p.scanCoupons(file, fileIndex)
}

func (p *Processor) processZipFile(path string, fileIndex byte) (FileMetrics, error) {
	metrics := FileMetrics{}

	archive, err := zip.OpenReader(path)
	if err != nil {
		return metrics, err
	}
	defer archive.Close()

	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}

		reader, err := entry.Open()
		if err != nil {
			return metrics, err
		}

		// handle gzip entries inside zips
		var r io.Reader = reader
		var gz *gzip.Reader
		if strings.HasSuffix(strings.ToLower(entry.Name), ".gz") {
			gz, err = gzip.NewReader(reader)
			if err != nil {
				reader.Close()
				return metrics, err
			}
			r = gz
		}

		entryMetrics, err := p.scanCoupons(r, fileIndex)

		if gz != nil {
			gz.Close()
		}
		reader.Close()
		if err != nil {
			return metrics, err
		}

		metrics.add(entryMetrics)
	}

	return metrics, nil
}

func (p *Processor) scanCoupons(reader io.Reader, fileIndex byte) (FileMetrics, error) {
	metrics := FileMetrics{}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		metrics.TotalLines++

		code := strings.TrimSpace(scanner.Text())
		if code == "" {
			continue
		}

		if !util.IsCouponCodeFormatValid(code) {
			metrics.InvalidFormat++
			continue
		}

		hash := util.HashCoupon(code)
		p.dispatch(hash, fileIndex)
		metrics.Valid++
	}

	if err := scanner.Err(); err != nil {
		return metrics, err
	}

	return metrics, nil
}

package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"coupon-platform/internal/processor"
)

var uploadedFiles []string
var uploadedFilesMu sync.Mutex
var couponProcessor *processor.Processor

// If not set, handlers will lazily initialize a default processor on first use.
func SetProcessor(p *processor.Processor) {
	couponProcessor = p
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {

	r.Body = http.MaxBytesReader(w, r.Body, 10<<30) // 10 GB hard limit

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Printf("upload read form file failed: error=%v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer file.Close()

	path := filepath.Join("./uploads", filepath.Base(handler.Filename))

	out, err := os.Create(path)
	if err != nil {
		log.Printf("upload create file failed: path=%s error=%v", path, err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer out.Close()

	_, err = out.ReadFrom(file)
	if err != nil {
		log.Printf("upload write file failed: path=%s error=%v", path, err)
		http.Error(w, err.Error(), 500)
		return
	}

	uploadedFilesMu.Lock()
	if len(uploadedFiles) >= 3 {
		uploadedFiles = nil
	}
	uploadedFiles = append(uploadedFiles, path)
	fileCount := len(uploadedFiles)
	uploadedFilesMu.Unlock()

	log.Printf("upload success: filename=%s savedPath=%s batchFiles=%d", handler.Filename, path, fileCount)

	resp := map[string]interface{}{
		"message": "file uploaded",
		"files":   fileCount,
	}

	json.NewEncoder(w).Encode(resp)

}

func ProcessHandler(w http.ResponseWriter, r *http.Request) {

	uploadedFilesMu.Lock()
	if len(uploadedFiles) != 3 {
		uploadedFilesMu.Unlock()
		log.Printf("processing request rejected: uploadedFiles=%d", len(uploadedFiles))
		http.Error(w, "please upload exactly 3 coupon files", 400)
		return
	}

	filesToProcess := append([]string(nil), uploadedFiles...)
	uploadedFiles = nil
	uploadedFilesMu.Unlock()

	if err := couponProcessor.ValidateRunFiles(filesToProcess); err != nil {
		log.Printf("processing request rejected: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !couponProcessor.TryStartRun() {
		log.Printf("processing request rejected: already running")
		http.Error(w, "processing already running", http.StatusConflict)
		return
	}

	log.Printf("processing started: files=%v", filesToProcess)

	go func() {

		couponProcessor.Run(filesToProcess)

	}()

	resp := map[string]string{
		"status": "processing started",
	}

	json.NewEncoder(w).Encode(resp)

}

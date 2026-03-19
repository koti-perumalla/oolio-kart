package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"coupon-platform/internal/api"
	"coupon-platform/internal/cache"
	"coupon-platform/internal/db"
	"coupon-platform/internal/processor"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func main() {

	os.MkdirAll("./uploads", os.ModePerm)

	err := db.InitPostgres()

	if err != nil {
		log.Fatal(err)
	}

	cache.InitRedis()
	// configure processor worker count: min(env(PROCESSOR_WORKERS), NumCPU())
	workers := runtime.NumCPU()
	if v := os.Getenv("PROCESSOR_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < workers {
			workers = n
		}
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 64 {
		workers = 64
	}
	log.Printf("processor worker count=%d", workers)
	p := processor.NewProcessor(workers)
	api.SetProcessor(p)
	srv := api.Router()

	go func() {
		log.Printf("server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	p.Close()
	db.DB.Close()
	cache.Client.Close()
	log.Println("shutdown complete")
}

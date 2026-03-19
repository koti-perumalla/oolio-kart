package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var Ctx = context.Background()

func InitRedis() {

	addr := os.Getenv("REDIS_ADDR")

	Client = redis.NewClient(&redis.Options{
		Addr: addr,
	})

	const maxAttempts = 5
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		lastErr = Client.Ping(Ctx).Err()
		if lastErr == nil {
			log.Printf("redis connected: addr=%s", addr)
			return
		}
		log.Printf("redis ping failed (attempt %d/%d): %v", i+1, maxAttempts, lastErr)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	panic(fmt.Sprintf("redis unreachable after %d attempts: %v", maxAttempts, lastErr))
}

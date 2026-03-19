package service

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"coupon-platform/internal/cache"
	"coupon-platform/internal/db"
	"coupon-platform/internal/util"
)

var (
	couponCacheTTL     = 24 * time.Hour
	couponCacheTTLOnce sync.Once
)

func getCouponCacheTTL() time.Duration {
	couponCacheTTLOnce.Do(func() {
		value := os.Getenv("COUPON_CACHE_TTL")
		if value == "" {
			return
		}

		ttl, err := time.ParseDuration(value)
		if err != nil {
			log.Printf("invalid COUPON_CACHE_TTL=%q, using default %s", value, couponCacheTTL)
			return
		}

		if ttl <= 0 {
			log.Printf("non-positive COUPON_CACHE_TTL=%q, using default %s", value, couponCacheTTL)
			return
		}

		couponCacheTTL = ttl
		log.Printf("coupon cache TTL configured: %s", couponCacheTTL)
	})

	return couponCacheTTL
}

func IsCouponValid(hash util.CouponHash) bool {

	key := hash.CacheKey()

	val, err := cache.Client.Get(cache.Ctx, key).Result()

	if err == nil && val == "1" {
		return true
	}

	var exists bool

	err = db.DB.QueryRowContext(
		context.Background(),
		`SELECT EXISTS(
		 SELECT 1 FROM coupons WHERE hash1=$1 AND hash2=$2
		)`,
		hash.Hash1String(),
		hash.Hash2String(),
	).Scan(&exists)

	if err != nil {
		return false
	}

	if exists {
		cache.Client.Set(cache.Ctx, key, "1", getCouponCacheTTL())
	}

	return exists
}

func IsCouponCodeValid(code string) bool {
	if !util.IsCouponCodeFormatValid(code) {
		return false
	}

	return IsCouponValid(util.HashCoupon(code))
}

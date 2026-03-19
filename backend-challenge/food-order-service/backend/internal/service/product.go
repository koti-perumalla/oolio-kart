package service

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/lib/pq"

	"coupon-platform/internal/cache"
	"coupon-platform/internal/db"
)

var (
	productCacheTTL     = 15 * time.Minute
	productCacheTTLOnce sync.Once
)

func getProductCacheTTL() time.Duration {
	productCacheTTLOnce.Do(func() {
		v := os.Getenv("PRODUCT_CACHE_TTL")
		if v == "" {
			return
		}
		ttl, err := time.ParseDuration(v)
		if err != nil {
			log.Printf("invalid PRODUCT_CACHE_TTL=%q, using default %s", v, productCacheTTL)
			return
		}
		if ttl <= 0 {
			log.Printf("non-positive PRODUCT_CACHE_TTL=%q, using default %s", v, productCacheTTL)
			return
		}
		productCacheTTL = ttl
		log.Printf("product cache TTL configured: %s", productCacheTTL)
	})
	return productCacheTTL
}

type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
}

// GetProduct fetches a single product; it will try Redis first then fallback to Postgres.
func GetProduct(id string) (*Product, error) {

	key := "product:" + id

	val, err := cache.Client.Get(cache.Ctx, key).Result()
	if err == nil {
		var p Product
		if err := json.Unmarshal([]byte(val), &p); err == nil {
			return &p, nil
		}
		// fall through to DB on unmarshal error
	}

	var p Product
	err = db.DB.QueryRowContext(
		context.Background(),
		`SELECT id,name,price,category FROM products WHERE id=$1`,
		id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Category)

	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(p)
	cache.Client.Set(cache.Ctx, key, data, getProductCacheTTL())

	return &p, nil
}

// GetProducts fetches multiple products by ID
func GetProducts(ids []string) (map[string]*Product, error) {
	if len(ids) == 0 {
		return map[string]*Product{}, nil
	}

	result := make(map[string]*Product, len(ids))
	missing := make([]string, 0, len(ids))

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = "product:" + id
	}

	vals, err := cache.Client.MGet(cache.Ctx, keys...).Result()
	if err == nil {
		for i, val := range vals {
			if val == nil {
				missing = append(missing, ids[i])
				continue
			}
			var p Product
			if jsonErr := json.Unmarshal([]byte(val.(string)), &p); jsonErr == nil {
				result[ids[i]] = &p
			} else {
				missing = append(missing, ids[i])
			}
		}
	} else {
		missing = append(missing, ids...)
	}

	if len(missing) == 0 {
		return result, nil
	}

	rows, err := db.DB.QueryContext(
		context.Background(),
		`SELECT id, name, price, category FROM products WHERE id = ANY($1)`,
		pq.Array(missing),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ttl := getProductCacheTTL()
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Category); err != nil {
			return nil, err
		}
		result[p.ID] = &p
		data, _ := json.Marshal(p)
		cache.Client.Set(cache.Ctx, "product:"+p.ID, data, ttl)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// ListProducts returns all products, using Redis caching for the full list.
func ListProducts(ctx context.Context) ([]Product, error) {
	key := "products:all"

	val, err := cache.Client.Get(cache.Ctx, key).Result()
	if err == nil {
		var products []Product
		if err := json.Unmarshal([]byte(val), &products); err == nil {
			return products, nil
		}
		// fall through to DB on unmarshal error
	}

	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, name, price, category
		FROM products
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Category); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(products)
	cache.Client.Set(cache.Ctx, key, data, getProductCacheTTL())

	return products, nil
}

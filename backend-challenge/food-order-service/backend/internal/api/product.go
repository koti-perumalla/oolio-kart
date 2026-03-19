package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"coupon-platform/internal/service"
)

type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
}

// ListProducts returns all products
// Todo: pagination
func ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := service.ListProducts(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)

}

// GetProduct handles GET /product/{id}
func GetProduct(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/product/")
	if id == "" || id == "/" {
		http.NotFound(w, r)
		return
	}

	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	product, err := service.GetProduct(id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

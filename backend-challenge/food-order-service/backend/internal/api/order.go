package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"coupon-platform/internal/db"
	"coupon-platform/internal/service"
	"coupon-platform/internal/util"
)

type OrderReq struct {
	CouponCode string `json:"couponCode"`
	Items      []struct {
		ProductId string `json:"productId"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
}

type OrderItem struct {
	ProductId string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type OrderResp struct {
	ID    string      `json:"id"`
	Items []OrderItem `json:"items"`
}

var (
	isCouponCodeFormatValidFn = util.IsCouponCodeFormatValid
	hashCouponFn              = util.HashCoupon
	isCouponValidFn           = service.IsCouponValid
	getProductsFn             = service.GetProducts
)

func PlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req OrderReq

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		http.Error(w, "order must have at least one item", http.StatusBadRequest)
		return
	}
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			http.Error(w, "item quantity must be positive", http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()

	var couponHash util.CouponHash
	var couponHash1Value interface{}
	var couponHash2Value interface{}

	// Validate coupon
	if req.CouponCode != "" {
		if !isCouponCodeFormatValidFn(req.CouponCode) {
			http.Error(w, "invalid coupon", 400)
			return
		}

		couponHash = hashCouponFn(req.CouponCode)
		couponHash1Value = couponHash.Hash1String()
		couponHash2Value = couponHash.Hash2String()

		if !isCouponValidFn(couponHash) {
			http.Error(w, "invalid coupon", 400)
			return
		}
	}

	orderID := uuid.New()

	// Begin transaction using database/sql
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// safe deferred rollback (ignored if Commit succeeds)
	defer func() { _ = tx.Rollback() }()

	// Batch-fetch all products in a single round-trip (eliminates N+1)
	productIDs := make([]string, len(req.Items))
	for i, item := range req.Items {
		productIDs[i] = item.ProductId
	}

	products, err := getProductsFn(productIDs)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	var orderTotal float64

	var respItems []OrderItem

	for _, item := range req.Items {
		product, ok := products[item.ProductId]
		if !ok {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}

		orderTotal += product.Price * float64(item.Quantity)

		_, err = tx.ExecContext(ctx,
			`INSERT INTO order_items(order_id,product_id,quantity)
				VALUES($1,$2,$3)`,
			orderID.String(),
			product.ID,
			item.Quantity,
		)

		if err != nil {
			http.Error(w, "db error", 500)
			return
		}

		respItems = append(respItems, OrderItem{
			ProductId: product.ID,
			Quantity:  item.Quantity,
		})
	}

	// Insert order with total
	// Keeping hash1value, hash2value in two different columns
	_, err = tx.ExecContext(ctx,
		`INSERT INTO orders(id,coupon_hash1,coupon_hash2,total)
	         VALUES($1,$2,$3,$4)`,
		orderID.String(),
		couponHash1Value,
		couponHash2Value,
		orderTotal,
	)

	if err != nil {
		http.Error(w, "db insert error", 500)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "commit failed", http.StatusInternalServerError)
		return
	}

	resp := OrderResp{
		ID:    orderID.String(),
		Items: respItems,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

CREATE TABLE coupons (
    hash1 NUMERIC(20,0) NOT NULL,
    hash2 NUMERIC(20,0) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

ALTER TABLE coupons ADD PRIMARY KEY (hash1, hash2);

CREATE TABLE products (
    id TEXT PRIMARY KEY,
    name TEXT,
    price DOUBLE PRECISION,
    category TEXT
);

CREATE TABLE orders (
    id UUID PRIMARY KEY,
    coupon_hash1 NUMERIC(20,0),
    coupon_hash2 NUMERIC(20,0),
    total DOUBLE PRECISION,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE order_items (
    order_id UUID,
    product_id TEXT,
    quantity INT
);

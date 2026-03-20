# Food Order Service — README

## Problem Context
- Built a system that supports two core workflows:
  - Customer ordering flow
    - Browse products
    - Apply coupon (optional)
    - Place order with validation
  - Coupon ingestion pipeline
    - Bulk upload of coupon files
    - Validate coupons
    - Persist only valid coupons (appearing in ≥2 files)

## Quick goals of this README
- Build and run the frontend & backend using docker.
- Explain the work-flows, file-processing path and how coupon validation computed.
- Technical details

# Features
- **Product Catalog** — Browse and retrieve products
- **Order Placement** — Place orders with optional coupon discounts
- **Bulk Coupon Upload & Processing** — Upload up to 3 coupon files, process them in parallel, validate coupons, and save valid coupons in DB
- **Real-time Progress Tracking** — Monitor coupon processing status with detailed metrics
- **Coupon Validation** — Two-level hashing (xxhash64 + FNV-64a) with Redis caching

## GIT Repository details
- GIT User: koti-perumalla
- GIT Repository: https://github.com/koti-perumalla/oolio-kart.git
- Code base root directory: backend-challenge/food-order-service
- ReadMe: https://github.com/koti-perumalla/oolio-kart/blob/main/backend-challenge/food-order-service/README.md

- Requirements Details: https://github.com/koti-perumalla/oolio-kart/blob/main/backend-challenge/README.md

## Build & Run with Docker Compose

This repo contains a `docker-compose.yml` in the root and Dockerfiles in the frontend & backend.
`docker-compose.yml`and Dockerfiles contains instructions to build docker images. 

Docker compose builds both backend & frontend applications.

To build & run:

```bash
cd 	oolio-kart-challenge/backend-challenge/food-order-service

docker compose up --build

OR

docker compose build --no-cache
docker compose up

Note: If you are building locally, you may have to install RocksDB.
brew install rocksdb

(inside docker, docker compose took care of it)

```

Note: Set the database connection in the DATABASE_URL env variable in docker-compose.yml

# Getting Started

## Key Technical Notes
- I observed the complexity came from
  - Large input size (tens of thousands to millions of coupons)
  - Need for fast validation at order time
  - Avoid DB overload during ingestion
  - Ensure correctness despite hashing
- Key design consideration
  - Tried to keep computation as fast as possible, used the database only for durability & final correctness and used redis cache to reduce read requests hits to db within ttl.
  
**Key design decisions:**

- **RocksDB per worker** — Each of the N workers (scaled to CPU count) owns an isolated RocksDB instance. Coupons are sharded by hash, eliminating cross-worker coordination.
- **Bitwise merge operator** — Each coupon's source file is encoded as a bit in a 64-bit mask. RocksDB's merge operator performs `OR` on bit masks. (Reduced locks contention as much as possible).
- **Cross-file validation** — Only coupons present in 2+ files are considered valid (`bits  count >= 2`).
- **Write-optimized RocksDB** — Universal compaction, direct I/O, 64 MB write buffers.

## Where hashing is computed
- The function `HashCoupon(code string) CouponHash` in `backend/internal/util/hash.go` performs the two hashes and returns `CouponHash{Hash1, Hash2}`.
- That value is used by the reader, the processor sharding, the rocksDB, and the DB persistence path.

## Important schema note (Postgres)
- Stored hash as `NUMERIC(20,0)` in PostgreSQL to avoid `BIGINT` overflow for the full `uint64` range.
- Two NUMERIC(20,0) used to store two hashes of a each valid coupon.

## Coupon Processing Pipeline
Coupons are distributed across three plain-text files (one coupon code per line). A coupon is considered **valid** when the same code appears in **at least 2 of the 3 files**.

```
  Upload 3 Files
      │
      ▼
  Validate Files
      │
      ▼
  Read in Parallel 
      │
      ▼
  Validate Format (8–10 char, alphabets only) ->  Hash (xxhash64 + FNV-64a)
      │
      ▼
  Route to Worker Shards (RocksDB)
      │
      ▼
  Mark cooupon presence via Bitwise OR Merge
      │
      ▼
  Scan Workers for valid Coupons (appearing in ≥2 files)
      │
      ▼
  Batch Insert into PostgreSQL 

```

**Hashing strategy:** 
Each coupon code produces a `CouponHash` with two independent 64-bit hashes:
- `Hash1` — [xxhash](https://github.com/cespare/xxhash) (`xxhash.Sum64String`)
- `Hash2` — FNV-1a 64-bit (`hash/fnv`)
- I selected Hashes over string as a key to improve computation.
- Reduced the collisions probability to almost 0.

**Detecting Valid Coupon:**: Requirement: Coupon is valid if it appears in ≥2 of 3 files
- I have used bit masking. 
  - Why bit masking approach?
    - Bit operations are very fast
    - Trade off: Works well for the requirement of 3 files (Max can scale up to max 64 files). 

**Concurrency model:** 
- Design details
  - Reader streams lines (no full file load)
  - Workers process independently (Sharded Workers with rocksDB)
    - Coupons are dispatched to workers using hash, ensuring all occurrences of the same coupon are always handled by the same worker — this avoids locking.
  - Valid coupons sent via buffered channel.
  - Persistence happens in batches.

**Coupon validation at order time:**
1. Check Redis cache key `coupon:<hash1>:<hash2>` — return immediately on hit.
2. On miss, query `coupons` table; cache the result with `COUPON_CACHE_TTL` on success.


## Project Structure
- `docker-compose.yml` — Docker compose file
- `backend/cmd/server/main.go` — Entry point
- `backend/internal/api` — HTTP handlers and router
- `backend/internal/processor` — Coupon file processing pipeline
- `backend/internal/service` — Business logic (products/coupons)
- `backend/internal/cache` — Redis client
- `backend/internal/db` — PostgreSQL connection
- `backend/internal/utli` — Hashing utilities
- `backend/internal/Dockerfile` — Dockerfile for backend
- `backend/public/openapi.yaml` — OpenAPI spec served by backend
- `frontend/src` — React app (UI + API client helpers)
- `frontend/Dockerfile` — Dockerfile for frontend
- `backend/migrations` — DB schema + seed data
- `backend/testscripts` — test Coupon file generators & samples

## Core routes
- `GET /api/product`
- `GET /api/product/{productId}`
- `POST /api/order` (requires header: `api_key: apitest`)
- `POST /api/upload`
- `POST /api/process`
- `GET /api/progress`
- `GET /api/processing-status`
- `GET /api/openapi.yaml`


**Request flow for placing an order:**
1. Frontend sends `POST /api/order` with an optional coupon code and line items.
2. API middleware validates the `api_key` header.
3. Coupon is checked: format validation → Redis cache → PostgreSQL fallback.
4. A database transaction inserts `order_items` and the `orders` record.
5. Order summary is returned to the client.

## Tech Stack

| Layer          | Technology                         |
|----------------|------------------------------------|
| Backend        | Go 1.24 (standard library `net/http`) |
| Frontend       | React 18, React Router 6          |
| Database       | PostgreSQL 15                      |
| Cache          | Redis 7                           |
| Deduplication  | RocksDB (per-worker instances)     |
| Infrastructure | Docker & Docker Compose            |
| Migrations     | golang-migrate                     |


### APIs - Coupon File Processing

#### `POST /api/upload`

Uploads a coupon file (multipart form, field name `file`). Up to 3 files are buffered before processing can start.

#### `POST /api/process`

Triggers async processing of the 3 uploaded coupon files. Returns `409 Conflict` if a run is already in progress.

#### `GET /api/progress`

Returns the total number of coupons processed across all runs.

```json
{ "processed": 142500 }
```

### `GET /api/processing-status`

Returns detailed metrics for the current or most recent processing run.

```json
{
  "isRunning": false,
  "totalProcessed": 142500,
  "currentRunProcessed": 47500,
  "currentRunPersisted": 315,
  "currentRunTotalLines": 47500,
  "currentRunValid": 315,
  "currentRunInvalidFormat": 12,
  "lastStartedAt": "2026-03-16T10:00:00Z",
  "lastCompletedAt": "2026-03-16T10:00:04Z"
}
```

## Todos
- Maintain runid & it's filles etc details for each run in db & it's status.
- Add functionality to gracefully handle run failures. (Design a solution to trigger pending runs, cleanup of failure runs)

## Scaling to Next Level
- Current design technically works well upto 5 t 10 billion coupon files with a decent 24 core, 32GB RAM and 2TB SSD machine or little more higher. (By increasing resources it can process unto 20 billion to 30 billion files)

- To Scale to next level
  - Introduce Kafka/Queue based distributed model with distributed key-value store kind of DB with high consistency configuration for couponos ( Ex: AWS Dynamo or Scylla DB)
    - This is horzantally scales
    - Trade offs: Lot of network i/o, processing across multiple machines etc may increases processing time. Cost increases significantly.

## Unit Test Results
- When I tested this applocation in my laptop docker instance (resources are less)
  - Test1: 3 files, each file with 100K coupons (with around 75% overall valid coupons) processed with in few secs
  - Test2: 3 files, each file with 10 million coupons (with around 75% overall valid coupons) processed with in few mins




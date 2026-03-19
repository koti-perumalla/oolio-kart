# ---------- BUILD STAGE ----------
FROM golang:1.24-alpine AS builder

WORKDIR /app

# C++ toolchain + RocksDB headers required for grocksdb CGO bindings
RUN apk add --no-cache \
    git gcc g++ make cmake \
    rocksdb-dev snappy-dev lz4-dev zstd-dev bzip2-dev \
    libstdc++ linux-headers

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO must be enabled for grocksdb
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/server

# ---------- RUNTIME STAGE ----------
FROM alpine:3.23

WORKDIR /app

# Runtime libs needed by the RocksDB-linked binary
RUN apk add --no-cache ca-certificates rocksdb libstdc++ snappy lz4-libs zstd-libs

# copy server binary
COPY --from=builder /app/server .
COPY --from=builder /app/public ./public

ENV DATABASE_URL=postgres://coupon:coupon@postgres:5432/coupondb?sslmode=disable
ENV PROCESSOR_WORKERS=8

EXPOSE 8080

CMD sleep 5 && ./server

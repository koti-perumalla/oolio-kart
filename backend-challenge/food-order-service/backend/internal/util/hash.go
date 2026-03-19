package util

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"strconv"
	"unicode/utf8"

	"github.com/cespare/xxhash/v2"
)

type CouponHash struct {
	Hash1 uint64
	Hash2 uint64
}

func IsCouponCodeFormatValid(code string) bool {
	length := utf8.RuneCountInString(code)
	if length < 8 || length > 10 {
		return false
	}
	for _, r := range code {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func HashCoupon(code string) CouponHash {
	hash1 := xxhash.Sum64String(code)

	h := fnv.New64a()
	_, _ = h.Write([]byte(code))
	hash2 := h.Sum64()

	return CouponHash{Hash1: hash1, Hash2: hash2}
}

func (h CouponHash) CacheKey() string {
	return fmt.Sprintf("coupon:%d:%d", h.Hash1, h.Hash2)
}

func (h CouponHash) ShardKey() uint64 {
	return h.Hash1 ^ h.Hash2
}

func (h CouponHash) Hash1String() string {
	return strconv.FormatUint(h.Hash1, 10)
}

func (h CouponHash) Hash2String() string {
	return strconv.FormatUint(h.Hash2, 10)
}

// Bytes encodes the hash as a 16-byte big-endian key for use in rocksdb.
func (h CouponHash) Bytes() []byte {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], h.Hash1)
	binary.BigEndian.PutUint64(b[8:], h.Hash2)
	return b[:]
}

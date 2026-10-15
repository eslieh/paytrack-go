// Package ids generates prefixed, time-sortable identifiers
// (e.g. "pay_01j9z..."). Lexical order matches creation order, which lets
// list endpoints paginate with a simple "id < cursor" predicate.
package ids

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"time"
)

var enc = base32.NewEncoding("0123456789abcdefghjkmnpqrstvwxyz").WithPadding(base32.NoPadding)

// New returns prefix + "_" + 26 chars: 48-bit ms timestamp + 80 random bits.
func New(prefix string) string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(time.Now().UnixMilli())<<16)
	if _, err := rand.Read(b[6:]); err != nil {
		panic(err)
	}
	return prefix + "_" + enc.EncodeToString(b[:])
}

// Secret returns n random bytes, hex encoded.
func Secret(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// SecretBase32 returns n random bytes, base32 encoded (URL/header friendly).
func SecretBase32(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return enc.EncodeToString(b)
}

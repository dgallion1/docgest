package pipeline

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// Simple ULID generator that doesn't require external dependencies.
// ULIDs are 26-character Crockford Base32 encoded strings with timestamp prefix.

var (
	ulidMu  sync.Mutex
	lastTS  uint64
	lastSeq uint16
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func generateULID() string {
	ulidMu.Lock()
	defer ulidMu.Unlock()

	ts := uint64(time.Now().UnixMilli())
	if ts == lastTS {
		lastSeq++
	} else {
		lastTS = ts
		lastSeq = 0
	}

	var b [16]byte
	// Timestamp in first 6 bytes (big-endian 48-bit).
	b[0] = byte(ts >> 40)
	b[1] = byte(ts >> 32)
	b[2] = byte(ts >> 24)
	b[3] = byte(ts >> 16)
	b[4] = byte(ts >> 8)
	b[5] = byte(ts)
	// Random in remaining 10 bytes.
	rand.Read(b[6:])
	// Embed sequence in bytes 6-7 to ensure uniqueness within same ms.
	binary.BigEndian.PutUint16(b[6:8], lastSeq)

	return encode(b)
}

func encode(b [16]byte) string {
	// Crockford Base32 encoding of 128 bits = 26 characters.
	var out [26]byte
	// Encode 6 bytes of timestamp (48 bits -> 10 chars).
	out[0] = crockford[(b[0]&224)>>5]
	out[1] = crockford[b[0]&31]
	out[2] = crockford[(b[1]&248)>>3]
	out[3] = crockford[((b[1]&7)<<2)|((b[2]&192)>>6)]
	out[4] = crockford[(b[2]&62)>>1]
	out[5] = crockford[((b[2]&1)<<4)|((b[3]&240)>>4)]
	out[6] = crockford[((b[3]&15)<<1)|((b[4]&128)>>7)]
	out[7] = crockford[(b[4]&124)>>2]
	out[8] = crockford[((b[4]&3)<<3)|((b[5]&224)>>5)]
	out[9] = crockford[b[5]&31]
	// Encode 10 bytes of randomness (80 bits -> 16 chars).
	out[10] = crockford[(b[6]&248)>>3]
	out[11] = crockford[((b[6]&7)<<2)|((b[7]&192)>>6)]
	out[12] = crockford[(b[7]&62)>>1]
	out[13] = crockford[((b[7]&1)<<4)|((b[8]&240)>>4)]
	out[14] = crockford[((b[8]&15)<<1)|((b[9]&128)>>7)]
	out[15] = crockford[(b[9]&124)>>2]
	out[16] = crockford[((b[9]&3)<<3)|((b[10]&224)>>5)]
	out[17] = crockford[b[10]&31]
	out[18] = crockford[(b[11]&248)>>3]
	out[19] = crockford[((b[11]&7)<<2)|((b[12]&192)>>6)]
	out[20] = crockford[(b[12]&62)>>1]
	out[21] = crockford[((b[12]&1)<<4)|((b[13]&240)>>4)]
	out[22] = crockford[((b[13]&15)<<1)|((b[14]&128)>>7)]
	out[23] = crockford[(b[14]&124)>>2]
	out[24] = crockford[((b[14]&3)<<3)|((b[15]&224)>>5)]
	out[25] = crockford[b[15]&31]

	return fmt.Sprintf("%s", out[:])
}

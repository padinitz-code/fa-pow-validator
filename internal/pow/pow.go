package pow

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"golang.org/x/crypto/argon2"
)

var ErrCalcProofHash = errors.New("cannot to calculate proof hash")

const nonceSize = 8

// Argon2id parameters (can be tuned)
const (
	ArgonTime    = 2
	ArgonMemory  = 64 * 1024 // 64 MB
	ArgonThreads = 2
	ArgonKeyLen  = 32
)

func CalcProof(difficulty byte, salt []byte) (proofNonce, proofHash []byte, err error) {
	nonceOffset := len(salt)
	buf := make([]byte, nonceOffset+nonceSize)
	copy(buf, salt)
	// buf = | salt | nonce

	var nonce uint64
	for nonce < math.MaxUint64 {
		binary.BigEndian.PutUint64(buf[nonceOffset:], nonce)
		hash := argon2.IDKey(buf, nil, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)
		if leadingZerosCount(hash) >= difficulty {
			proofNonce = buf[nonceOffset:]
			proofHash = hash
			return
		} else {
			nonce++
		}
	}

	err = fmt.Errorf("could not find valid proof: %w", ErrCalcProofHash)
	return
}

func CheckProof(difficulty byte, salt []byte, proofNonce []byte) bool {
	nonceOffset := len(salt)
	buf := make([]byte, nonceOffset+nonceSize)
	copy(buf, salt)
	copy(buf[nonceOffset:], proofNonce)
	// buf = | salt | nonce

	return CheckBufProof(difficulty, buf)
}

func CheckBufProof(difficulty byte, buf []byte) bool {
	hash := argon2.IDKey(buf, nil, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)
	return leadingZerosCount(hash) >= difficulty
}

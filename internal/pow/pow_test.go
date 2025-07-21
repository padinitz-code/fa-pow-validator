package pow

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcAndCheckPoW(t *testing.T) {
	var difficulty byte = 10

	data := make([]byte, 64)
	rand.Read(data)

	// calc proof
	nonce, _, err := CalcProof(difficulty, data)
	assert.NoError(t, err)

	// check valid proof
	isValid := CheckProof(difficulty, data, nonce)
	assert.True(t, isValid)

	// if invalid data
	data[0], data[1] = data[1], data[0]
	isValid = CheckProof(difficulty, data, nonce)
	assert.False(t, isValid)
	data[0], data[1] = data[1], data[0]

	// is invalid proof nonce
	rand.Read(nonce)
	isValid = CheckProof(difficulty, data, nonce)
	assert.False(t, isValid)
}

func TestCalcAndCheckPoW_EdgeCases(t *testing.T) {
	// Zero difficulty: should always succeed
	data := make([]byte, 64)
	rand.Read(data)
	nonce, _, err := CalcProof(0, data)
	assert.NoError(t, err)
	assert.True(t, CheckProof(0, data, nonce))

	// Max difficulty: should fail to find a proof (simulate by limiting attempts)
	// Not running CalcProof(255, ...) as it would take too long, but CheckProof should always fail
	fakeNonce := make([]byte, 8)
	assert.False(t, CheckProof(255, data, fakeNonce))

	// Empty data
	data = []byte{}
	nonce, _, err = CalcProof(5, data)
	assert.NoError(t, err)
	assert.True(t, CheckProof(5, data, nonce))

	// Large data
	largeData := make([]byte, 1024)
	rand.Read(largeData)
	nonce, _, err = CalcProof(5, largeData)
	assert.NoError(t, err)
	assert.True(t, CheckProof(5, largeData, nonce))

	// Non-8-byte nonce (should fail)
	badNonce := make([]byte, 4)
	assert.False(t, CheckProof(5, data, badNonce))
}

func BenchmarkCalculateProof(b *testing.B) {
	data := make([]byte, 64)
	rand.Read(data)

	var difficulty byte = 25

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalcProof(difficulty, data)
	}
}

func BenchmarkCheckProof(b *testing.B) {
	data := make([]byte, 64)
	rand.Read(data)

	var difficulty byte = 10

	nonce, _, _ := CalcProof(difficulty, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckProof(difficulty, data, nonce)
	}
}

func BenchmarkCheckBufProof(b *testing.B) {
	const tokenSize = 64
	data := make([]byte, tokenSize+nonceSize)
	rand.Read(data[:tokenSize])

	var difficulty byte = 10

	nonce, _, _ := CalcProof(difficulty, data)
	copy(data[tokenSize:], nonce)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckBufProof(difficulty, data)
	}
}

func BenchmarkArgon2idDifficulty(b *testing.B) {
	difficulties := []byte{5, 10, 15, 20}
	saltSizes := []int{16, 32, 64}
	for _, saltSize := range saltSizes {
		b.Run(fmt.Sprintf("SaltSize_%d", saltSize), func(b *testing.B) {
			for _, diff := range difficulties {
				b.Run(fmt.Sprintf("Difficulty_%d", diff), func(b *testing.B) {
					salt := make([]byte, saltSize)
					rand.Read(salt)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, _, err := CalcProof(diff, salt)
						if err != nil {
							b.Fatalf("failed to calc proof: %v", err)
						}
					}
				})
			}
		})
	}
}

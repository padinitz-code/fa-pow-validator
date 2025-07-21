package pow

import (
	"crypto/hmac"
	"crypto/sha256"
	"math/bits"
)

func leadingZerosCount(data []byte) byte {
	count := 0
	for _, v := range data {
		if v == 0 {
			count += 8
		} else {
			count += bits.LeadingZeros8(v)
			break
		}
	}
	if count > 255 {
		return 255
	}
	return byte(count)
}

func SignChallenge(secret, data []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return h.Sum(nil)
}

func VerifyChallengeSignature(secret, data, sig []byte) bool {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return hmac.Equal(h.Sum(nil), sig)
}

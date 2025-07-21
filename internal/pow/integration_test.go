package pow_test

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/avpetkun/pow-wow/internal/pow"
	"gitlab.com/avpetkun/pow-wow/internal/server"
)

func TestStatelessHandshake_Valid(t *testing.T) {
	secret := []byte("supersecretkey")
	difficulty := byte(5)
	salt := make([]byte, 16)
	rand.Read(salt)
	sig := pow.SignChallenge(secret, append([]byte{difficulty}, salt...))
	nonce, _, err := pow.CalcProof(difficulty, salt)
	assert.NoError(t, err)

	payload := struct {
		Difficulty byte   `json:"difficulty"`
		Salt       []byte `json:"salt"`
		Signature  []byte `json:"signature"`
		Nonce      []byte `json:"nonce"`
	}{
		Difficulty: difficulty,
		Salt:       salt,
		Signature:  sig,
		Nonce:      nonce,
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(payload)

	clientConn, serverConn := net.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		receiver := server.NewStatelessReceiver(secret)
		_, err := receiver(serverConn)
		assert.NoError(t, err)
		serverConn.Close()
	}()
	clientConn.Write(buf.Bytes())
	clientConn.Close()
	wg.Wait()
}

func TestStatelessHandshake_InvalidSignature(t *testing.T) {
	secret := []byte("supersecretkey")
	difficulty := byte(5)
	salt := make([]byte, 16)
	rand.Read(salt)
	sig := make([]byte, 32) // invalid signature
	nonce, _, err := pow.CalcProof(difficulty, salt)
	assert.NoError(t, err)

	payload := struct {
		Difficulty byte   `json:"difficulty"`
		Salt       []byte `json:"salt"`
		Signature  []byte `json:"signature"`
		Nonce      []byte `json:"nonce"`
	}{
		Difficulty: difficulty,
		Salt:       salt,
		Signature:  sig,
		Nonce:      nonce,
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(payload)

	clientConn, serverConn := net.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		receiver := server.NewStatelessReceiver(secret)
		_, err := receiver(serverConn)
		assert.Error(t, err)
		serverConn.Close()
	}()
	clientConn.Write(buf.Bytes())
	clientConn.Close()
	wg.Wait()
}

func TestStatelessHandshake_InvalidPoW(t *testing.T) {
	secret := []byte("supersecretkey")
	difficulty := byte(5)
	salt := make([]byte, 16)
	rand.Read(salt)
	sig := pow.SignChallenge(secret, append([]byte{difficulty}, salt...))
	nonce := make([]byte, 8) // invalid nonce

	payload := struct {
		Difficulty byte   `json:"difficulty"`
		Salt       []byte `json:"salt"`
		Signature  []byte `json:"signature"`
		Nonce      []byte `json:"nonce"`
	}{
		Difficulty: difficulty,
		Salt:       salt,
		Signature:  sig,
		Nonce:      nonce,
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(payload)

	clientConn, serverConn := net.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		receiver := server.NewStatelessReceiver(secret)
		_, err := receiver(serverConn)
		assert.Error(t, err)
		serverConn.Close()
	}()
	clientConn.Write(buf.Bytes())
	clientConn.Close()
	wg.Wait()
}

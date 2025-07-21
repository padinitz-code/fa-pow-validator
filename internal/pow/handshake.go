package pow

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	ErrReadCryptoRand = errors.New("failed to read crypto rand")
	ErrWritePacket    = errors.New("failed to write PoW packet")
	ErrReadPacket     = errors.New("failed to read PoW packet")
	ErrNotValidProof  = errors.New("is not valid proof")

	ErrReadHeader = errors.New("failed to read PoW header")
	ErrReadData   = errors.New("failed to read PoW data")
	ErrWriteNonce = errors.New("failed to write nonce")
)

const (
	powHeaderSize = 3
)

type Receiver func(net.Conn) (checkDuration time.Duration, err error)

func NewReceiver(difficulty byte, proofTokenSize int) Receiver {
	bufPool := &sync.Pool{
		New: func() interface{} {
			b := make([]byte, proofTokenSize+powHeaderSize+nonceSize)
			return &b
		},
	}
	return func(conn net.Conn) (checkDuration time.Duration, err error) {
		bufPtr := bufPool.Get().(*[]byte)
		defer bufPool.Put(bufPtr)
		buf := *bufPtr

		// "Proof Of Work" packet struct
		// ----------------------------------------------------
		// | offset             | name         | length
		// | -------------------|--------------|---------------
		// |                  0 | difficulty   | 1 byte
		// |                  1 | token size   | 2 bytes
		// |                  3 | rand token   | ProofTokenSize
		// | 3 + ProofTokenSize | result nonce | 8 bytes

		resultOffset := powHeaderSize + proofTokenSize

		puzzleData := buf[powHeaderSize:resultOffset]

		// read rand data
		_, err = rand.Read(puzzleData)
		if err != nil {
			return 0, fmt.Errorf("rand.Read puzzleData: %w", ErrReadCryptoRand)
		}

		buf[0] = difficulty
		binary.BigEndian.PutUint16(buf[1:], uint16(proofTokenSize))

		// write puzzle packet
		if _, err = conn.Write(buf[:resultOffset]); err != nil {
			return 0, fmt.Errorf("write puzzle packet: %w", ErrWritePacket)
		}

		// read nonce packet answer
		_, err = conn.Read(buf[resultOffset:])
		if err != nil {
			return 0, fmt.Errorf("read nonce packet: %w", ErrReadPacket)
		}

		// check proof
		beginCheck := time.Now()
		isValid := CheckBufProof(difficulty, buf[powHeaderSize:])
		checkDuration = time.Since(beginCheck)
		if !isValid {
			return checkDuration, fmt.Errorf("invalid proof: %w", ErrNotValidProof)
		}

		return checkDuration, nil
	}
}

func Establish(conn net.Conn) (calcDifficulty byte, calcDuration time.Duration, err error) {
	// "Proof Of Work" received packet struct
	// --------------------------------------
	// | offset | name       | length
	// | -------|------------|---------------
	// |      0 | difficulty | 1 byte
	// |      1 | token size | 2 bytes
	// |      3 | rand token | token size

	buf := make([]byte, powHeaderSize)
	_, err = conn.Read(buf)
	if err != nil {
		return 0, 0, fmt.Errorf("read header: %w", ErrReadHeader)
	}

	calcDifficulty = buf[0]
	tokenSize := binary.BigEndian.Uint16(buf[1:])

	buf = make([]byte, tokenSize)
	_, err = conn.Read(buf)
	if err != nil {
		err = fmt.Errorf("read data: %w", ErrReadData)
		return
	}

	beginCalc := time.Now()
	nonce, _, calcErr := CalcProof(calcDifficulty, buf)
	calcDuration = time.Since(beginCalc)
	if calcErr != nil {
		err = fmt.Errorf("calc proof: %w", calcErr)
		return
	}

	_, err = conn.Write(nonce)
	if err != nil {
		err = fmt.Errorf("write nonce: %w", ErrWriteNonce)
		return
	}

	return
}

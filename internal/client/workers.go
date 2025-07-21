package client

import (
	"context"
	"net"
	"time"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/rs/zerolog"
	"gitlab.com/avpetkun/pow-wow/internal/pow"
)

func StartFetchWorkers(ctx context.Context, conf *Config, log zerolog.Logger) {
	creationPause := conf.Timeout / time.Duration(conf.FetchWorkers)
	for i := 0; i < conf.FetchWorkers; i++ {
		go runFetchWorker(ctx, conf, log, i)
		time.Sleep(creationPause)
	}
}

func runFetchWorker(ctx context.Context, conf *Config, log zerolog.Logger, workerID int) {
	log = log.With().Int("worker_id", workerID).Logger()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("worker shutting down due to context cancellation")
			return
		default:
			if !fetchQuote(ctx, log, conf.ServerAddr) {
				time.Sleep(conf.Timeout)
			}
		}
	}
}

func fetchQuote(ctx context.Context, log zerolog.Logger, serverAddr string) bool {
	// Fetch challenge from HTTP endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/challenge", nil)
	if err != nil {
		log.Err(fmt.Errorf("failed to create challenge request: %w", err)).Msg("")
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Err(fmt.Errorf("failed to fetch challenge: %w", err)).Msg("")
		return false
	}
	defer resp.Body.Close()
	var challenge struct {
		Difficulty byte   `json:"difficulty"`
		Salt       []byte `json:"salt"`
		Signature  []byte `json:"signature"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&challenge); err != nil {
		log.Err(fmt.Errorf("invalid challenge response: %w", err)).Msg("")
		return false
	}
	log.Debug().Int("difficulty", int(challenge.Difficulty)).Msg("starting PoW solve")
	startSolve := time.Now()
	// Solve PoW
	nonce, _, err := pow.CalcProof(challenge.Difficulty, challenge.Salt)
	if err != nil {
		log.Err(fmt.Errorf("failed to solve PoW: %w", err)).Msg("")
		return false
	}
	solveDuration := time.Since(startSolve)
	log.Info().Dur("solve_duration", solveDuration).Msg("PoW solved successfully")
	// Connect to TCP server and send payload
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", serverAddr)
	if err != nil {
		log.Err(fmt.Errorf("failed to connect: %w", err)).Msg("")
		return false
	}
	defer conn.Close()
	payload := struct {
		Difficulty byte   `json:"difficulty"`
		Salt       []byte `json:"salt"`
		Signature  []byte `json:"signature"`
		Nonce      []byte `json:"nonce"`
	}{
		Difficulty: challenge.Difficulty,
		Salt:       challenge.Salt,
		Signature:  challenge.Signature,
		Nonce:      nonce,
	}
	if err := json.NewEncoder(conn).Encode(payload); err != nil {
		log.Err(fmt.Errorf("failed to send PoW payload: %w", err)).Msg("")
		return false
	}
	response, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Error().Err(fmt.Errorf("failed to read from connection: %w", err)).Msg("")
		return false
	}
	if len(response) == 0 {
		log.Warn().Msg("empty response")
		return false
	}
	log.Info().Str("quote", string(response)).Int("difficulty", int(challenge.Difficulty)).Dur("solve_duration", solveDuration).Msg("received quote from server")
	return true
}

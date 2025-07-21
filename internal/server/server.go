package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"crypto/rand"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/quic-go/quic-go/http3"
	"gitlab.com/avpetkun/pow-wow/internal/pow"
)

var powSecret = []byte("supersecretkey") // TODO: move to config/env

// Configurable: use cache or on-demand
var UseChallengeCache = true // TODO: make configurable via env
const challengeCacheSize = 100

var challengeCache chan *Challenge
var cacheOnce sync.Once

var (
	promChallengeServed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pow_challenge_served_total",
		Help: "Total number of PoW challenges served",
	})
	promChallengeCacheHit = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pow_challenge_cache_hit_total",
		Help: "Total number of PoW challenge cache hits",
	})
	promChallengeCacheMiss = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pow_challenge_cache_miss_total",
		Help: "Total number of PoW challenge cache misses (on-demand)",
	})
	promPoWValidation = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pow_validation_total",
		Help: "Total number of PoW validations performed",
	})
	promPoWValidationFail = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "pow_validation_fail_total",
		Help: "Total number of failed PoW validations",
	})
	promChallengeGenLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "pow_challenge_generation_seconds",
		Help:    "Challenge generation latency (seconds)",
		Buckets: prometheus.DefBuckets,
	})
	promPoWValidationLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "pow_validation_seconds",
		Help:    "PoW validation latency (seconds)",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(promChallengeServed)
	prometheus.MustRegister(promChallengeCacheHit)
	prometheus.MustRegister(promChallengeCacheMiss)
	prometheus.MustRegister(promPoWValidation)
	prometheus.MustRegister(promPoWValidationFail)
	prometheus.MustRegister(promChallengeGenLatency)
	prometheus.MustRegister(promPoWValidationLatency)
}

type Challenge struct {
	Difficulty byte   `json:"difficulty"`
	Salt       []byte `json:"salt"`
	Signature  []byte `json:"signature"`
}

func prewarmChallengeCache(ctx context.Context, conf *Config, secret []byte) {
	cacheOnce.Do(func() {
		challengeCache = make(chan *Challenge, challengeCacheSize)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if len(challengeCache) < challengeCacheSize {
						salt := make([]byte, conf.ProofTokenSize)
						rand.Read(salt)
						sig := pow.SignChallenge(secret, append([]byte{conf.Difficulty}, salt...))
						challenge := &Challenge{
							Difficulty: conf.Difficulty,
							Salt:       salt,
							Signature:  sig,
						}
						challengeCache <- challenge
					} else {
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}()
	})
}

type Server struct {
	conf *Config
	log  zerolog.Logger

	sock net.Listener

	handler func(net.Conn, zerolog.Logger)

	powReceive pow.Receiver
}

func StartServer(ctx context.Context, conf *Config, log zerolog.Logger, handler func(net.Conn, zerolog.Logger)) (*Server, error) {
	socket, err := net.Listen("tcp", conf.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen failed: %w", err)
	}
	s := &Server{
		conf:       conf,
		log:        log,
		sock:       socket,
		handler:    handler,
		powReceive: NewStatelessReceiver(powSecret),
	}
	go s.listen(ctx)
	return s, nil
}

func StartHTTPChallengeServer(ctx context.Context, conf *Config, log zerolog.Logger, tcpListener net.Listener) *http.Server {
	prewarmChallengeCache(ctx, conf, powSecret)
	mux := http.NewServeMux()
	mux.HandleFunc("/challenge", func(w http.ResponseWriter, r *http.Request) {
		var challenge *Challenge
		var genStart = time.Now()
		if UseChallengeCache {
			select {
			case challenge = <-challengeCache:
				promChallengeCacheHit.Inc()
				// got a prewarmed challenge
			default:
				promChallengeCacheMiss.Inc()
				// cache empty, generate on-demand
				salt := make([]byte, conf.ProofTokenSize)
				rand.Read(salt)
				sig := pow.SignChallenge(powSecret, append([]byte{conf.Difficulty}, salt...))
				challenge = &Challenge{conf.Difficulty, salt, sig}
			}
		} else {
			salt := make([]byte, conf.ProofTokenSize)
			rand.Read(salt)
			sig := pow.SignChallenge(powSecret, append([]byte{conf.Difficulty}, salt...))
			challenge = &Challenge{conf.Difficulty, salt, sig}
		}
		promChallengeServed.Inc()
		promChallengeGenLatency.Observe(time.Since(genStart).Seconds())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(challenge)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		addr := conf.ListenAddr
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("TCP listener not available"))
			return
		}
		conn.Close()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/metrics", promhttp.Handler().ServeHTTP)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	go func() {
		log.Info().Msg("HTTP PoW challenge endpoint and healthz listening on :8080")
		httpServer.ListenAndServe()
	}()
	return httpServer
}

// StartHTTP3ChallengeServer starts an HTTP/3 server on :8443 using the same mux as HTTP/1.1
func StartHTTP3ChallengeServer(ctx context.Context, conf *Config, log zerolog.Logger) *http3.Server {
	prewarmChallengeCache(ctx, conf, powSecret)
	mux := http.NewServeMux()
	mux.HandleFunc("/challenge", func(w http.ResponseWriter, r *http.Request) {
		var challenge *Challenge
		var genStart = time.Now()
		if UseChallengeCache {
			select {
			case challenge = <-challengeCache:
				promChallengeCacheHit.Inc()
				// got a prewarmed challenge
			default:
				promChallengeCacheMiss.Inc()
				// cache empty, generate on-demand
				salt := make([]byte, conf.ProofTokenSize)
				rand.Read(salt)
				sig := pow.SignChallenge(powSecret, append([]byte{conf.Difficulty}, salt...))
				challenge = &Challenge{conf.Difficulty, salt, sig}
			}
		} else {
			salt := make([]byte, conf.ProofTokenSize)
			rand.Read(salt)
			sig := pow.SignChallenge(powSecret, append([]byte{conf.Difficulty}, salt...))
			challenge = &Challenge{conf.Difficulty, salt, sig}
		}
		promChallengeServed.Inc()
		promChallengeGenLatency.Observe(time.Since(genStart).Seconds())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(challenge)
	})

	certFile := "deploy/cert.pem"
	keyFile := "deploy/key.pem"

	h3Server := &http3.Server{
		Addr:      ":8443",
		Handler:   mux,
		TLSConfig: nil, // use default, loads from certFile/keyFile
	}
	go func() {
		log.Info().Msg("HTTP/3 PoW challenge endpoint listening on :8443 (QUIC)")
		err := h3Server.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			log.Error().Err(err).Msg("HTTP/3 server error")
		}
	}()
	return h3Server
}

func (s *Server) Close() error {
	return s.sock.Close()
}

func (s *Server) GetListener() net.Listener {
	return s.sock
}

func (s *Server) listen(ctx context.Context) {
	var maxConnections = 100
	var sem = make(chan struct{}, maxConnections)
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("TCP server listener exiting due to context cancellation")
			return
		default:
			conn, err := s.sock.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				s.log.Warn().Err(fmt.Errorf("accept failed: %w", err)).Msg("failed to listen socket")
				continue
			}
			sem <- struct{}{} // acquire slot
			go func(conn net.Conn, connID int) {
				defer func() { <-sem }() // release slot
				s.serveConn(conn, connID)
			}(conn, i)
		}
	}
}

func (s *Server) serveConn(conn net.Conn, connID int) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second)) // 3s for handshake/PoW

	log := s.log.With().
		Int("id", connID).
		Str("addr", conn.RemoteAddr().String()).
		Logger()
	log.Trace().Msg("receive conn")

	checkDuration, err := s.powReceive(conn)
	if err != nil {
		log.Warn().Err(fmt.Errorf("PoW validation failed: %w", err)).Dur("check_duration", checkDuration).Msg("refuse conn")
		return
	}
	log.Debug().
		Int("difficulty", int(s.conf.Difficulty)).
		Dur("check_duration", checkDuration).
		Msg("is valid proof")

	s.handler(conn, log)
}

func NewStatelessReceiver(secret []byte) pow.Receiver {
	return func(conn net.Conn) (checkDuration time.Duration, err error) {
		var payload struct {
			Difficulty byte   `json:"difficulty"`
			Salt       []byte `json:"salt"`
			Signature  []byte `json:"signature"`
			Nonce      []byte `json:"nonce"`
		}
		dec := json.NewDecoder(conn)
		if err := dec.Decode(&payload); err != nil {
			promPoWValidationFail.Inc()
			return 0, errors.New("invalid challenge payload")
		}
		// Validate signature
		challengeData := append([]byte{payload.Difficulty}, payload.Salt...)
		promPoWValidation.Inc()
		var powStart = time.Now()
		if !pow.VerifyChallengeSignature(secret, challengeData, payload.Signature) {
			promPoWValidationFail.Inc()
			promPoWValidationLatency.Observe(time.Since(powStart).Seconds())
			return 0, errors.New("invalid challenge signature")
		}
		// Validate PoW
		beginCheck := time.Now()
		isValid := pow.CheckProof(payload.Difficulty, payload.Salt, payload.Nonce)
		checkDuration = time.Since(beginCheck)
		promPoWValidationLatency.Observe(time.Since(powStart).Seconds())
		if !isValid {
			promPoWValidationFail.Inc()
			return checkDuration, errors.New("invalid PoW solution")
		}
		return checkDuration, nil
	}
}

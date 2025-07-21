package main

import (
	"context"
	_ "embed"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/avpetkun/pow-wow/internal/server"
)

//go:embed quotes.json
var jsonQuotes []byte

func main() {
	zerolog.DurationFieldUnit = time.Millisecond

	conf := server.NewConfig()

	log := zerolog.New(&zerolog.ConsoleWriter{Out: os.Stdout}).
		Level(zerolog.TraceLevel).
		With().Timestamp().
		Logger()

	log.Debug().
		Str("listen_addr", conf.ListenAddr).
		Int("proof_token_size", conf.ProofTokenSize).
		Int("difficulty", int(conf.Difficulty)).
		Msg("server started")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	book, err := server.NewBook(jsonQuotes)
	check(log, err, "book init")

	tcpServer, err := server.StartServer(ctx, conf, log, book.ServeRequest)
	check(log, err, "tcp server start")
	defer tcpServer.Close()

	httpServer := server.StartHTTPChallengeServer(ctx, conf, log, tcpServer.GetListener())
	http3Server := server.StartHTTP3ChallengeServer(ctx, conf, log)

	waitForExit(ctx, log, httpServer, http3Server, tcpServer)
}

func waitForExit(ctx context.Context, log zerolog.Logger, httpServer *http.Server, http3Server interface{ Close() error }, tcpServer *server.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Info().Msg("server shutting down")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}
	if err := http3Server.Close(); err != nil {
		log.Error().Err(err).Msg("HTTP/3 server shutdown error")
	}
	if err := tcpServer.Close(); err != nil {
		log.Error().Err(err).Msg("TCP server shutdown error")
	}
}

func check(log zerolog.Logger, err error, msg string) {
	if err != nil {
		log.Fatal().Err(err).CallerSkipFrame(1).Msgf("start failed: %s", msg)
		panic(err)
	}
}

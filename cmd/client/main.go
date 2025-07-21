package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/avpetkun/pow-wow/internal/client"
)

func main() {
	zerolog.DurationFieldUnit = time.Millisecond

	ctx, ctxCancel := context.WithCancel(context.TODO())
	defer ctxCancel()

	conf := client.NewConfig()

	log := zerolog.New(&zerolog.ConsoleWriter{Out: os.Stdout}).
		Level(zerolog.TraceLevel).
		With().Timestamp().
		Logger()

	log.Debug().
		Str("server_addr", conf.ServerAddr).
		Int("fetch_workers", conf.FetchWorkers).
		Dur("timeout", conf.Timeout).
		Msg("server client")

	client.StartFetchWorkers(ctx, conf, log)

	waitForExit(log)
}

func waitForExit(log zerolog.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Info().Msg("client shutting down")
}

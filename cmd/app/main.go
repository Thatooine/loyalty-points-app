package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// configure global logger with colorful console output
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
	).With().Timestamp().Caller().Logger()

	log.Info().Msg("starting app")

	ctx := context.Background()

	config, secureConfig := GetConfig("")

	deps, err := NewDependencies(ctx, config, secureConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create dependencies")
	}

	// setup the server communications here
	server := setupRPCServer(*deps)

	// shut down signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	log.Info().Msg("shutting down app")

	// Drain in-flight requests before tearing down the DB pool they depend on:
	// stop accepting new connections, let outstanding ledger writes finish, then
	// close the pool. The timeout bounds how long we wait for stragglers.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("failed to gracefully shut down server")
	}

	if err := deps.Close(); err != nil {
		log.Error().Err(err).Msg("failed to close dependencies")
	}
}

// Package natsconn provides a standard NATS connection factory for Relay services.
// Every service that publishes or subscribes to events calls Connect() once at
// startup and passes the returned *nats.Conn to its event bus.
package natsconn

import (
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

// Connect establishes a NATS connection using the NATS_URL environment variable.
// It retries with exponential back-off for up to 30 seconds so services can
// start before NATS is fully up (common in docker-compose).
//
// Usage in any service main.go:
//
//	nc, err := natsconn.Connect()
//	if err != nil { log.Fatal().Err(err).Msg("nats") }
//	defer nc.Drain()
func Connect() (*nats.Conn, error) {
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL // nats://localhost:4222
	}

	nc, err := nats.Connect(url,
		nats.Name("relay-service"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Error().Err(err).Msg("nats disconnected")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info().Str("url", nc.ConnectedUrl()).Msg("nats reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	log.Info().Str("url", nc.ConnectedUrl()).Msg("nats connected")
	return nc, nil
}

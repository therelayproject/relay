module github.com/relay-im/relay/services/auth-service

go 1.22

require (
	github.com/relay-im/relay/shared v0.0.0
	golang.org/x/crypto v0.22.0
	golang.org/x/oauth2 v0.19.0
	github.com/nats-io/nats.go v1.34.1
	github.com/pquerna/otp v1.4.0
)

require (
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgx/v5 v5.5.5 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/redis/go-redis/v9 v9.5.1 // indirect
	github.com/rs/zerolog v1.32.0 // indirect
	github.com/spf13/viper v1.18.2 // indirect
	golang.org/x/sys v0.19.0 // indirect
	google.golang.org/grpc v1.63.2 // indirect
	google.golang.org/protobuf v1.34.0 // indirect
)

replace github.com/relay-im/relay/shared => ../../shared

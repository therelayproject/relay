module github.com/relay-im/relay/services/user-service

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.5
	github.com/minio/minio-go/v7 v7.0.70
	github.com/nats-io/nats.go v1.34.1
	github.com/redis/go-redis/v9 v9.5.1
	github.com/relay-im/relay/shared v0.0.0
	github.com/rs/zerolog v1.32.0
	github.com/spf13/viper v1.18.2
	google.golang.org/grpc v1.63.2
	google.golang.org/protobuf v1.34.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20231201235250-de7065d787b7 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240415180920-8c6c420018be // indirect
)

replace github.com/relay-im/relay/shared => ../../shared

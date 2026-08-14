# rh-portfolio-go

Go portfolio read service migrated from the `rh-trading` Django estate.

## Run

The service reads `portfolio_lot` from PostgreSQL:

```sh
export DATABASE_URL='postgres://rh:rh@localhost:5432/rh_trading?sslmode=disable'
go run ./cmd/server
```

HTTP listens on `PORT` (default `8081`) and gRPC listens on `GRPC_PORT`
(default `9090`).

## Test and build

```sh
make test
make build
```

The exact-decimal money implementation uses `math/big.Int` coefficients and
decimal scales. It performs half-even rounding with integer arithmetic and
preserves negative zero for rounded P&L.

## Protobuf and deployment

Regenerate the checked-in gRPC bindings with `make proto`. Render the
Dockerfile and paved-road deployment manifest with:

```sh
make render
```

The render script accepts `PAVED_ROAD_ROOT`, `SERVICE_NAME`, and `PORT`
overrides. The generated deployment keeps `{{VERSION}}` for CI image
substitution.

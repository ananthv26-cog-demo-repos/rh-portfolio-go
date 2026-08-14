.PHONY: build test proto render

build:
	go build ./...

test:
	go test ./...

proto:
	mkdir -p gen
	protoc -I ../rh-proto/proto \
		--go_out=paths=source_relative:gen \
		--go-grpc_out=paths=source_relative:gen \
		../rh-proto/proto/portfolio/v1/portfolio.proto

render:
	./scripts/render-deploy.sh

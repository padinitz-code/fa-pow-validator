.PHONY: vet test run_server run_client build_server build_client build_all

vet:
	go vet ./...

test:
	go test ./...

run_server:
	cd ./cmd/server && \
	go run .

run_client:
	cd ./cmd/client && \
	go run .

build_server: vet
	go build -o server ./cmd/server

build_client: vet
	go build -o client ./cmd/client

build_all: build_server build_client

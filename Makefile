.PHONY: build run-api run-prober lint fmt test clean

build:
	go build -o bin/beacon-api ./cmd/beacon-api
	go build -o bin/beacon-prober ./cmd/beacon-prober

run-api:
	go run ./cmd/beacon-api

run-prober:
	go run ./cmd/beacon-prober

lint:
	golangci-lint run

fmt:
	go fmt ./...

test:
	go test ./...

clean:
	rm -rf bin/
	go clean -cache

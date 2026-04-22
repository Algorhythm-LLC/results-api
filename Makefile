.PHONY: test run build

test:
	go test ./...

run:
	go run ./cmd/api

build:
	go build -o bin/results-api ./cmd/api

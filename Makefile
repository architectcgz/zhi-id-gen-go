APP_NAME := id-generator-server

.PHONY: run build test fmt

run:
	go run ./cmd/id-generator-server

build:
	mkdir -p bin
	go build -o bin/$(APP_NAME) ./cmd/id-generator-server

test:
	go test ./...

fmt:
	go fmt ./...


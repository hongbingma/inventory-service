.PHONY: test build sqlc

test:
	go test ./...

build:
	go build ./cmd/inventory-service

sqlc:
	sqlc generate

.PHONY: test build api sqlc

test:
	go test ./...

build:
	go build ./cmd/inventory-service

sqlc:
	sqlc generate

api:
	protoc -I api -I $$(go env GOPATH)/pkg/mod/github.com/go-kratos/kratos/v2@v2.8.4/third_party --go_out=paths=source_relative:api --go-grpc_out=paths=source_relative:api --go-http_out=paths=source_relative:api api/inventory/v1/inventory.proto

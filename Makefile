.PHONY: build test cover vet run tidy docker

build:
	go build ./...

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

run:
	go run ./cmd/server

tidy:
	go mod tidy

docker:
	docker build -t meta-ads-manager:local .

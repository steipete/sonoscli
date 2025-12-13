.PHONY: fmt fmt-check lint test build ci

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)"

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/sonos ./cmd/sonos

lint:
	golangci-lint run ./...

ci: fmt-check test
	go vet ./...

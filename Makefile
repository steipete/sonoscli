.PHONY: fmt fmt-check lint test ci

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)"

test:
	go test ./...

lint:
	golangci-lint run ./...

ci: fmt-check test
	go vet ./...


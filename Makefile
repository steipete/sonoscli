.PHONY: fmt fmt-check lint test race coverage build ci docs-site

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)"

test:
	go test ./...

race:
	go test -race ./...

coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@total="$$(go tool cover -func=coverage.out | tail -n 1 | awk '{print $$3}' | sed 's/%//')"; \
	echo "total coverage: $${total}%"; \
	awk -v cov="$${total}" 'BEGIN { exit !(cov+0 >= 75.0) }' || { echo "coverage floor not met (min 75.0%)"; exit 1; }
	go test ./internal/streamproxy -coverprofile=streamproxy-coverage.out -covermode=atomic
	@total="$$(go tool cover -func=streamproxy-coverage.out | tail -n 1 | awk '{print $$3}' | sed 's/%//')"; \
	echo "stream proxy coverage: $${total}%"; \
	awk -v cov="$${total}" 'BEGIN { exit !(cov+0 >= 85.0) }' || { echo "stream proxy coverage floor not met (min 85.0%)"; exit 1; }

build:
	mkdir -p bin
	go build -o bin/sonos ./cmd/sonos

lint:
	golangci-lint run ./...

ci: fmt-check coverage lint race
	go vet ./...

docs-site:
	node scripts/build-docs-site.mjs

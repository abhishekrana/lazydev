BINARY := lazydev
GOFLAGS := -ldflags="-s -w"

.PHONY: build run clean tidy fmt lint check

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/lazydev/

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy

fmt:
	gofmt -s -w .
	goimports -w .
	npx prettier --write "**/*.md" 2>/dev/null || true

lint:
	golangci-lint run ./...

check: fmt lint build

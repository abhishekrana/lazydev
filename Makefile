BINARY := lazydk
GOFLAGS := -ldflags="-s -w"

.PHONY: build run clean tidy fmt lint check

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/lazydk/

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy

fmt:
	gofmt -s -w .
	goimports -w .

lint:
	golangci-lint run ./...

check: fmt lint build
